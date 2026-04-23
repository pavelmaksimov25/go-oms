package main

import (
	"fmt"
	"sync"
)

var seedStock = map[string]int32{
	"item-1": 100,
	"item-2": 50,
	"item-3": 200,
}

type Store struct {
	mu    sync.RWMutex
	stock map[string]int32
}

func NewStore() *Store {
	s := &Store{stock: make(map[string]int32, len(seedStock))}
	for k, v := range seedStock {
		s.stock[k] = v
	}
	return s
}

func (s *Store) Reserve(itemID string, qty int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stock[itemID] < qty {
		return fmt.Errorf("insufficient stock for %s: have %d, need %d", itemID, s.stock[itemID], qty)
	}
	s.stock[itemID] -= qty
	return nil
}

func (s *Store) Release(itemID string, qty int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stock[itemID] += qty
}

func (s *Store) GetStock(itemID string) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stock[itemID]
}
