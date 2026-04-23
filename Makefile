.PHONY: proto-gen build test clean \
	docker-build compose-up compose-down \
	kind-create kind-delete kind-load k8s-deploy k8s-delete \
	run-order run-inventory run-payment run-orchestrator \
	test-order tools

# ── Tools ────────────────────────────────────────────────────────────────────

tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Install buf:     https://buf.build/docs/installation"
	@echo "Install kind:    go install sigs.k8s.io/kind@latest"
	@echo "Install grpcurl: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest"

# ── Proto ────────────────────────────────────────────────────────────────────

proto-gen:
	buf generate

proto-lint:
	buf lint

# ── Build ────────────────────────────────────────────────────────────────────

build:
	@mkdir -p bin
	go build -o bin/order-service ./services/order
	go build -o bin/inventory-service ./services/inventory
	go build -o bin/payment-service ./services/payment
	go build -o bin/saga-orchestrator ./services/orchestrator

test:
	go test ./pkg/... ./services/order/... ./services/inventory/... ./services/payment/... ./services/orchestrator/...

clean:
	rm -f bin/*

# ── Run locally (Kafka must be running via compose-up) ───────────────────────

run-order:
	GRPC_PORT=50051 go run ./services/order

run-inventory:
	GRPC_PORT=50052 go run ./services/inventory

run-payment:
	GRPC_PORT=50053 go run ./services/payment

run-orchestrator:
	go run ./services/orchestrator

# ── Docker ───────────────────────────────────────────────────────────────────

SERVICES := order inventory payment orchestrator

docker-build:
	@for svc in $(SERVICES); do \
		docker build -f deployments/docker/Dockerfile \
			--build-arg SERVICE_NAME=$$svc \
			-t go-oms-$$svc:latest . ; \
	done

compose-up:
	docker compose -f deployments/docker/docker-compose.yaml up -d

compose-down:
	docker compose -f deployments/docker/docker-compose.yaml down

# ── Kubernetes (Kind) ────────────────────────────────────────────────────────

kind-create:
	kind create cluster --name oms --config deployments/k8s/kind-config.yaml

kind-delete:
	kind delete cluster --name oms

kind-load:
	@for svc in $(SERVICES); do \
		kind load docker-image go-oms-$$svc:latest --name oms ; \
	done

k8s-deploy:
	kubectl apply -f deployments/k8s/namespace.yaml
	kubectl apply -f deployments/k8s/kafka/
	kubectl apply -f deployments/k8s/order/
	kubectl apply -f deployments/k8s/inventory/
	kubectl apply -f deployments/k8s/payment/
	kubectl apply -f deployments/k8s/orchestrator/

k8s-delete:
	kubectl delete namespace oms --ignore-not-found

# ── Integration test ─────────────────────────────────────────────────────────

test-order:
	./scripts/test-order.sh
