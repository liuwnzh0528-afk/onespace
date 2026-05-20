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
	if !strings.Contains(yamlStr, "/go/bin:/usr/local/go/bin") {
		t.Error("compose YAML missing Go tool PATH")
	}
	if !strings.Contains(yamlStr, "/opt/java/openjdk/bin") {
		t.Error("compose YAML missing Java tool PATH")
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
	if !strings.Contains(yamlStr, "name: order-system-dev") {
		t.Error("compose YAML missing project name")
	}
}

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

func TestGenerateComposeIncludesContainerServiceWithUDPPort(t *testing.T) {
	ws := domain.Workspace{
		Name: "metal-forge-dev",
		Runtime: domain.RuntimeConfig{
			Type:        "docker-compose",
			ProjectName: "metal-forge-dev",
			Network:     "metal-forge-dev-default",
		},
		Services: map[string]domain.Service{
			"bmc-a": {
				Name:             "bmc-a",
				Kind:             "container",
				Image:            "metal-forge/mock-ipmi:dev",
				ContainerCommand: "python3 /app/server.py",
				Env: map[string]string{
					"MOCK_IPMI_NAME": "bmc-a",
				},
				Ports: []domain.Port{
					{Name: "ipmi", Container: 623, Host: 6230, Protocol: "udp"},
				},
			},
		},
	}

	data, err := GenerateCompose(ws)
	if err != nil {
		t.Fatalf("GenerateCompose: %v", err)
	}
	yamlStr := string(data)

	for _, want := range []string{
		"image: metal-forge/mock-ipmi:dev",
		"command: python3 /app/server.py",
		"MOCK_IPMI_NAME: bmc-a",
		"6230:623/udp",
	} {
		if !strings.Contains(yamlStr, want) {
			t.Fatalf("compose YAML missing %q:\n%s", want, yamlStr)
		}
	}
	for _, notWant := range []string{
		"sleep",
		"/workspace",
		"ONESPACE_STATE_DIR",
		"go-cache-bmc-a",
	} {
		if strings.Contains(yamlStr, notWant) {
			t.Fatalf("compose YAML contains dev-runner field %q:\n%s", notWant, yamlStr)
		}
	}
}

func TestComposeRuntimeServiceLifecycleUsesComposeServiceCommands(t *testing.T) {
	var calls []string
	runner := &FakeCommandRunner{
		RunFunc: func(ctx context.Context, dir string, name string, args []string, stdout io.Writer, stderr io.Writer) error {
			calls = append(calls, strings.Join(args, " "))
			return nil
		},
	}
	runtime := &ComposeRuntime{Runner: runner}

	if err := runtime.UpService(context.Background(), "/workspace-root", "bmc-a"); err != nil {
		t.Fatalf("UpService returned error: %v", err)
	}
	if err := runtime.RestartService(context.Background(), "/workspace-root", "bmc-a"); err != nil {
		t.Fatalf("RestartService returned error: %v", err)
	}
	if err := runtime.StopService(context.Background(), "/workspace-root", "bmc-a"); err != nil {
		t.Fatalf("StopService returned error: %v", err)
	}

	want := []string{
		"compose -f generated/docker-compose.yml up -d bmc-a",
		"compose -f generated/docker-compose.yml restart bmc-a",
		"compose -f generated/docker-compose.yml stop bmc-a",
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestComposeRuntimeServiceLogsReturnsTailWithoutComposePrefixes(t *testing.T) {
	runner := &FakeCommandRunner{
		RunFunc: func(ctx context.Context, dir string, name string, args []string, stdout io.Writer, stderr io.Writer) error {
			if got := strings.Join(args, " "); got != "compose -f generated/docker-compose.yml logs --no-color --no-log-prefix --tail 2 bmc-a" {
				t.Fatalf("args = %q, want compose logs tail", got)
			}
			_, _ = stdout.Write([]byte("line 1\nline 2\n"))
			return nil
		},
	}
	runtime := &ComposeRuntime{Runner: runner}

	lines, err := runtime.ServiceLogs(context.Background(), "/workspace-root", "bmc-a", 2)
	if err != nil {
		t.Fatalf("ServiceLogs returned error: %v", err)
	}
	if strings.Join(lines, "|") != "line 1|line 2" {
		t.Fatalf("lines = %v, want [line 1 line 2]", lines)
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
	if calls[1] != "compose -f generated/docker-compose.yml exec -T user-api sh -c go build ./cmd/user-api" {
		t.Fatalf("second call = %q, want compose exec", calls[1])
	}
}

func TestStartProcessChecksSupervisorStatus(t *testing.T) {
	var calls []string
	runner := &FakeCommandRunner{
		RunFunc: func(ctx context.Context, dir string, name string, args []string, stdout io.Writer, stderr io.Writer) error {
			calls = append(calls, strings.Join(args, " "))
			if strings.Contains(strings.Join(args, " "), "onespace-supervisor status") {
				_, _ = stdout.Write([]byte("running\n"))
			}
			return nil
		},
	}
	runtime := &ComposeRuntime{Runner: runner}

	err := runtime.StartProcess(context.Background(), "/workspace-root", "order-api", "java -jar target/*.jar")
	if err != nil {
		t.Fatalf("StartProcess returned error: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("calls = %v, want start container, supervisor start, supervisor status", calls)
	}
	if calls[2] != "compose -f generated/docker-compose.yml exec -T order-api onespace-supervisor status" {
		t.Fatalf("status call = %q, want supervisor status", calls[2])
	}
}
