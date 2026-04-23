package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/pavelmaksimov25/go-oms/pkg/config"
	"github.com/pavelmaksimov25/go-oms/pkg/kafka"
)

const (
	serviceName = "saga-orchestrator"

	topicOrderCreated = "saga.order.created"
	topicSagaEvents   = "saga.events"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("%s: %v", serviceName, err)
	}
}

func run() error {
	cfg := config.Load(serviceName)

	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	orderCreatedConsumer := kafka.NewConsumer(cfg.KafkaBrokers, topicOrderCreated, serviceName)
	defer orderCreatedConsumer.Close()

	sagaEventsConsumer := kafka.NewConsumer(cfg.KafkaBrokers, topicSagaEvents, serviceName)
	defer sagaEventsConsumer.Close()

	orchestrator := NewOrchestrator(NewStateStore(), producer)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go func() {
		errCh <- orderCreatedConsumer.Consume(ctx, orchestrator.HandleOrderCreated)
	}()
	go func() {
		errCh <- sagaEventsConsumer.Consume(ctx, orchestrator.HandleSagaEvent)
	}()

	log.Printf("%s: running, consuming %s and %s", serviceName, topicOrderCreated, topicSagaEvents)

	select {
	case <-ctx.Done():
		log.Printf("%s: shutting down", serviceName)
	case err := <-errCh:
		if err != nil {
			log.Printf("%s: consumer exited: %v", serviceName, err)
		}
	}
	return nil
}
