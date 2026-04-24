package idempotency

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestGuard_Run_FirstCall_InvokesHandler(t *testing.T) {
	g := NewGuard()
	called := 0

	err := g.Run("order-1", "order.created", func() error {
		called++
		return nil
	})

	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if called != 1 {
		t.Errorf("handler called %d times, want 1", called)
	}
}

func TestGuard_Run_Duplicate_SkipsHandler(t *testing.T) {
	g := NewGuard()
	called := 0
	handler := func() error {
		called++
		return nil
	}

	_ = g.Run("order-1", "order.created", handler)
	_ = g.Run("order-1", "order.created", handler)

	if called != 1 {
		t.Errorf("handler called %d times, want 1 (duplicate should be skipped)", called)
	}
}

func TestGuard_Run_DifferentKeys_BothInvoke(t *testing.T) {
	g := NewGuard()
	called := 0
	handler := func() error {
		called++
		return nil
	}

	_ = g.Run("order-1", "order.created", handler)
	_ = g.Run("order-2", "order.created", handler)
	_ = g.Run("order-1", "order.confirmed", handler)

	if called != 3 {
		t.Errorf("handler called %d times, want 3 (distinct keys)", called)
	}
}

func TestGuard_Run_HandlerError_DoesNotMarkProcessed(t *testing.T) {
	g := NewGuard()
	called := 0
	handler := func() error {
		called++
		return errors.New("boom")
	}

	_ = g.Run("order-1", "order.created", handler)
	_ = g.Run("order-1", "order.created", handler)

	if called != 2 {
		t.Errorf("handler called %d times, want 2 (failed run should not be marked)", called)
	}
}

func TestGuard_Run_HandlerError_PropagatesError(t *testing.T) {
	g := NewGuard()
	want := errors.New("boom")

	got := g.Run("order-1", "order.created", func() error { return want })

	if !errors.Is(got, want) {
		t.Errorf("Run err = %v, want %v", got, want)
	}
}

func TestGuard_Run_Concurrent_HandlerRunsOnce(t *testing.T) {
	g := NewGuard()
	var called int32
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_ = g.Run("order-1", "order.created", func() error {
				atomic.AddInt32(&called, 1)
				return nil
			})
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&called); got != 1 {
		t.Errorf("handler called %d times concurrently, want 1", got)
	}
}
