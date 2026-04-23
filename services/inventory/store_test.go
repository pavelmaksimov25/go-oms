package main

import (
	"sync"
	"testing"
)

func TestStore_SeedStock(t *testing.T) {
	s := NewStore()

	tests := map[string]int32{"item-1": 100, "item-2": 50, "item-3": 200}
	for item, want := range tests {
		if got := s.GetStock(item); got != want {
			t.Errorf("GetStock(%q) = %d, want %d", item, got, want)
		}
	}
}

func TestStore_GetStock_Unknown(t *testing.T) {
	s := NewStore()
	if got := s.GetStock("unknown"); got != 0 {
		t.Errorf("GetStock(unknown) = %d, want 0", got)
	}
}

func TestStore_Reserve_DecrementsStock(t *testing.T) {
	s := NewStore()
	if err := s.Reserve("item-1", 30); err != nil {
		t.Fatalf("Reserve err = %v", err)
	}
	if got := s.GetStock("item-1"); got != 70 {
		t.Errorf("stock after reserve = %d, want 70", got)
	}
}

func TestStore_Reserve_InsufficientStock(t *testing.T) {
	s := NewStore()
	if err := s.Reserve("item-2", 999); err == nil {
		t.Fatal("expected error for over-reserving, got nil")
	}
	if got := s.GetStock("item-2"); got != 50 {
		t.Errorf("stock mutated on failed reserve: got %d, want 50", got)
	}
}

func TestStore_Release_IncrementsStock(t *testing.T) {
	s := NewStore()
	_ = s.Reserve("item-1", 40)
	s.Release("item-1", 25)
	if got := s.GetStock("item-1"); got != 85 {
		t.Errorf("stock after release = %d, want 85", got)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := NewStore()
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if err := s.Reserve("item-3", 1); err == nil {
				s.Release("item-3", 1)
			}
			_ = s.GetStock("item-3")
		}()
	}
	wg.Wait()
}
