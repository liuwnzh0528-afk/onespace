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
		t.Fatalf("Image = %q, want onespace/go-dev:1.23", svc.Image)
	}
	if svc.Build.Command == "" || svc.Run.Command == "" || svc.Debug.Command == "" {
		t.Fatalf("language defaults were not applied: %+v", svc)
	}
	if svc.Debug.Port != 40000 {
		t.Fatalf("Debug.Port = %d, want 40000", svc.Debug.Port)
	}
}

func TestLoadWorkspaceResolvesRelativePathsFromConfigDirectory(t *testing.T) {
	dir := t.TempDir()
	repoRoot := filepath.Join(dir, "repos")
	if err := os.MkdirAll(filepath.Join(repoRoot, "user-api"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "onespace.yaml")
	yaml := `
version: 1
name: relative-ws
allowedRepoRoots:
  - repos
services:
  user-api:
    language: go
    repoPath: repos/user-api
    main: ./cmd/user-api
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := LoadWorkspace(configPath)
	if err != nil {
		t.Fatalf("LoadWorkspace returned error: %v", err)
	}

	if ws.Path != dir {
		t.Fatalf("Workspace.Path = %q, want %q", ws.Path, dir)
	}
	if len(ws.AllowedRepoRoots) != 1 || ws.AllowedRepoRoots[0] != repoRoot {
		t.Fatalf("AllowedRepoRoots = %v, want [%q]", ws.AllowedRepoRoots, repoRoot)
	}
	svc := ws.Services["user-api"]
	if svc.RepoPath != filepath.Join(repoRoot, "user-api") {
		t.Fatalf("RepoPath = %q, want %q", svc.RepoPath, filepath.Join(repoRoot, "user-api"))
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

func TestLoadWorkspaceLoadsSmokeJavaExample(t *testing.T) {
	configPath := filepath.Join("..", "..", "examples", "workspaces", "smoke-java", "onespace.yaml")

	ws, err := LoadWorkspace(configPath)
	if err != nil {
		t.Fatalf("LoadWorkspace returned error: %v", err)
	}

	svc := ws.Services["order-api"]
	if svc.Language != "java-maven" {
		t.Fatalf("Language = %q, want java-maven", svc.Language)
	}
	if svc.Image != "onespace/java-dev:21-maven" {
		t.Fatalf("Image = %q, want onespace/java-dev:21-maven", svc.Image)
	}
	if svc.Build.Command != "mvn package -DskipTests" {
		t.Fatalf("Build command = %q, want Maven package default", svc.Build.Command)
	}
	if svc.Debug.Port != 40002 {
		t.Fatalf("Debug.Port = %d, want 40002", svc.Debug.Port)
	}
}

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
