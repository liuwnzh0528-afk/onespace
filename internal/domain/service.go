package domain

type Service struct {
	Name        string
	Language    string
	RepoPath    string
	Workdir     string
	Image       string
	Main        string
	Ports       []Port
	Health      HealthCheck
	Build       Command
	Run         Command
	Debug       DebugConfig
	Env         map[string]string
	EnvFrom     []EnvFrom
	Files       []FileMount
	Secrets     []SecretEnv
	SecretFiles []FileMount
	Volumes     []VolumeMount
	DependsOn   []string
}

type Port struct {
	Name      string
	Container int
	Host      int
}

type HealthCheck struct {
	Type           string
	URL            string
	TimeoutSeconds int
}

type Command struct {
	Command string
}

type DebugConfig struct {
	Port         int
	BuildCommand string
	Command      string
}

type EnvFrom struct {
	File     string
	Optional bool
}

type FileMount struct {
	Source string
	Target string
	Mode   string
}

type SecretEnv struct {
	Name     string
	FromFile string
}

type VolumeMount struct {
	Source string
	Target string
}
