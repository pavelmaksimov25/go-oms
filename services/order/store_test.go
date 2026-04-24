package main

import (
	"context"
	"errors"
	"sync"
	"testing"

	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
)

func TestMemoryStore_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	o := &order.Order{OrderId: "abc", TotalAmount: 42, Status: order.OrderStatus_ORDER_STATUS_PENDING}

	if err := s.Create(ctx, o); err != nil {
		t.Fatalf("Create err = %v", err)
	}

	got, err := s.Get(ctx, "abc")
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if got.OrderId != "abc" || got.TotalAmount != 42 {
		t.Errorf("Get returned %+v, want order with id=abc amount=42", got)
	}
}

func TestMemoryStore_GetMissing_ErrNotFound(t *testing.T) {
	s := NewMemoryStore()

	got, err := s.Get(context.Background(), "missing")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestMemoryStore_SetStatus(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	_ = s.Create(ctx, &order.Order{OrderId: "abc", Status: order.OrderStatus_ORDER_STATUS_PENDING})

	if err := s.SetStatus(ctx, "abc", order.OrderStatus_ORDER_STATUS_CONFIRMED); err != nil {
		t.Fatalf("SetStatus err = %v", err)
	}

	got, _ := s.Get(ctx, "abc")
	if got.Status != order.OrderStatus_ORDER_STATUS_CONFIRMED {
		t.Errorf("Status = %v, want CONFIRMED", got.Status)
	}
}

func TestMemoryStore_SetStatusMissing_NoOp(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	if err := s.SetStatus(ctx, "missing", order.OrderStatus_ORDER_STATUS_CONFIRMED); err != nil {
		t.Fatalf("SetStatus err = %v", err)
	}

	if _, err := s.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetStatus on missing order should not create it; got err = %v", err)
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(id int) {
			defer wg.Done()
			orderID := string(rune('a' + (id % 26)))
			_ = s.Create(ctx, &order.Order{OrderId: orderID})
			_ = s.SetStatus(ctx, orderID, order.OrderStatus_ORDER_STATUS_CONFIRMED)
			_, _ = s.Get(ctx, orderID)
		}(i)
	}
	wg.Wait()
}
