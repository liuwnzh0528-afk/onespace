package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wnzhone/onespace/internal/serviceops"
)

func newTestDaemon(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]ServiceSummary{
			{Name: "user-api", Language: "go", Health: "passing"},
		})
	})

	mux.HandleFunc("POST /api/services/{service}/deploy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(serviceops.Result{
			Service: r.PathValue("service"),
			Status:  "success",
			Stage:   "done",
		})
	})

	mux.HandleFunc("GET /api/jobs/{jobId}/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Lines []string `json:"lines"`
		}{
			Lines: []string{"line 1", "line 2"},
		})
	})

	mux.HandleFunc("GET /api/services/{service}/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "passing"})
	})

	return httptest.NewServer(mux)
}

func TestDeployWaitJSONPrintsResultAndReturnsZeroOnSuccess(t *testing.T) {
	server := newTestDaemon(t)
	defer server.Close()

	var stdout, stderr strings.Builder
	code := Run([]string{"deploy", "user-api", "--wait", "--json"}, &stdout, &stderr, func(key string) string {
		if key == "ONESPACE_URL" {
			return server.URL
		}
		return ""
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	var result serviceops.Result
	if err := json.NewDecoder(strings.NewReader(stdout.String())).Decode(&result); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success", result.Status)
	}
}

func TestDeployWaitJSONReturnsNonZeroOnFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/services/{service}/deploy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(serviceops.Result{
			Service: "user-api",
			Status:  "failed",
			Stage:   "build",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var stdout, stderr strings.Builder
	code := Run([]string{"deploy", "user-api", "--wait", "--json"}, &stdout, &stderr, func(key string) string {
		if key == "ONESPACE_URL" {
			return server.URL
		}
		return ""
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code for failure")
	}
}

func TestLogsCommandPrintsTail(t *testing.T) {
	server := newTestDaemon(t)
	defer server.Close()

	var stdout, stderr strings.Builder
	code := Run([]string{"logs", "user-api", "--job", "job-123", "--tail", "200"}, &stdout, &stderr, func(key string) string {
		if key == "ONESPACE_URL" {
			return server.URL
		}
		return ""
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "line 1") {
		t.Fatalf("expected log lines in output, got: %s", stdout.String())
	}
}

func TestStatusCommandPrintsServiceSummary(t *testing.T) {
	server := newTestDaemon(t)
	defer server.Close()

	var stdout, stderr strings.Builder
	code := Run([]string{"status"}, &stdout, &stderr, func(key string) string {
		if key == "ONESPACE_URL" {
			return server.URL
		}
		return ""
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "user-api") {
		t.Fatalf("expected user-api in output, got: %s", stdout.String())
	}
}
