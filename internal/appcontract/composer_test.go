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
