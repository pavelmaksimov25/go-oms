package main

import (
	"context"
	"testing"

	inventory "github.com/pavelmaksimov25/go-oms/pkg/proto/inventory/v1"
)

func TestCheckStock_ReturnsAvailable(t *testing.T) {
	h := NewHandler(NewStore())

	resp, err := h.CheckStock(context.Background(), &inventory.CheckStockRequest{ItemId: "item-1"})
	if err != nil {
		t.Fatalf("CheckStock err = %v", err)
	}
	if resp.GetItemId() != "item-1" || resp.GetAvailable() != 100 {
		t.Errorf("got %+v, want item-1 / 100", resp)
	}
}

func TestCheckStock_UnknownItem_ReturnsZero(t *testing.T) {
	h := NewHandler(NewStore())

	resp, err := h.CheckStock(context.Background(), &inventory.CheckStockRequest{ItemId: "unknown"})
	if err != nil {
		t.Fatalf("CheckStock err = %v", err)
	}
	if resp.GetAvailable() != 0 {
		t.Errorf("Available = %d, want 0", resp.GetAvailable())
	}
}
