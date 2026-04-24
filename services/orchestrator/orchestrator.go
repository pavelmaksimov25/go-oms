package main

import (
	"context"
	"fmt"

	"github.com/pavelmaksimov25/go-oms/pkg/idempotency"
	saga "github.com/pavelmaksimov25/go-oms/pkg/proto/saga/v1"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	topicInventoryCommands = "inventory.commands"
	topicPaymentCommands   = "payment.commands"
	topicOrderEvents       = "order.events"

	eventInventoryReserve = "inventory.reserve"
	eventInventoryRelease = "inventory.release"
	eventPaymentCharge    = "payment.charge"
	eventOrderConfirmed   = "order.confirmed"
	eventOrderRejected    = "order.rejected"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type Orchestrator struct {
	store     *StateStore
	publisher Publisher
	guard     *idempotency.Guard
}

func NewOrchestrator(store *StateStore, publisher Publisher) *Orchestrator {
	return &Orchestrator{store: store, publisher: publisher, guard: idempotency.NewGuard()}
}

func (o *Orchestrator) HandleOrderCreated(ctx context.Context, msg kafkago.Message) error {
	var env saga.SagaEvent
	if err := proto.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	return o.guard.Run(env.GetSagaId(), env.GetEventType(), func() error {
		var evt saga.OrderCreatedEvent
		if err := proto.Unmarshal(env.GetPayload(), &evt); err != nil {
			return fmt.Errorf("unmarshal order.created payload: %w", err)
		}
		o.store.Create(&SagaData{
			OrderID:     evt.GetOrderId(),
			Items:       evt.GetItems(),
			TotalAmount: evt.GetTotalAmount(),
			State:       StateCreated,
		})
		if err := o.publish(ctx, topicInventoryCommands, evt.GetOrderId(), eventInventoryReserve,
			&saga.InventoryReserveCommand{OrderId: evt.GetOrderId(), Items: evt.GetItems()}); err != nil {
			return err
		}
		o.store.Transition(evt.GetOrderId(), StateInventoryReserving)
		return nil
	})
}

func (o *Orchestrator) HandleSagaEvent(ctx context.Context, msg kafkago.Message) error {
	var env saga.SagaEvent
	if err := proto.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	sagaID := env.GetSagaId()
	eventType := env.GetEventType()
	handler, ok := o.sagaEventHandler(eventType)
	if !ok {
		return nil
	}
	return o.guard.Run(sagaID, eventType, func() error {
		return handler(ctx, sagaID)
	})
}

func (o *Orchestrator) sagaEventHandler(eventType string) (func(context.Context, string) error, bool) {
	switch eventType {
	case "inventory.reserved":
		return o.onInventoryReserved, true
	case "inventory.failed":
		return o.onInventoryFailed, true
	case "payment.charged":
		return o.onPaymentCharged, true
	case "payment.failed":
		return o.onPaymentFailed, true
	case "inventory.released":
		return o.onInventoryReleased, true
	}
	return nil, false
}

func (o *Orchestrator) onInventoryReserved(ctx context.Context, orderID string) error {
	data, ok := o.store.Get(orderID)
	if !ok {
		return fmt.Errorf("saga %s not found", orderID)
	}
	if err := o.publish(ctx, topicPaymentCommands, orderID, eventPaymentCharge,
		&saga.PaymentChargeCommand{OrderId: orderID, Amount: data.TotalAmount}); err != nil {
		return err
	}
	o.store.Transition(orderID, StatePaymentCharging)
	return nil
}

func (o *Orchestrator) onInventoryFailed(ctx context.Context, orderID string) error {
	if err := o.publish(ctx, topicOrderEvents, orderID, eventOrderRejected,
		&saga.OrderRejectedEvent{OrderId: orderID, Reason: "inventory failed"}); err != nil {
		return err
	}
	o.store.Transition(orderID, StateFailed)
	return nil
}

func (o *Orchestrator) onPaymentCharged(ctx context.Context, orderID string) error {
	if err := o.publish(ctx, topicOrderEvents, orderID, eventOrderConfirmed,
		&saga.OrderConfirmedEvent{OrderId: orderID}); err != nil {
		return err
	}
	o.store.Transition(orderID, StateCompleted)
	return nil
}

func (o *Orchestrator) onPaymentFailed(ctx context.Context, orderID string) error {
	data, ok := o.store.Get(orderID)
	if !ok {
		return fmt.Errorf("saga %s not found", orderID)
	}
	if err := o.publish(ctx, topicInventoryCommands, orderID, eventInventoryRelease,
		&saga.InventoryReleaseCommand{OrderId: orderID, Items: data.Items}); err != nil {
		return err
	}
	o.store.Transition(orderID, StateCompensating)
	return nil
}

func (o *Orchestrator) onInventoryReleased(ctx context.Context, orderID string) error {
	if err := o.publish(ctx, topicOrderEvents, orderID, eventOrderRejected,
		&saga.OrderRejectedEvent{OrderId: orderID, Reason: "payment failed, inventory released"}); err != nil {
		return err
	}
	o.store.Transition(orderID, StateFailed)
	return nil
}

func (o *Orchestrator) publish(ctx context.Context, topic, sagaID, eventType string, payload proto.Message) error {
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
	return o.publisher.Publish(ctx, topic, []byte(sagaID), envBytes)
}
