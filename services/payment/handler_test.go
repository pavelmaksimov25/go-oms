package main

import (
	"context"
	"testing"

	payment "github.com/pavelmaksimov25/go-oms/pkg/proto/payment/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetPaymentStatus_Found(t *testing.T) {
	store := NewStore()
	store.Create(&Payment{ID: "pay-1", OrderID: "order-1", Amount: 100, Status: payment.PaymentStatus_PAYMENT_STATUS_CHARGED})
	h := NewHandler(store)

	resp, err := h.GetPaymentStatus(context.Background(), &payment.GetPaymentStatusRequest{PaymentId: "pay-1"})
	if err != nil {
		t.Fatalf("GetPaymentStatus err = %v", err)
	}
	if resp.GetPaymentId() != "pay-1" || resp.GetStatus() != payment.PaymentStatus_PAYMENT_STATUS_CHARGED {
		t.Errorf("got %+v, want pay-1/CHARGED", resp)
	}
}

func TestGetPaymentStatus_NotFound(t *testing.T) {
	h := NewHandler(NewStore())

	_, err := h.GetPaymentStatus(context.Background(), &payment.GetPaymentStatusRequest{PaymentId: "missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("status code = %v, want NotFound", status.Code(err))
	}
}
