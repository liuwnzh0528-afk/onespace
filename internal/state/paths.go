package state

import "path/filepath"

type Paths struct {
	WorkspaceRoot string
}

func (p Paths) StateDir() string {
	return filepath.Join(p.WorkspaceRoot, "state")
}

func (p Paths) JobsLogDir() string {
	return filepath.Join(p.StateDir(), "logs", "jobs")
}

func (p Paths) ServicesLogDir() string {
	return filepath.Join(p.StateDir(), "logs", "services")
}

func (p Paths) GeneratedDir() string {
	return filepath.Join(p.WorkspaceRoot, "generated")
}
