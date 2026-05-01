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
