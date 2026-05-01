package jobs

import "time"

type Type string
type Status string

const (
	TypePull    Type = "pull"
	TypeBuild   Type = "build"
	TypeUp      Type = "up"
	TypeRestart Type = "restart"
	TypeDeploy  Type = "deploy"
	TypeDebug   Type = "debug"
	TypeStop    Type = "stop"
	TypeLogs    Type = "logs"
	TypeHealth  Type = "health"
)

const (
	StatusQueued   Status = "queued"
	StatusRunning  Status = "running"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

type Job struct {
	ID         string    `json:"id"`
	Type       Type      `json:"type"`
	Workspace  string    `json:"workspace"`
	Service    string    `json:"service"`
	Status     Status    `json:"status"`
	Stage      string    `json:"stage"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
	ExitCode   int       `json:"exitCode,omitempty"`
	LogRef     string    `json:"logRef,omitempty"`
	Result     []byte    `json:"result,omitempty"`
}
