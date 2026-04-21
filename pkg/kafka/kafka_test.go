package kafka

import (
	"context"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

func TestNewProducer(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"})
	if p == nil || p.writer == nil {
		t.Fatal("NewProducer returned nil")
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close() err = %v", err)
	}
}

func TestProducer_Publish_CanceledContext(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"})
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := p.Publish(ctx, "any-topic", []byte("k"), []byte("v")); err == nil {
		t.Error("expected error with canceled context, got nil")
	}
}

func TestNewConsumer(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}, "topic", "group")
	if c == nil || c.reader == nil {
		t.Fatal("NewConsumer returned nil")
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close() err = %v", err)
	}
}

func TestConsumer_Consume_CanceledContext(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}, "topic", "group")
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	handlerCalled := false
	err := c.Consume(ctx, func(_ context.Context, _ kafkago.Message) error {
		handlerCalled = true
		return nil
	})
	if err != nil {
		t.Errorf("Consume on canceled ctx returned err = %v, want nil", err)
	}
	if handlerCalled {
		t.Error("handler should not have been called without messages")
	}
}
