# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Distributed Order Management System (OMS) built with Go microservices. Uses gRPC for synchronous communication, Kafka for async event-driven messaging, and the orchestration-based Saga pattern for distributed transactions. Designed as a learning project for Kubernetes, Go, gRPC, Protobuf, Kafka, and Saga.

## Build & Run Commands

```bash
# Generate proto stubs (requires buf)
make proto-gen

# Build all services
make build

# Run tests
make test

# Run individual services (Kafka must be running)
make run-order          # gRPC :50051
make run-inventory      # gRPC :50052
make run-payment        # gRPC :50053
make run-orchestrator   # Kafka only (no gRPC)

# Start/stop Kafka (Docker Compose, KRaft mode)
make compose-up
make compose-down

# Create Kafka topics
./scripts/create-topics.sh

# End-to-end test via grpcurl
make test-order

# Kubernetes (Kind)
make docker-build
make kind-create
make kind-load
make k8s-deploy
```

## Architecture

### Module Structure

Uses `go.work` with per-service Go modules. Each service has its own `go.mod` and depends on the shared `pkg/` module via `replace` directive.

```
go.work
├── pkg/                  # shared module: proto stubs, Kafka wrappers, config
├── services/order/       # accepts orders via gRPC, publishes saga start events
├── services/inventory/   # reserves/releases stock via Kafka commands
├── services/payment/     # charges/refunds (simulated) via Kafka commands
└── services/orchestrator/# saga state machine, Kafka only (no gRPC server)
```

### Saga Flow (Orchestration Pattern)

The `saga-orchestrator` is the central coordinator. It consumes events and produces commands via Kafka:

1. **order-service** receives `CreateOrder` gRPC call, publishes `saga.order.created`
2. **orchestrator** sends `inventory.reserve` command
3. **inventory-service** reserves stock, replies `inventory.reserved` (or `inventory.failed`)
4. **orchestrator** sends `payment.charge` command
5. **payment-service** charges payment, replies `payment.charged` (or `payment.failed`)
6. **orchestrator** publishes `order.confirmed` (or compensates: `inventory.release` then `order.rejected`)

Payment simulates failure for amounts > 1000 to test compensation.

### Kafka Topics

| Topic | Producer | Consumer |
|-------|----------|----------|
| `saga.order.created` | order | orchestrator |
| `inventory.commands` | orchestrator | inventory |
| `payment.commands` | orchestrator | payment |
| `saga.events` | inventory, payment | orchestrator |
| `order.events` | orchestrator | order |

### Message Format

All Kafka messages use protobuf-encoded `SagaEvent` envelopes (defined in `proto/saga/v1/events.proto`). The `event_type` field acts as a discriminator and `payload` contains type-specific protobuf bytes.

### Key Files (to be implemented)

- `services/orchestrator/orchestrator.go` — saga state machine, the core distributed transaction logic
- `proto/saga/v1/events.proto` — defines all saga event/command message types
- `pkg/kafka/` — shared Kafka producer/consumer wrappers (segmentio/kafka-go)
- `pkg/config/config.go` — shared config loaded from env vars (`GRPC_PORT`, `KAFKA_BROKERS`)

### Implementation Plan

See `PLAN.md` for step-by-step guide to implement all Go source files from scratch.

### Service Ports

| Service | gRPC Port | K8s NodePort |
|---------|-----------|--------------|
| order | 50051 | 30051 |
| inventory | 50052 | — |
| payment | 50053 | — |
| orchestrator | (none) | — |

## Dependencies

- `segmentio/kafka-go` — pure Go Kafka client (no CGO)
- `buf` — proto generation (see `buf.yaml`, `buf.gen.yaml`)
- `bitnami/kafka:3.7` — KRaft-mode Kafka (no Zookeeper)
- `kind` — local Kubernetes clusters

## Proto Generation

Proto files live in `proto/` and generated Go code goes to `pkg/proto/`. Always run `make proto-gen` after modifying `.proto` files. Generated code is committed to the repo.
