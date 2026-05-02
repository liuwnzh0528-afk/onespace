package runtime

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/wnzhone/onespace/internal/domain"
)

func TestGenerateComposeIncludesDevRunnerVolumesPortsAndAddons(t *testing.T) {
	ws := domain.Workspace{
		Name: "order-system-dev",
		Runtime: domain.RuntimeConfig{
			Type:        "docker-compose",
			ProjectName: "order-system-dev",
			Network:     "order-system-dev-default",
		},
		Services: map[string]domain.Service{
			"user-api": {
				Name:     "user-api",
				Language: "go",
				RepoPath: "/data/repos/user-api",
				Workdir:  "/workspace",
				Image:    "onespace/go-dev:1.23",
				Main:     "./cmd/user-api",
				Ports: []domain.Port{
					{Name: "http", Container: 8080, Host: 18081},
				},
				Debug: domain.DebugConfig{Port: 40001},
			},
			"order-api": {
				Name:     "order-api",
				Language: "java-maven",
				RepoPath: "/data/repos/order-api",
				Workdir:  "/workspace",
				Image:    "onespace/java-dev:21-maven",
				Ports: []domain.Port{
					{Name: "http", Container: 8080, Host: 18082},
				},
				Debug: domain.DebugConfig{Port: 40002},
			},
		},
		Addons: map[string]domain.Addon{
			"redis": {
				Image: "redis:7-alpine",
				Ports: []string{"6379:6379"},
			},
		},
	}

	data, err := GenerateCompose(ws)
	if err != nil {
		t.Fatalf("GenerateCompose: %v", err)
	}

	yamlStr := string(data)

	for _, name := range []string{"user-api", "order-api", "redis"} {
		if !strings.Contains(yamlStr, name) {
			t.Errorf("compose YAML missing service %q", name)
		}
	}

	if !strings.Contains(yamlStr, "/data/repos/user-api:/workspace") {
		t.Error("compose YAML missing repo mount for user-api")
	}
	if !strings.Contains(yamlStr, "/data/repos/order-api:/workspace") {
		t.Error("compose YAML missing repo mount for order-api")
	}

	if !strings.Contains(yamlStr, "go-cache-user-api") {
		t.Error("compose YAML missing Go cache volume")
	}
	if !strings.Contains(yamlStr, "maven-cache-order-api") {
		t.Error("compose YAML missing Maven cache volume")
	}

	if !strings.Contains(yamlStr, "18081:8080") {
		t.Error("compose YAML missing port mapping for user-api")
	}
	if !strings.Contains(yamlStr, "18082:8080") {
		t.Error("compose YAML missing port mapping for order-api")
	}

	if !strings.Contains(yamlStr, "order-system-dev-default") {
		t.Error("compose YAML missing network")
	}
}

func TestExecStartsServiceBeforeDockerExec(t *testing.T) {
	var calls []string
	runner := &FakeCommandRunner{
		RunFunc: func(ctx context.Context, dir string, name string, args []string, stdout io.Writer, stderr io.Writer) error {
			calls = append(calls, strings.Join(args, " "))
			return nil
		},
	}
	runtime := &ComposeRuntime{Runner: runner}

	err := runtime.Exec(context.Background(), ExecOptions{
		WorkspaceRoot: "/workspace-root",
		Service:       "user-api",
		Command:       "go build ./cmd/user-api",
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("calls = %v, want start then exec", calls)
	}
	if calls[0] != "compose -f generated/docker-compose.yml start user-api" {
		t.Fatalf("first call = %q, want compose start", calls[0])
	}
	if calls[1] != "compose -f generated/docker-compose.yml exec -T user-api sh -lc go build ./cmd/user-api" {
		t.Fatalf("second call = %q, want compose exec", calls[1])
	}
}
