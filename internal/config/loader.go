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
	if len(raw.AllowedRepoRoots) == 0 {
		return domain.Workspace{}, domain.ValidationError{Field: "allowedRepoRoots", Reason: "at least one root is required"}
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
	if raw.Language == "" {
		return domain.Service{}, domain.ValidationError{Field: "services." + name + ".language", Reason: "is required"}
	}
	if raw.RepoPath == "" {
		return domain.Service{}, domain.ValidationError{Field: "services." + name + ".repoPath", Reason: "is required"}
	}
	repoPath := resolveWorkspacePath(workspaceDir, raw.RepoPath)
	if !pathUnderAnyRoot(repoPath, allowedRoots) {
		return domain.Service{}, domain.ValidationError{
			Field:  "services." + name + ".repoPath",
			Reason: fmt.Sprintf("%q is not under any allowedRepoRoot", repoPath),
		}
	}

	debugPort := raw.Debug.Port
	if debugPort == 0 {
		debugPort = defaultDebugPort
	}

	svc := domain.Service{
		Name:     name,
		Language: raw.Language,
		RepoPath: repoPath,
		Workdir:  raw.Workdir,
		Image:    raw.Image,
		Main:     raw.Main,
		Health: domain.HealthCheck{
			Type:           raw.Health.Type,
			URL:            raw.Health.URL,
			TimeoutSeconds: raw.Health.TimeoutSeconds,
		},
		Build: domain.Command{Command: raw.Build.Command},
		Run:   domain.Command{Command: raw.Run.Command},
		Debug: domain.DebugConfig{
			Port:         debugPort,
			BuildCommand: raw.Debug.BuildCommand,
			Command:      raw.Debug.Command,
		},
	}

	for _, p := range raw.Ports {
		svc.Ports = append(svc.Ports, domain.Port{
			Name:      p.Name,
			Container: p.Container,
			Host:      p.Host,
		})
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

func resolveWorkspacePath(workspaceDir string, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(workspaceDir, path))
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
