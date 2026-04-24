package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/pavelmaksimov25/go-oms/pkg/config"
	"github.com/pavelmaksimov25/go-oms/pkg/health"
	"github.com/pavelmaksimov25/go-oms/pkg/kafka"
	inventory "github.com/pavelmaksimov25/go-oms/pkg/proto/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	serviceName   = "inventory-service"
	consumerTopic = "inventory.commands"
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

	consumer := kafka.NewConsumer(cfg.KafkaBrokers, consumerTopic, serviceName)
	defer consumer.Close()

	store := NewStore()
	handler := NewHandler(store)
	sagaConsumer := NewSagaConsumer(store, producer)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	consumeErr := make(chan error, 1)
	go func() {
		consumeErr <- consumer.Consume(ctx, sagaConsumer.Handle)
	}()

	healthServer := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           health.NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("%s: health listening on :%s", serviceName, cfg.HTTPPort)
		if err := healthServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("%s: health server exited: %v", serviceName, err)
		}
	}()
	defer healthServer.Shutdown(context.Background())

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	inventory.RegisterInventoryServiceServer(grpcServer, handler)
	reflection.Register(grpcServer)

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("%s: gRPC listening on :%s", serviceName, cfg.GRPCPort)
		serveErr <- grpcServer.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		log.Printf("%s: shutting down", serviceName)
	case err := <-serveErr:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return err
		}
	case err := <-consumeErr:
		if err != nil {
			log.Printf("%s: consumer exited: %v", serviceName, err)
		}
	}

	grpcServer.GracefulStop()
	return nil
}
