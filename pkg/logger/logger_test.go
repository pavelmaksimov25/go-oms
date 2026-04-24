package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNew_JSONOutput_IncludesServiceField(t *testing.T) {
	var buf bytes.Buffer
	l := New("order-service", &buf)

	l.Info("hello", slog.String("foo", "bar"))

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, buf.String())
	}
	if got["service"] != "order-service" {
		t.Errorf("service = %v, want order-service", got["service"])
	}
	if got["msg"] != "hello" {
		t.Errorf("msg = %v, want hello", got["msg"])
	}
	if got["foo"] != "bar" {
		t.Errorf("foo = %v, want bar", got["foo"])
	}
	if got["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", got["level"])
	}
}

func TestNew_DropsBelowInfo(t *testing.T) {
	var buf bytes.Buffer
	l := New("svc", &buf)

	l.Debug("secret")

	if buf.Len() != 0 {
		t.Errorf("expected debug to be dropped, got %s", buf.String())
	}
}
