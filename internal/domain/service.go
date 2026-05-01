package domain

type Service struct {
	Name     string
	Language string
	RepoPath string
	Workdir  string
	Image    string
	Main     string
	Ports    []Port
	Health   HealthCheck
	Build    Command
	Run      Command
	Debug    DebugConfig
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
