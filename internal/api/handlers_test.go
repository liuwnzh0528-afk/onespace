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
	logLines     []string
	logErr       error
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
func (f *fakeOps) ServiceLogs(_ context.Context, service string, tail int) ([]string, error) {
	return f.logLines, f.logErr
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
		"",
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

func TestGetServiceLogsReturnsTail(t *testing.T) {
	srv := newTestServer(t)
	if err := srv.Logs.AppendService(context.Background(), "user-api", []byte("line 1\nline 2\n")); err != nil {
		t.Fatalf("AppendService: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/services/user-api/logs?tail=1", nil)
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Lines []string `json:"lines"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Lines) != 1 || resp.Lines[0] != "line 2" {
		t.Fatalf("lines = %v, want [line 2]", resp.Lines)
	}
}

func TestGetServiceLogsFallsBackToRunnerLog(t *testing.T) {
	srv := newTestServer(t)
	repoPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoPath, ".onespace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".onespace", "service.log"), []byte("runner line 1\nrunner line 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := srv.Workspace.Services["user-api"]
	svc.RepoPath = repoPath
	srv.Workspace.Services["user-api"] = svc

	req := httptest.NewRequest(http.MethodGet, "/api/services/user-api/logs?tail=1", nil)
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Lines []string `json:"lines"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Lines) != 1 || resp.Lines[0] != "runner line 2" {
		t.Fatalf("lines = %v, want [runner line 2]", resp.Lines)
	}
}

func TestGetContainerServiceLogsFallsBackToRuntimeLogs(t *testing.T) {
	srv := newTestServer(t)
	svc := srv.Workspace.Services["user-api"]
	svc.Kind = "container"
	svc.Language = ""
	svc.RepoPath = ""
	svc.Image = "metal-forge/mock-ipmi:dev"
	delete(srv.Workspace.Services, "user-api")
	srv.Workspace.Services["bmc-a"] = svc
	srv.Ops = &fakeOps{logLines: []string{"serving mock IPMI", "power=off"}}

	req := httptest.NewRequest(http.MethodGet, "/api/services/bmc-a/logs?tail=2", nil)
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Lines []string `json:"lines"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if strings.Join(resp.Lines, "|") != "serving mock IPMI|power=off" {
		t.Fatalf("lines = %v, want runtime container logs", resp.Lines)
	}
}

func TestGetServiceConfigReturnsRedactedConfigInspector(t *testing.T) {
	srv := newTestServer(t)
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "db_password")
	if err := os.WriteFile(secretPath, []byte("super-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := srv.Workspace.Services["user-api"]
	svc.Workdir = "/workspace"
	svc.Env = map[string]string{"APP_ENV": "local"}
	svc.Secrets = []domain.SecretEnv{{Name: "DB_PASSWORD", FromFile: secretPath}}
	srv.Workspace.Services["user-api"] = svc

	req := httptest.NewRequest(http.MethodGet, "/api/services/user-api/config", nil)
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Service string `json:"service"`
		Env     []struct {
			Name   string `json:"name"`
			Value  string `json:"value"`
			Secret bool   `json:"secret"`
		} `json:"env"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Service != "user-api" {
		t.Fatalf("Service = %q, want user-api", resp.Service)
	}
	foundSecret := false
	for _, entry := range resp.Env {
		if entry.Name == "DB_PASSWORD" {
			foundSecret = true
			if entry.Value != "******" || !entry.Secret {
				t.Fatalf("DB_PASSWORD entry = %+v, want redacted secret", entry)
			}
		}
	}
	if !foundSecret {
		t.Fatal("response missing DB_PASSWORD entry")
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
