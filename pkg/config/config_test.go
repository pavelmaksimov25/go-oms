package config

import (
	"reflect"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("GRPC_PORT", "")
	t.Setenv("KAFKA_BROKERS", "")

	cfg := Load("order-service")

	if cfg.ServiceName != "order-service" {
		t.Errorf("ServiceName = %q, want %q", cfg.ServiceName, "order-service")
	}
	if cfg.GRPCPort != "50051" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "50051")
	}
	if !reflect.DeepEqual(cfg.KafkaBrokers, []string{"localhost:9092"}) {
		t.Errorf("KafkaBrokers = %v, want [localhost:9092]", cfg.KafkaBrokers)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("GRPC_PORT", "60000")
	t.Setenv("KAFKA_BROKERS", "kafka-1:9092,kafka-2:9092")

	cfg := Load("inventory-service")

	if cfg.GRPCPort != "60000" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "60000")
	}
	want := []string{"kafka-1:9092", "kafka-2:9092"}
	if !reflect.DeepEqual(cfg.KafkaBrokers, want) {
		t.Errorf("KafkaBrokers = %v, want %v", cfg.KafkaBrokers, want)
	}
}

func TestLoad_TrimsBrokerWhitespace(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", " kafka-1:9092 , kafka-2:9092 ")

	cfg := Load("payment-service")

	want := []string{"kafka-1:9092", "kafka-2:9092"}
	if !reflect.DeepEqual(cfg.KafkaBrokers, want) {
		t.Errorf("KafkaBrokers = %v, want %v", cfg.KafkaBrokers, want)
	}
}
