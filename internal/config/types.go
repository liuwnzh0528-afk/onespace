package config

type workspaceYAML struct {
	Version          int                    `yaml:"version"`
	Name             string                 `yaml:"name"`
	AllowedRepoRoots []string               `yaml:"allowedRepoRoots"`
	Server           serverYAML             `yaml:"server"`
	Runtime          runtimeYAML            `yaml:"runtime"`
	Ports            portRangesYAML         `yaml:"ports"`
	Services         map[string]serviceYAML `yaml:"services"`
	Addons           map[string]addonYAML   `yaml:"addons"`
}

type serverYAML struct {
	Bind string `yaml:"bind"`
}

type runtimeYAML struct {
	Type        string `yaml:"type"`
	ProjectName string `yaml:"projectName"`
	Network     string `yaml:"network"`
}

type portRangesYAML struct {
	AppRange   string `yaml:"appRange"`
	DebugRange string `yaml:"debugRange"`
}

type serviceYAML struct {
	Language string      `yaml:"language"`
	RepoPath string      `yaml:"repoPath"`
	Workdir  string      `yaml:"workdir"`
	Image    string      `yaml:"image"`
	Main     string      `yaml:"main"`
	Ports    []portYAML  `yaml:"ports"`
	Health   healthYAML  `yaml:"health"`
	Build    commandYAML `yaml:"build"`
	Run      commandYAML `yaml:"run"`
	Debug    debugYAML   `yaml:"debug"`
}

type portYAML struct {
	Name      string `yaml:"name"`
	Container int    `yaml:"container"`
	Host      int    `yaml:"host"`
}

type healthYAML struct {
	Type           string `yaml:"type"`
	URL            string `yaml:"url"`
	TimeoutSeconds int    `yaml:"timeoutSeconds"`
}

type commandYAML struct {
	Command string `yaml:"command"`
}

type debugYAML struct {
	Port         int    `yaml:"port"`
	BuildCommand string `yaml:"buildCommand"`
	Command      string `yaml:"command"`
}

type addonYAML struct {
	Image string            `yaml:"image"`
	Ports []string          `yaml:"ports"`
	Env   map[string]string `yaml:"env"`
}
