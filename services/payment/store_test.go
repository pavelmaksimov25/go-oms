package main

import (
	"testing"

	payment "github.com/pavelmaksimov25/go-oms/pkg/proto/payment/v1"
)

func TestStore_CreateAndGet(t *testing.T) {
	s := NewStore()
	p := &Payment{ID: "pay-1", OrderID: "order-1", Amount: 42, Status: payment.PaymentStatus_PAYMENT_STATUS_CHARGED}

	s.Create(p)

	got, ok := s.Get("pay-1")
	if !ok {
		t.Fatal("Get returned ok=false for existing payment")
	}
	if got.OrderID != "order-1" || got.Amount != 42 {
		t.Errorf("Get returned %+v, want order-1/42", got)
	}
}

func TestStore_GetMissing(t *testing.T) {
	s := NewStore()

	got, ok := s.Get("missing")

	if ok || got != nil {
		t.Errorf("Get(missing) = (%v, %v), want (nil, false)", got, ok)
	}
}
