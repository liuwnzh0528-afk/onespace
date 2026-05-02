package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wnzhone/onespace/internal/domain"
)

func TestHTTPHealthCheckPassesOnTwoHundred(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := Checker{Client: server.Client()}
	result := checker.Check(context.Background(), domain.HealthCheck{
		Type: "http",
		URL:  server.URL,
	})
	if result.Status != "passing" {
		t.Fatalf("expected passing, got %q: %s", result.Status, result.Message)
	}
}

func TestHTTPHealthCheckFailsOnTimeout(t *testing.T) {
	checker := Checker{Client: &http.Client{Timeout: 1 * time.Millisecond}}
	result := checker.Check(context.Background(), domain.HealthCheck{
		Type:           "http",
		URL:            "http://127.0.0.1:1",
		TimeoutSeconds: 1,
	})
	if result.Status != "failing" {
		t.Fatalf("expected failing, got %q", result.Status)
	}
}

func TestHTTPHealthCheckRetriesUntilPassing(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := Checker{Client: server.Client()}
	result := checker.Check(context.Background(), domain.HealthCheck{
		Type:           "http",
		URL:            server.URL,
		TimeoutSeconds: 1,
	})
	if result.Status != "passing" {
		t.Fatalf("expected passing, got %q: %s", result.Status, result.Message)
	}
	if atomic.LoadInt32(&attempts) < 3 {
		t.Fatalf("attempts = %d, want at least 3", attempts)
	}
}
