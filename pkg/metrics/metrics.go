package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ResultOK / ResultError are the canonical values for the "result" label.
const (
	ResultOK    = "ok"
	ResultError = "error"
)

var (
	EventsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "saga_events_processed_total",
		Help: "Total saga events processed, labeled by event type and result.",
	}, []string{"event_type", "result"})

	EventDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "saga_event_duration_seconds",
		Help:    "Saga event handler duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"event_type"})
)

// Handler exposes the default prometheus registry at /metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}

// Observe runs fn and records its duration and outcome under the given
// event type. The returned error is fn's error, unchanged.
func Observe(eventType string, fn func() error) error {
	start := time.Now()
	err := fn()
	EventDuration.WithLabelValues(eventType).Observe(time.Since(start).Seconds())
	result := ResultOK
	if err != nil {
		result = ResultError
	}
	EventsProcessed.WithLabelValues(eventType, result).Inc()
	return err
}
