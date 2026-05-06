package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wnzhone/onespace/internal/appcontract"
	"github.com/wnzhone/onespace/internal/domain"
)

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

	for name, addon := range ws.Addons {
		addonDef := map[string]interface{}{
			"image":    addon.Image,
			"networks": []string{ws.Runtime.Network},
		}
		if len(addon.Ports) > 0 {
			addonDef["ports"] = addon.Ports
		}
		if len(addon.Env) > 0 {
			addonDef["environment"] = addon.Env
		}
		services[name] = addonDef
	}

	return services, nil
}

func buildPortMappings(svc domain.Service) []string {
	var ports []string
	for _, p := range svc.Ports {
		ports = append(ports, fmt.Sprintf("%d:%d", p.Host, p.Container))
	}
	if svc.Debug.Port != 0 {
		ports = append(ports, fmt.Sprintf("%d:%d", svc.Debug.Port, svc.Debug.Port))
	}
	return ports
}

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

func buildVolumes(ws domain.Workspace) map[string]interface{} {
	volumes := make(map[string]interface{})
	for _, svc := range ws.Services {
		switch svc.Language {
		case "go":
			volumes["go-cache-"+svc.Name] = map[string]interface{}{}
		case "java-maven":
			volumes["maven-cache-"+svc.Name] = map[string]interface{}{}
		}
		for _, volume := range svc.Volumes {
			if !strings.HasPrefix(volume.Source, "/") {
				volumes[volume.Source] = map[string]interface{}{}
			}
		}
	}
	return volumes
}

type ComposeRuntime struct {
	Runner CommandRunner
}

func (r *ComposeRuntime) Ensure(ctx context.Context, workspaceRoot string) error {
	composeFile := filepath.Join(workspaceRoot, "generated", "docker-compose.yml")
	dir := filepath.Dir(composeFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create generated dir: %w", err)
	}

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("compose file not found: %s", composeFile)
	}

	var stdout, stderr strings.Builder
	err := r.Runner.Run(ctx, workspaceRoot, "docker", []string{"compose", "-f", "generated/docker-compose.yml", "up", "-d", "--no-start"}, &stdout, &stderr)
	if err != nil {
		return fmt.Errorf("docker compose ensure: %w: %s", err, stderr.String())
	}
	return nil
}

// ensureServiceRunning starts the service container if it is not already running.
// docker compose start is a no-op when the container is already running, so it
// is safe to call unconditionally before exec-ing into a container.
func (r *ComposeRuntime) ensureServiceRunning(ctx context.Context, workspaceRoot, service string) error {
	var stdout, stderr strings.Builder
	args := []string{"compose", "-f", "generated/docker-compose.yml", "start", service}
	err := r.Runner.Run(ctx, workspaceRoot, "docker", args, &stdout, &stderr)
	if err != nil {
		return fmt.Errorf("docker compose start %s: %w: %s", service, err, stderr.String())
	}
	return nil
}

func (r *ComposeRuntime) Exec(ctx context.Context, opts ExecOptions) error {
	if err := r.ensureServiceRunning(ctx, opts.WorkspaceRoot, opts.Service); err != nil {
		return err
	}
	var stderr strings.Builder
	args := []string{"compose", "-f", "generated/docker-compose.yml", "exec", "-T", opts.Service, "sh", "-c", opts.Command}
	err := r.Runner.Run(ctx, opts.WorkspaceRoot, "docker", args, opts.Stdout, &stderr)
	if err != nil {
		return fmt.Errorf("docker compose exec: %w: %s", err, stderr.String())
	}
	return nil
}

func (r *ComposeRuntime) StopProcess(ctx context.Context, workspaceRoot string, service string) error {
	if err := r.ensureServiceRunning(ctx, workspaceRoot, service); err != nil {
		return err
	}
	var stdout, stderr strings.Builder
	args := []string{"compose", "-f", "generated/docker-compose.yml", "exec", "-T", service, "onespace-supervisor", "stop"}
	err := r.Runner.Run(ctx, workspaceRoot, "docker", args, &stdout, &stderr)
	if err != nil {
		return fmt.Errorf("stop process: %w: %s", err, stderr.String())
	}
	return nil
}

func (r *ComposeRuntime) StartProcess(ctx context.Context, workspaceRoot string, service string, command string) error {
	if err := r.ensureServiceRunning(ctx, workspaceRoot, service); err != nil {
		return err
	}
	var stdout, stderr strings.Builder
	args := []string{"compose", "-f", "generated/docker-compose.yml", "exec", "-T", service, "onespace-supervisor", "start", command}
	err := r.Runner.Run(ctx, workspaceRoot, "docker", args, &stdout, &stderr)
	if err != nil {
		return fmt.Errorf("start process: %w: %s", err, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	args = []string{"compose", "-f", "generated/docker-compose.yml", "exec", "-T", service, "onespace-supervisor", "status"}
	err = r.Runner.Run(ctx, workspaceRoot, "docker", args, &stdout, &stderr)
	if err != nil {
		return fmt.Errorf("check process status: %w: %s", err, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "running" {
		return fmt.Errorf("process did not start: %s", strings.TrimSpace(stdout.String()))
	}
	return nil
}

func (r *ComposeRuntime) ServiceStatus(ctx context.Context, workspaceRoot string, service string) (ServiceStatus, error) {
	if err := r.ensureServiceRunning(ctx, workspaceRoot, service); err != nil {
		return ServiceStatus{Container: "unknown", Process: "unknown"}, nil
	}
	var stdout, stderr strings.Builder
	args := []string{"compose", "-f", "generated/docker-compose.yml", "exec", "-T", service, "onespace-supervisor", "status"}
	err := r.Runner.Run(ctx, workspaceRoot, "docker", args, &stdout, &stderr)
	if err != nil {
		return ServiceStatus{Container: "running", Process: "unknown"}, nil
	}
	container := "running"
	process := strings.TrimSpace(stdout.String())
	return ServiceStatus{Container: container, Process: process}, nil
}

func WriteComposeFile(workspaceRoot string, data []byte) error {
	dir := filepath.Join(workspaceRoot, "generated")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "docker-compose.yml"), data, 0o644)
}
