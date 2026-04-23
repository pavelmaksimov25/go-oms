package main

import (
	"context"

	payment "github.com/pavelmaksimov25/go-oms/pkg/proto/payment/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	payment.UnimplementedPaymentServiceServer
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) GetPaymentStatus(_ context.Context, req *payment.GetPaymentStatusRequest) (*payment.GetPaymentStatusResponse, error) {
	p, ok := h.store.Get(req.GetPaymentId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "payment %s not found", req.GetPaymentId())
	}
	return &payment.GetPaymentStatusResponse{
		PaymentId: p.ID,
		Status:    p.Status,
	}, nil
}
