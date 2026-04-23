package main

import (
	"context"
	"sync"
	"testing"

	saga "github.com/pavelmaksimov25/go-oms/pkg/proto/saga/v1"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

type publishCall struct {
	topic     string
	eventType string
	sagaID    string
	value     []byte
}

type fakePublisher struct {
	mu    sync.Mutex
	calls []publishCall
}

func (f *fakePublisher) Publish(_ context.Context, topic string, _, value []byte) error {
	var env saga.SagaEvent
	if err := proto.Unmarshal(value, &env); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, publishCall{topic, env.GetEventType(), env.GetSagaId(), value})
	return nil
}

func (f *fakePublisher) last(t *testing.T) publishCall {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		t.Fatal("no Publish calls")
	}
	return f.calls[len(f.calls)-1]
}

func sagaMsg(t *testing.T, sagaID, eventType string, payload proto.Message) kafkago.Message {
	t.Helper()
	var payloadBytes []byte
	if payload != nil {
		b, err := proto.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		payloadBytes = b
	}
	env, err := proto.Marshal(&saga.SagaEvent{SagaId: sagaID, EventType: eventType, Payload: payloadBytes})
	if err != nil {
		t.Fatal(err)
	}
	return kafkago.Message{Value: env}
}

func newOrchestrator() (*Orchestrator, *fakePublisher) {
	pub := &fakePublisher{}
	return NewOrchestrator(NewStateStore(), pub), pub
}

func TestHandleOrderCreated_PublishesReserveAndTransitions(t *testing.T) {
	o, pub := newOrchestrator()

	items := []*saga.OrderItem{{ItemId: "item-1", Quantity: 2}}
	evt := &saga.OrderCreatedEvent{OrderId: "order-1", Items: items, TotalAmount: 50}
	msg := sagaMsg(t, "order-1", "order.created", evt)

	if err := o.HandleOrderCreated(context.Background(), msg); err != nil {
		t.Fatalf("HandleOrderCreated err = %v", err)
	}

	call := pub.last(t)
	if call.topic != "inventory.commands" || call.eventType != "inventory.reserve" {
		t.Errorf("call = %+v, want inventory.commands/inventory.reserve", call)
	}

	data, ok := o.store.Get("order-1")
	if !ok {
		t.Fatal("saga not stored")
	}
	if data.State != StateInventoryReserving {
		t.Errorf("State = %v, want InventoryReserving", data.State)
	}
	if data.TotalAmount != 50 {
		t.Errorf("TotalAmount = %v, want 50", data.TotalAmount)
	}
}

func TestHandleSagaEvent_InventoryReserved_ChargesPayment(t *testing.T) {
	o, pub := newOrchestrator()
	o.store.Create(&SagaData{OrderID: "order-1", TotalAmount: 500, State: StateInventoryReserving})

	msg := sagaMsg(t, "order-1", "inventory.reserved", &saga.InventoryReservedEvent{OrderId: "order-1"})
	if err := o.HandleSagaEvent(context.Background(), msg); err != nil {
		t.Fatalf("err = %v", err)
	}

	call := pub.last(t)
	if call.topic != "payment.commands" || call.eventType != "payment.charge" {
		t.Errorf("call = %+v, want payment.commands/payment.charge", call)
	}
	var env saga.SagaEvent
	_ = proto.Unmarshal(call.value, &env)
	var cmd saga.PaymentChargeCommand
	_ = proto.Unmarshal(env.GetPayload(), &cmd)
	if cmd.GetAmount() != 500 {
		t.Errorf("charge amount = %v, want 500", cmd.GetAmount())
	}

	data, _ := o.store.Get("order-1")
	if data.State != StatePaymentCharging {
		t.Errorf("State = %v, want PaymentCharging", data.State)
	}
}

func TestHandleSagaEvent_InventoryFailed_RejectsOrder(t *testing.T) {
	o, pub := newOrchestrator()
	o.store.Create(&SagaData{OrderID: "order-1", State: StateInventoryReserving})

	msg := sagaMsg(t, "order-1", "inventory.failed", &saga.InventoryFailedEvent{OrderId: "order-1", Reason: "oos"})
	if err := o.HandleSagaEvent(context.Background(), msg); err != nil {
		t.Fatalf("err = %v", err)
	}

	call := pub.last(t)
	if call.topic != "order.events" || call.eventType != "order.rejected" {
		t.Errorf("call = %+v, want order.events/order.rejected", call)
	}
	data, _ := o.store.Get("order-1")
	if data.State != StateFailed {
		t.Errorf("State = %v, want Failed", data.State)
	}
}

func TestHandleSagaEvent_PaymentCharged_ConfirmsOrder(t *testing.T) {
	o, pub := newOrchestrator()
	o.store.Create(&SagaData{OrderID: "order-1", State: StatePaymentCharging})

	msg := sagaMsg(t, "order-1", "payment.charged", &saga.PaymentChargedEvent{OrderId: "order-1", PaymentId: "pay-1"})
	if err := o.HandleSagaEvent(context.Background(), msg); err != nil {
		t.Fatalf("err = %v", err)
	}

	call := pub.last(t)
	if call.topic != "order.events" || call.eventType != "order.confirmed" {
		t.Errorf("call = %+v, want order.events/order.confirmed", call)
	}
	data, _ := o.store.Get("order-1")
	if data.State != StateCompleted {
		t.Errorf("State = %v, want Completed", data.State)
	}
}

func TestHandleSagaEvent_PaymentFailed_TriggersCompensation(t *testing.T) {
	o, pub := newOrchestrator()
	items := []*saga.OrderItem{{ItemId: "item-1", Quantity: 2}}
	o.store.Create(&SagaData{OrderID: "order-1", Items: items, State: StatePaymentCharging})

	msg := sagaMsg(t, "order-1", "payment.failed", &saga.PaymentFailedEvent{OrderId: "order-1", Reason: "limit"})
	if err := o.HandleSagaEvent(context.Background(), msg); err != nil {
		t.Fatalf("err = %v", err)
	}

	call := pub.last(t)
	if call.topic != "inventory.commands" || call.eventType != "inventory.release" {
		t.Errorf("call = %+v, want inventory.commands/inventory.release", call)
	}

	var env saga.SagaEvent
	_ = proto.Unmarshal(call.value, &env)
	var cmd saga.InventoryReleaseCommand
	_ = proto.Unmarshal(env.GetPayload(), &cmd)
	if len(cmd.GetItems()) != 1 || cmd.GetItems()[0].GetItemId() != "item-1" {
		t.Errorf("release items = %+v, want original items", cmd.GetItems())
	}

	data, _ := o.store.Get("order-1")
	if data.State != StateCompensating {
		t.Errorf("State = %v, want Compensating", data.State)
	}
}

func TestHandleSagaEvent_InventoryReleased_RejectsOrder(t *testing.T) {
	o, pub := newOrchestrator()
	o.store.Create(&SagaData{OrderID: "order-1", State: StateCompensating})

	msg := sagaMsg(t, "order-1", "inventory.released", &saga.InventoryReleasedEvent{OrderId: "order-1"})
	if err := o.HandleSagaEvent(context.Background(), msg); err != nil {
		t.Fatalf("err = %v", err)
	}

	call := pub.last(t)
	if call.topic != "order.events" || call.eventType != "order.rejected" {
		t.Errorf("call = %+v, want order.events/order.rejected", call)
	}
	data, _ := o.store.Get("order-1")
	if data.State != StateFailed {
		t.Errorf("State = %v, want Failed", data.State)
	}
}

func TestHandleSagaEvent_UnknownEvent_Ignored(t *testing.T) {
	o, pub := newOrchestrator()

	msg := sagaMsg(t, "order-1", "unrelated.event", nil)
	if err := o.HandleSagaEvent(context.Background(), msg); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if len(pub.calls) != 0 {
		t.Errorf("unexpected calls: %+v", pub.calls)
	}
}

func TestHandleOrderCreated_InvalidEnvelope_ReturnsError(t *testing.T) {
	o, _ := newOrchestrator()

	if err := o.HandleOrderCreated(context.Background(), kafkago.Message{Value: []byte("bad")}); err == nil {
		t.Error("expected error for invalid envelope, got nil")
	}
}

func TestHandleSagaEvent_InvalidEnvelope_ReturnsError(t *testing.T) {
	o, _ := newOrchestrator()

	if err := o.HandleSagaEvent(context.Background(), kafkago.Message{Value: []byte("bad")}); err == nil {
		t.Error("expected error for invalid envelope, got nil")
	}
}
