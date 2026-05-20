package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wnzhone/onespace/internal/domain"
)

func LoadWorkspace(path string) (domain.Workspace, error) {
	configPath, err := filepath.Abs(path)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("resolve config path: %w", err)
	}
	workspaceDir := filepath.Dir(configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("read config: %w", err)
	}

	var raw workspaceYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return domain.Workspace{}, fmt.Errorf("parse config: %w", err)
	}

	if raw.Version != 1 {
		return domain.Workspace{}, domain.ValidationError{Field: "version", Reason: "only version 1 is supported"}
	}
	if raw.Name == "" {
		return domain.Workspace{}, domain.ValidationError{Field: "name", Reason: "is required"}
	}
	allowedRoots := make([]string, 0, len(raw.AllowedRepoRoots))
	for _, root := range raw.AllowedRepoRoots {
		allowedRoots = append(allowedRoots, resolveWorkspacePath(workspaceDir, root))
	}

	ws := domain.Workspace{
		Version:          raw.Version,
		Name:             raw.Name,
		Path:             workspaceDir,
		AllowedRepoRoots: allowedRoots,
		Server: domain.ServerConfig{
			Bind: raw.Server.Bind,
		},
		Runtime: domain.RuntimeConfig{
			Type:        raw.Runtime.Type,
			ProjectName: raw.Runtime.ProjectName,
			Network:     raw.Runtime.Network,
		},
		Ports: domain.PortRanges{
			AppRange:   raw.Ports.AppRange,
			DebugRange: raw.Ports.DebugRange,
		},
	}

	if ws.Server.Bind == "" {
		ws.Server.Bind = "127.0.0.1:18080"
	}
	if ws.Runtime.Type == "" {
		ws.Runtime.Type = "docker-compose"
	}
	if ws.Runtime.ProjectName == "" {
		ws.Runtime.ProjectName = ws.Name
	}
	if ws.Runtime.Network == "" {
		ws.Runtime.Network = ws.Name + "-default"
	}

	ws.Services = make(map[string]domain.Service, len(raw.Services))
	serviceNames := make([]string, 0, len(raw.Services))
	for name := range raw.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)
	debugPortBase := portRangeStart(ws.Ports.DebugRange, 40000)
	for i, name := range serviceNames {
		svcYAML := raw.Services[name]
		svc, err := mapService(name, svcYAML, ws.AllowedRepoRoots, workspaceDir, debugPortBase+i)
		if err != nil {
			return domain.Workspace{}, err
		}
		ws.Services[name] = svc
	}

	ws.Addons = make(map[string]domain.Addon, len(raw.Addons))
	for name, addonYAML := range raw.Addons {
		ws.Addons[name] = domain.Addon{
			Image: addonYAML.Image,
			Ports: addonYAML.Ports,
			Env:   addonYAML.Env,
		}
	}

	return ws, nil
}

func mapService(name string, raw serviceYAML, allowedRoots []string, workspaceDir string, defaultDebugPort int) (domain.Service, error) {
	kind := raw.Kind
	if kind == "" {
		kind = domain.ServiceKindDevRunner
	}

	svc := domain.Service{
		Name:             name,
		Kind:             kind,
		Language:         raw.Language,
		Workdir:          raw.Workdir,
		Image:            raw.Image,
		ContainerCommand: raw.Command,
		Main:             raw.Main,
		Health: domain.HealthCheck{
			Type:           raw.Health.Type,
			URL:            raw.Health.URL,
			TimeoutSeconds: raw.Health.TimeoutSeconds,
		},
		Build: domain.Command{Command: raw.Build.Command},
		Run:   domain.Command{Command: raw.Run.Command},
	}

	for _, p := range raw.Ports {
		protocol := strings.ToLower(strings.TrimSpace(p.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		if protocol != "tcp" && protocol != "udp" {
			return domain.Service{}, domain.ValidationError{
				Field:  "services." + name + ".ports." + p.Name + ".protocol",
				Reason: fmt.Sprintf("unsupported protocol %q", p.Protocol),
			}
		}
		svc.Ports = append(svc.Ports, domain.Port{
			Name:      p.Name,
			Container: p.Container,
			Host:      p.Host,
			Protocol:  protocol,
		})
	}
	mapRuntimeContract(&svc, raw, workspaceDir)

	switch kind {
	case domain.ServiceKindContainer:
		if svc.Image == "" {
			return domain.Service{}, domain.ValidationError{Field: "services." + name + ".image", Reason: "is required for container services"}
		}
		return svc, nil
	case domain.ServiceKindDevRunner:
		return mapDevRunnerService(name, raw, allowedRoots, workspaceDir, defaultDebugPort, svc)
	default:
		return domain.Service{}, domain.ValidationError{
			Field:  "services." + name + ".kind",
			Reason: fmt.Sprintf("unsupported kind %q", kind),
		}
	}
}

func mapDevRunnerService(name string, raw serviceYAML, allowedRoots []string, workspaceDir string, defaultDebugPort int, svc domain.Service) (domain.Service, error) {
	if raw.Language == "" {
		return domain.Service{}, domain.ValidationError{Field: "services." + name + ".language", Reason: "is required"}
	}
	if raw.RepoPath == "" {
		return domain.Service{}, domain.ValidationError{Field: "services." + name + ".repoPath", Reason: "is required"}
	}
	if len(allowedRoots) == 0 {
		return domain.Service{}, domain.ValidationError{Field: "allowedRepoRoots", Reason: "at least one root is required for dev-runner services"}
	}
	repoPath := resolveWorkspacePath(workspaceDir, raw.RepoPath)
	if !pathUnderAnyRoot(repoPath, allowedRoots) {
		return domain.Service{}, domain.ValidationError{
			Field:  "services." + name + ".repoPath",
			Reason: fmt.Sprintf("%q is not under any allowedRepoRoot", repoPath),
		}
	}
	svc.RepoPath = repoPath

	debugPort := raw.Debug.Port
	if debugPort == 0 {
		debugPort = defaultDebugPort
	}
	svc.Debug = domain.DebugConfig{
		Port:         debugPort,
		BuildCommand: raw.Debug.BuildCommand,
		Command:      raw.Debug.Command,
	}

	var defaults languageDefaults
	switch raw.Language {
	case "go":
		defaults = defaultsForGo(raw.Main, debugPort)
	case "java-maven":
		defaults = defaultsForJavaMaven(debugPort)
	default:
		return domain.Service{}, domain.ValidationError{
			Field:  "services." + name + ".language",
			Reason: fmt.Sprintf("unsupported language %q", raw.Language),
		}
	}

	if svc.Workdir == "" {
		svc.Workdir = defaults.Workdir
	}
	if svc.Image == "" {
		svc.Image = defaults.Image
	}
	if svc.Build.Command == "" {
		svc.Build.Command = defaults.BuildCommand
	}
	if svc.Run.Command == "" {
		svc.Run.Command = defaults.RunCommand
	}
	if svc.Debug.BuildCommand == "" {
		svc.Debug.BuildCommand = defaults.DebugBuild
	}
	if svc.Debug.Command == "" {
		svc.Debug.Command = defaults.DebugCommand
	}

	if svc.Health.Type == "" && len(svc.Ports) > 0 {
		svc.Health.Type = "http"
		hostPort := svc.Ports[0].Host
		svc.Health.URL = fmt.Sprintf("http://127.0.0.1:%d/health", hostPort)
		if svc.Health.TimeoutSeconds == 0 {
			svc.Health.TimeoutSeconds = 30
		}
	}

	return svc, nil
}

func mapRuntimeContract(svc *domain.Service, raw serviceYAML, workspaceDir string) {
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
}

func resolveWorkspacePath(workspaceDir string, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(workspaceDir, path))
}

func isRelativeHostPath(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

func portRangeStart(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	start, _, _ := strings.Cut(raw, "-")
	port, err := strconv.Atoi(strings.TrimSpace(start))
	if err != nil || port <= 0 {
		return fallback
	}
	return port
}
