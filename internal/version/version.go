package version

var (
	Version = "dev"
	Commit  = "none"
)

type BuildInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func Info() BuildInfo {
	return BuildInfo{
		Name:    "onespace",
		Version: Version,
		Commit:  Commit,
	}
}
