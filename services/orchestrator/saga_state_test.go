package main

import (
	"testing"
)

func TestStateStore_CreateAndGet(t *testing.T) {
	s := NewStateStore()
	data := &SagaData{OrderID: "abc", TotalAmount: 100, State: StateCreated}

	s.Create(data)

	got, ok := s.Get("abc")
	if !ok {
		t.Fatal("Get ok=false for existing saga")
	}
	if got.TotalAmount != 100 || got.State != StateCreated {
		t.Errorf("got %+v, want amount=100 state=Created", got)
	}
}

func TestStateStore_GetMissing(t *testing.T) {
	s := NewStateStore()
	got, ok := s.Get("missing")
	if ok || got != nil {
		t.Errorf("got (%v, %v), want (nil, false)", got, ok)
	}
}

func TestStateStore_Transition(t *testing.T) {
	s := NewStateStore()
	s.Create(&SagaData{OrderID: "abc", State: StateCreated})

	s.Transition("abc", StateInventoryReserving)

	got, _ := s.Get("abc")
	if got.State != StateInventoryReserving {
		t.Errorf("State = %v, want InventoryReserving", got.State)
	}
}

func TestStateStore_TransitionMissing_NoOp(t *testing.T) {
	s := NewStateStore()

	s.Transition("missing", StateCompleted)

	if _, ok := s.Get("missing"); ok {
		t.Error("Transition on missing saga should not create it")
	}
}
