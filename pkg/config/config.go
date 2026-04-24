package config

import (
	"os"
	"strings"
)

type Config struct {
	ServiceName  string
	GRPCPort     string
	HTTPPort     string
	KafkaBrokers []string
}

func Load(serviceName string) *Config {
	return &Config{
		ServiceName:  serviceName,
		GRPCPort:     getEnv("GRPC_PORT", "50051"),
		HTTPPort:     getEnv("HTTP_PORT", "8080"),
		KafkaBrokers: parseBrokers(getEnv("KAFKA_BROKERS", "localhost:9092")),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func parseBrokers(raw string) []string {
	parts := strings.Split(raw, ",")
	brokers := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			brokers = append(brokers, trimmed)
		}
	}
	return brokers
}
