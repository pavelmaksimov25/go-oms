# Implementation Plan

Step-by-step guide to implement the OMS from the provided scaffolding. Each step builds on the previous and should compile before moving on.

---

## What's already provided

- **Proto definitions** (`proto/`) and generated Go stubs (`pkg/proto/`)
- **Go modules** (`go.work`, `go.mod` files for `pkg/` and each service)
- **Infrastructure** (Dockerfile, docker-compose with KRaft Kafka, K8s manifests)
- **Makefile** with build/run/deploy targets
- **Scripts** for topic creation and e2e testing

You write all the `.go` source files.

---

## Step 1: Shared Config (`pkg/config/`)

**Create** `pkg/config/config.go`

A simple struct + loader that reads from environment variables:

```go
type Config struct {
    ServiceName  string
    GRPCPort     string
    KafkaBrokers []string
}
```

- `GRPC_PORT` defaults to `"50051"`
- `KAFKA_BROKERS` defaults to `"localhost:9092"` (comma-separated)

**Verify**: `go build ./pkg/...` compiles.

---

## Step 2: Kafka Wrappers (`pkg/kafka/`)

**Create** `pkg/kafka/producer.go` and `pkg/kafka/consumer.go`

Thin wrappers around `github.com/segmentio/kafka-go` (already in `pkg/go.mod`).

### Producer
- Wraps `kafka.Writer`
- `NewProducer(brokers []string) *Producer`
- `Publish(ctx, topic string, key, value []byte) error`
- `Close() error`

### Consumer
- Wraps `kafka.Reader`
- `NewConsumer(brokers []string, topic, groupID string) *Consumer`
- `Consume(ctx, handler func(ctx, kafka.Message) error)` — blocking loop that calls handler for each message
- `Close() error`

**Verify**: `go build ./pkg/...` compiles.

---

## Step 3: Order Service — Store

**Create** `services/order/store.go`

In-memory thread-safe store for orders using `sync.RWMutex` + `map[string]*orderv1.Order`.

Methods:
- `Create(order *orderv1.Order)`
- `Get(orderID string) (*orderv1.Order, bool)`
- `SetStatus(orderID string, status orderv1.OrderStatus)`

Use the generated `orderv1` types from `pkg/proto/order/v1`.

---

## Step 4: Order Service — gRPC Handler

**Create** `services/order/handler.go`

Implements `orderv1.OrderServiceServer` (embed `UnimplementedOrderServiceServer`).

### CreateOrder
1. Generate a UUID for the order (`github.com/google/uuid`)
2. Store the order with status `ORDER_STATUS_PENDING`
3. Build a `sagav1.OrderCreatedEvent` with order details
4. Wrap it in a `sagav1.SagaEvent` envelope (saga_id = order_id, event_type = `"order.created"`)
5. Marshal and publish to Kafka topic `saga.order.created`
6. Return the order ID and PENDING status

### GetOrder
- Look up by ID, return `NOT_FOUND` if missing

**Key pattern**: the handler takes both the store and a Kafka producer as constructor args.

---

## Step 5: Order Service — Saga Consumer

**Create** `services/order/saga_consumer.go`

Consumes from Kafka topic `order.events`. Handles two event types:

- `"order.confirmed"` → set order status to `CONFIRMED`
- `"order.rejected"` → set order status to `REJECTED`

Each message is a `sagav1.SagaEvent` envelope. Unmarshal the envelope, switch on `EventType`, then unmarshal `Payload` for the specific event type if needed.

---

## Step 6: Order Service — Main

**Create** `services/order/main.go`

Wire everything together:
1. Load config (service name = `"order-service"`)
2. Create Kafka producer and consumer (topic `"order.events"`, group `"order-service"`)
3. Create store, handler, saga consumer
4. Start Kafka consumer loop in a goroutine
5. Start gRPC server on `:50051` with reflection enabled
6. Wait for `SIGINT`/`SIGTERM`, then graceful shutdown

**Verify**: `go build ./services/order` compiles. Run it (`make run-order`) — it should start and log the listening port (Kafka connection will fail without docker-compose, that's fine).

---

## Step 7: Inventory Service

**Create** 4 files in `services/inventory/`:

### store.go
In-memory stock map (`map[string]int32`). Seed with sample data:
- `"item-1": 100`, `"item-2": 50`, `"item-3": 200`

Methods: `Reserve(itemID, qty)`, `Release(itemID, qty)`, `GetStock(itemID)`

### handler.go
Implements `inventoryv1.InventoryServiceServer`. Single RPC: `CheckStock` returns available quantity.

### saga_consumer.go
Consumes from topic `inventory.commands`. Two command types:

- `"inventory.reserve"` — try to reserve all items. If any fail, rollback already-reserved ones. Reply to topic `saga.events` with either `"inventory.reserved"` or `"inventory.failed"`.
- `"inventory.release"` — release items (compensation). Reply with `"inventory.released"`.

**Reply pattern**: build a `sagav1.SagaEvent` envelope with the appropriate event_type and payload, publish to `saga.events`.

### main.go
Same pattern as order service but on port `50052`, consuming from `inventory.commands`.

**Verify**: `go build ./services/inventory`

---

## Step 8: Payment Service

**Create** 4 files in `services/payment/`:

### store.go
In-memory payment map. Store payment ID, order ID, amount, status.

### handler.go
Implements `paymentv1.PaymentServiceServer`. Single RPC: `GetPaymentStatus`.

### saga_consumer.go
Consumes from topic `payment.commands`. Handles `"payment.charge"`:

- **Simulate failure**: if amount > 1000, reply `"payment.failed"` to `saga.events`
- **Otherwise**: create a payment record, reply `"payment.charged"` to `saga.events`

### main.go
Same pattern, port `50053`, consuming from `payment.commands`.

**Verify**: `go build ./services/payment`

---

## Step 9: Saga Orchestrator

This is the core piece — no gRPC server, just Kafka.

**Create** 3 files in `services/orchestrator/`:

### saga_state.go
Define the saga state machine states:
```
Created → InventoryReserving → InventoryReserved → PaymentCharging → Completed
                                                                   ↘ Compensating → Failed
```

In-memory store: `map[string]*SagaData` tracking state, order ID, items, and total amount per saga.

### orchestrator.go
Two handler methods, each processing a Kafka message:

**HandleOrderCreated** (topic: `saga.order.created`):
1. Parse the `OrderCreatedEvent` from the envelope
2. Create saga tracking entry with state `Created`
3. Publish `"inventory.reserve"` command to `inventory.commands`
4. Transition to `InventoryReserving`

**HandleSagaEvent** (topic: `saga.events`):

Switch on `event_type`:

| Event | Action | Next State |
|-------|--------|------------|
| `inventory.reserved` | Send `payment.charge` to `payment.commands` | PaymentCharging |
| `inventory.failed` | Send `order.rejected` to `order.events` | Failed |
| `payment.charged` | Send `order.confirmed` to `order.events` | Completed |
| `payment.failed` | Send `inventory.release` to `inventory.commands` (compensate) | Compensating |
| `inventory.released` | Send `order.rejected` to `order.events` | Failed |

### main.go
1. Create producer + two consumers (topics: `saga.order.created` and `saga.events`)
2. Run both consumer loops in goroutines
3. Wait for shutdown signal

**Verify**: `go build ./services/orchestrator`

---

## Step 10: End-to-End Test

```bash
# Start Kafka
make compose-up
./scripts/create-topics.sh

# Run all 4 services in separate terminals
make run-order
make run-inventory
make run-payment
make run-orchestrator

# Test happy path + compensation
make test-order
```

Expected results:
- **amount=100**: order status transitions PENDING → CONFIRMED
- **amount=5000**: order status transitions PENDING → REJECTED (payment fails, inventory compensated)

---

## Step 11: Kubernetes Deployment

```bash
make docker-build
make kind-create
make kind-load
make k8s-deploy
kubectl get pods -n oms        # all 4 pods + kafka should be Running
```

Test via NodePort:
```bash
grpcurl -plaintext localhost:30051 order.v1.OrderService/CreateOrder \
  -d '{"items":[{"item_id":"item-1","quantity":1}],"total_amount":100}'
```

---

## Kafka Topics Reference

| Topic | Producer | Consumer | Message |
|-------|----------|----------|---------|
| `saga.order.created` | order | orchestrator | OrderCreatedEvent |
| `inventory.commands` | orchestrator | inventory | InventoryReserve/ReleaseCommand |
| `payment.commands` | orchestrator | payment | PaymentChargeCommand |
| `saga.events` | inventory, payment | orchestrator | *Reserved/*Failed/*Charged events |
| `order.events` | orchestrator | order | OrderConfirmed/RejectedEvent |

## Message Envelope Pattern

Every Kafka message is a protobuf-encoded `SagaEvent`:
```proto
message SagaEvent {
  string saga_id = 1;     // == order_id
  string event_type = 2;  // discriminator string
  bytes payload = 3;      // type-specific protobuf bytes
  Timestamp created_at = 4;
}
```

To send: marshal inner message → set as `payload` → marshal `SagaEvent` → publish.
To receive: unmarshal `SagaEvent` → switch on `event_type` → unmarshal `payload` to correct type.
