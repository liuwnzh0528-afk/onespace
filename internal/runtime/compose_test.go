package runtime

import (
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
