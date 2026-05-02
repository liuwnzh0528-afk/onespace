package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wnzhone/onespace/internal/domain"
	"github.com/wnzhone/onespace/internal/health"
	"github.com/wnzhone/onespace/internal/jobs"
	"github.com/wnzhone/onespace/internal/logs"
	"github.com/wnzhone/onespace/internal/serviceops"
)

type fakeOps struct {
	deployResult serviceops.Result
	deployErr    error
}

func (f *fakeOps) Deploy(_ context.Context, service string) (serviceops.Result, error) {
	return f.deployResult, f.deployErr
}
func (f *fakeOps) Debug(_ context.Context, service string) (serviceops.Result, error) {
	return f.deployResult, f.deployErr
}
func (f *fakeOps) Pull(_ context.Context, service string) (serviceops.Result, error) {
	return f.deployResult, f.deployErr
}
func (f *fakeOps) Build(_ context.Context, service string) (serviceops.Result, error) {
	return f.deployResult, f.deployErr
}
func (f *fakeOps) Restart(_ context.Context, service string) (serviceops.Result, error) {
	return f.deployResult, f.deployErr
}
func (f *fakeOps) Stop(_ context.Context, service string) (serviceops.Result, error) {
	return f.deployResult, f.deployErr
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	ws := domain.Workspace{
		Name: "test-ws",
		Path: "/tmp/test-ws",
		Services: map[string]domain.Service{
			"user-api": {
				Name:     "user-api",
				Language: "go",
				Image:    "onespace/go-dev:1.23",
				Health: domain.HealthCheck{
					Type: "http",
					URL:  "http://127.0.0.1:18081/health",
				},
			},
		},
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := jobs.OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	return NewServer(
		ws,
		&fakeOps{deployResult: serviceops.Result{Service: "user-api", Status: "success", Stage: "done"}},
		store,
		logs.Store{Root: dir},
		health.Checker{},
		NewEventBroker(),
	)
}

func TestGetServicesReturnsWorkspaceServices(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/services", nil)
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var summaries []serviceSummary
	if err := json.NewDecoder(w.Body).Decode(&summaries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Name != "user-api" {
		t.Fatalf("unexpected services: %+v", summaries)
	}
}

func TestPostDeployReturnsJobResult(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/services/user-api/deploy", nil)
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result serviceops.Result
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Service != "user-api" || result.Status != "success" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGetJobLogsReturnsTail(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/job-123/logs", nil)
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["lines"]; !ok {
		t.Fatal("response missing 'lines' key")
	}
}

func TestEventsStreamsJobEvents(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.Mux.ServeHTTP(w, req)
		close(done)
	}()

	cancel()
	<-done
}

func TestStaticIndexServedAtRoot(t *testing.T) {
	staticDir := t.TempDir()
	indexHTML := `<!doctype html><html><head><title>Onespace</title></head><body><h1>Onespace</h1></body></html>`
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte(indexHTML), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := jobs.OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	srv := &Server{
		Workspace: domain.Workspace{Name: "test-ws", Path: "/tmp/test-ws"},
		Ops:       &fakeOps{deployResult: serviceops.Result{Service: "user-api", Status: "success", Stage: "done"}},
		JobStore:  store,
		Logs:      logs.Store{Root: dir},
		Health:    health.Checker{},
		Events:    NewEventBroker(),
		StaticDir: staticDir,
		Mux:       http.NewServeMux(),
	}
	srv.registerRoutes()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "Onespace") {
		t.Fatalf("response body does not contain Onespace: %s", w.Body.String())
	}
}
