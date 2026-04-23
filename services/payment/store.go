package main

import (
	"sync"

	payment "github.com/pavelmaksimov25/go-oms/pkg/proto/payment/v1"
)

type Payment struct {
	ID      string
	OrderID string
	Amount  float64
	Status  payment.PaymentStatus
}

type Store struct {
	mu       sync.RWMutex
	payments map[string]*Payment
}

func NewStore() *Store {
	return &Store{payments: make(map[string]*Payment)}
}

func (s *Store) Create(p *Payment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[p.ID] = p
}

func (s *Store) Get(id string) (*Payment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.payments[id]
	return p, ok
}
