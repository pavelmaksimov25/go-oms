package main

import (
	"context"
	"sync"

	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
)

type memoryStore struct {
	mu     sync.RWMutex
	orders map[string]*order.Order
}

func NewMemoryStore() Store {
	return &memoryStore{orders: make(map[string]*order.Order)}
}

func (s *memoryStore) Create(_ context.Context, o *order.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.GetOrderId()] = o
	return nil
}

func (s *memoryStore) Get(_ context.Context, orderID string) (*order.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[orderID]
	if !ok {
		return nil, ErrNotFound
	}
	return o, nil
}

func (s *memoryStore) SetStatus(_ context.Context, orderID string, status order.OrderStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o, ok := s.orders[orderID]; ok {
		o.Status = status
	}
	return nil
}
