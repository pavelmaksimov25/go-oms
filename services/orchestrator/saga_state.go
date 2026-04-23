package main

import (
	"sync"

	saga "github.com/pavelmaksimov25/go-oms/pkg/proto/saga/v1"
)

type SagaState int

const (
	StateCreated SagaState = iota
	StateInventoryReserving
	StateInventoryReserved
	StatePaymentCharging
	StateCompleted
	StateCompensating
	StateFailed
)

func (s SagaState) String() string {
	switch s {
	case StateCreated:
		return "Created"
	case StateInventoryReserving:
		return "InventoryReserving"
	case StateInventoryReserved:
		return "InventoryReserved"
	case StatePaymentCharging:
		return "PaymentCharging"
	case StateCompleted:
		return "Completed"
	case StateCompensating:
		return "Compensating"
	case StateFailed:
		return "Failed"
	}
	return "Unknown"
}

type SagaData struct {
	OrderID     string
	Items       []*saga.OrderItem
	TotalAmount float64
	State       SagaState
}

type StateStore struct {
	mu    sync.RWMutex
	sagas map[string]*SagaData
}

func NewStateStore() *StateStore {
	return &StateStore{sagas: make(map[string]*SagaData)}
}

func (s *StateStore) Create(data *SagaData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sagas[data.OrderID] = data
}

func (s *StateStore) Get(orderID string) (*SagaData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.sagas[orderID]
	return d, ok
}

func (s *StateStore) Transition(orderID string, state SagaState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.sagas[orderID]; ok {
		d.State = state
	}
}
