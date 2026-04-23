package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	payment "github.com/pavelmaksimov25/go-oms/pkg/proto/payment/v1"
	saga "github.com/pavelmaksimov25/go-oms/pkg/proto/saga/v1"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	topicSagaEvents = "saga.events"

	cmdPaymentCharge = "payment.charge"

	eventPaymentCharged = "payment.charged"
	eventPaymentFailed  = "payment.failed"

	failureThreshold = 1000.0
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
	if env.GetEventType() == cmdPaymentCharge {
		return c.handleCharge(ctx, &env)
	}
	return nil
}

func (c *SagaConsumer) handleCharge(ctx context.Context, env *saga.SagaEvent) error {
	var cmd saga.PaymentChargeCommand
	if err := proto.Unmarshal(env.GetPayload(), &cmd); err != nil {
		return fmt.Errorf("unmarshal charge cmd: %w", err)
	}

	if cmd.GetAmount() > failureThreshold {
		return c.publishEvent(ctx, cmd.GetOrderId(), eventPaymentFailed,
			&saga.PaymentFailedEvent{OrderId: cmd.GetOrderId(), Reason: "amount exceeds limit"})
	}

	paymentID := uuid.NewString()
	c.store.Create(&Payment{
		ID:      paymentID,
		OrderID: cmd.GetOrderId(),
		Amount:  cmd.GetAmount(),
		Status:  payment.PaymentStatus_PAYMENT_STATUS_CHARGED,
	})
	return c.publishEvent(ctx, cmd.GetOrderId(), eventPaymentCharged,
		&saga.PaymentChargedEvent{OrderId: cmd.GetOrderId(), PaymentId: paymentID})
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
