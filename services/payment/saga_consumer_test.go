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
	topic string
	key   []byte
	value []byte
}

type fakePublisher struct {
	mu    sync.Mutex
	calls []publishCall
	err   error
}

func (f *fakePublisher) Publish(_ context.Context, topic string, key, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, publishCall{topic, key, value})
	return f.err
}

func (f *fakePublisher) last(t *testing.T) publishCall {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		t.Fatal("expected at least one Publish call")
	}
	return f.calls[len(f.calls)-1]
}

func chargeCmdEnvelope(t *testing.T, orderID string, amount float64) kafkago.Message {
	t.Helper()
	payload, err := proto.Marshal(&saga.PaymentChargeCommand{OrderId: orderID, Amount: amount})
	if err != nil {
		t.Fatal(err)
	}
	env, err := proto.Marshal(&saga.SagaEvent{SagaId: orderID, EventType: "payment.charge", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	return kafkago.Message{Value: env}
}

func TestCharge_BelowThreshold_PublishesCharged(t *testing.T) {
	store := NewStore()
	pub := &fakePublisher{}
	c := NewSagaConsumer(store, pub)

	if err := c.Handle(context.Background(), chargeCmdEnvelope(t, "order-1", 500)); err != nil {
		t.Fatalf("Handle err = %v", err)
	}

	call := pub.last(t)
	if call.topic != "saga.events" {
		t.Errorf("topic = %q, want saga.events", call.topic)
	}
	var env saga.SagaEvent
	if err := proto.Unmarshal(call.value, &env); err != nil {
		t.Fatal(err)
	}
	if env.GetEventType() != "payment.charged" {
		t.Errorf("EventType = %q, want payment.charged", env.GetEventType())
	}

	var evt saga.PaymentChargedEvent
	if err := proto.Unmarshal(env.GetPayload(), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.GetOrderId() != "order-1" {
		t.Errorf("OrderId = %q, want order-1", evt.GetOrderId())
	}
	if evt.GetPaymentId() == "" {
		t.Error("PaymentId is empty, want a generated id")
	}

	stored, ok := store.Get(evt.GetPaymentId())
	if !ok {
		t.Fatal("payment record was not persisted")
	}
	if stored.Amount != 500 || stored.Status != 1 /* CHARGED */ {
		t.Errorf("stored = %+v, want amount=500 CHARGED", stored)
	}
}

func TestCharge_AboveThreshold_PublishesFailed_NoRecord(t *testing.T) {
	store := NewStore()
	pub := &fakePublisher{}
	c := NewSagaConsumer(store, pub)

	if err := c.Handle(context.Background(), chargeCmdEnvelope(t, "order-big", 5000)); err != nil {
		t.Fatalf("Handle err = %v", err)
	}

	var env saga.SagaEvent
	if err := proto.Unmarshal(pub.last(t).value, &env); err != nil {
		t.Fatal(err)
	}
	if env.GetEventType() != "payment.failed" {
		t.Errorf("EventType = %q, want payment.failed", env.GetEventType())
	}
	var evt saga.PaymentFailedEvent
	if err := proto.Unmarshal(env.GetPayload(), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.GetOrderId() != "order-big" {
		t.Errorf("OrderId = %q, want order-big", evt.GetOrderId())
	}
}

func TestChargeAtThreshold_1000_Charges(t *testing.T) {
	pub := &fakePublisher{}
	c := NewSagaConsumer(NewStore(), pub)

	if err := c.Handle(context.Background(), chargeCmdEnvelope(t, "order-edge", 1000)); err != nil {
		t.Fatalf("Handle err = %v", err)
	}

	var env saga.SagaEvent
	_ = proto.Unmarshal(pub.last(t).value, &env)
	if env.GetEventType() != "payment.charged" {
		t.Errorf("amount=1000 should charge (not failed), got %q", env.GetEventType())
	}
}

func TestCharge_DuplicateDelivery_ChargesOnce(t *testing.T) {
	store := NewStore()
	pub := &fakePublisher{}
	c := NewSagaConsumer(store, pub)
	msg := chargeCmdEnvelope(t, "order-dup", 250)

	if err := c.Handle(context.Background(), msg); err != nil {
		t.Fatalf("first Handle err = %v", err)
	}
	if err := c.Handle(context.Background(), msg); err != nil {
		t.Fatalf("duplicate Handle err = %v", err)
	}

	if len(pub.calls) != 1 {
		t.Errorf("Publish called %d times, want 1 (duplicate should be skipped)", len(pub.calls))
	}
	count := 0
	store.mu.RLock()
	count = len(store.payments)
	store.mu.RUnlock()
	if count != 1 {
		t.Errorf("payment records = %d, want 1", count)
	}
}

func TestUnknownEvent_NoPublish(t *testing.T) {
	pub := &fakePublisher{}
	c := NewSagaConsumer(NewStore(), pub)

	env, _ := proto.Marshal(&saga.SagaEvent{SagaId: "x", EventType: "unrelated"})
	if err := c.Handle(context.Background(), kafkago.Message{Value: env}); err != nil {
		t.Errorf("Handle err = %v, want nil", err)
	}
	if len(pub.calls) != 0 {
		t.Errorf("unexpected publish calls: %+v", pub.calls)
	}
}

func TestInvalidEnvelope_ReturnsError(t *testing.T) {
	c := NewSagaConsumer(NewStore(), &fakePublisher{})

	if err := c.Handle(context.Background(), kafkago.Message{Value: []byte("garbage")}); err == nil {
		t.Error("expected error for invalid envelope, got nil")
	}
}
