package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/wnzhone/onespace/internal/jobs"
	"github.com/wnzhone/onespace/internal/logs"
)

type serviceSummary struct {
	Name     string `json:"name"`
	Language string `json:"language"`
	Image    string `json:"image"`
	Branch   string `json:"branch,omitempty"`
	Commit   string `json:"commit,omitempty"`
	Health   string `json:"health,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleGetServices(w http.ResponseWriter, r *http.Request) {
	summaries := make([]serviceSummary, 0, len(s.Workspace.Services))
	for name, svc := range s.Workspace.Services {
		summary := serviceSummary{
			Name:     name,
			Language: svc.Language,
			Image:    svc.Image,
		}
		summaries = append(summaries, summary)
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("service")
	svc, ok := s.Workspace.Services[name]
	if !ok {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleGetServiceHealth(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("service")
	svc, ok := s.Workspace.Services[name]
	if !ok {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	if svc.Health.Type == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "unknown"})
		return
	}
	result := s.Health.Check(r.Context(), svc.Health)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePostDeploy(w http.ResponseWriter, r *http.Request) {
	result, err := s.Ops.Deploy(r.Context(), r.PathValue("service"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, result)
		return
	}
	if s.Events != nil {
		s.Events.Publish(Event{Type: "job", Data: result})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePostPull(w http.ResponseWriter, r *http.Request) {
	result, err := s.Ops.Pull(r.Context(), r.PathValue("service"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePostBuild(w http.ResponseWriter, r *http.Request) {
	result, err := s.Ops.Build(r.Context(), r.PathValue("service"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePostRestart(w http.ResponseWriter, r *http.Request) {
	result, err := s.Ops.Restart(r.Context(), r.PathValue("service"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePostDebug(w http.ResponseWriter, r *http.Request) {
	result, err := s.Ops.Debug(r.Context(), r.PathValue("service"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePostStop(w http.ResponseWriter, r *http.Request) {
	result, err := s.Ops.Stop(r.Context(), r.PathValue("service"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetJobs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	jobList, err := s.JobStore.List(r.Context(), s.Workspace.Name, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if jobList == nil {
		jobList = []jobs.Job{}
	}
	writeJSON(w, http.StatusOK, jobList)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.JobStore.Get(r.Context(), r.PathValue("jobId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleGetJobLogs(w http.ResponseWriter, r *http.Request) {
	tail := 200
	if t := r.URL.Query().Get("tail"); t != "" {
		if n, err := strconv.Atoi(t); err == nil {
			tail = n
		}
	}
	lines, err := s.Logs.ReadJobTail(r.Context(), r.PathValue("jobId"), tail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if lines == nil {
		lines = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"lines": lines})
}

func (s *Server) handleGetServiceLogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("service")
	svc, ok := s.Workspace.Services[name]
	if !ok {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	tail := 200
	if t := r.URL.Query().Get("tail"); t != "" {
		if n, err := strconv.Atoi(t); err == nil {
			tail = n
		}
	}
	lines, err := s.Logs.ReadServiceTail(r.Context(), name, tail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(lines) == 0 && svc.RepoPath != "" {
		runnerLines, err := logs.ReadFileTail(filepath.Join(svc.RepoPath, ".onespace", "service.log"), tail)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		lines = runnerLines
	}
	if lines == nil {
		lines = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"lines": lines})
}
