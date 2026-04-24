package main

import (
	"context"
	"errors"

	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
)

// ErrNotFound is returned by Store.Get when the order does not exist.
var ErrNotFound = errors.New("order not found")

// Store is the persistence contract for orders. Two implementations exist:
// an in-memory map (used in tests) and a Postgres-backed store (used when
// DATABASE_URL is set).
type Store interface {
	Create(ctx context.Context, o *order.Order) error
	Get(ctx context.Context, orderID string) (*order.Order, error)
	SetStatus(ctx context.Context, orderID string, status order.OrderStatus) error
}
