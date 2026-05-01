package serviceops

import "encoding/json"

type Result struct {
	Service   string          `json:"service"`
	Status    string          `json:"status"`
	JobID     string          `json:"jobId"`
	Stage     string          `json:"stage,omitempty"`
	Commit    string          `json:"commit,omitempty"`
	Dirty     bool            `json:"dirty"`
	Container string          `json:"container,omitempty"`
	Health    string          `json:"health,omitempty"`
	URL       string          `json:"url,omitempty"`
	Debug     *DebugAttach    `json:"debug,omitempty"`
	LogRef    string          `json:"logRef,omitempty"`
	ExitCode  int             `json:"exitCode,omitempty"`
	Details   json.RawMessage `json:"details,omitempty"`
}

type DebugAttach struct {
	Debugger string `json:"debugger"`
	Address  string `json:"address"`
}
