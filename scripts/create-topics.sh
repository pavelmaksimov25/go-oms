#!/usr/bin/env bash
set -euo pipefail

BROKER="${KAFKA_BROKERS:-localhost:9092}"

TOPICS=(
    "saga.order.created"
    "inventory.commands"
    "payment.commands"
    "saga.events"
    "order.events"
)

echo "==> Creating Kafka topics on ${BROKER}..."

for topic in "${TOPICS[@]}"; do
    echo "    Creating topic: ${topic}"
    docker compose -f deployments/docker/docker-compose.yaml exec kafka \
        /opt/kafka/bin/kafka-topics.sh --create \
        --bootstrap-server localhost:9092 \
        --topic "${topic}" \
        --partitions 1 \
        --replication-factor 1 \
        --if-not-exists \
        2>/dev/null || echo "    (topic may already exist)"
done

echo "==> Done. Listing topics:"
docker compose -f deployments/docker/docker-compose.yaml exec kafka \
    /opt/kafka/bin/kafka-topics.sh --list --bootstrap-server localhost:9092
