package main

import (
	"sync"

	orderv1 "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
)

type Store struct {
	mu     sync.RWMutex
	orders map[string]*orderv1.Order
}

func NewStore() *Store {
	return &Store{orders: make(map[string]*orderv1.Order)}
}

func (s *Store) Create(order *orderv1.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[order.GetOrderId()] = order
}

func (s *Store) Get(orderID string) (*orderv1.Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[orderID]
	return o, ok
}

func (s *Store) SetStatus(orderID string, status orderv1.OrderStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o, ok := s.orders[orderID]; ok {
		o.Status = status
	}
}
