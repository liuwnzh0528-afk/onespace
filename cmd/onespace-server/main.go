package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wnzhone/onespace/internal/api"
	"github.com/wnzhone/onespace/internal/config"
	"github.com/wnzhone/onespace/internal/gitx"
	"github.com/wnzhone/onespace/internal/health"
	"github.com/wnzhone/onespace/internal/jobs"
	"github.com/wnzhone/onespace/internal/logs"
	"github.com/wnzhone/onespace/internal/runtime"
	"github.com/wnzhone/onespace/internal/serviceops"
	"github.com/wnzhone/onespace/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		if err := json.NewEncoder(os.Stdout).Encode(version.Info()); err != nil {
			fmt.Fprintf(os.Stderr, "onespace-server: write version: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "serve" {
		serve()
		return
	}

	fmt.Fprintln(os.Stderr, "usage: onespace-server serve --config <path> | version")
	os.Exit(2)
}

func serve() {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "", "path to onespace.yaml")
	fs.Parse(os.Args[2:])

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "onespace-server serve: --config is required")
		os.Exit(2)
	}

	ws, err := config.LoadWorkspace(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "onespace-server: load config: %v\n", err)
		os.Exit(1)
	}

	// Generate compose file
	composeData, err := runtime.GenerateCompose(ws)
	if err != nil {
		fmt.Fprintf(os.Stderr, "onespace-server: generate compose: %v\n", err)
		os.Exit(1)
	}
	if err := runtime.WriteComposeFile(ws.Path, composeData); err != nil {
		fmt.Fprintf(os.Stderr, "onespace-server: write compose file: %v\n", err)
		os.Exit(1)
	}

	stateDir := filepath.Join(ws.Path, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "onespace-server: create state dir: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(stateDir, "onespace.db")
	jobStore, err := jobs.OpenSQLiteStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "onespace-server: open job store: %v\n", err)
		os.Exit(1)
	}
	defer jobStore.Close()

	logStore := logs.Store{Root: stateDir}

	events := api.NewEventBroker()
	checker := health.Checker{}

	osRunner := runtime.OSCommandRunner{}
	composeRuntime := &runtime.ComposeRuntime{Runner: osRunner}

	gitRunner := &gitCommandRunner{}
	gitSvc := &gitx.Service{Runner: gitRunner}

	manager := &serviceops.Manager{
		Workspace: ws,
		Git:       gitSvc,
		Runtime:   composeRuntime,
		Health:    checker,
		Jobs:      jobs.NewRunner(jobStore),
		Logs:      logStore,
	}

	staticDir := ""
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	for _, candidate := range []string{
		filepath.Join(exeDir, "web", "static"),
		filepath.Join(exeDir, "..", "web", "static"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			staticDir, _ = filepath.Abs(candidate)
			break
		}
	}

	server := api.NewServer(ws, manager, jobStore, logStore, checker, events, staticDir)

	bind := ws.Server.Bind
	if bind == "" {
		bind = "127.0.0.1:18080"
	}

	log.Printf("onespace-server: listening on %s", bind)
	if err := http.ListenAndServe(bind, server.Handler()); err != nil {
		log.Fatalf("onespace-server: %v", err)
	}
}

type gitCommandRunner struct{}

func (g *gitCommandRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
