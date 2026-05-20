package serviceops

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wnzhone/onespace/internal/domain"
	"github.com/wnzhone/onespace/internal/gitx"
	"github.com/wnzhone/onespace/internal/health"
	"github.com/wnzhone/onespace/internal/jobs"
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

type recordingJobStore struct {
	created []jobs.Job
	updated []jobs.Job
}

func (s *recordingJobStore) Create(ctx context.Context, job jobs.Job) error {
	s.created = append(s.created, job)
	return nil
}

func (s *recordingJobStore) Update(ctx context.Context, job jobs.Job) error {
	s.updated = append(s.updated, job)
	return nil
}

func (s *recordingJobStore) Get(ctx context.Context, id string) (jobs.Job, error) {
	return jobs.Job{}, errors.New("not implemented")
}

func (s *recordingJobStore) List(ctx context.Context, workspace string, limit int) ([]jobs.Job, error) {
	return nil, errors.New("not implemented")
}

func TestDeployRunsThroughJobRunnerAndPersistsResult(t *testing.T) {
	ws := testWorkspace()
	store := &recordingJobStore{}
	fakeGitSvc := &fakeGit{
		status: gitx.Status{Commit: "abc123", Branch: "main"},
	}

	mgr := &Manager{
		Workspace: ws,
		Git:       fakeGitSvc,
		Runtime:   &runtime.FakeRuntime{},
		Health:    health.Checker{},
		Jobs:      jobs.NewRunner(store),
	}

	result, err := mgr.Deploy(context.Background(), "user-api")
	if err != nil {
		t.Fatalf("Deploy error: %v", err)
	}
	if result.JobID == "" {
		t.Fatal("Deploy result missing JobID")
	}
	if len(store.created) != 1 || len(store.updated) != 1 {
		t.Fatalf("job store writes = created %d updated %d, want 1/1", len(store.created), len(store.updated))
	}
	job := store.updated[0]
	if job.ID != result.JobID {
		t.Fatalf("updated job ID = %q, want result JobID %q", job.ID, result.JobID)
	}
	if job.Type != jobs.TypeDeploy || job.Status != jobs.StatusSuccess || job.Stage != "done" {
		t.Fatalf("unexpected updated job: %+v", job)
	}
	var persisted Result
	if err := json.Unmarshal(job.Result, &persisted); err != nil {
		t.Fatalf("unmarshal job result: %v", err)
	}
	if persisted.JobID != result.JobID || persisted.Service != "user-api" {
		t.Fatalf("persisted result = %+v, want jobID %q service user-api", persisted, result.JobID)
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

func TestDebugReportsJDWPForJavaMaven(t *testing.T) {
	ws := testWorkspace()
	ws.Services["order-api"] = domain.Service{
		Name:     "order-api",
		Language: "java-maven",
		RepoPath: "/tmp/repos/order-api",
		Workdir:  "/workspace",
		Image:    "onespace/java-dev:21-maven",
		Build:    domain.Command{Command: "mvn package -DskipTests"},
		Run:      domain.Command{Command: "java -jar target/*.jar"},
		Debug: domain.DebugConfig{
			Port:         40002,
			BuildCommand: "mvn package -DskipTests",
			Command:      "java -agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:40002 -jar target/*.jar",
		},
	}

	mgr := &Manager{
		Workspace: ws,
		Git:       &fakeGit{},
		Runtime:   &runtime.FakeRuntime{},
		Health:    health.Checker{},
	}

	result, err := mgr.Debug(context.Background(), "order-api")
	if err != nil {
		t.Fatalf("Debug error: %v", err)
	}
	if result.Debug == nil {
		t.Fatal("Debug attach should not be nil")
	}
	if result.Debug.Debugger != "jdwp" {
		t.Fatalf("Debugger = %q, want jdwp", result.Debug.Debugger)
	}
	if result.Debug.Address != "127.0.0.1:40002" {
		t.Fatalf("Address = %q, want 127.0.0.1:40002", result.Debug.Address)
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

func TestDeployContainerServiceStartsComposeServiceWithoutGitOrBuild(t *testing.T) {
	ws := testWorkspace()
	ws.Services["bmc-a"] = domain.Service{
		Name:  "bmc-a",
		Kind:  "container",
		Image: "metal-forge/mock-ipmi:dev",
		Ports: []domain.Port{
			{Name: "ipmi", Container: 623, Host: 6230, Protocol: "udp"},
		},
	}
	delete(ws.Services, "user-api")

	var calls []string
	fakeRT := &runtime.FakeRuntime{
		UpServiceFunc: func(ctx context.Context, workspaceRoot string, service string) error {
			calls = append(calls, "up:"+service)
			return nil
		},
		ExecFunc: func(ctx context.Context, opts runtime.ExecOptions) error {
			t.Fatalf("container deploy should not exec build command: %+v", opts)
			return nil
		},
	}
	mgr := &Manager{
		Workspace: ws,
		Git:       &fakeGit{statusErr: errors.New("git should not be called")},
		Runtime:   fakeRT,
		Health:    health.Checker{},
	}

	result, err := mgr.Deploy(context.Background(), "bmc-a")
	if err != nil {
		t.Fatalf("Deploy error: %v", err)
	}
	if result.Status != "success" || result.Stage != "done" || result.Container != "running" {
		t.Fatalf("result = %+v, want successful running container", result)
	}
	if strings.Join(calls, ",") != "up:bmc-a" {
		t.Fatalf("calls = %v, want only compose up", calls)
	}
}

func TestRestartAndStopContainerServiceUseComposeServiceLifecycle(t *testing.T) {
	ws := testWorkspace()
	ws.Services["bmc-a"] = domain.Service{Name: "bmc-a", Kind: "container", Image: "metal-forge/mock-ipmi:dev"}
	delete(ws.Services, "user-api")

	var calls []string
	fakeRT := &runtime.FakeRuntime{
		RestartServiceFunc: func(ctx context.Context, workspaceRoot string, service string) error {
			calls = append(calls, "restart:"+service)
			return nil
		},
		StopServiceFunc: func(ctx context.Context, workspaceRoot string, service string) error {
			calls = append(calls, "stop:"+service)
			return nil
		},
		StopProcessFunc: func(ctx context.Context, workspaceRoot string, service string) error {
			t.Fatalf("container lifecycle should not call supervisor stop")
			return nil
		},
	}
	mgr := &Manager{Workspace: ws, Runtime: fakeRT}

	restartResult, err := mgr.Restart(context.Background(), "bmc-a")
	if err != nil {
		t.Fatalf("Restart error: %v", err)
	}
	stopResult, err := mgr.Stop(context.Background(), "bmc-a")
	if err != nil {
		t.Fatalf("Stop error: %v", err)
	}

	if restartResult.Stage != "restart" || stopResult.Stage != "stop" {
		t.Fatalf("restart=%+v stop=%+v, want restart/stop stages", restartResult, stopResult)
	}
	if strings.Join(calls, ",") != "restart:bmc-a,stop:bmc-a" {
		t.Fatalf("calls = %v, want compose restart then stop", calls)
	}
}

func TestBuildDebugAndPullAreUnsupportedForContainerService(t *testing.T) {
	ws := testWorkspace()
	ws.Services["bmc-a"] = domain.Service{Name: "bmc-a", Kind: "container", Image: "metal-forge/mock-ipmi:dev"}
	delete(ws.Services, "user-api")
	mgr := &Manager{Workspace: ws, Runtime: &runtime.FakeRuntime{}, Git: &fakeGit{}}

	for name, fn := range map[string]func(context.Context, string) (Result, error){
		"build": mgr.Build,
		"debug": mgr.Debug,
		"pull":  mgr.Pull,
	} {
		result, err := fn(context.Background(), "bmc-a")
		if err == nil {
			t.Fatalf("%s: expected unsupported error", name)
		}
		if result.Status != "failed" || result.Stage != "unsupported" {
			t.Fatalf("%s: result = %+v, want failed unsupported", name, result)
		}
	}
}

func TestContainerServiceLogsComeFromRuntime(t *testing.T) {
	ws := testWorkspace()
	ws.Services["bmc-a"] = domain.Service{Name: "bmc-a", Kind: "container", Image: "metal-forge/mock-ipmi:dev"}
	delete(ws.Services, "user-api")
	fakeRT := &runtime.FakeRuntime{
		ServiceLogsFunc: func(ctx context.Context, workspaceRoot string, service string, tail int) ([]string, error) {
			if service != "bmc-a" || tail != 2 {
				t.Fatalf("service=%q tail=%d, want bmc-a tail 2", service, tail)
			}
			return []string{"serving mock IPMI", "power=off"}, nil
		},
	}
	mgr := &Manager{Workspace: ws, Runtime: fakeRT}

	lines, err := mgr.ServiceLogs(context.Background(), "bmc-a", 2)
	if err != nil {
		t.Fatalf("ServiceLogs error: %v", err)
	}
	if strings.Join(lines, "|") != "serving mock IPMI|power=off" {
		t.Fatalf("lines = %v, want container logs", lines)
	}
}
