package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func TestInit_NoEndpoint_InstallsPropagatorOnly(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := Init(context.Background(), "test-service")
	if err != nil {
		t.Fatalf("Init err = %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown is nil")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown err = %v", err)
	}

	// Propagator should be installed even without exporter so trace context
	// still flows across service boundaries in local mode.
	prop := otel.GetTextMapPropagator()
	if _, ok := prop.(propagation.TextMapPropagator); !ok {
		t.Errorf("propagator = %T, expected TextMapPropagator", prop)
	}
}
