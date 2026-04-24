package main

import (
	"context"
	"testing"

	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
	saga "github.com/pavelmaksimov25/go-oms/pkg/proto/saga/v1"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

func envelope(t *testing.T, sagaID, eventType string) []byte {
	t.Helper()
	b, err := proto.Marshal(&saga.SagaEvent{SagaId: sagaID, EventType: eventType})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return b
}

func TestSagaConsumer_OrderConfirmed_SetsConfirmed(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	_ = store.Create(ctx, &order.Order{OrderId: "abc", Status: order.OrderStatus_ORDER_STATUS_PENDING})
	c := NewSagaConsumer(store)

	err := c.Handle(ctx, kafkago.Message{Value: envelope(t, "abc", "order.confirmed")})
	if err != nil {
		t.Fatalf("Handle err = %v", err)
	}

	got, _ := store.Get(ctx, "abc")
	if got.GetStatus() != order.OrderStatus_ORDER_STATUS_CONFIRMED {
		t.Errorf("Status = %v, want CONFIRMED", got.GetStatus())
	}
}

func TestSagaConsumer_OrderRejected_SetsRejected(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	_ = store.Create(ctx, &order.Order{OrderId: "abc", Status: order.OrderStatus_ORDER_STATUS_PENDING})
	c := NewSagaConsumer(store)

	err := c.Handle(ctx, kafkago.Message{Value: envelope(t, "abc", "order.rejected")})
	if err != nil {
		t.Fatalf("Handle err = %v", err)
	}

	got, _ := store.Get(ctx, "abc")
	if got.GetStatus() != order.OrderStatus_ORDER_STATUS_REJECTED {
		t.Errorf("Status = %v, want REJECTED", got.GetStatus())
	}
}

func TestSagaConsumer_UnknownEvent_NoError_NoChange(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	_ = store.Create(ctx, &order.Order{OrderId: "abc", Status: order.OrderStatus_ORDER_STATUS_PENDING})
	c := NewSagaConsumer(store)

	err := c.Handle(ctx, kafkago.Message{Value: envelope(t, "abc", "something.else")})
	if err != nil {
		t.Errorf("Handle err = %v, want nil for unknown event", err)
	}

	got, _ := store.Get(ctx, "abc")
	if got.GetStatus() != order.OrderStatus_ORDER_STATUS_PENDING {
		t.Errorf("Status = %v, want unchanged PENDING", got.GetStatus())
	}
}

func TestSagaConsumer_InvalidEnvelope_ReturnsError(t *testing.T) {
	c := NewSagaConsumer(NewMemoryStore())

	err := c.Handle(context.Background(), kafkago.Message{Value: []byte("not-a-protobuf")})
	if err == nil {
		t.Error("expected error for invalid envelope, got nil")
	}
}
