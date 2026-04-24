package kafka

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	topic := c.reader.Config().Topic
	groupID := c.reader.Config().GroupID
	tracer := otel.Tracer("kafka")
	for {
		if ctx.Err() != nil {
			return nil
		}
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			slog.Warn("kafka fetch failed, retrying", "topic", topic, "error", err)
			if sleep(ctx, retryBackoff) {
				return nil
			}
			continue
		}

		if err := c.handleWithTrace(ctx, tracer, groupID, msg, handler); err != nil {
			slog.Error("kafka handler failed", "topic", msg.Topic, "error", err)
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			slog.Error("kafka commit failed", "topic", msg.Topic, "error", err)
		}
	}
}

func (c *Consumer) handleWithTrace(ctx context.Context, tracer trace.Tracer, groupID string, msg kafka.Message, handler Handler) error {
	carrier := headerCarrier(msg.Headers)
	ctx = otel.GetTextMapPropagator().Extract(ctx, &carrier)
	ctx, span := tracer.Start(ctx, "kafka.consume",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.source.name", msg.Topic),
			attribute.String("messaging.consumer.group", groupID),
		),
	)
	defer span.End()

	err := handler(ctx, msg)
	if err != nil {
		span.RecordError(err)
	}
	return err
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
