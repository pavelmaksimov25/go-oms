package main

import (
	"context"
	"fmt"

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
}

func NewSagaConsumer(store *Store) *SagaConsumer {
	return &SagaConsumer{store: store}
}

func (c *SagaConsumer) Handle(_ context.Context, msg kafkago.Message) error {
	var env saga.SagaEvent
	if err := proto.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("unmarshal saga envelope: %w", err)
	}
	switch env.GetEventType() {
	case eventOrderConfirmed:
		c.store.SetStatus(env.GetSagaId(), order.OrderStatus_ORDER_STATUS_CONFIRMED)
	case eventOrderRejected:
		c.store.SetStatus(env.GetSagaId(), order.OrderStatus_ORDER_STATUS_REJECTED)
	}
	return nil
}
