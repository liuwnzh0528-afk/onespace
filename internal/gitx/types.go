package gitx

type Status struct {
	RepoPath       string `json:"repoPath"`
	Remote         string `json:"remote"`
	Branch         string `json:"branch"`
	TrackingBranch string `json:"trackingBranch"`
	Commit         string `json:"commit"`
	Dirty          bool   `json:"dirty"`
	Ahead          int    `json:"ahead"`
	Behind         int    `json:"behind"`
	Detached       bool   `json:"detached"`
}

type PullResult struct {
	Status Status `json:"status"`
	Output string `json:"output"`
}
