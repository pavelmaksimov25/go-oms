package main

import (
	"context"
	"fmt"

	"github.com/pavelmaksimov25/go-oms/pkg/idempotency"
	"github.com/pavelmaksimov25/go-oms/pkg/metrics"
	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
	saga "github.com/pavelmaksimov25/go-oms/pkg/proto/saga/v1"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

const (
	eventOrderConfirmed = "order.confirmed"
	eventOrderRejected  = "order.rejected"
)

type SagaConsumer struct {
	store *Store
	guard *idempotency.Guard
}

func NewSagaConsumer(store *Store) *SagaConsumer {
	return &SagaConsumer{store: store, guard: idempotency.NewGuard()}
}

func (c *SagaConsumer) Handle(_ context.Context, msg kafkago.Message) error {
	var env saga.SagaEvent
	if err := proto.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("unmarshal saga envelope: %w", err)
	}
	var status order.OrderStatus
	switch env.GetEventType() {
	case eventOrderConfirmed:
		status = order.OrderStatus_ORDER_STATUS_CONFIRMED
	case eventOrderRejected:
		status = order.OrderStatus_ORDER_STATUS_REJECTED
	default:
		return nil
	}
	return metrics.Observe(env.GetEventType(), func() error {
		return c.guard.Run(env.GetSagaId(), env.GetEventType(), func() error {
			c.store.SetStatus(env.GetSagaId(), status)
			return nil
		})
	})
}
