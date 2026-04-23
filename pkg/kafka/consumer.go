package kafka

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

const retryBackoff = time.Second

type Handler func(ctx context.Context, msg kafka.Message) error

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   topic,
			GroupID: groupID,
		}),
	}
}

func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			log.Printf("kafka: fetch error on %s (will retry): %v", c.reader.Config().Topic, err)
			if sleep(ctx, retryBackoff) {
				return nil
			}
			continue
		}
		if err := handler(ctx, msg); err != nil {
			log.Printf("kafka: handler error on %s: %v", msg.Topic, err)
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			log.Printf("kafka: commit error on %s: %v", msg.Topic, err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

func sleep(ctx context.Context, d time.Duration) (canceled bool) {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(d):
		return false
	}
}
