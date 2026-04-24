package metrics

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestObserve_Success_IncrementsOK(t *testing.T) {
	EventsProcessed.Reset()

	err := Observe("test.success", func() error { return nil })
	if err != nil {
		t.Fatalf("Observe err = %v", err)
	}

	if got := testutil.ToFloat64(EventsProcessed.WithLabelValues("test.success", ResultOK)); got != 1 {
		t.Errorf("ok counter = %v, want 1", got)
	}
	if got := testutil.ToFloat64(EventsProcessed.WithLabelValues("test.success", ResultError)); got != 0 {
		t.Errorf("error counter = %v, want 0", got)
	}
}

func TestObserve_Error_IncrementsError_PropagatesErr(t *testing.T) {
	EventsProcessed.Reset()
	boom := errors.New("boom")

	err := Observe("test.fail", func() error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}

	if got := testutil.ToFloat64(EventsProcessed.WithLabelValues("test.fail", ResultError)); got != 1 {
		t.Errorf("error counter = %v, want 1", got)
	}
}

func TestObserve_RecordsDuration(t *testing.T) {
	EventDuration.Reset()

	_ = Observe("test.dur", func() error { return nil })

	if got := testutil.CollectAndCount(EventDuration); got == 0 {
		t.Error("histogram recorded no samples")
	}
}

func TestHandler_Serves_AppMetrics(t *testing.T) {
	EventsProcessed.Reset()
	_ = Observe("test.http", func() error { return nil })

	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "saga_events_processed_total") {
		t.Errorf("/metrics output missing saga_events_processed_total:\n%s", body)
	}
}
