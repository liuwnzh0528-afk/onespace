package serviceops

import (
	"context"
	"errors"
	"testing"

	"github.com/wnzhone/onespace/internal/domain"
	"github.com/wnzhone/onespace/internal/gitx"
	"github.com/wnzhone/onespace/internal/health"
	"github.com/wnzhone/onespace/internal/runtime"
)

type fakeGit struct {
	status     gitx.Status
	statusErr  error
	pullResult gitx.PullResult
	pullErr    error
}

func (f *fakeGit) Status(ctx context.Context, repoPath string) (gitx.Status, error) {
	return f.status, f.statusErr
}

func (f *fakeGit) PullFastForwardOnly(ctx context.Context, repoPath string) (gitx.PullResult, error) {
	return f.pullResult, f.pullErr
}

func testWorkspace() domain.Workspace {
	return domain.Workspace{
		Name: "test-ws",
		Path: "/tmp/test-ws",
		Runtime: domain.RuntimeConfig{
			Network: "test-ws-default",
		},
		Services: map[string]domain.Service{
			"user-api": {
				Name:     "user-api",
				Language: "go",
				RepoPath: "/tmp/repos/user-api",
				Workdir:  "/workspace",
				Image:    "onespace/go-dev:1.23",
				Build:    domain.Command{Command: "go build ./cmd/user-api"},
				Run:      domain.Command{Command: "/workspace/.onespace/bin/app"},
				Debug: domain.DebugConfig{
					Port:         40001,
					BuildCommand: "go build -gcflags=\"all=-N -l\" ./cmd/user-api",
					Command:      "dlv exec /workspace/.onespace/bin/app --headless --listen=:40001",
				},
				Health: domain.HealthCheck{
					Type:           "http",
					URL:            "http://127.0.0.1:18081/health",
					TimeoutSeconds: 5,
				},
			},
		},
	}
}

func TestDeployBuildsRestartsAndChecksHealth(t *testing.T) {
	ws := testWorkspace()
	fakeRT := &runtime.FakeRuntime{}
	fakeGitSvc := &fakeGit{
		status: gitx.Status{Commit: "abc123", Branch: "main"},
	}

	mgr := &Manager{
		Workspace: ws,
		Git:       fakeGitSvc,
		Runtime:   fakeRT,
		Health:    health.Checker{},
	}

	result, err := mgr.Deploy(context.Background(), "user-api")
	if err != nil {
		t.Fatalf("Deploy error: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("Status = %q, want success", result.Status)
	}
	if result.Stage != "done" {
		t.Fatalf("Stage = %q, want done", result.Stage)
	}
	if result.Commit != "abc123" {
		t.Fatalf("Commit = %q, want abc123", result.Commit)
	}
}

func TestDeployReturnsBuildStageOnBuildFailure(t *testing.T) {
	ws := testWorkspace()
	fakeRT := &runtime.FakeRuntime{
		ExecFunc: func(ctx context.Context, opts runtime.ExecOptions) error {
			return errors.New("build failed")
		},
	}
	fakeGitSvc := &fakeGit{
		status: gitx.Status{Commit: "abc123"},
	}

	mgr := &Manager{
		Workspace: ws,
		Git:       fakeGitSvc,
		Runtime:   fakeRT,
		Health:    health.Checker{},
	}

	result, err := mgr.Deploy(context.Background(), "user-api")
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Stage != "build" {
		t.Fatalf("Stage = %q, want build", result.Stage)
	}
}

func TestDebugUsesDebugBuildWhenConfigured(t *testing.T) {
	ws := testWorkspace()
	fakeRT := &runtime.FakeRuntime{}

	mgr := &Manager{
		Workspace: ws,
		Git:       &fakeGit{},
		Runtime:   fakeRT,
		Health:    health.Checker{},
	}

	result, err := mgr.Debug(context.Background(), "user-api")
	if err != nil {
		t.Fatalf("Debug error: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("Status = %q, want success", result.Status)
	}
	if result.Debug == nil {
		t.Fatal("Debug attach should not be nil")
	}
	if result.Debug.Debugger != "dlv" {
		t.Fatalf("Debugger = %q, want dlv", result.Debug.Debugger)
	}
	if result.Debug.Address != "127.0.0.1:40001" {
		t.Fatalf("Address = %q, want 127.0.0.1:40001", result.Debug.Address)
	}
}

func TestPullRefusesDirtyRepo(t *testing.T) {
	ws := testWorkspace()
	fakeGitSvc := &fakeGit{
		pullResult: gitx.PullResult{},
		pullErr:    errors.New("refusing pull: working tree is dirty"),
	}

	mgr := &Manager{
		Workspace: ws,
		Git:       fakeGitSvc,
		Runtime:   &runtime.FakeRuntime{},
		Health:    health.Checker{},
	}

	_, err := mgr.Pull(context.Background(), "user-api")
	if err == nil {
		t.Fatal("expected error for dirty repo")
	}
}
