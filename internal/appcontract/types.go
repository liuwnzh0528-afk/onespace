package appcontract

type ServiceConfig struct {
	Service    string            `json:"service"`
	Env        []EnvEntry        `json:"env"`
	Files      []FileEntry       `json:"files"`
	Volumes    []VolumeEntry     `json:"volumes"`
	DependsOn  []string          `json:"dependsOn"`
	Warnings   []Warning         `json:"warnings,omitempty"`
	RuntimeEnv map[string]string `json:"-"`
}

type EnvEntry struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Source string `json:"source"`
	Secret bool   `json:"secret"`
}

type FileEntry struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Mode   string `json:"mode"`
	Secret bool   `json:"secret"`
}

type VolumeEntry struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type Warning struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

func (c ServiceConfig) EnvValue(name string) EnvEntry {
	for _, entry := range c.Env {
		if entry.Name == name {
			return entry
		}
	}
	return EnvEntry{}
}
