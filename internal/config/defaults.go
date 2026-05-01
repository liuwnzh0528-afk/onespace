package config

import "fmt"

type languageDefaults struct {
	Workdir      string
	Image        string
	BuildCommand string
	RunCommand   string
	DebugBuild   string
	DebugCommand string
}

func defaultsForGo(main string, debugPort int) languageDefaults {
	if main == "" {
		main = "."
	}
	return languageDefaults{
		Workdir:      "/workspace",
		Image:        "onespace/go-dev:1.23",
		BuildCommand: fmt.Sprintf("go build -o /workspace/.onespace/bin/app %s", main),
		RunCommand:   "/workspace/.onespace/bin/app",
		DebugBuild:   fmt.Sprintf("go build -gcflags=\"all=-N -l\" -o /workspace/.onespace/bin/app %s", main),
		DebugCommand: fmt.Sprintf("dlv exec /workspace/.onespace/bin/app --headless --listen=:%d --api-version=2 --accept-multiclient --continue", debugPort),
	}
}

func defaultsForJavaMaven(debugPort int) languageDefaults {
	return languageDefaults{
		Workdir:      "/workspace",
		Image:        "onespace/java-dev:21-maven",
		BuildCommand: "mvn package -DskipTests",
		RunCommand:   "java -jar target/*.jar",
		DebugCommand: fmt.Sprintf("java -agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:%d -jar target/*.jar", debugPort),
	}
}
