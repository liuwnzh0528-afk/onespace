package cli

import (
	"context"
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

	mux.HandleFunc("GET /api/services/{service}/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Lines []string `json:"lines"`
		}{
			Lines: []string{"service line"},
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

func TestBuildReturnsNonZeroWhenDaemonRejectsOperation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/services/{service}/build", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(serviceops.Result{
			Service: r.PathValue("service"),
			Status:  "failed",
			Stage:   "unsupported",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var stdout, stderr strings.Builder
	code := Run([]string{"build", "bmc-a"}, &stdout, &stderr, func(key string) string {
		if key == "ONESPACE_URL" {
			return server.URL
		}
		return ""
	})

	if code == 0 {
		t.Fatalf("expected non-zero exit code; stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "HTTP 500") {
		t.Fatalf("stderr = %q, want HTTP 500", stderr.String())
	}
}

func TestRestartStopAndPullClientsReturnErrorsOnHTTPFailure(t *testing.T) {
	mux := http.NewServeMux()
	for _, path := range []string{
		"/api/services/{service}/restart",
		"/api/services/{service}/stop",
		"/api/services/{service}/pull",
	} {
		mux.HandleFunc("POST "+path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(serviceops.Result{
				Service: r.PathValue("service"),
				Status:  "failed",
				Stage:   "unsupported",
			})
		})
	}
	server := httptest.NewServer(mux)
	defer server.Close()

	client := Client{BaseURL: server.URL}
	for name, fn := range map[string]func() (serviceops.Result, error){
		"restart": func() (serviceops.Result, error) { return client.Restart(context.Background(), "bmc-a") },
		"stop":    func() (serviceops.Result, error) { return client.Stop(context.Background(), "bmc-a") },
		"pull":    func() (serviceops.Result, error) { return client.Pull(context.Background(), "bmc-a") },
	} {
		result, err := fn()
		if err == nil {
			t.Fatalf("%s: expected HTTP error, got result %+v", name, result)
		}
	}
}

func TestDebugCommandPostsDebugEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/services/{service}/debug", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(serviceops.Result{
			Service: r.PathValue("service"),
			Status:  "success",
			Stage:   "debug",
			Debug: &serviceops.DebugAttach{
				Debugger: "dlv",
				Address:  "127.0.0.1:40001",
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var stdout, stderr strings.Builder
	code := Run([]string{"debug", "user-api", "--wait", "--json"}, &stdout, &stderr, func(key string) string {
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
	if result.Stage != "debug" || result.Debug == nil {
		t.Fatalf("unexpected debug result: %+v", result)
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

func TestLogsCommandAcceptsTailAfterServiceName(t *testing.T) {
	server := newTestDaemon(t)
	defer server.Close()

	var stdout, stderr strings.Builder
	code := Run([]string{"logs", "user-api", "--tail", "200"}, &stdout, &stderr, func(key string) string {
		if key == "ONESPACE_URL" {
			return server.URL
		}
		return ""
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "service line") {
		t.Fatalf("expected service log line in output, got: %s", stdout.String())
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

func TestWriteServicesTableDisplaysContainerKindWhenLanguageIsEmpty(t *testing.T) {
	var stdout strings.Builder
	err := WriteServicesTable(&stdout, []ServiceSummary{
		{Name: "bmc-a", Kind: "container", Image: "metal-forge/mock-ipmi:dev"},
	})
	if err != nil {
		t.Fatalf("WriteServicesTable: %v", err)
	}
	if !strings.Contains(stdout.String(), "container") {
		t.Fatalf("expected container kind in output, got: %s", stdout.String())
	}
}

func TestConfigCommandPrintsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/services/user-api/config" {
			t.Fatalf("path = %q, want /api/services/user-api/config", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"user-api","env":[{"name":"DB_PASSWORD","value":"******","source":".secrets/db_password","secret":true}],"files":[],"volumes":[],"dependsOn":[]}`))
	}))
	defer server.Close()

	var stdout, stderr strings.Builder
	code := Run([]string{"config", "user-api", "--json"}, &stdout, &stderr, func(key string) string {
		if key == "ONESPACE_URL" {
			return server.URL
		}
		return ""
	})

	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"DB_PASSWORD"`) || !strings.Contains(stdout.String(), `"******"`) {
		t.Fatalf("stdout = %s, want redacted config JSON", stdout.String())
	}
}
