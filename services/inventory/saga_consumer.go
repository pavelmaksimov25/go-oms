package main

import (
	"context"
	"fmt"

	saga "github.com/pavelmaksimov25/go-oms/pkg/proto/saga/v1"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	topicSagaEvents = "saga.events"

	cmdInventoryReserve = "inventory.reserve"
	cmdInventoryRelease = "inventory.release"

	eventInventoryReserved = "inventory.reserved"
	eventInventoryFailed   = "inventory.failed"
	eventInventoryReleased = "inventory.released"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type SagaConsumer struct {
	store     *Store
	publisher Publisher
}

func NewSagaConsumer(store *Store, publisher Publisher) *SagaConsumer {
	return &SagaConsumer{store: store, publisher: publisher}
}

func (c *SagaConsumer) Handle(ctx context.Context, msg kafkago.Message) error {
	var env saga.SagaEvent
	if err := proto.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("unmarshal saga envelope: %w", err)
	}
	switch env.GetEventType() {
	case cmdInventoryReserve:
		return c.handleReserve(ctx, &env)
	case cmdInventoryRelease:
		return c.handleRelease(ctx, &env)
	}
	return nil
}

func (c *SagaConsumer) handleReserve(ctx context.Context, env *saga.SagaEvent) error {
	var cmd saga.InventoryReserveCommand
	if err := proto.Unmarshal(env.GetPayload(), &cmd); err != nil {
		return fmt.Errorf("unmarshal reserve cmd: %w", err)
	}
	reserved := make([]*saga.OrderItem, 0, len(cmd.GetItems()))
	for _, item := range cmd.GetItems() {
		if err := c.store.Reserve(item.GetItemId(), item.GetQuantity()); err != nil {
			for _, r := range reserved {
				c.store.Release(r.GetItemId(), r.GetQuantity())
			}
			return c.publishEvent(ctx, cmd.GetOrderId(), eventInventoryFailed,
				&saga.InventoryFailedEvent{OrderId: cmd.GetOrderId(), Reason: err.Error()})
		}
		reserved = append(reserved, item)
	}
	return c.publishEvent(ctx, cmd.GetOrderId(), eventInventoryReserved,
		&saga.InventoryReservedEvent{OrderId: cmd.GetOrderId()})
}

func (c *SagaConsumer) handleRelease(ctx context.Context, env *saga.SagaEvent) error {
	var cmd saga.InventoryReleaseCommand
	if err := proto.Unmarshal(env.GetPayload(), &cmd); err != nil {
		return fmt.Errorf("unmarshal release cmd: %w", err)
	}
	for _, item := range cmd.GetItems() {
		c.store.Release(item.GetItemId(), item.GetQuantity())
	}
	return c.publishEvent(ctx, cmd.GetOrderId(), eventInventoryReleased,
		&saga.InventoryReleasedEvent{OrderId: cmd.GetOrderId()})
}

func (c *SagaConsumer) publishEvent(ctx context.Context, sagaID, eventType string, payload proto.Message) error {
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	envBytes, err := proto.Marshal(&saga.SagaEvent{
		SagaId:    sagaID,
		EventType: eventType,
		Payload:   payloadBytes,
		CreatedAt: timestamppb.Now(),
	})
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	return c.publisher.Publish(ctx, topicSagaEvents, []byte(sagaID), envBytes)
}
