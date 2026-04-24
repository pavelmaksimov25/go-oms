package idempotency

import "sync"

type Guard struct {
	mu        sync.Mutex
	processed map[string]struct{}
}

func NewGuard() *Guard {
	return &Guard{processed: make(map[string]struct{})}
}

// Run invokes handler if the (sagaID, eventType) pair has not been
// successfully processed before. On handler error the pair remains
// unmarked so a subsequent redelivery will retry. Concurrent callers
// with the same key serialize; only the winner runs the handler.
func (g *Guard) Run(sagaID, eventType string, handler func() error) error {
	key := sagaID + "|" + eventType
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.processed[key]; ok {
		return nil
	}
	if err := handler(); err != nil {
		return err
	}
	g.processed[key] = struct{}{}
	return nil
}
