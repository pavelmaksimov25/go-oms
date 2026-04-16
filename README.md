# go-oms

A distributed Order Management System built with Go to practice microservices, gRPC, Kafka, Kubernetes, and the Saga pattern.

## Architecture

Four services communicate via gRPC (sync) and Kafka (async). The saga orchestrator coordinates distributed transactions across services.

```
                         ┌──────────────────┐
                         │  Order Service    │
  Client ──gRPC──────────│  :50051           │
                         └────────┬─────────┘
                                  │ Kafka
                         ┌────────▼─────────┐
                         │  Saga             │
                         │  Orchestrator     │
                         └──┬────────────┬──┘
                   Kafka    │            │    Kafka
               ┌────────────▼──┐   ┌────▼───────────┐
               │  Inventory    │   │  Payment        │
               │  Service      │   │  Service        │
               │  :50052       │   │  :50053         │
               └───────────────┘   └─────────────────┘
```

### Saga Flow (Happy Path)

1. Client calls `CreateOrder` on order-service via gRPC
2. Order service publishes `saga.order.created` to Kafka
3. Orchestrator receives event, sends `inventory.reserve` command
4. Inventory service reserves stock, replies `inventory.reserved`
5. Orchestrator sends `payment.charge` command
6. Payment service charges payment, replies `payment.charged`
7. Orchestrator publishes `order.confirmed`
8. Order service updates order status to `CONFIRMED`

**Compensation**: if payment fails (amount > 1000), orchestrator sends `inventory.release` to undo the reservation, then `order.rejected`.

## Prerequisites

- Go 1.25+
- Docker & Docker Compose
- [buf](https://buf.build/docs/installation) (for proto generation)
- [kind](https://kind.sigs.k8s.io/) (for local Kubernetes)
- [grpcurl](https://github.com/fullstorydev/grpcurl) (for testing)

```bash
make tools   # install Go-based tools
```

## Quick Start

### Local development (services on host, Kafka in Docker)

```bash
# 1. Generate proto stubs
make proto-gen

# 2. Start Kafka
make compose-up

# 3. Create Kafka topics
./scripts/create-topics.sh

# 4. Run services (each in a separate terminal)
make run-order
make run-inventory
make run-payment
make run-orchestrator

# 5. Test
make test-order
```

### Kubernetes (everything in Kind)

```bash
make docker-build
make kind-create
make kind-load
make k8s-deploy

# Verify
kubectl get pods -n oms
```

## Project Structure

```
go-oms/
├── go.work              # Go workspace tying all modules together
├── proto/               # Protobuf definitions
├── pkg/                 # Shared module (proto stubs, Kafka wrappers, config)
├── services/            # One directory per microservice, each with its own go.mod
│   ├── order/
│   ├── inventory/
│   ├── payment/
│   └── orchestrator/
├── deployments/
│   ├── docker/          # Dockerfile + docker-compose
│   └── k8s/             # Kind config + K8s manifests
└── scripts/             # Dev scripts
```
