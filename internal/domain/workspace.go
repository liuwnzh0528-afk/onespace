package domain

type Workspace struct {
	Version          int
	Name             string
	Path             string
	AllowedRepoRoots []string
	Server           ServerConfig
	Runtime          RuntimeConfig
	Ports            PortRanges
	Services         map[string]Service
	Addons           map[string]Addon
}

type ServerConfig struct {
	Bind string
}

type RuntimeConfig struct {
	Type        string
	ProjectName string
	Network     string
}

type PortRanges struct {
	AppRange   string
	DebugRange string
}

type Addon struct {
	Image string
	Ports []string
	Env   map[string]string
}
