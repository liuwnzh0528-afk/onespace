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
	if svc.Workdir != "" {
		setEnv(cfg, "ONESPACE_STATE_DIR", svc.Workdir+"/.onespace", "generated runtime env", false)
	}
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
