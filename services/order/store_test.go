package main

import (
	"sync"
	"testing"

	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
)

func TestStore_CreateAndGet(t *testing.T) {
	s := NewStore()
	o := &order.Order{OrderId: "abc", TotalAmount: 42, Status: order.OrderStatus_ORDER_STATUS_PENDING}

	s.Create(o)

	got, ok := s.Get("abc")
	if !ok {
		t.Fatal("Get returned ok=false for existing order")
	}
	if got.OrderId != "abc" || got.TotalAmount != 42 {
		t.Errorf("Get returned %+v, want order with id=abc amount=42", got)
	}
}

func TestStore_GetMissing(t *testing.T) {
	s := NewStore()

	got, ok := s.Get("missing")

	if ok || got != nil {
		t.Errorf("Get(missing) = (%v, %v), want (nil, false)", got, ok)
	}
}

func TestStore_SetStatus(t *testing.T) {
	s := NewStore()
	s.Create(&order.Order{OrderId: "abc", Status: order.OrderStatus_ORDER_STATUS_PENDING})

	s.SetStatus("abc", order.OrderStatus_ORDER_STATUS_CONFIRMED)

	got, _ := s.Get("abc")
	if got.Status != order.OrderStatus_ORDER_STATUS_CONFIRMED {
		t.Errorf("Status = %v, want CONFIRMED", got.Status)
	}
}

func TestStore_SetStatusMissing_NoOp(t *testing.T) {
	s := NewStore()

	s.SetStatus("missing", order.OrderStatus_ORDER_STATUS_CONFIRMED)

	if _, ok := s.Get("missing"); ok {
		t.Error("SetStatus on missing order should not create it")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := NewStore()
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(id int) {
			defer wg.Done()
			orderID := string(rune('a' + (id % 26)))
			s.Create(&order.Order{OrderId: orderID})
			s.SetStatus(orderID, order.OrderStatus_ORDER_STATUS_CONFIRMED)
			_, _ = s.Get(orderID)
		}(i)
	}
	wg.Wait()
}
