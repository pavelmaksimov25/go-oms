package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
	"google.golang.org/protobuf/proto"
)

// PostgresSchema bootstraps the orders table. Runs on startup.
const PostgresSchema = `
CREATE TABLE IF NOT EXISTS orders (
    order_id     TEXT PRIMARY KEY,
    status       SMALLINT NOT NULL,
    total_amount DOUBLE PRECISION NOT NULL,
    items        BYTEA NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

type postgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) Store {
	return &postgresStore{pool: pool}
}

func (s *postgresStore) Create(ctx context.Context, o *order.Order) error {
	items, err := marshalItems(o.GetItems())
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO orders (order_id, status, total_amount, items) VALUES ($1, $2, $3, $4)`,
		o.GetOrderId(), int16(o.GetStatus()), o.GetTotalAmount(), items,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}
	return nil
}

func (s *postgresStore) Get(ctx context.Context, orderID string) (*order.Order, error) {
	var (
		status      int16
		totalAmount float64
		items       []byte
	)
	row := s.pool.QueryRow(ctx,
		`SELECT status, total_amount, items FROM orders WHERE order_id = $1`,
		orderID,
	)
	if err := row.Scan(&status, &totalAmount, &items); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("select order: %w", err)
	}
	parsed, err := unmarshalItems(items)
	if err != nil {
		return nil, err
	}
	return &order.Order{
		OrderId:     orderID,
		Items:       parsed,
		TotalAmount: totalAmount,
		Status:      order.OrderStatus(status),
	}, nil
}

func (s *postgresStore) SetStatus(ctx context.Context, orderID string, status order.OrderStatus) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE orders SET status = $1, updated_at = NOW() WHERE order_id = $2`,
		int16(status), orderID,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}

// items are stored as a protobuf-encoded Order (only the Items field is set)
// to keep the schema simple and avoid a second table. If they grow into a
// first-class concern they can be normalized later.
func marshalItems(items []*order.OrderItem) ([]byte, error) {
	return proto.Marshal(&order.Order{Items: items})
}

func unmarshalItems(raw []byte) ([]*order.OrderItem, error) {
	var msg order.Order
	if err := proto.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal items: %w", err)
	}
	return msg.GetItems(), nil
}
