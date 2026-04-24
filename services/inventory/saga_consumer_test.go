package main

import (
	"context"
	"errors"
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

func reserveCmdEnvelope(t *testing.T, orderID string, items []*saga.OrderItem) kafkago.Message {
	t.Helper()
	payload, err := proto.Marshal(&saga.InventoryReserveCommand{OrderId: orderID, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	env, err := proto.Marshal(&saga.SagaEvent{SagaId: orderID, EventType: "inventory.reserve", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	return kafkago.Message{Value: env}
}

func releaseCmdEnvelope(t *testing.T, orderID string, items []*saga.OrderItem) kafkago.Message {
	t.Helper()
	payload, err := proto.Marshal(&saga.InventoryReleaseCommand{OrderId: orderID, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	env, err := proto.Marshal(&saga.SagaEvent{SagaId: orderID, EventType: "inventory.release", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	return kafkago.Message{Value: env}
}

func assertReply(t *testing.T, call publishCall, wantEventType, wantSagaID string) {
	t.Helper()
	if call.topic != "saga.events" {
		t.Errorf("topic = %q, want saga.events", call.topic)
	}
	var env saga.SagaEvent
	if err := proto.Unmarshal(call.value, &env); err != nil {
		t.Fatalf("unmarshal reply envelope: %v", err)
	}
	if env.GetEventType() != wantEventType {
		t.Errorf("EventType = %q, want %q", env.GetEventType(), wantEventType)
	}
	if env.GetSagaId() != wantSagaID {
		t.Errorf("SagaId = %q, want %q", env.GetSagaId(), wantSagaID)
	}
}

func TestReserve_Success_PublishesReserved(t *testing.T) {
	store := NewStore()
	pub := &fakePublisher{}
	c := NewSagaConsumer(store, pub)

	items := []*saga.OrderItem{{ItemId: "item-1", Quantity: 5}, {ItemId: "item-2", Quantity: 10}}
	if err := c.Handle(context.Background(), reserveCmdEnvelope(t, "order-1", items)); err != nil {
		t.Fatalf("Handle err = %v", err)
	}

	assertReply(t, pub.last(t), "inventory.reserved", "order-1")
	if got := store.GetStock("item-1"); got != 95 {
		t.Errorf("item-1 stock = %d, want 95", got)
	}
	if got := store.GetStock("item-2"); got != 40 {
		t.Errorf("item-2 stock = %d, want 40", got)
	}
}

func TestReserve_PartialFailure_RollsBack_PublishesFailed(t *testing.T) {
	store := NewStore()
	pub := &fakePublisher{}
	c := NewSagaConsumer(store, pub)

	// item-2 only has 50 — second item requests 999 and triggers rollback
	items := []*saga.OrderItem{{ItemId: "item-1", Quantity: 10}, {ItemId: "item-2", Quantity: 999}}
	if err := c.Handle(context.Background(), reserveCmdEnvelope(t, "order-2", items)); err != nil {
		t.Fatalf("Handle err = %v", err)
	}

	assertReply(t, pub.last(t), "inventory.failed", "order-2")
	if got := store.GetStock("item-1"); got != 100 {
		t.Errorf("item-1 stock = %d after rollback, want 100 (original)", got)
	}
}

func TestRelease_PublishesReleasedAndRestoresStock(t *testing.T) {
	store := NewStore()
	_ = store.Reserve("item-1", 20)
	pub := &fakePublisher{}
	c := NewSagaConsumer(store, pub)

	items := []*saga.OrderItem{{ItemId: "item-1", Quantity: 20}}
	if err := c.Handle(context.Background(), releaseCmdEnvelope(t, "order-3", items)); err != nil {
		t.Fatalf("Handle err = %v", err)
	}

	assertReply(t, pub.last(t), "inventory.released", "order-3")
	if got := store.GetStock("item-1"); got != 100 {
		t.Errorf("item-1 stock after release = %d, want 100", got)
	}
}

func TestUnknownEvent_NoError_NoPublish(t *testing.T) {
	pub := &fakePublisher{}
	c := NewSagaConsumer(NewStore(), pub)

	env, _ := proto.Marshal(&saga.SagaEvent{SagaId: "x", EventType: "irrelevant"})
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

func TestReserve_DuplicateDelivery_IsNoOp(t *testing.T) {
	store := NewStore()
	pub := &fakePublisher{}
	c := NewSagaConsumer(store, pub)

	items := []*saga.OrderItem{{ItemId: "item-1", Quantity: 5}}
	msg := reserveCmdEnvelope(t, "order-1", items)

	if err := c.Handle(context.Background(), msg); err != nil {
		t.Fatalf("first Handle err = %v", err)
	}
	if err := c.Handle(context.Background(), msg); err != nil {
		t.Fatalf("duplicate Handle err = %v", err)
	}

	if got := store.GetStock("item-1"); got != 95 {
		t.Errorf("stock = %d after duplicate delivery, want 95 (reserved once)", got)
	}
	if len(pub.calls) != 1 {
		t.Errorf("Publish called %d times, want 1", len(pub.calls))
	}
}

func TestReserve_PublishError_IsPropagated(t *testing.T) {
	pub := &fakePublisher{err: errors.New("kafka down")}
	c := NewSagaConsumer(NewStore(), pub)

	items := []*saga.OrderItem{{ItemId: "item-1", Quantity: 1}}
	if err := c.Handle(context.Background(), reserveCmdEnvelope(t, "order-x", items)); err == nil {
		t.Error("expected error when publish fails, got nil")
	}
}
