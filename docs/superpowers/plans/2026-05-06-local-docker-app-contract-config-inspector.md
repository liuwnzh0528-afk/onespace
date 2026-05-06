# Local Docker App Contract & Config Inspector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend Onespace so `onespace.yaml` can describe local Docker runtime configuration and expose a Config Inspector through Compose generation, API, CLI, and Web UI.

**Architecture:** Add an `internal/appcontract` package that composes service env, files, secrets, volumes, and dependencies from the existing `domain.Workspace`. Keep Docker Compose as the only runtime materialization path, and keep K8s/kind/k3d/Terraform/file-watch/test-timeline concepts out of the implementation.

**Tech Stack:** Go 1.22+, `gopkg.in/yaml.v3`, standard library HTTP server, existing Docker Compose adapter, existing CLI client/output pattern, static HTML/CSS/JavaScript.

---

## Scope

Implement only:

- `onespace.yaml` schema extensions for `env`, `envFrom`, `files`, `secrets`, `secretFiles`, `volumes`, and `dependsOn`.
- Config Composer with source tracking and secret redaction.
- Compose materialization for local Docker.
- API endpoint: `GET /api/services/{service}/config`.
- CLI command: `onespace config <service> [--json]`.
- Web UI Config action.
- Example workspace and user-guide updates.

Do not implement:

- File watching.
- Automatic rebuild or restart.
- Generic test command orchestration.
- Build/Test Timeline.
- Kubernetes, k3d, kind, Terraform export, or drift check.

## File Structure

Create:

```text
internal/appcontract/
  composer.go
  composer_test.go
  envfile.go
  envfile_test.go
  types.go
```

Modify:

```text
internal/domain/service.go
internal/config/types.go
internal/config/loader.go
internal/config/loader_test.go
internal/runtime/compose.go
internal/runtime/compose_test.go
internal/api/server.go
internal/api/handlers.go
internal/api/handlers_test.go
internal/cli/client.go
internal/cli/commands.go
internal/cli/commands_test.go
internal/cli/output.go
web/static/app.js
web/static/styles.css
examples/workspaces/smoke-go/onespace.yaml
docs/user-guide.md
```

Responsibilities:

- `internal/domain`: hold parsed declarative fields with no I/O behavior.
- `internal/config`: parse and validate YAML into domain structs, resolving workspace-relative host paths where needed.
- `internal/appcontract`: read env files and secret files, apply source precedence, produce redacted inspector output and raw runtime output.
- `internal/runtime`: render appcontract output into Docker Compose.
- `internal/api`: expose read-only config inspector endpoint.
- `internal/cli`: expose config inspector to agents and scripts.
- `web/static`: expose read-only config inspection in the existing Activity panel.

## Task 1: Domain And YAML Schema

**Files:**
- Modify: `internal/domain/service.go`
- Modify: `internal/config/types.go`
- Modify: `internal/config/loader.go`
- Test: `internal/config/loader_test.go`

- [ ] **Step 1: Write the failing loader test**

Add this test to `internal/config/loader_test.go`:

```go
func TestLoadWorkspaceLoadsLocalDockerRuntimeContract(t *testing.T) {
	dir := t.TempDir()
	repoRoot := filepath.Join(dir, "repos")
	repoPath := filepath.Join(repoRoot, "user-api")
	for _, path := range []string{
		repoPath,
		filepath.Join(dir, "config"),
		filepath.Join(dir, ".secrets"),
		filepath.Join(dir, "tmp"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(dir, "onespace.yaml")
	yaml := `
version: 1
name: contract-ws
allowedRepoRoots:
  - repos
services:
  user-api:
    language: go
    repoPath: repos/user-api
    env:
      APP_ENV: local
      LOG_LEVEL: debug
    envFrom:
      - file: .env
      - file: .env.local
        optional: true
    files:
      - source: config/local.yaml
        target: /etc/user-api/config.yaml
        mode: "0444"
    secrets:
      - name: DB_PASSWORD
        fromFile: .secrets/db_password
    secretFiles:
      - source: .secrets/client.key
        target: /etc/user-api/client.key
        mode: "0400"
    volumes:
      - source: onespace-user-api-cache
        target: /workspace/.cache
      - source: ./tmp
        target: /workspace/tmp
    dependsOn:
      - redis
addons:
  redis:
    image: redis:7-alpine
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := LoadWorkspace(configPath)
	if err != nil {
		t.Fatalf("LoadWorkspace returned error: %v", err)
	}

	svc := ws.Services["user-api"]
	if svc.Env["APP_ENV"] != "local" || svc.Env["LOG_LEVEL"] != "debug" {
		t.Fatalf("Env = %#v, want APP_ENV and LOG_LEVEL", svc.Env)
	}
	if len(svc.EnvFrom) != 2 || svc.EnvFrom[1].Optional != true {
		t.Fatalf("EnvFrom = %#v, want two entries with second optional", svc.EnvFrom)
	}
	if svc.EnvFrom[0].File != filepath.Join(dir, ".env") {
		t.Fatalf("EnvFrom[0].File = %q, want workspace-relative absolute path", svc.EnvFrom[0].File)
	}
	if len(svc.Files) != 1 || svc.Files[0].Source != filepath.Join(dir, "config", "local.yaml") {
		t.Fatalf("Files = %#v, want resolved config file", svc.Files)
	}
	if len(svc.Secrets) != 1 || svc.Secrets[0].FromFile != filepath.Join(dir, ".secrets", "db_password") {
		t.Fatalf("Secrets = %#v, want resolved secret file", svc.Secrets)
	}
	if len(svc.SecretFiles) != 1 || svc.SecretFiles[0].Mode != "0400" {
		t.Fatalf("SecretFiles = %#v, want secret file with mode", svc.SecretFiles)
	}
	if len(svc.Volumes) != 2 || svc.Volumes[1].Source != filepath.Join(dir, "tmp") {
		t.Fatalf("Volumes = %#v, want named volume and resolved bind mount", svc.Volumes)
	}
	if len(svc.DependsOn) != 1 || svc.DependsOn[0] != "redis" {
		t.Fatalf("DependsOn = %#v, want redis", svc.DependsOn)
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

Run:

```bash
go test ./internal/config -run TestLoadWorkspaceLoadsLocalDockerRuntimeContract -count=1
```

Expected: FAIL because `domain.Service` and YAML structs do not yet contain the new fields.

- [ ] **Step 3: Extend domain structs**

Modify `internal/domain/service.go` by adding these types and fields:

```go
type Service struct {
	Name        string
	Language    string
	RepoPath    string
	Workdir     string
	Image       string
	Main        string
	Ports       []Port
	Health      HealthCheck
	Build       Command
	Run         Command
	Debug       DebugConfig
	Env         map[string]string
	EnvFrom     []EnvFrom
	Files       []FileMount
	Secrets     []SecretEnv
	SecretFiles []FileMount
	Volumes     []VolumeMount
	DependsOn   []string
}

type EnvFrom struct {
	File     string
	Optional bool
}

type FileMount struct {
	Source string
	Target string
	Mode   string
}

type SecretEnv struct {
	Name     string
	FromFile string
}

type VolumeMount struct {
	Source string
	Target string
}
```

- [ ] **Step 4: Extend YAML structs**

Modify `internal/config/types.go` by adding matching YAML types:

```go
type serviceYAML struct {
	Language    string            `yaml:"language"`
	RepoPath    string            `yaml:"repoPath"`
	Workdir     string            `yaml:"workdir"`
	Image       string            `yaml:"image"`
	Main        string            `yaml:"main"`
	Ports       []portYAML        `yaml:"ports"`
	Health      healthYAML        `yaml:"health"`
	Build       commandYAML       `yaml:"build"`
	Run         commandYAML       `yaml:"run"`
	Debug       debugYAML         `yaml:"debug"`
	Env         map[string]string `yaml:"env"`
	EnvFrom     []envFromYAML     `yaml:"envFrom"`
	Files       []fileYAML        `yaml:"files"`
	Secrets     []secretEnvYAML   `yaml:"secrets"`
	SecretFiles []fileYAML        `yaml:"secretFiles"`
	Volumes     []volumeYAML      `yaml:"volumes"`
	DependsOn   []string          `yaml:"dependsOn"`
}

type envFromYAML struct {
	File     string `yaml:"file"`
	Optional bool   `yaml:"optional"`
}

type fileYAML struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Mode   string `yaml:"mode"`
}

type secretEnvYAML struct {
	Name     string `yaml:"name"`
	FromFile string `yaml:"fromFile"`
}

type volumeYAML struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}
```

- [ ] **Step 5: Map YAML fields to domain fields**

In `internal/config/loader.go`, update `mapService` after base service initialization:

```go
svc.Env = raw.Env
if svc.Env == nil {
	svc.Env = map[string]string{}
}

for _, item := range raw.EnvFrom {
	svc.EnvFrom = append(svc.EnvFrom, domain.EnvFrom{
		File:     resolveWorkspacePath(workspaceDir, item.File),
		Optional: item.Optional,
	})
}
for _, item := range raw.Files {
	svc.Files = append(svc.Files, domain.FileMount{
		Source: resolveWorkspacePath(workspaceDir, item.Source),
		Target: item.Target,
		Mode:   item.Mode,
	})
}
for _, item := range raw.Secrets {
	svc.Secrets = append(svc.Secrets, domain.SecretEnv{
		Name:     item.Name,
		FromFile: resolveWorkspacePath(workspaceDir, item.FromFile),
	})
}
for _, item := range raw.SecretFiles {
	svc.SecretFiles = append(svc.SecretFiles, domain.FileMount{
		Source: resolveWorkspacePath(workspaceDir, item.Source),
		Target: item.Target,
		Mode:   item.Mode,
	})
}
for _, item := range raw.Volumes {
	source := item.Source
	if isRelativeHostPath(source) {
		source = resolveWorkspacePath(workspaceDir, source)
	}
	svc.Volumes = append(svc.Volumes, domain.VolumeMount{
		Source: source,
		Target: item.Target,
	})
}
svc.DependsOn = append([]string(nil), raw.DependsOn...)
```

Add this helper near `resolveWorkspacePath`:

```go
func isRelativeHostPath(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}
```

- [ ] **Step 6: Run config tests**

Run:

```bash
go test ./internal/config -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/domain/service.go internal/config/types.go internal/config/loader.go internal/config/loader_test.go
git commit -m "feat: extend service runtime contract schema"
```

## Task 2: Env File Parser

**Files:**
- Create: `internal/appcontract/envfile.go`
- Test: `internal/appcontract/envfile_test.go`

- [ ] **Step 1: Write env file parser tests**

Create `internal/appcontract/envfile_test.go`:

```go
package appcontract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadEnvFileParsesSimpleEnvSyntax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	data := `
# comment
APP_ENV=local
LOG_LEVEL="debug"
TOKEN='abc123'
EMPTY=
export EXPORTED=yes
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	env, err := ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile returned error: %v", err)
	}

	want := map[string]string{
		"APP_ENV":  "local",
		"LOG_LEVEL": "debug",
		"TOKEN":    "abc123",
		"EMPTY":    "",
		"EXPORTED": "yes",
	}
	for key, value := range want {
		if env[key] != value {
			t.Fatalf("env[%s] = %q, want %q", key, env[key], value)
		}
	}
}

func TestReadEnvFileRejectsMalformedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("not valid\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadEnvFile(path)
	if err == nil {
		t.Fatal("expected malformed line error")
	}
}
```

- [ ] **Step 2: Run parser tests and verify they fail**

Run:

```bash
go test ./internal/appcontract -run TestReadEnvFile -count=1
```

Expected: FAIL because `ReadEnvFile` does not exist.

- [ ] **Step 3: Implement env parser**

Create `internal/appcontract/envfile.go`:

```go
package appcontract

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ReadEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	env := map[string]string{}
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%s:%d: malformed env line", path, lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		env[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return env, nil
}
```

- [ ] **Step 4: Run appcontract tests**

Run:

```bash
go test ./internal/appcontract -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/appcontract/envfile.go internal/appcontract/envfile_test.go
git commit -m "feat: parse local env files"
```

## Task 3: Config Composer

**Files:**
- Create: `internal/appcontract/types.go`
- Create: `internal/appcontract/composer.go`
- Test: `internal/appcontract/composer_test.go`

- [ ] **Step 1: Write composer tests**

Create `internal/appcontract/composer_test.go`:

```go
package appcontract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wnzhone/onespace/internal/domain"
)

func TestComposerBuildsInspectorWithSourcesAndRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	envLocalPath := filepath.Join(dir, ".env.local")
	secretPath := filepath.Join(dir, ".secrets", "db_password")
	if err := os.MkdirAll(filepath.Dir(secretPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("LOG_LEVEL=info\nFROM_FILE=yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envLocalPath, []byte("LOG_LEVEL=debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretPath, []byte("super-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ws := domain.Workspace{
		Name: "test-ws",
		Path: dir,
		Services: map[string]domain.Service{
			"user-api": {
				Name:     "user-api",
				Language: "go",
				Workdir:  "/workspace",
				Env: map[string]string{
					"APP_ENV":   "local",
					"LOG_LEVEL": "warn",
				},
				EnvFrom: []domain.EnvFrom{
					{File: envPath},
					{File: envLocalPath},
				},
				Secrets: []domain.SecretEnv{
					{Name: "DB_PASSWORD", FromFile: secretPath},
				},
				Files: []domain.FileMount{
					{Source: filepath.Join(dir, "config", "local.yaml"), Target: "/etc/user-api/config.yaml", Mode: "0444"},
				},
				SecretFiles: []domain.FileMount{
					{Source: filepath.Join(dir, ".secrets", "client.key"), Target: "/etc/user-api/client.key", Mode: "0400"},
				},
				Volumes: []domain.VolumeMount{
					{Source: "onespace-user-api-cache", Target: "/workspace/.cache"},
				},
				DependsOn: []string{"redis"},
			},
		},
	}

	cfg, err := Composer{Workspace: ws}.ComposeService("user-api")
	if err != nil {
		t.Fatalf("ComposeService returned error: %v", err)
	}

	if cfg.RuntimeEnv["LOG_LEVEL"] != "debug" {
		t.Fatalf("RuntimeEnv[LOG_LEVEL] = %q, want debug", cfg.RuntimeEnv["LOG_LEVEL"])
	}
	if cfg.RuntimeEnv["DB_PASSWORD"] != "super-secret" {
		t.Fatalf("RuntimeEnv[DB_PASSWORD] was not populated from secret file")
	}
	if got := cfg.EnvValue("DB_PASSWORD"); got.Value != "******" || !got.Secret {
		t.Fatalf("DB_PASSWORD inspector entry = %+v, want redacted secret", got)
	}
	if got := cfg.EnvValue("LOG_LEVEL"); got.Source != envLocalPath {
		t.Fatalf("LOG_LEVEL source = %q, want %q", got.Source, envLocalPath)
	}
	if len(cfg.Files) != 2 || !cfg.Files[1].Secret {
		t.Fatalf("Files = %+v, want config file and secret file", cfg.Files)
	}
	if len(cfg.Volumes) != 1 || cfg.Volumes[0].Type != "volume" {
		t.Fatalf("Volumes = %+v, want named volume", cfg.Volumes)
	}
	if len(cfg.DependsOn) != 1 || cfg.DependsOn[0] != "redis" {
		t.Fatalf("DependsOn = %+v, want redis", cfg.DependsOn)
	}
}

func TestComposerSkipsOptionalMissingEnvFile(t *testing.T) {
	ws := domain.Workspace{
		Name: "test-ws",
		Services: map[string]domain.Service{
			"user-api": {
				Name: "user-api",
				EnvFrom: []domain.EnvFrom{
					{File: filepath.Join(t.TempDir(), ".env.local"), Optional: true},
				},
			},
		},
	}

	cfg, err := Composer{Workspace: ws}.ComposeService("user-api")
	if err != nil {
		t.Fatalf("ComposeService returned error: %v", err)
	}
	if len(cfg.Warnings) != 1 {
		t.Fatalf("Warnings = %+v, want one skipped optional env file warning", cfg.Warnings)
	}
}
```

- [ ] **Step 2: Run composer tests and verify they fail**

Run:

```bash
go test ./internal/appcontract -run TestComposer -count=1
```

Expected: FAIL because `Composer` and related types do not exist.

- [ ] **Step 3: Add appcontract types**

Create `internal/appcontract/types.go`:

```go
package appcontract

type ServiceConfig struct {
	Service    string            `json:"service"`
	Env        []EnvEntry        `json:"env"`
	Files      []FileEntry       `json:"files"`
	Volumes    []VolumeEntry     `json:"volumes"`
	DependsOn  []string          `json:"dependsOn"`
	Warnings   []Warning         `json:"warnings,omitempty"`
	RuntimeEnv map[string]string `json:"-"`
}

type EnvEntry struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Source string `json:"source"`
	Secret bool   `json:"secret"`
}

type FileEntry struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Mode   string `json:"mode"`
	Secret bool   `json:"secret"`
}

type VolumeEntry struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type Warning struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

func (c ServiceConfig) EnvValue(name string) EnvEntry {
	for _, entry := range c.Env {
		if entry.Name == name {
			return entry
		}
	}
	return EnvEntry{}
}
```

- [ ] **Step 4: Implement composer**

Create `internal/appcontract/composer.go`:

```go
package appcontract

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/wnzhone/onespace/internal/domain"
)

type Composer struct {
	Workspace domain.Workspace
}

func (c Composer) ComposeService(name string) (ServiceConfig, error) {
	svc, ok := c.Workspace.Services[name]
	if !ok {
		return ServiceConfig{Service: name}, fmt.Errorf("service %q not found", name)
	}

	cfg := ServiceConfig{
		Service:    name,
		RuntimeEnv: map[string]string{},
		DependsOn:  append([]string(nil), svc.DependsOn...),
	}

	c.applyGeneratedEnv(&cfg, svc)
	c.applyInlineEnv(&cfg, svc)
	if err := c.applyEnvFiles(&cfg, svc); err != nil {
		return cfg, err
	}
	if err := c.applySecrets(&cfg, svc); err != nil {
		return cfg, err
	}
	c.applyFiles(&cfg, svc)
	c.applyVolumes(&cfg, svc)
	sort.Slice(cfg.Env, func(i, j int) bool { return cfg.Env[i].Name < cfg.Env[j].Name })
	return cfg, nil
}

func (c Composer) applyGeneratedEnv(cfg *ServiceConfig, svc domain.Service) {
	setEnv(cfg, "ONESPACE_STATE_DIR", svc.Workdir+"/.onespace", "generated runtime env", false)
	switch svc.Language {
	case "go":
		setEnv(cfg, "PATH", "/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "generated go runtime env", false)
	case "java-maven":
		setEnv(cfg, "PATH", "/opt/java/openjdk/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "generated java runtime env", false)
	}
}

func (c Composer) applyInlineEnv(cfg *ServiceConfig, svc domain.Service) {
	for key, value := range svc.Env {
		setEnv(cfg, key, value, "onespace.yaml env", false)
	}
}

func (c Composer) applyEnvFiles(cfg *ServiceConfig, svc domain.Service) error {
	for _, source := range svc.EnvFrom {
		env, err := ReadEnvFile(source.File)
		if err != nil {
			if source.Optional && os.IsNotExist(err) {
				cfg.Warnings = append(cfg.Warnings, Warning{Source: source.File, Reason: "optional env file not found"})
				continue
			}
			return err
		}
		for key, value := range env {
			setEnv(cfg, key, value, source.File, false)
		}
	}
	return nil
}

func (c Composer) applySecrets(cfg *ServiceConfig, svc domain.Service) error {
	for _, secret := range svc.Secrets {
		data, err := os.ReadFile(secret.FromFile)
		if err != nil {
			return err
		}
		value := strings.TrimRight(string(data), "\r\n")
		setEnv(cfg, secret.Name, value, secret.FromFile, true)
	}
	return nil
}

func (c Composer) applyFiles(cfg *ServiceConfig, svc domain.Service) {
	for _, file := range svc.Files {
		mode := file.Mode
		if mode == "" {
			mode = "0444"
		}
		cfg.Files = append(cfg.Files, FileEntry{Source: file.Source, Target: file.Target, Mode: mode})
	}
	for _, file := range svc.SecretFiles {
		mode := file.Mode
		if mode == "" {
			mode = "0400"
		}
		cfg.Files = append(cfg.Files, FileEntry{Source: file.Source, Target: file.Target, Mode: mode, Secret: true})
	}
}

func (c Composer) applyVolumes(cfg *ServiceConfig, svc domain.Service) {
	for _, volume := range svc.Volumes {
		volumeType := "volume"
		if strings.HasPrefix(volume.Source, "/") {
			volumeType = "bind"
		}
		cfg.Volumes = append(cfg.Volumes, VolumeEntry{Source: volume.Source, Target: volume.Target, Type: volumeType})
	}
}

func setEnv(cfg *ServiceConfig, name string, value string, source string, secret bool) {
	cfg.RuntimeEnv[name] = value
	displayValue := value
	if secret {
		displayValue = "******"
	}
	entry := EnvEntry{Name: name, Value: displayValue, Source: source, Secret: secret}
	for i := range cfg.Env {
		if cfg.Env[i].Name == name {
			cfg.Env[i] = entry
			return
		}
	}
	cfg.Env = append(cfg.Env, entry)
}
```

- [ ] **Step 5: Run appcontract tests**

Run:

```bash
go test ./internal/appcontract -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/appcontract
git commit -m "feat: compose service config sources"
```

## Task 4: Compose Materialization

**Files:**
- Modify: `internal/runtime/compose.go`
- Test: `internal/runtime/compose_test.go`

- [ ] **Step 1: Write Compose test for app contract fields**

Add this test to `internal/runtime/compose_test.go`:

```go
func TestGenerateComposeIncludesRuntimeContractConfig(t *testing.T) {
	ws := domain.Workspace{
		Name: "contract-ws",
		Runtime: domain.RuntimeConfig{
			Type:        "docker-compose",
			ProjectName: "contract-ws",
			Network:     "contract-ws-default",
		},
		Services: map[string]domain.Service{
			"user-api": {
				Name:     "user-api",
				Language: "go",
				RepoPath: "/data/repos/user-api",
				Workdir:  "/workspace",
				Image:    "onespace/go-dev:1.23",
				Env: map[string]string{
					"APP_ENV": "local",
				},
				Files: []domain.FileMount{
					{Source: "/data/workspace/config/local.yaml", Target: "/etc/user-api/config.yaml", Mode: "0444"},
				},
				SecretFiles: []domain.FileMount{
					{Source: "/data/workspace/.secrets/client.key", Target: "/etc/user-api/client.key", Mode: "0400"},
				},
				Volumes: []domain.VolumeMount{
					{Source: "onespace-user-api-cache", Target: "/workspace/.cache"},
				},
				DependsOn: []string{"redis"},
			},
		},
		Addons: map[string]domain.Addon{
			"redis": {Image: "redis:7-alpine"},
		},
	}

	data, err := GenerateCompose(ws)
	if err != nil {
		t.Fatalf("GenerateCompose: %v", err)
	}
	yamlStr := string(data)

	for _, want := range []string{
		"APP_ENV: local",
		"/data/workspace/config/local.yaml:/etc/user-api/config.yaml:ro",
		"/data/workspace/.secrets/client.key:/etc/user-api/client.key:ro",
		"onespace-user-api-cache:/workspace/.cache",
		"depends_on:",
		"- redis",
	} {
		if !strings.Contains(yamlStr, want) {
			t.Fatalf("compose YAML missing %q:\n%s", want, yamlStr)
		}
	}
}
```

- [ ] **Step 2: Run runtime test and verify it fails**

Run:

```bash
go test ./internal/runtime -run TestGenerateComposeIncludesRuntimeContractConfig -count=1
```

Expected: FAIL because Compose generation ignores the new contract fields.

- [ ] **Step 3: Render appcontract output in Compose**

In `internal/runtime/compose.go`, import `internal/appcontract` and update `GenerateCompose` / `buildServices` so composer errors are returned instead of hidden:

```go
func GenerateCompose(ws domain.Workspace) ([]byte, error) {
	projectName := ws.Runtime.ProjectName
	if projectName == "" {
		projectName = ws.Name
	}
	services, err := buildServices(ws)
	if err != nil {
		return nil, err
	}
	compose := map[string]interface{}{
		"name":     projectName,
		"services": services,
		"networks": map[string]interface{}{
			ws.Runtime.Network: map[string]interface{}{
				"external": false,
			},
		},
	}

	volumes := buildVolumes(ws)
	if len(volumes) > 0 {
		compose["volumes"] = volumes
	}

	return yaml.Marshal(compose)
}

func buildServices(ws domain.Workspace) (map[string]interface{}, error) {
	services := make(map[string]interface{})
	composer := appcontract.Composer{Workspace: ws}

	for name, svc := range ws.Services {
		cfg, err := composer.ComposeService(name)
		if err != nil {
			return nil, err
		}
		serviceDef := map[string]interface{}{
			"image":       svc.Image,
			"working_dir": svc.Workdir,
			"command":     []string{"sleep", "infinity"},
			"volumes":     buildServiceVolumes(svc, cfg),
			"networks":    []string{ws.Runtime.Network},
		}

		ports := buildPortMappings(svc)
		if len(ports) > 0 {
			serviceDef["ports"] = ports
		}
		if len(cfg.RuntimeEnv) > 0 {
			serviceDef["environment"] = cfg.RuntimeEnv
		}
		if len(cfg.DependsOn) > 0 {
			serviceDef["depends_on"] = cfg.DependsOn
		}

		services[name] = serviceDef
	}
	return services, nil
}
```

Add this helper:

```go
func buildServiceVolumes(svc domain.Service, cfg appcontract.ServiceConfig) []string {
	volumes := []string{svc.RepoPath + ":" + svc.Workdir}
	for _, file := range cfg.Files {
		volumes = append(volumes, file.Source+":"+file.Target+":ro")
	}
	for _, volume := range cfg.Volumes {
		volumes = append(volumes, volume.Source+":"+volume.Target)
	}
	return volumes
}
```

Keep existing addon rendering unchanged.

- [ ] **Step 4: Remove duplicated generated env logic**

Delete `buildServiceEnv` from `internal/runtime/compose.go` after `internal/appcontract.Composer` owns generated env. Run the runtime tests before editing any other file.

- [ ] **Step 5: Run runtime tests**

Run:

```bash
go test ./internal/runtime -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/runtime/compose.go internal/runtime/compose_test.go
git commit -m "feat: render app contract into compose"
```

## Task 5: Config Inspector API

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write API test**

Add this test to `internal/api/handlers_test.go`:

```go
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
		Env []struct {
			Name string `json:"name"`
			Value string `json:"value"`
			Secret bool `json:"secret"`
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
```

- [ ] **Step 2: Run API test and verify it fails**

Run:

```bash
go test ./internal/api -run TestGetServiceConfigReturnsRedactedConfigInspector -count=1
```

Expected: FAIL with 404 because the route is not registered.

- [ ] **Step 3: Register API route**

In `internal/api/server.go`, add the route near the other service routes:

```go
s.Mux.HandleFunc("GET /api/services/{service}/config", s.handleGetServiceConfig)
```

- [ ] **Step 4: Implement handler**

In `internal/api/handlers.go`, import `github.com/wnzhone/onespace/internal/appcontract` and add:

```go
func (s *Server) handleGetServiceConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("service")
	if _, ok := s.Workspace.Services[name]; !ok {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	cfg, err := appcontract.Composer{Workspace: s.Workspace}.ComposeService(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}
```

- [ ] **Step 5: Run API tests**

Run:

```bash
go test ./internal/api -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/api/server.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: expose config inspector api"
```

## Task 6: CLI Config Command

**Files:**
- Modify: `internal/cli/client.go`
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/output.go`
- Test: `internal/cli/commands_test.go`

- [ ] **Step 1: Write CLI test**

Add this test to `internal/cli/commands_test.go`:

```go
func TestConfigCommandPrintsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/services/user-api/config" {
			t.Fatalf("path = %q, want /api/services/user-api/config", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"user-api","env":[{"name":"DB_PASSWORD","value":"******","source":".secrets/db_password","secret":true}],"files":[],"volumes":[],"dependsOn":[]}`))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
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
```

- [ ] **Step 2: Run CLI test and verify it fails**

Run:

```bash
go test ./internal/cli -run TestConfigCommandPrintsJSON -count=1
```

Expected: FAIL because the CLI does not know the `config` command.

- [ ] **Step 3: Add client method and response structs**

In `internal/cli/client.go`, add:

```go
type ServiceConfig struct {
	Service   string        `json:"service"`
	Env       []ConfigEnv   `json:"env"`
	Files     []ConfigFile  `json:"files"`
	Volumes   []ConfigVolume `json:"volumes"`
	DependsOn []string      `json:"dependsOn"`
	Warnings  []ConfigWarning `json:"warnings,omitempty"`
}

type ConfigEnv struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Source string `json:"source"`
	Secret bool   `json:"secret"`
}

type ConfigFile struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Mode   string `json:"mode"`
	Secret bool   `json:"secret"`
}

type ConfigVolume struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type ConfigWarning struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

func (c Client) Config(ctx context.Context, service string) (ServiceConfig, error) {
	var cfg ServiceConfig
	err := c.getJSON(ctx, "/api/services/"+service+"/config", &cfg)
	return cfg, err
}
```

- [ ] **Step 4: Add output formatter**

In `internal/cli/output.go`, add:

```go
func WriteConfigText(w io.Writer, cfg ServiceConfig) {
	fmt.Fprintf(w, "SERVICE %s\n\n", cfg.Service)
	fmt.Fprintln(w, "ENV")
	for _, entry := range cfg.Env {
		secret := ""
		if entry.Secret {
			secret = " secret"
		}
		fmt.Fprintf(w, "%-24s %-16s %s%s\n", entry.Name, entry.Value, entry.Source, secret)
	}
	fmt.Fprintln(w, "\nFILES")
	for _, file := range cfg.Files {
		secret := ""
		if file.Secret {
			secret = " secret"
		}
		fmt.Fprintf(w, "%-32s %-32s %s%s\n", file.Target, file.Source, file.Mode, secret)
	}
	fmt.Fprintln(w, "\nVOLUMES")
	for _, volume := range cfg.Volumes {
		fmt.Fprintf(w, "%-32s %-32s %s\n", volume.Target, volume.Source, volume.Type)
	}
	if len(cfg.DependsOn) > 0 {
		fmt.Fprintln(w, "\nDEPENDS ON")
		for _, dep := range cfg.DependsOn {
			fmt.Fprintln(w, dep)
		}
	}
}
```

- [ ] **Step 5: Add command wiring**

In `internal/cli/commands.go`, update usage text to include `config`, add a switch case:

```go
case "config":
	return runConfig(client, args[1:], stdout, stderr)
```

Add:

```go
func runConfig(client Client, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "JSON output")
	fs.SetOutput(stderr)
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return 2
	}
	serviceArgs := fs.Args()
	if len(serviceArgs) == 0 {
		fmt.Fprintln(stderr, "onespace config: service name required")
		return 2
	}

	cfg, err := client.Config(context.Background(), serviceArgs[0])
	if err != nil {
		fmt.Fprintf(stderr, "onespace config: %v\n", err)
		return 1
	}
	if *jsonOutput {
		WriteJSON(stdout, cfg)
	} else {
		WriteConfigText(stdout, cfg)
	}
	return 0
}
```

- [ ] **Step 6: Run CLI tests**

Run:

```bash
go test ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/cli/client.go internal/cli/commands.go internal/cli/output.go internal/cli/commands_test.go
git commit -m "feat: add config inspector cli"
```

## Task 7: Web UI Config Inspector

**Files:**
- Modify: `web/static/app.js`
- Modify: `web/static/styles.css`

- [ ] **Step 1: Add Config action button**

In `web/static/app.js`, update the actions block in `renderServices`:

```js
<button data-action="deploy" data-service="${service.name}">Deploy</button>
<button data-action="debug" data-service="${service.name}">Debug</button>
<button data-action="logs" data-service="${service.name}">Logs</button>
<button data-action="config" data-service="${service.name}">Config</button>
```

- [ ] **Step 2: Add config rendering functions**

In `web/static/app.js`, add:

```js
function renderConfig(config) {
  const envRows = config.env.map((entry) => `
    <tr>
      <td>${escapeHTML(entry.name)}</td>
      <td>${escapeHTML(entry.value)}</td>
      <td>${escapeHTML(entry.source)}</td>
      <td>${entry.secret ? "yes" : "no"}</td>
    </tr>
  `).join("");
  const fileRows = config.files.map((file) => `
    <tr>
      <td>${escapeHTML(file.target)}</td>
      <td>${escapeHTML(file.source)}</td>
      <td>${escapeHTML(file.mode)}</td>
      <td>${file.secret ? "yes" : "no"}</td>
    </tr>
  `).join("");
  const volumeRows = config.volumes.map((volume) => `
    <tr>
      <td>${escapeHTML(volume.target)}</td>
      <td>${escapeHTML(volume.source)}</td>
      <td>${escapeHTML(volume.type)}</td>
    </tr>
  `).join("");
  activityEl.innerHTML = `
    <div class="config-view">
      <h3>${escapeHTML(config.service)} config</h3>
      <h4>Env</h4>
      <table><thead><tr><th>Name</th><th>Value</th><th>Source</th><th>Secret</th></tr></thead><tbody>${envRows}</tbody></table>
      <h4>Files</h4>
      <table><thead><tr><th>Target</th><th>Source</th><th>Mode</th><th>Secret</th></tr></thead><tbody>${fileRows}</tbody></table>
      <h4>Volumes</h4>
      <table><thead><tr><th>Target</th><th>Source</th><th>Type</th></tr></thead><tbody>${volumeRows}</tbody></table>
      <h4>Depends On</h4>
      <pre>${escapeHTML((config.dependsOn || []).join("\\n"))}</pre>
    </div>
  `;
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
```

- [ ] **Step 3: Wire config action**

Update `runAction` in `web/static/app.js`:

```js
if (action === "config") {
  const config = await api(`/api/services/${service}/config`);
  renderConfig(config);
  return;
}
```

Keep the existing logs and deploy/debug paths unchanged.

- [ ] **Step 4: Add CSS**

In `web/static/styles.css`, add:

```css
.config-view {
  overflow: auto;
}

.config-view table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 18px;
  font-size: 13px;
}

.config-view th,
.config-view td {
  border-bottom: 1px solid #d8dee4;
  padding: 8px;
  text-align: left;
  vertical-align: top;
}

.config-view th {
  color: #57606a;
  font-weight: 600;
}
```

- [ ] **Step 5: Run Go tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add web/static/app.js web/static/styles.css
git commit -m "feat: show config inspector in web ui"
```

## Task 8: Example Workspace And Docs

**Files:**
- Modify: `examples/workspaces/smoke-go/onespace.yaml`
- Create: `examples/workspaces/smoke-go/.env`
- Create: `examples/workspaces/smoke-go/.env.local`
- Create: `examples/workspaces/smoke-go/config/local.yaml`
- Create: `examples/workspaces/smoke-go/.secrets/db_password.example`
- Modify: `docs/user-guide.md`

- [ ] **Step 1: Update smoke Go workspace config**

Add contract fields under `services.user-api` in `examples/workspaces/smoke-go/onespace.yaml`:

```yaml
    env:
      APP_ENV: local
    envFrom:
      - file: .env
      - file: .env.local
        optional: true
    files:
      - source: config/local.yaml
        target: /etc/user-api/config.yaml
        mode: "0444"
    secrets:
      - name: DB_PASSWORD
        fromFile: .secrets/db_password.example
    volumes:
      - source: onespace-smoke-user-api-cache
        target: /workspace/.cache
    dependsOn:
      - redis
```

Ensure the smoke workspace has an addon named `redis`; if it does not, add:

```yaml
addons:
  redis:
    image: redis:7-alpine
```

- [ ] **Step 2: Add example config files**

Create `examples/workspaces/smoke-go/.env`:

```text
LOG_LEVEL=info
FEATURE_FLAG_SMOKE=true
```

Create `examples/workspaces/smoke-go/.env.local`:

```text
LOG_LEVEL=debug
```

Create `examples/workspaces/smoke-go/config/local.yaml`:

```yaml
service:
  name: user-api
  mode: local
```

Create `examples/workspaces/smoke-go/.secrets/db_password.example`:

```text
local-dev-password
```

- [ ] **Step 3: Document the new schema**

In `docs/user-guide.md`, add a section after current workspace configuration:

````markdown
### 运行配置契约

服务可以声明环境变量、环境文件、配置文件、secret 文件、volume 和启动依赖：

```yaml
env:
  APP_ENV: local
envFrom:
  - file: .env
  - file: .env.local
    optional: true
files:
  - source: config/local.yaml
    target: /etc/user-api/config.yaml
    mode: "0444"
secrets:
  - name: DB_PASSWORD
    fromFile: .secrets/db_password.example
volumes:
  - source: onespace-user-api-cache
    target: /workspace/.cache
dependsOn:
  - redis
```

`onespace config <service> --json` 可以查看最终配置及来源。Secret 值在 CLI、API 和 Web UI 中显示为 `******`。
````

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add examples/workspaces/smoke-go docs/user-guide.md
git commit -m "docs: document local docker app contract"
```

## Task 9: End-To-End Verification

**Files:**
- No source changes unless verification exposes a defect in the implemented tasks.

- [ ] **Step 1: Run full tests**

Run:

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Build binaries**

Run:

```bash
go build ./cmd/onespace-server
go build ./cmd/onespace
```

Expected: both commands exit 0.

- [ ] **Step 3: Generate Compose through daemon startup**

Run:

```bash
go run ./cmd/onespace-server serve --config examples/workspaces/smoke-go/onespace.yaml
```

Expected: daemon starts and writes `examples/workspaces/smoke-go/generated/docker-compose.yml`. Stop the daemon after generated file verification.

- [ ] **Step 4: Inspect generated Compose**

Run:

```bash
rg -n "APP_ENV|LOG_LEVEL|/etc/user-api/config.yaml|DB_PASSWORD|depends_on|onespace-smoke-user-api-cache" examples/workspaces/smoke-go/generated/docker-compose.yml
```

Expected: matches for each configured env, file mount, secret env, dependency, and volume.

- [ ] **Step 5: Verify CLI config output**

With the daemon still running or restarted, run:

```bash
go run ./cmd/onespace config user-api --json
```

Expected: JSON contains `APP_ENV`, `LOG_LEVEL`, `DB_PASSWORD`, and `DB_PASSWORD` has `"value":"******"`.

- [ ] **Step 6: Commit any verification fixes**

If Steps 1-5 expose a defect and you fix source files, commit with:

```bash
git add internal/appcontract internal/config internal/runtime internal/api internal/cli web/static examples/workspaces/smoke-go docs/user-guide.md
git commit -m "fix: complete local docker config inspector verification"
```

If no fixes are required, do not create an empty commit.

## Self-Review Checklist

- Every task maps to the 2026-05-06 design spec.
- No task adds file watching, automatic rebuild/restart, test timeline, K8s, kind, k3d, Terraform, or drift check.
- Secret values are redacted in API, CLI, UI, and persisted job-facing output.
- Existing deploy/debug/logs/health behavior remains compatible.
- Docker Compose remains the only runtime materialization path.
