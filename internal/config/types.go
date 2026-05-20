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
	Kind        string            `yaml:"kind"`
	Language    string            `yaml:"language"`
	RepoPath    string            `yaml:"repoPath"`
	Workdir     string            `yaml:"workdir"`
	Image       string            `yaml:"image"`
	Command     string            `yaml:"command"`
	Main        string            `yaml:"main"`
	Ports       []portYAML        `yaml:"ports"`
	Health      healthYAML        `yaml:"health"`
	Build       commandYAML       `yaml:"build"`
	Run         commandYAML       `yaml:"run"`
	Debug       debugYAML         `yaml:"debug"`
	Env         map[string]string `yaml:"env"`
	EnvFrom     []envFromYAML     `yaml:"envFrom"`
	Files       []fileYAML        `yaml:"files"`
	Secrets     []secretEnvYAML   `yaml:"secrets"`
	SecretFiles []fileYAML        `yaml:"secretFiles"`
	Volumes     []volumeYAML      `yaml:"volumes"`
	DependsOn   []string          `yaml:"dependsOn"`
}

type portYAML struct {
	Name      string `yaml:"name"`
	Container int    `yaml:"container"`
	Host      int    `yaml:"host"`
	Protocol  string `yaml:"protocol"`
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

type envFromYAML struct {
	File     string `yaml:"file"`
	Optional bool   `yaml:"optional"`
}

type fileYAML struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Mode   string `yaml:"mode"`
}

type secretEnvYAML struct {
	Name     string `yaml:"name"`
	FromFile string `yaml:"fromFile"`
}

type volumeYAML struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

type addonYAML struct {
	Image string            `yaml:"image"`
	Ports []string          `yaml:"ports"`
	Env   map[string]string `yaml:"env"`
}
