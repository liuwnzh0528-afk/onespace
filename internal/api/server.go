package api

import (
	"context"
	"net/http"

	"github.com/wnzhone/onespace/internal/domain"
	"github.com/wnzhone/onespace/internal/health"
	"github.com/wnzhone/onespace/internal/jobs"
	"github.com/wnzhone/onespace/internal/logs"
	"github.com/wnzhone/onespace/internal/serviceops"
)

type Operations interface {
	Deploy(ctx context.Context, service string) (serviceops.Result, error)
	Debug(ctx context.Context, service string) (serviceops.Result, error)
	Pull(ctx context.Context, service string) (serviceops.Result, error)
	Build(ctx context.Context, service string) (serviceops.Result, error)
	Restart(ctx context.Context, service string) (serviceops.Result, error)
	Stop(ctx context.Context, service string) (serviceops.Result, error)
}

type Server struct {
	Workspace domain.Workspace
	Ops       Operations
	JobStore  jobs.Store
	Logs      logs.Store
	Health    health.Checker
	Events    *EventBroker
	StaticDir string
	Mux       *http.ServeMux
}

func NewServer(workspace domain.Workspace, ops Operations, jobStore jobs.Store, logStore logs.Store, healthChecker health.Checker, events *EventBroker, staticDir string) *Server {
	s := &Server{
		Workspace: workspace,
		Ops:       ops,
		JobStore:  jobStore,
		Logs:      logStore,
		Health:    healthChecker,
		Events:    events,
		StaticDir: staticDir,
		Mux:       http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.Mux.HandleFunc("GET /api/services", s.handleGetServices)
	s.Mux.HandleFunc("GET /api/services/{service}", s.handleGetService)
	s.Mux.HandleFunc("GET /api/services/{service}/config", s.handleGetServiceConfig)
	s.Mux.HandleFunc("GET /api/services/{service}/logs", s.handleGetServiceLogs)
	s.Mux.HandleFunc("GET /api/services/{service}/health", s.handleGetServiceHealth)
	s.Mux.HandleFunc("POST /api/services/{service}/pull", s.handlePostPull)
	s.Mux.HandleFunc("POST /api/services/{service}/build", s.handlePostBuild)
	s.Mux.HandleFunc("POST /api/services/{service}/restart", s.handlePostRestart)
	s.Mux.HandleFunc("POST /api/services/{service}/deploy", s.handlePostDeploy)
	s.Mux.HandleFunc("POST /api/services/{service}/debug", s.handlePostDebug)
	s.Mux.HandleFunc("POST /api/services/{service}/stop", s.handlePostStop)
	s.Mux.HandleFunc("GET /api/jobs", s.handleGetJobs)
	s.Mux.HandleFunc("GET /api/jobs/{jobId}", s.handleGetJob)
	s.Mux.HandleFunc("GET /api/jobs/{jobId}/logs", s.handleGetJobLogs)
	if s.Events != nil {
		s.Mux.Handle("GET /api/events", s.Events)
	}
	if s.StaticDir != "" {
		s.Mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(s.StaticDir))))
		s.Mux.HandleFunc("GET /", s.serveIndex)
	}
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, s.StaticDir+"/index.html")
}

func (s *Server) Handler() http.Handler {
	return s.Mux
}
