package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pavelmaksimov25/go-oms/pkg/config"
	"github.com/pavelmaksimov25/go-oms/pkg/health"
	"github.com/pavelmaksimov25/go-oms/pkg/kafka"
	"github.com/pavelmaksimov25/go-oms/pkg/logger"
	"github.com/pavelmaksimov25/go-oms/pkg/metrics"
)

const (
	serviceName = "saga-orchestrator"

	topicOrderCreated = "saga.order.created"
	topicSagaEvents   = "saga.events"
)

func main() {
	logger.Init(serviceName)
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
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

	httpMux := http.NewServeMux()
	health.RegisterRoutes(httpMux)
	httpMux.Handle("/metrics", metrics.Handler())
	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           httpMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.Info("http listening", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server exited", "error", err)
		}
	}()
	defer httpServer.Shutdown(context.Background())

	errCh := make(chan error, 2)
	go func() {
		errCh <- orderCreatedConsumer.Consume(ctx, orchestrator.HandleOrderCreated)
	}()
	go func() {
		errCh <- sagaEventsConsumer.Consume(ctx, orchestrator.HandleSagaEvent)
	}()

	slog.Info("running", "topics", []string{topicOrderCreated, topicSagaEvents})

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
	case err := <-errCh:
		if err != nil {
			slog.Error("consumer exited", "error", err)
		}
	}
	return nil
}
