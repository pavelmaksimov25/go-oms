package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
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
	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	serviceName   = "order-service"
	consumerTopic = "order.events"
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

	consumer := kafka.NewConsumer(cfg.KafkaBrokers, consumerTopic, serviceName)
	defer consumer.Close()

	store := NewStore()
	handler := NewHandler(store, producer)
	sagaConsumer := NewSagaConsumer(store)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	consumeErr := make(chan error, 1)
	go func() {
		consumeErr <- consumer.Consume(ctx, sagaConsumer.Handle)
	}()

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

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	order.RegisterOrderServiceServer(grpcServer, handler)
	reflection.Register(grpcServer)

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("grpc listening", "port", cfg.GRPCPort)
		serveErr <- grpcServer.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
	case err := <-serveErr:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return err
		}
	case err := <-consumeErr:
		if err != nil {
			slog.Error("consumer exited", "error", err)
		}
	}

	grpcServer.GracefulStop()
	return nil
}
