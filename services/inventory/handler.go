package main

import (
	"context"

	inventory "github.com/pavelmaksimov25/go-oms/pkg/proto/inventory/v1"
)

type Handler struct {
	inventory.UnimplementedInventoryServiceServer
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) CheckStock(_ context.Context, req *inventory.CheckStockRequest) (*inventory.CheckStockResponse, error) {
	return &inventory.CheckStockResponse{
		ItemId:    req.GetItemId(),
		Available: h.store.GetStock(req.GetItemId()),
	}, nil
}
