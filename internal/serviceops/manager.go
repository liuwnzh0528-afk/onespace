package serviceops

import (
	"context"
	"fmt"

	"github.com/wnzhone/onespace/internal/domain"
	"github.com/wnzhone/onespace/internal/gitx"
	"github.com/wnzhone/onespace/internal/health"
	"github.com/wnzhone/onespace/internal/jobs"
	"github.com/wnzhone/onespace/internal/logs"
	"github.com/wnzhone/onespace/internal/runtime"
)

type GitService interface {
	Status(ctx context.Context, repoPath string) (gitx.Status, error)
	PullFastForwardOnly(ctx context.Context, repoPath string) (gitx.PullResult, error)
}

type Manager struct {
	Workspace domain.Workspace
	Git       GitService
	Runtime   runtime.Runtime
	Health    health.Checker
	Jobs      *jobs.Runner
	Logs      logs.Store
}

func (m *Manager) Deploy(ctx context.Context, service string) (Result, error) {
	svc, ok := m.Workspace.Services[service]
	if !ok {
		return Result{Service: service, Status: "failed", Stage: "validate"}, fmt.Errorf("service %q not found", service)
	}

	// git-status
	gitStatus, err := m.Git.Status(ctx, svc.RepoPath)
	if err != nil {
		return Result{Service: service, Status: "failed", Stage: "git-status"}, err
	}

	// ensure container
	if err := m.Runtime.Ensure(ctx, m.Workspace.Path); err != nil {
		return Result{Service: service, Status: "failed", Stage: "ensure-container"}, err
	}

	// build
	if err := m.Runtime.Exec(ctx, runtime.ExecOptions{
		WorkspaceRoot: m.Workspace.Path,
		Service:       service,
		Command:       svc.Build.Command,
	}); err != nil {
		return Result{Service: service, Status: "failed", Stage: "build", ExitCode: 1}, err
	}

	// stop process
	_ = m.Runtime.StopProcess(ctx, m.Workspace.Path, service)

	// start process
	if err := m.Runtime.StartProcess(ctx, m.Workspace.Path, service, svc.Run.Command); err != nil {
		return Result{Service: service, Status: "failed", Stage: "start-process"}, err
	}

	// health check
	var healthResult health.Result
	if svc.Health.Type != "" {
		healthResult = m.Health.Check(ctx, svc.Health)
	}

	result := Result{
		Service: service,
		Status:  "success",
		Stage:   "done",
		Commit:  gitStatus.Commit,
		Dirty:   gitStatus.Dirty,
		Health:  healthResult.Status,
		URL:     svc.Health.URL,
	}

	return result, nil
}

func (m *Manager) Debug(ctx context.Context, service string) (Result, error) {
	svc, ok := m.Workspace.Services[service]
	if !ok {
		return Result{Service: service, Status: "failed"}, fmt.Errorf("service %q not found", service)
	}

	if err := m.Runtime.Ensure(ctx, m.Workspace.Path); err != nil {
		return Result{Service: service, Status: "failed", Stage: "ensure-container"}, err
	}

	debugBuild := svc.Debug.BuildCommand
	if debugBuild == "" {
		debugBuild = svc.Build.Command
	}
	if err := m.Runtime.Exec(ctx, runtime.ExecOptions{
		WorkspaceRoot: m.Workspace.Path,
		Service:       service,
		Command:       debugBuild,
	}); err != nil {
		return Result{Service: service, Status: "failed", Stage: "build"}, err
	}

	_ = m.Runtime.StopProcess(ctx, m.Workspace.Path, service)

	if err := m.Runtime.StartProcess(ctx, m.Workspace.Path, service, svc.Debug.Command); err != nil {
		return Result{Service: service, Status: "failed", Stage: "start-process"}, err
	}

	debugAddr := fmt.Sprintf("127.0.0.1:%d", svc.Debug.Port)
	return Result{
		Service: service,
		Status:  "success",
		Stage:   "debug",
		Debug: &DebugAttach{
			Debugger: "dlv",
			Address:  debugAddr,
		},
	}, nil
}

func (m *Manager) Pull(ctx context.Context, service string) (Result, error) {
	svc, ok := m.Workspace.Services[service]
	if !ok {
		return Result{Service: service, Status: "failed"}, fmt.Errorf("service %q not found", service)
	}

	pullResult, err := m.Git.PullFastForwardOnly(ctx, svc.RepoPath)
	if err != nil {
		return Result{
			Service: service,
			Status:  "failed",
			Stage:   "pull",
			Dirty:   pullResult.Status.Dirty,
			Commit:  pullResult.Status.Commit,
		}, err
	}

	return Result{
		Service: service,
		Status:  "success",
		Stage:   "pull",
		Commit:  pullResult.Status.Commit,
		Dirty:   pullResult.Status.Dirty,
	}, nil
}

func (m *Manager) Build(ctx context.Context, service string) (Result, error) {
	svc, ok := m.Workspace.Services[service]
	if !ok {
		return Result{Service: service, Status: "failed"}, fmt.Errorf("service %q not found", service)
	}

	if err := m.Runtime.Ensure(ctx, m.Workspace.Path); err != nil {
		return Result{Service: service, Status: "failed", Stage: "ensure-container"}, err
	}

	if err := m.Runtime.Exec(ctx, runtime.ExecOptions{
		WorkspaceRoot: m.Workspace.Path,
		Service:       service,
		Command:       svc.Build.Command,
	}); err != nil {
		return Result{Service: service, Status: "failed", Stage: "build"}, err
	}

	return Result{Service: service, Status: "success", Stage: "build"}, nil
}

func (m *Manager) Restart(ctx context.Context, service string) (Result, error) {
	svc, ok := m.Workspace.Services[service]
	if !ok {
		return Result{Service: service, Status: "failed"}, fmt.Errorf("service %q not found", service)
	}

	_ = m.Runtime.StopProcess(ctx, m.Workspace.Path, service)

	if err := m.Runtime.StartProcess(ctx, m.Workspace.Path, service, svc.Run.Command); err != nil {
		return Result{Service: service, Status: "failed", Stage: "restart"}, err
	}

	return Result{Service: service, Status: "success", Stage: "restart"}, nil
}

func (m *Manager) Stop(ctx context.Context, service string) (Result, error) {
	if err := m.Runtime.StopProcess(ctx, m.Workspace.Path, service); err != nil {
		return Result{Service: service, Status: "failed", Stage: "stop"}, err
	}
	return Result{Service: service, Status: "success", Stage: "stop"}, nil
}
