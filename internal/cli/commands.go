package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
)

func Run(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: onespace <command> [args]")
		fmt.Fprintln(stderr, "commands: status, pull, build, restart, deploy, debug, health, logs, stop, version")
		return 2
	}

	baseURL := getenv("ONESPACE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:18080"
	}

	client := Client{BaseURL: baseURL, HTTP: nil}

	switch args[0] {
	case "version":
		return runVersion(stdout, stderr)
	case "status":
		return runStatus(client, args[1:], stdout, stderr)
	case "pull":
		return runPull(client, args[1:], stdout, stderr)
	case "build":
		return runBuild(client, args[1:], stdout, stderr)
	case "restart":
		return runRestart(client, args[1:], stdout, stderr)
	case "deploy":
		return runDeploy(client, args[1:], stdout, stderr)
	case "debug":
		return runDebug(client, args[1:], stdout, stderr)
	case "health":
		return runHealth(client, args[1:], stdout, stderr)
	case "logs":
		return runLogs(client, args[1:], stdout, stderr)
	case "stop":
		return runStop(client, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "onespace: unknown command %q\n", args[0])
		return 2
	}
}

func runVersion(stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "onespace dev")
	return 0
}

func runStatus(client Client, args []string, stdout, stderr io.Writer) int {
	ctx := context.Background()
	services, err := client.GetServices(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "onespace status: %v\n", err)
		return 1
	}
	WriteServicesTable(stdout, services)
	return 0
}

func runPull(client Client, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "onespace pull: service name required")
		return 2
	}
	ctx := context.Background()
	result, err := client.Pull(ctx, args[0])
	if err != nil {
		fmt.Fprintf(stderr, "onespace pull: %v\n", err)
		return 1
	}
	WriteDeployText(stdout, result)
	return 0
}

func runBuild(client Client, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "onespace build: service name required")
		return 2
	}
	ctx := context.Background()
	result, err := client.Build(ctx, args[0])
	if err != nil {
		fmt.Fprintf(stderr, "onespace build: %v\n", err)
		return 1
	}
	WriteDeployText(stdout, result)
	return 0
}

func runRestart(client Client, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "onespace restart: service name required")
		return 2
	}
	ctx := context.Background()
	result, err := client.Restart(ctx, args[0])
	if err != nil {
		fmt.Fprintf(stderr, "onespace restart: %v\n", err)
		return 1
	}
	WriteDeployText(stdout, result)
	return 0
}

func runDeploy(client Client, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
	wait := fs.Bool("wait", true, "wait for completion")
	jsonOutput := fs.Bool("json", false, "JSON output")
	fs.SetOutput(stderr)
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return 2
	}

	serviceArgs := fs.Args()
	if len(serviceArgs) == 0 {
		fmt.Fprintln(stderr, "onespace deploy: service name required")
		return 2
	}
	service := serviceArgs[0]

	ctx := context.Background()
	result, err := client.Deploy(ctx, service, *wait)
	if err != nil {
		if *jsonOutput {
			WriteJSON(stdout, result)
		} else {
			fmt.Fprintf(stderr, "onespace deploy: %v\n", err)
		}
		return 1
	}

	if result.Status == "failed" {
		if *jsonOutput {
			WriteJSON(stdout, result)
		} else {
			WriteDeployText(stdout, result)
		}
		return 1
	}

	if *jsonOutput {
		WriteJSON(stdout, result)
	} else {
		WriteDeployText(stdout, result)
	}
	return 0
}

func runDebug(client Client, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("debug", flag.ContinueOnError)
	wait := fs.Bool("wait", true, "wait for completion")
	jsonOutput := fs.Bool("json", false, "JSON output")
	fs.SetOutput(stderr)
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return 2
	}

	serviceArgs := fs.Args()
	if len(serviceArgs) == 0 {
		fmt.Fprintln(stderr, "onespace debug: service name required")
		return 2
	}
	service := serviceArgs[0]

	ctx := context.Background()
	result, err := client.Debug(ctx, service, *wait)
	if err != nil {
		if *jsonOutput {
			WriteJSON(stdout, result)
		} else {
			fmt.Fprintf(stderr, "onespace debug: %v\n", err)
		}
		return 1
	}

	if result.Status == "failed" {
		if *jsonOutput {
			WriteJSON(stdout, result)
		} else {
			WriteDeployText(stdout, result)
		}
		return 1
	}

	if *jsonOutput {
		WriteJSON(stdout, result)
	} else {
		WriteDeployText(stdout, result)
	}
	return 0
}

func runHealth(client Client, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "onespace health: service name required")
		return 2
	}
	ctx := context.Background()
	result, err := client.Health(ctx, args[0])
	if err != nil {
		fmt.Fprintf(stderr, "onespace health: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", result.Status)
	return 0
}

func runLogs(client Client, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	jobID := fs.String("job", "", "job ID")
	tail := fs.Int("tail", 200, "number of lines")
	fs.SetOutput(stderr)
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return 2
	}

	serviceArgs := fs.Args()
	if len(serviceArgs) == 0 && *jobID == "" {
		fmt.Fprintln(stderr, "onespace logs: service name or --job required")
		return 2
	}

	service := ""
	if len(serviceArgs) > 0 {
		service = serviceArgs[0]
	}

	ctx := context.Background()
	lines, err := client.Logs(ctx, service, *jobID, *tail)
	if err != nil {
		fmt.Fprintf(stderr, "onespace logs: %v\n", err)
		return 1
	}
	for _, line := range lines {
		fmt.Fprintln(stdout, line)
	}
	return 0
}

func runStop(client Client, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "onespace stop: service name required")
		return 2
	}
	ctx := context.Background()
	result, err := client.Stop(ctx, args[0])
	if err != nil {
		fmt.Fprintf(stderr, "onespace stop: %v\n", err)
		return 1
	}
	WriteDeployText(stdout, result)
	return 0
}

// reorderArgs moves flag arguments before positional arguments so Go's
// flag package can parse them correctly (it stops at the first non-flag arg).
func reorderArgs(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if flagTakesValue(arg) && !strings.Contains(arg, "=") && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}

func flagTakesValue(arg string) bool {
	name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
	return name == "job" || name == "tail"
}
