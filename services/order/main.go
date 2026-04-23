package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os/signal"
	"syscall"

	"github.com/pavelmaksimov25/go-oms/pkg/config"
	"github.com/pavelmaksimov25/go-oms/pkg/kafka"
	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	serviceName   = "order-service"
	consumerTopic = "order.events"
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
	handler := NewHandler(store, producer)
	sagaConsumer := NewSagaConsumer(store)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	consumeErr := make(chan error, 1)
	go func() {
		consumeErr <- consumer.Consume(ctx, sagaConsumer.Handle)
	}()

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	order.RegisterOrderServiceServer(grpcServer, handler)
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
