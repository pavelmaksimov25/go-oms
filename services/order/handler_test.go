package main

import (
	"context"
	"errors"
	"sync"
	"testing"

	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
	saga "github.com/pavelmaksimov25/go-oms/pkg/proto/saga/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (f *fakePublisher) lastCall(t *testing.T) publishCall {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		t.Fatal("expected at least one Publish call")
	}
	return f.calls[len(f.calls)-1]
}

func TestCreateOrder_StoresAndPublishesSagaEvent(t *testing.T) {
	store := NewMemoryStore()
	pub := &fakePublisher{}
	h := NewHandler(store, pub)

	req := &order.CreateOrderRequest{
		Items:       []*order.OrderItem{{ItemId: "item-1", Quantity: 2}},
		TotalAmount: 100,
	}

	resp, err := h.CreateOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateOrder err = %v", err)
	}
	if resp.GetOrderId() == "" {
		t.Error("response OrderId is empty")
	}
	if resp.GetStatus() != order.OrderStatus_ORDER_STATUS_PENDING {
		t.Errorf("response Status = %v, want PENDING", resp.GetStatus())
	}

	stored, err := store.Get(context.Background(), resp.GetOrderId())
	if err != nil {
		t.Fatalf("order not persisted in store: %v", err)
	}
	if stored.GetStatus() != order.OrderStatus_ORDER_STATUS_PENDING {
		t.Errorf("stored Status = %v, want PENDING", stored.GetStatus())
	}

	call := pub.lastCall(t)
	if call.topic != "saga.order.created" {
		t.Errorf("topic = %q, want saga.order.created", call.topic)
	}
	if string(call.key) != resp.GetOrderId() {
		t.Errorf("key = %q, want %q", call.key, resp.GetOrderId())
	}

	var envelope saga.SagaEvent
	if err := proto.Unmarshal(call.value, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.GetSagaId() != resp.GetOrderId() {
		t.Errorf("SagaId = %q, want %q", envelope.GetSagaId(), resp.GetOrderId())
	}
	if envelope.GetEventType() != "order.created" {
		t.Errorf("EventType = %q, want order.created", envelope.GetEventType())
	}

	var evt saga.OrderCreatedEvent
	if err := proto.Unmarshal(envelope.GetPayload(), &evt); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if evt.GetOrderId() != resp.GetOrderId() {
		t.Errorf("event OrderId = %q, want %q", evt.GetOrderId(), resp.GetOrderId())
	}
	if evt.GetTotalAmount() != 100 {
		t.Errorf("event TotalAmount = %v, want 100", evt.GetTotalAmount())
	}
	if len(evt.GetItems()) != 1 || evt.GetItems()[0].GetItemId() != "item-1" || evt.GetItems()[0].GetQuantity() != 2 {
		t.Errorf("event Items = %+v, want [{item-1 2}]", evt.GetItems())
	}
}

func TestCreateOrder_PublishFailure_ReturnsError(t *testing.T) {
	store := NewMemoryStore()
	pub := &fakePublisher{err: errors.New("kafka down")}
	h := NewHandler(store, pub)

	_, err := h.CreateOrder(context.Background(), &order.CreateOrderRequest{TotalAmount: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("status code = %v, want Internal", status.Code(err))
	}
}

func TestGetOrder_Found(t *testing.T) {
	store := NewMemoryStore()
	_ = store.Create(context.Background(), &order.Order{OrderId: "abc", Status: order.OrderStatus_ORDER_STATUS_CONFIRMED, TotalAmount: 50})
	h := NewHandler(store, &fakePublisher{})

	resp, err := h.GetOrder(context.Background(), &order.GetOrderRequest{OrderId: "abc"})
	if err != nil {
		t.Fatalf("GetOrder err = %v", err)
	}
	if resp.GetOrder().GetOrderId() != "abc" {
		t.Errorf("OrderId = %q, want abc", resp.GetOrder().GetOrderId())
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	h := NewHandler(NewMemoryStore(), &fakePublisher{})

	_, err := h.GetOrder(context.Background(), &order.GetOrderRequest{OrderId: "missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("status code = %v, want NotFound", status.Code(err))
	}
}
