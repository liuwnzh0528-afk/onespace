# Onespace MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first working Onespace MVP: a single-user daemon with HTTP API, CLI, static Web UI, local Git status/pull, Docker Compose dev-runner orchestration, container-side Go/Java build/run/debug, logs, jobs, and health checks.

**Architecture:** Use a Go monorepo with one shared domain core, one daemon binary, and one CLI binary. The daemon owns workspace config, job state, logs, Docker Compose execution, Git operations, and static Web UI serving; the CLI and future MCP layer call the daemon API instead of duplicating logic. Runtime integration uses Docker Compose CLI and controlled command execution, with tests covering config, Git, job orchestration, API contracts, CLI JSON output, and generated Compose files.

**Tech Stack:** Go 1.22+, standard library HTTP server, `gopkg.in/yaml.v3`, `modernc.org/sqlite`, Docker Compose CLI, POSIX shell for runner helper scripts, static HTML/CSS/JavaScript for Web UI.

---

## Scope And Milestones

This spec spans several subsystems, so the implementation is split into independently testable milestones:

1. Foundation: Go module, config loader, domain types.
2. Local state: SQLite-backed jobs and file-backed logs.
3. Local code integration: Git status and conservative pull.
4. Runtime: Compose generation and controlled Docker command adapter.
5. Service operations: build, restart, deploy, debug, health checks.
6. Interfaces: HTTP API, CLI, SSE events, static Web UI.
7. Packaging aids: dev runner images, sample workspace, user docs.

The earliest useful checkpoint is Task 8: `onespace deploy <service> --wait --json` works against a fake runtime in tests and against Docker in an integration environment. Web UI arrives after the API and CLI are stable.

## File Structure

Create this structure:

```text
go.mod
go.sum

cmd/
  onespace-server/
    main.go
  onespace/
    main.go

internal/
  api/
    handlers.go
    handlers_test.go
    server.go
    sse.go
  cli/
    client.go
    commands.go
    commands_test.go
    output.go
  config/
    defaults.go
    loader.go
    loader_test.go
    merge.go
    types.go
  domain/
    errors.go
    service.go
    workspace.go
  gitx/
    service.go
    service_test.go
    types.go
  health/
    checker.go
    checker_test.go
  jobs/
    runner.go
    runner_test.go
    store.go
    store_sqlite.go
    types.go
  logs/
    store.go
    store_test.go
    tail.go
  runtime/
    command.go
    compose.go
    compose_test.go
    fake.go
    supervisor.go
    types.go
  serviceops/
    manager.go
    manager_test.go
    result.go
  state/
    paths.go
    paths_test.go
  version/
    version.go
    version_test.go

deploy/
  images/
    go-dev/
      Dockerfile
    java-dev-maven/
      Dockerfile
  runner/
    onespace-supervisor.sh

examples/
  workspaces/
    order-system-dev/
      onespace.yaml

web/
  static/
    app.js
    index.html
    styles.css

docs/
  cli.md
  dev-runner-images.md
  local-development.md
```

Responsibilities:

- `internal/config`: parse and validate workspace/service YAML; merge workspace config, service-local config, and language defaults.
- `internal/domain`: shared service/workspace/error types with no I/O.
- `internal/gitx`: read local repo status and run `git pull --ff-only` when safe.
- `internal/jobs`: serialize mutating service actions and persist job metadata.
- `internal/logs`: write, tail, and stream daemon/job/service logs.
- `internal/runtime`: generate Compose files and run controlled Docker Compose / Docker commands.
- `internal/serviceops`: compose Git, runtime, job, logs, and health behavior into pull/build/restart/deploy/debug operations.
- `internal/api`: expose daemon HTTP API and SSE events.
- `internal/cli`: implement CLI command parsing, daemon client calls, and text/JSON output.
- `web/static`: static single-page operational UI.
- `deploy`: dev runner images and the container-side supervisor helper.

## Task 1: Repository Scaffold And Toolchain Gate

**Files:**
- Create: `go.mod`
- Create: `cmd/onespace-server/main.go`
- Create: `cmd/onespace/main.go`
- Create: `internal/version/version.go`
- Create: `internal/version/version_test.go`
- Create: `docs/local-development.md`

- [ ] **Step 1: Verify Go is available**

Run:

```bash
go version
```

Expected: prints Go 1.22 or newer, for example `go version go1.22.0 darwin/arm64`.

If the command is missing, install Go 1.22+ on the VM before starting implementation. The current planning environment returned `zsh:1: command not found: go`, so this is a real precondition.

- [ ] **Step 2: Create module**

Run:

```bash
go mod init github.com/wnzhone/onespace
```

Expected: `go.mod` is created with module `github.com/wnzhone/onespace`.

- [ ] **Step 3: Add version package test**

Create `internal/version/version_test.go`:

```go
package version

import "testing"

func TestInfoUsesDevDefaults(t *testing.T) {
	info := Info()
	if info.Name != "onespace" {
		t.Fatalf("Name = %q, want onespace", info.Name)
	}
	if info.Version == "" {
		t.Fatal("Version is empty")
	}
	if info.Commit == "" {
		t.Fatal("Commit is empty")
	}
}
```

- [ ] **Step 4: Add version package implementation**

Create `internal/version/version.go`:

```go
package version

var (
	Version = "dev"
	Commit  = "none"
)

type BuildInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func Info() BuildInfo {
	return BuildInfo{
		Name:    "onespace",
		Version: Version,
		Commit:  Commit,
	}
}
```

- [ ] **Step 5: Add binary entry points**

Create `cmd/onespace-server/main.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/wnzhone/onespace/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		_ = json.NewEncoder(os.Stdout).Encode(version.Info())
		return
	}
	fmt.Fprintln(os.Stderr, "onespace-server: no command specified")
	os.Exit(2)
}
```

Create `cmd/onespace/main.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/wnzhone/onespace/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		_ = json.NewEncoder(os.Stdout).Encode(version.Info())
		return
	}
	fmt.Fprintln(os.Stderr, "onespace: no command specified")
	os.Exit(2)
}
```

- [ ] **Step 6: Add local development doc**

Create `docs/local-development.md`:

```markdown
# Local Development

Onespace is implemented in Go. Install Go 1.22 or newer, Docker Engine, and Docker Compose v2 before running the daemon or integration tests.

Useful commands:

```bash
go test ./...
go run ./cmd/onespace-server version
go run ./cmd/onespace version
```
```

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 8: Commit**

Run:

```bash
git add go.mod cmd internal/version docs/local-development.md
git commit -m "chore: scaffold Go module"
```

## Task 2: Config Schema, Defaults, And Validation

**Files:**
- Create: `internal/domain/workspace.go`
- Create: `internal/domain/service.go`
- Create: `internal/domain/errors.go`
- Create: `internal/config/types.go`
- Create: `internal/config/defaults.go`
- Create: `internal/config/loader.go`
- Create: `internal/config/merge.go`
- Create: `internal/config/loader_test.go`
- Create: `examples/workspaces/order-system-dev/onespace.yaml`

- [ ] **Step 1: Add failing config loader tests**

Create `internal/config/loader_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWorkspaceAppliesDefaultsAndValidatesRepoRoots(t *testing.T) {
	dir := t.TempDir()
	repoRoot := filepath.Join(dir, "repos")
	if err := os.MkdirAll(filepath.Join(repoRoot, "user-api"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "onespace.yaml")
	yaml := `
version: 1
name: order-system-dev
allowedRepoRoots:
  - ` + repoRoot + `
services:
  user-api:
    language: go
    repoPath: ` + filepath.Join(repoRoot, "user-api") + `
    main: ./cmd/user-api
    ports:
      - name: http
        container: 8080
        host: 18081
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := LoadWorkspace(configPath)
	if err != nil {
		t.Fatalf("LoadWorkspace returned error: %v", err)
	}
	if ws.Name != "order-system-dev" {
		t.Fatalf("Name = %q", ws.Name)
	}
	svc := ws.Services["user-api"]
	if svc.Workdir != "/workspace" {
		t.Fatalf("Workdir = %q, want /workspace", svc.Workdir)
	}
	if svc.Image != "onespace/go-dev:1.23" {
		t.Fatalf("Image = %q", svc.Image)
	}
	if svc.Build.Command == "" || svc.Run.Command == "" || svc.Debug.Command == "" {
		t.Fatalf("language defaults were not applied: %+v", svc)
	}
}

func TestLoadWorkspaceRejectsRepoOutsideAllowedRoots(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "onespace.yaml")
	yaml := `
version: 1
name: bad
allowedRepoRoots:
  - ` + filepath.Join(dir, "repos") + `
services:
  bad-api:
    language: go
    repoPath: /tmp/outside-repo
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadWorkspace(configPath)
	if err == nil {
		t.Fatal("expected repo root validation error")
	}
}
```

- [ ] **Step 2: Add domain types**

Create `internal/domain/workspace.go`:

```go
package domain

type Workspace struct {
	Version          int
	Name             string
	Path             string
	AllowedRepoRoots []string
	Server           ServerConfig
	Runtime          RuntimeConfig
	Ports            PortRanges
	Services         map[string]Service
	Addons           map[string]Addon
}

type ServerConfig struct {
	Bind string
}

type RuntimeConfig struct {
	Type        string
	ProjectName string
	Network     string
}

type PortRanges struct {
	AppRange   string
	DebugRange string
}

type Addon struct {
	Image string
	Ports []string
	Env   map[string]string
}
```

Create `internal/domain/service.go`:

```go
package domain

type Service struct {
	Name     string
	Language string
	RepoPath string
	Workdir  string
	Image    string
	Main     string
	Ports    []Port
	Health   HealthCheck
	Build    Command
	Run      Command
	Debug    DebugConfig
}

type Port struct {
	Name      string
	Container int
	Host      int
}

type HealthCheck struct {
	Type           string
	URL            string
	TimeoutSeconds int
}

type Command struct {
	Command string
}

type DebugConfig struct {
	Port         int
	BuildCommand string
	Command      string
}
```

Create `internal/domain/errors.go`:

```go
package domain

import "fmt"

type ValidationError struct {
	Field  string
	Reason string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}
```

- [ ] **Step 3: Add config parsing types and defaults**

Create `internal/config/types.go` with YAML tags mirroring `domain.Workspace`.

Create `internal/config/defaults.go`:

```go
package config

import "fmt"

type languageDefaults struct {
	Workdir      string
	Image        string
	BuildCommand string
	RunCommand   string
	DebugBuild   string
	DebugCommand string
}

func defaultsForGo(main string, debugPort int) languageDefaults {
	if main == "" {
		main = "."
	}
	return languageDefaults{
		Workdir:      "/workspace",
		Image:        "onespace/go-dev:1.23",
		BuildCommand: fmt.Sprintf("go build -o /workspace/.onespace/bin/app %s", main),
		RunCommand:   "/workspace/.onespace/bin/app",
		DebugBuild:   fmt.Sprintf("go build -gcflags=\"all=-N -l\" -o /workspace/.onespace/bin/app %s", main),
		DebugCommand: fmt.Sprintf("dlv exec /workspace/.onespace/bin/app --headless --listen=:%d --api-version=2 --accept-multiclient --continue", debugPort),
	}
}

func defaultsForJavaMaven(debugPort int) languageDefaults {
	return languageDefaults{
		Workdir:      "/workspace",
		Image:        "onespace/java-dev:21-maven",
		BuildCommand: "mvn package -DskipTests",
		RunCommand:   "java -jar target/*.jar",
		DebugCommand: fmt.Sprintf("java -agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:%d -jar target/*.jar", debugPort),
	}
}
```

- [ ] **Step 4: Implement loader, merge, and validation**

Create `internal/config/loader.go` using `gopkg.in/yaml.v3` to read YAML, map to domain types, apply defaults, and validate repo roots.

Create `internal/config/merge.go` with helpers:

```go
package config

import (
	"path/filepath"
	"strings"

	"github.com/wnzhone/onespace/internal/domain"
)

func pathUnderAnyRoot(path string, roots []string) bool {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, root := range roots {
		cleanRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(cleanRoot, cleanPath)
		if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}

func setServiceName(name string, svc domain.Service) domain.Service {
	svc.Name = name
	return svc
}
```

- [ ] **Step 5: Add example workspace**

Create `examples/workspaces/order-system-dev/onespace.yaml` using the example from the design spec, with repo paths under `/data/workspaces/order-system-dev/repos`.

- [ ] **Step 6: Fetch dependencies and test**

Run:

```bash
go get gopkg.in/yaml.v3
go test ./internal/config ./internal/domain
```

Expected: config and domain tests pass.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/domain internal/config examples/workspaces/order-system-dev/onespace.yaml go.mod go.sum
git commit -m "feat: load workspace configuration"
```

## Task 3: Git Status And Conservative Pull

**Files:**
- Create: `internal/gitx/types.go`
- Create: `internal/gitx/service.go`
- Create: `internal/gitx/service_test.go`

- [ ] **Step 1: Add Git service tests**

Create tests that initialize a local repo and bare remote under `t.TempDir()`. The test must verify:

```go
func TestStatusReportsBranchCommitDirtyAndRemote(t *testing.T)
func TestPullRefusesDirtyWorkingTree(t *testing.T)
func TestPullUsesFastForwardOnly(t *testing.T)
```

Use helper functions in the test file:

```go
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
```

- [ ] **Step 2: Add Git status types**

Create `internal/gitx/types.go`:

```go
package gitx

type Status struct {
	RepoPath       string `json:"repoPath"`
	Remote         string `json:"remote"`
	Branch         string `json:"branch"`
	TrackingBranch string `json:"trackingBranch"`
	Commit         string `json:"commit"`
	Dirty          bool   `json:"dirty"`
	Ahead          int    `json:"ahead"`
	Behind         int    `json:"behind"`
	Detached       bool   `json:"detached"`
}

type PullResult struct {
	Status Status `json:"status"`
	Output string `json:"output"`
}
```

- [ ] **Step 3: Implement Git service**

Create `internal/gitx/service.go` with:

```go
type Service struct {
	Runner CommandRunner
}

type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

func (s Service) Status(ctx context.Context, repoPath string) (Status, error)
func (s Service) PullFastForwardOnly(ctx context.Context, repoPath string) (PullResult, error)
```

Implementation rules:

- Use `git -C <repoPath> rev-parse --is-inside-work-tree`.
- Use `git -C <repoPath> symbolic-ref --short HEAD` to detect detached HEAD.
- Use `git -C <repoPath> rev-parse HEAD` for commit.
- Use `git -C <repoPath> status --porcelain` for dirty.
- Use `git -C <repoPath> rev-parse --abbrev-ref --symbolic-full-name @{u}` for tracking branch.
- Use `git -C <repoPath> rev-list --left-right --count HEAD...@{u}` for ahead/behind.
- Refuse pull when dirty, detached, or no tracking branch.
- Run only `git -C <repoPath> pull --ff-only` for pull.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/gitx -v
```

Expected: all Git service tests pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/gitx
git commit -m "feat: add local git integration"
```

## Task 4: Job Model, Serialization, And SQLite Store

**Files:**
- Create: `internal/jobs/types.go`
- Create: `internal/jobs/store.go`
- Create: `internal/jobs/store_sqlite.go`
- Create: `internal/jobs/runner.go`
- Create: `internal/jobs/runner_test.go`

- [ ] **Step 1: Add runner tests**

Create tests:

```go
func TestRunnerSerializesMutatingJobsPerService(t *testing.T)
func TestRunnerAllowsReadOnlyJobsDuringMutatingJob(t *testing.T)
func TestSQLiteStorePersistsJobResult(t *testing.T)
```

The serialization test should enqueue two mutating jobs for `user-api`, block the first job on a channel, and assert the second job does not start until the first completes.

- [ ] **Step 2: Define job types**

Create `internal/jobs/types.go`:

```go
package jobs

import "time"

type Type string
type Status string
type Stage string

const (
	TypePull    Type = "pull"
	TypeBuild   Type = "build"
	TypeUp      Type = "up"
	TypeRestart Type = "restart"
	TypeDeploy  Type = "deploy"
	TypeDebug   Type = "debug"
	TypeStop    Type = "stop"
	TypeLogs    Type = "logs"
	TypeHealth  Type = "health"
)

const (
	StatusQueued   Status = "queued"
	StatusRunning  Status = "running"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

type Job struct {
	ID         string    `json:"id"`
	Type       Type      `json:"type"`
	Workspace  string    `json:"workspace"`
	Service    string    `json:"service"`
	Status     Status    `json:"status"`
	Stage      string    `json:"stage"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
	ExitCode   int       `json:"exitCode,omitempty"`
	LogRef     string    `json:"logRef,omitempty"`
	Result     []byte    `json:"result,omitempty"`
}
```

- [ ] **Step 3: Implement store interface and SQLite store**

Create `internal/jobs/store.go`:

```go
package jobs

import "context"

type Store interface {
	Create(ctx context.Context, job Job) error
	Update(ctx context.Context, job Job) error
	Get(ctx context.Context, id string) (Job, error)
	List(ctx context.Context, workspace string, limit int) ([]Job, error)
}
```

Create `internal/jobs/store_sqlite.go` using `database/sql` with `modernc.org/sqlite`. Add schema initialization in `OpenSQLiteStore(path string) (*SQLiteStore, error)`.

- [ ] **Step 4: Implement runner**

Create `internal/jobs/runner.go` with:

```go
type Runner struct {
	Store Store
	mu    sync.Mutex
	locks map[string]chan struct{}
}

func (r *Runner) Run(ctx context.Context, job Job, mutating bool, fn func(context.Context, *Job) error) (Job, error)
```

Lock key is `workspace + "/" + service` for mutating jobs. Read-only jobs skip that lock.

- [ ] **Step 5: Fetch SQLite dependency and test**

Run:

```bash
go get modernc.org/sqlite
go test ./internal/jobs -v
```

Expected: job tests pass.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/jobs go.mod go.sum
git commit -m "feat: persist and serialize jobs"
```

## Task 5: Log Storage And Tail Support

**Files:**
- Create: `internal/logs/store.go`
- Create: `internal/logs/tail.go`
- Create: `internal/logs/store_test.go`
- Create: `internal/state/paths.go`
- Create: `internal/state/paths_test.go`

- [ ] **Step 1: Add path and log tests**

Create tests for:

```go
func TestWorkspaceStatePathsStayUnderWorkspace(t *testing.T)
func TestLogStoreAppendsAndTailsLines(t *testing.T)
func TestLogStoreReturnsEmptyTailForMissingLog(t *testing.T)
```

- [ ] **Step 2: Implement state paths**

Create `internal/state/paths.go`:

```go
package state

import "path/filepath"

type Paths struct {
	WorkspaceRoot string
}

func (p Paths) StateDir() string {
	return filepath.Join(p.WorkspaceRoot, "state")
}

func (p Paths) JobsLogDir() string {
	return filepath.Join(p.StateDir(), "logs", "jobs")
}

func (p Paths) ServicesLogDir() string {
	return filepath.Join(p.StateDir(), "logs", "services")
}

func (p Paths) GeneratedDir() string {
	return filepath.Join(p.WorkspaceRoot, "generated")
}
```

- [ ] **Step 3: Implement log store**

Create `internal/logs/store.go` with append and reader methods:

```go
type Store struct {
	Root string
}

func (s Store) AppendJob(ctx context.Context, jobID string, data []byte) error
func (s Store) AppendService(ctx context.Context, service string, data []byte) error
func (s Store) ReadJobTail(ctx context.Context, jobID string, lines int) ([]string, error)
func (s Store) ReadServiceTail(ctx context.Context, service string, lines int) ([]string, error)
```

Create `internal/logs/tail.go` with a small line-tail implementation that reads the file from the end in chunks and returns the last N lines.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/state ./internal/logs -v
```

Expected: state and log tests pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/state internal/logs
git commit -m "feat: store job and service logs"
```

## Task 6: Compose Generation And Runtime Command Adapter

**Files:**
- Create: `internal/runtime/types.go`
- Create: `internal/runtime/command.go`
- Create: `internal/runtime/compose.go`
- Create: `internal/runtime/compose_test.go`
- Create: `internal/runtime/fake.go`
- Create: `deploy/runner/onespace-supervisor.sh`

- [ ] **Step 1: Add Compose generation test**

Create `internal/runtime/compose_test.go`:

```go
func TestGenerateComposeIncludesDevRunnerVolumesPortsAndAddons(t *testing.T) {
	// Build a domain.Workspace with one Go service, one Java service, and Redis addon.
	// Generate compose YAML.
	// Assert it contains:
	// - services user-api, order-api, redis
	// - repoPath mounted at /workspace
	// - Go cache and Maven cache volumes
	// - host/container port mappings
	// - workspace network
}
```

- [ ] **Step 2: Define runtime interfaces**

Create `internal/runtime/types.go`:

```go
package runtime

import (
	"context"
	"io"
)

type ExecOptions struct {
	WorkspaceRoot string
	Service       string
	Command       string
	Stdout        io.Writer
	Stderr        io.Writer
}

type Runtime interface {
	Ensure(ctx context.Context, workspaceRoot string) error
	Exec(ctx context.Context, opts ExecOptions) error
	StopProcess(ctx context.Context, workspaceRoot string, service string) error
	StartProcess(ctx context.Context, workspaceRoot string, service string, command string) error
	ServiceStatus(ctx context.Context, workspaceRoot string, service string) (ServiceStatus, error)
}

type ServiceStatus struct {
	Container string `json:"container"`
	Process   string `json:"process"`
}
```

- [ ] **Step 3: Implement command runner**

Create `internal/runtime/command.go`:

```go
type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args []string, stdout io.Writer, stderr io.Writer) error
}

type OSCommandRunner struct{}

func (OSCommandRunner) Run(ctx context.Context, dir string, name string, args []string, stdout io.Writer, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
```

- [ ] **Step 4: Implement Compose generator and adapter**

Create `internal/runtime/compose.go` with:

```go
func GenerateCompose(ws domain.Workspace) ([]byte, error)
type ComposeRuntime struct { Runner CommandRunner }
```

Adapter rules:

- Write generated YAML to `<workspace>/generated/docker-compose.yml`.
- Use `docker compose -f generated/docker-compose.yml -p <projectName> up -d <service>`.
- Use `docker compose ... exec -T <service> sh -lc <command>` for build commands.
- Use the supervisor helper for stop/start/status.

- [ ] **Step 5: Add supervisor shell contract**

Create `deploy/runner/onespace-supervisor.sh`:

```sh
#!/bin/sh
set -eu

STATE_DIR="${ONESPACE_STATE_DIR:-/workspace/.onespace}"
PID_FILE="$STATE_DIR/service.pid"
LOG_FILE="$STATE_DIR/service.log"

mkdir -p "$STATE_DIR"

case "${1:-}" in
  start)
    shift
    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      echo "service already running"
      exit 0
    fi
    sh -lc "$*" >>"$LOG_FILE" 2>&1 &
    echo "$!" >"$PID_FILE"
    ;;
  stop)
    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      kill "$(cat "$PID_FILE")"
      rm -f "$PID_FILE"
    fi
    ;;
  status)
    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      echo "running"
    else
      echo "stopped"
    fi
    ;;
  logs)
    if [ -f "$LOG_FILE" ]; then
      tail -n "${2:-200}" "$LOG_FILE"
    fi
    ;;
  *)
    echo "usage: onespace-supervisor.sh start <command> | stop | status | logs [lines]" >&2
    exit 2
    ;;
esac
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/runtime -v
```

Expected: runtime tests pass.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/runtime deploy/runner
git commit -m "feat: generate compose runtime"
```

## Task 7: Health Checks And Service Operation Manager

**Files:**
- Create: `internal/health/checker.go`
- Create: `internal/health/checker_test.go`
- Create: `internal/serviceops/result.go`
- Create: `internal/serviceops/manager.go`
- Create: `internal/serviceops/manager_test.go`

- [ ] **Step 1: Add health tests**

Create tests:

```go
func TestHTTPHealthCheckPassesOnTwoHundred(t *testing.T)
func TestHTTPHealthCheckFailsOnTimeout(t *testing.T)
```

Use `httptest.Server` for the passing case and a short context timeout for the timeout case.

- [ ] **Step 2: Implement health checker**

Create `internal/health/checker.go`:

```go
type Result struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Checker struct {
	Client *http.Client
}

func (c Checker) Check(ctx context.Context, hc domain.HealthCheck) Result
```

Return `passing`, `failing`, or `unknown`.

- [ ] **Step 3: Add service operation tests**

Create tests with fake Git, fake runtime, fake jobs, and fake logs:

```go
func TestDeployBuildsRestartsAndChecksHealth(t *testing.T)
func TestDeployReturnsBuildStageOnBuildFailure(t *testing.T)
func TestDebugUsesDebugBuildWhenConfigured(t *testing.T)
func TestPullRefusesDirtyRepo(t *testing.T)
```

- [ ] **Step 4: Define operation results**

Create `internal/serviceops/result.go`:

```go
type Result struct {
	Service   string          `json:"service"`
	Status    string          `json:"status"`
	JobID     string          `json:"jobId"`
	Stage     string          `json:"stage,omitempty"`
	Commit    string          `json:"commit,omitempty"`
	Dirty     bool            `json:"dirty"`
	Container string          `json:"container,omitempty"`
	Health    string          `json:"health,omitempty"`
	URL       string          `json:"url,omitempty"`
	Debug     *DebugAttach    `json:"debug,omitempty"`
	LogRef    string          `json:"logRef,omitempty"`
	ExitCode  int             `json:"exitCode,omitempty"`
	Details   json.RawMessage `json:"details,omitempty"`
}

type DebugAttach struct {
	Debugger string `json:"debugger"`
	Address  string `json:"address"`
}
```

- [ ] **Step 5: Implement manager**

Create `internal/serviceops/manager.go`:

```go
type Manager struct {
	Workspace domain.Workspace
	Git       GitService
	Runtime   runtime.Runtime
	Health    health.Checker
	Jobs      *jobs.Runner
	Logs      logs.Store
}

func (m *Manager) Deploy(ctx context.Context, service string) (Result, error)
func (m *Manager) Debug(ctx context.Context, service string) (Result, error)
func (m *Manager) Pull(ctx context.Context, service string) (Result, error)
func (m *Manager) Build(ctx context.Context, service string) (Result, error)
func (m *Manager) Restart(ctx context.Context, service string) (Result, error)
func (m *Manager) Stop(ctx context.Context, service string) (Result, error)
```

Stage order for deploy:

```text
validate -> git-status -> ensure-container -> build -> stop-process -> start-process -> health-check -> done
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/health ./internal/serviceops -v
```

Expected: health and operation tests pass.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/health internal/serviceops
git commit -m "feat: orchestrate service operations"
```

## Task 8: HTTP API And SSE Events

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/handlers.go`
- Create: `internal/api/sse.go`
- Create: `internal/api/handlers_test.go`
- Modify: `cmd/onespace-server/main.go`

- [ ] **Step 1: Add API handler tests**

Create tests with `httptest.Server`:

```go
func TestGetServicesReturnsWorkspaceServices(t *testing.T)
func TestPostDeployReturnsJobResult(t *testing.T)
func TestGetJobLogsReturnsTail(t *testing.T)
func TestEventsStreamsJobEvents(t *testing.T)
```

- [ ] **Step 2: Implement server wiring**

Create `internal/api/server.go`:

```go
type Server struct {
	Workspace domain.Workspace
	Ops       Operations
	Jobs      jobs.Store
	Logs      logs.Store
	Events    *EventBroker
	Mux       *http.ServeMux
}

func NewServer(deps Dependencies) *Server
func (s *Server) Handler() http.Handler
```

- [ ] **Step 3: Implement routes**

Create `internal/api/handlers.go` with handlers for:

```text
GET  /api/services
GET  /api/services/{service}
GET  /api/services/{service}/health
POST /api/services/{service}/pull
POST /api/services/{service}/build
POST /api/services/{service}/restart
POST /api/services/{service}/deploy
POST /api/services/{service}/debug
POST /api/services/{service}/stop
GET  /api/jobs
GET  /api/jobs/{jobId}
GET  /api/jobs/{jobId}/logs
```

Use JSON responses for all API routes.

- [ ] **Step 4: Implement SSE broker**

Create `internal/api/sse.go`:

```go
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type EventBroker struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
}

func NewEventBroker() *EventBroker
func (b *EventBroker) Publish(event Event)
func (b *EventBroker) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

- [ ] **Step 5: Wire daemon main**

Modify `cmd/onespace-server/main.go` so it supports:

```bash
onespace-server serve --config /path/to/onespace.yaml
onespace-server version
```

The `serve` command loads config, opens SQLite state under `state/onespace.db`, creates runtime/git/job/log/service manager dependencies, serves static files from `web/static`, and starts HTTP on configured bind address.

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/api ./cmd/onespace-server -v
```

Expected: API and daemon command tests pass.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/api cmd/onespace-server
git commit -m "feat: expose daemon API"
```

## Task 9: CLI Client And Agent-Friendly JSON

**Files:**
- Create: `internal/cli/client.go`
- Create: `internal/cli/commands.go`
- Create: `internal/cli/output.go`
- Create: `internal/cli/commands_test.go`
- Modify: `cmd/onespace/main.go`
- Create: `docs/cli.md`

- [ ] **Step 1: Add CLI command tests**

Create tests:

```go
func TestDeployWaitJSONPrintsResultAndReturnsZeroOnSuccess(t *testing.T)
func TestDeployWaitJSONReturnsNonZeroOnFailure(t *testing.T)
func TestLogsCommandPrintsTail(t *testing.T)
func TestStatusCommandPrintsServiceSummary(t *testing.T)
```

Use `httptest.Server` as the daemon.

- [ ] **Step 2: Implement API client**

Create `internal/cli/client.go`:

```go
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (c Client) GetServices(ctx context.Context) ([]ServiceSummary, error)
func (c Client) Deploy(ctx context.Context, service string, wait bool) (serviceops.Result, error)
func (c Client) Logs(ctx context.Context, service string, jobID string, tail int) ([]string, error)
func (c Client) Health(ctx context.Context, service string) (health.Result, error)
```

- [ ] **Step 3: Implement command parser with standard library**

Create `internal/cli/commands.go` using `flag.FlagSet` for:

```text
status [service]
pull <service>|--all
build <service>
restart <service>
deploy <service> [--wait] [--json]
debug <service> [--wait] [--json]
health <service>
logs <service> [--job <jobId>] [--tail 200]
workspace list
workspace use <name>
version
```

- [ ] **Step 4: Implement output formatting**

Create `internal/cli/output.go`:

```go
func WriteJSON(w io.Writer, v interface{}) error
func WriteDeployText(w io.Writer, result serviceops.Result) error
func WriteServicesTable(w io.Writer, services []ServiceSummary) error
```

Text output is for humans; JSON output must match the daemon result payload exactly.

- [ ] **Step 5: Wire CLI main**

Modify `cmd/onespace/main.go` to call `cli.Run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv)`.

- [ ] **Step 6: Document CLI**

Create `docs/cli.md` with examples:

```markdown
# Onespace CLI

Agent deploy:

```bash
onespace deploy user-api --wait --json
```

Read failed job logs:

```bash
onespace logs user-api --job job_20260501_0002 --tail 200
```
```

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./internal/cli ./cmd/onespace -v
```

Expected: CLI tests pass.

- [ ] **Step 8: Commit**

Run:

```bash
git add internal/cli cmd/onespace docs/cli.md
git commit -m "feat: add agent friendly CLI"
```

## Task 10: Static Web UI

**Files:**
- Create: `web/static/index.html`
- Create: `web/static/styles.css`
- Create: `web/static/app.js`
- Modify: `internal/api/server.go`

- [ ] **Step 1: Add static file server test**

Add an API test:

```go
func TestStaticIndexServedAtRoot(t *testing.T)
```

Expected: `GET /` returns HTML containing `Onespace`.

- [ ] **Step 2: Create HTML shell**

Create `web/static/index.html`:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Onespace</title>
    <link rel="stylesheet" href="/static/styles.css">
  </head>
  <body>
    <header class="topbar">
      <div>
        <h1>Onespace</h1>
        <p id="workspace-name">Loading workspace</p>
      </div>
      <button id="refresh" type="button">Refresh</button>
    </header>
    <main class="layout">
      <section class="panel">
        <h2>Services</h2>
        <div id="services"></div>
      </section>
      <section class="panel">
        <h2>Activity</h2>
        <pre id="activity"></pre>
      </section>
    </main>
    <script type="module" src="/static/app.js"></script>
  </body>
</html>
```

- [ ] **Step 3: Create focused CSS**

Create `web/static/styles.css` with dense operational layout, responsive table/card behavior, and no decorative hero section.

- [ ] **Step 4: Create Web UI JavaScript**

Create `web/static/app.js`:

```js
const servicesEl = document.querySelector("#services");
const activityEl = document.querySelector("#activity");
const refreshButton = document.querySelector("#refresh");

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "accept": "application/json" },
    ...options
  });
  if (!response.ok) {
    throw new Error(`${response.status} ${response.statusText}`);
  }
  return response.json();
}

function renderServices(services) {
  servicesEl.innerHTML = "";
  for (const service of services) {
    const row = document.createElement("article");
    row.className = "service-row";
    row.innerHTML = `
      <div>
        <strong>${service.name}</strong>
        <span>${service.language}</span>
      </div>
      <div>${service.git?.branch ?? "unknown"}</div>
      <div>${service.runtime?.container ?? "unknown"}</div>
      <div>${service.health ?? "unknown"}</div>
      <div class="actions">
        <button data-action="deploy" data-service="${service.name}">Deploy</button>
        <button data-action="debug" data-service="${service.name}">Debug</button>
        <button data-action="logs" data-service="${service.name}">Logs</button>
      </div>
    `;
    servicesEl.appendChild(row);
  }
}

async function refresh() {
  const services = await api("/api/services");
  renderServices(services);
}

async function runAction(action, service) {
  if (action === "logs") {
    const logs = await api(`/api/services/${service}/logs?tail=200`);
    activityEl.textContent = logs.lines.join("\n");
    return;
  }
  const result = await api(`/api/services/${service}/${action}`, { method: "POST" });
  activityEl.textContent = JSON.stringify(result, null, 2);
}

servicesEl.addEventListener("click", (event) => {
  const button = event.target.closest("button[data-action]");
  if (!button) return;
  runAction(button.dataset.action, button.dataset.service).catch((error) => {
    activityEl.textContent = error.message;
  });
});

refreshButton.addEventListener("click", () => refresh().catch((error) => {
  activityEl.textContent = error.message;
}));

refresh().catch((error) => {
  activityEl.textContent = error.message;
});
```

- [ ] **Step 5: Serve static files**

Modify `internal/api/server.go` to serve:

```text
/                 -> web/static/index.html
/static/app.js    -> web/static/app.js
/static/styles.css -> web/static/styles.css
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/api -v
```

Expected: static serving and API tests pass.

- [ ] **Step 7: Commit**

Run:

```bash
git add web/static internal/api
git commit -m "feat: add web dashboard"
```

## Task 11: Dev Runner Images

**Files:**
- Create: `deploy/images/go-dev/Dockerfile`
- Create: `deploy/images/java-dev-maven/Dockerfile`
- Create: `docs/dev-runner-images.md`

- [ ] **Step 1: Add Go dev image**

Create `deploy/images/go-dev/Dockerfile`:

```dockerfile
FROM golang:1.23-bookworm

RUN go install github.com/go-delve/delve/cmd/dlv@latest

COPY deploy/runner/onespace-supervisor.sh /usr/local/bin/onespace-supervisor
RUN chmod +x /usr/local/bin/onespace-supervisor

WORKDIR /workspace
CMD ["sleep", "infinity"]
```

- [ ] **Step 2: Add Java Maven dev image**

Create `deploy/images/java-dev-maven/Dockerfile`:

```dockerfile
FROM maven:3.9-eclipse-temurin-21

COPY deploy/runner/onespace-supervisor.sh /usr/local/bin/onespace-supervisor
RUN chmod +x /usr/local/bin/onespace-supervisor

WORKDIR /workspace
CMD ["sleep", "infinity"]
```

- [ ] **Step 3: Document image build commands**

Create `docs/dev-runner-images.md`:

```markdown
# Dev Runner Images

Build the Go runner:

```bash
docker build -t onespace/go-dev:1.23 -f deploy/images/go-dev/Dockerfile .
```

Build the Java Maven runner:

```bash
docker build -t onespace/java-dev:21-maven -f deploy/images/java-dev-maven/Dockerfile .
```
```

- [ ] **Step 4: Build images in an integration environment**

Run:

```bash
docker build -t onespace/go-dev:1.23 -f deploy/images/go-dev/Dockerfile .
docker build -t onespace/java-dev:21-maven -f deploy/images/java-dev-maven/Dockerfile .
```

Expected: both images build successfully.

- [ ] **Step 5: Commit**

Run:

```bash
git add deploy/images docs/dev-runner-images.md
git commit -m "feat: add dev runner images"
```

## Task 12: End-To-End Smoke Workspace

**Files:**
- Create: `examples/workspaces/smoke-go/onespace.yaml`
- Create: `examples/workspaces/smoke-go/repos/user-api/go.mod`
- Create: `examples/workspaces/smoke-go/repos/user-api/cmd/user-api/main.go`
- Create: `docs/smoke-test.md`

- [ ] **Step 1: Add tiny Go service**

Create `examples/workspaces/smoke-go/repos/user-api/go.mod`:

```go
module example.com/user-api

go 1.22
```

Create `examples/workspaces/smoke-go/repos/user-api/cmd/user-api/main.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
)

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"service": "user-api"})
	})
	_ = http.ListenAndServe(":8080", nil)
}
```

- [ ] **Step 2: Add smoke workspace config**

Create `examples/workspaces/smoke-go/onespace.yaml` with one Go service pointing to `examples/workspaces/smoke-go/repos/user-api`, app port `18081`, and debug port `40001`.

- [ ] **Step 3: Document smoke test**

Create `docs/smoke-test.md`:

```markdown
# Smoke Test

Build dev image:

```bash
docker build -t onespace/go-dev:1.23 -f deploy/images/go-dev/Dockerfile .
```

Start daemon:

```bash
go run ./cmd/onespace-server serve --config examples/workspaces/smoke-go/onespace.yaml
```

Deploy service:

```bash
go run ./cmd/onespace deploy user-api --wait --json
```

Check health:

```bash
curl -fsS http://127.0.0.1:18081/health
```
```

- [ ] **Step 4: Run smoke test**

Run the commands in `docs/smoke-test.md`.

Expected:

- `deploy user-api --wait --json` returns `status: success`.
- `curl -fsS http://127.0.0.1:18081/health` returns JSON containing `"status":"ok"`.

- [ ] **Step 5: Commit**

Run:

```bash
git add examples/workspaces/smoke-go docs/smoke-test.md
git commit -m "test: add smoke workspace"
```

## Task 13: Final Verification And Release Notes

**Files:**
- Create: `docs/mvp-verification.md`
- Modify: `README.md`

- [ ] **Step 1: Add README**

Create `README.md`:

```markdown
# Onespace

Onespace is a single-user development control plane for running, rebuilding, debugging, and observing local Go and Java microservices on one VM.

Primary workflows:

- Web UI for service status, logs, jobs, and manual deploy/debug actions.
- CLI for developers, scripts, and coding agents.
- Docker Compose dev runners for container-side build/run/debug.

Start with:

```bash
go run ./cmd/onespace-server serve --config examples/workspaces/smoke-go/onespace.yaml
go run ./cmd/onespace deploy user-api --wait --json
```
```

- [ ] **Step 2: Add verification doc**

Create `docs/mvp-verification.md`:

```markdown
# MVP Verification

Run unit tests:

```bash
go test ./...
```

Build binaries:

```bash
go build ./cmd/onespace-server
go build ./cmd/onespace
```

Build dev images:

```bash
docker build -t onespace/go-dev:1.23 -f deploy/images/go-dev/Dockerfile .
docker build -t onespace/java-dev:21-maven -f deploy/images/java-dev-maven/Dockerfile .
```

Run smoke test:

```bash
go run ./cmd/onespace-server serve --config examples/workspaces/smoke-go/onespace.yaml
go run ./cmd/onespace deploy user-api --wait --json
curl -fsS http://127.0.0.1:18081/health
```
```

- [ ] **Step 3: Run full verification**

Run:

```bash
go test ./...
go build ./cmd/onespace-server
go build ./cmd/onespace
```

Expected: all tests pass and both binaries build.

Run Docker smoke verification on a VM with Docker available.

Expected: dev image builds and `user-api` health check passes.

- [ ] **Step 4: Commit**

Run:

```bash
git add README.md docs/mvp-verification.md
git commit -m "docs: add MVP verification guide"
```

## Self-Review

Spec coverage:

- Single-user daemon: Tasks 1, 8, 13.
- Web UI: Task 10.
- CLI for agents: Task 9.
- Local workspace config: Task 2.
- Local repo status and conservative pull: Task 3.
- Go profile: Tasks 2, 6, 11, 12.
- Java Maven profile: Tasks 2, 6, 11.
- Container-side build/run/debug: Tasks 6, 7, 11, 12.
- Docker Compose runtime: Task 6.
- Job history: Task 4.
- Logs and tailing: Task 5.
- Health checks: Task 7.
- Structured JSON output: Tasks 7, 8, 9.
- Smoke verification: Tasks 12, 13.

Execution order:

1. Complete Tasks 1-5 for core local state and config.
2. Complete Tasks 6-7 for runtime and deploy semantics.
3. Complete Tasks 8-10 for daemon API, CLI, and Web UI.
4. Complete Tasks 11-13 for images, smoke workspace, and final verification.

