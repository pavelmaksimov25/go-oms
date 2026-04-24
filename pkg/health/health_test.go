package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_Healthz_AlwaysOK(t *testing.T) {
	srv := httptest.NewServer(NewHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/healthz status = %d, want 200", resp.StatusCode)
	}
}

func TestHandler_Readyz_NoChecks_ReturnsOK(t *testing.T) {
	srv := httptest.NewServer(NewHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/readyz status = %d, want 200", resp.StatusCode)
	}
}

func TestHandler_Readyz_CheckPasses_ReturnsOK(t *testing.T) {
	check := CheckerFunc(func(_ context.Context) error { return nil })
	srv := httptest.NewServer(NewHandler(check))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/readyz status = %d, want 200", resp.StatusCode)
	}
}

func TestHandler_Readyz_CheckFails_Returns503(t *testing.T) {
	check := CheckerFunc(func(_ context.Context) error { return errors.New("not ready") })
	srv := httptest.NewServer(NewHandler(check))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("/readyz status = %d, want 503", resp.StatusCode)
	}
}

func TestHandler_Readyz_MultipleChecks_AnyFailure_Returns503(t *testing.T) {
	ok := CheckerFunc(func(_ context.Context) error { return nil })
	bad := CheckerFunc(func(_ context.Context) error { return errors.New("down") })
	srv := httptest.NewServer(NewHandler(ok, bad))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("/readyz status = %d, want 503", resp.StatusCode)
	}
}
