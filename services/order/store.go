package main

import (
	"sync"

	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
)

type Store struct {
	mu     sync.RWMutex
	orders map[string]*order.Order
}

func NewStore() *Store {
	return &Store{orders: make(map[string]*order.Order)}
}

func (s *Store) Create(o *order.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.GetOrderId()] = o
}

func (s *Store) Get(orderID string) (*order.Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[orderID]
	return o, ok
}

func (s *Store) SetStatus(orderID string, status order.OrderStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o, ok := s.orders[orderID]; ok {
		o.Status = status
	}
}
