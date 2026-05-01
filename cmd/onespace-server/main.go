package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wnzhone/onespace/internal/api"
	"github.com/wnzhone/onespace/internal/config"
	"github.com/wnzhone/onespace/internal/health"
	"github.com/wnzhone/onespace/internal/jobs"
	"github.com/wnzhone/onespace/internal/logs"
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

	stateDir := filepath.Join(filepath.Dir(*configPath), "state")
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

	manager := &serviceops.Manager{
		Workspace: ws,
		Runtime:   nil,
		Health:    checker,
		Jobs:      jobs.NewRunner(jobStore),
		Logs:      logStore,
	}

	server := api.NewServer(ws, manager, jobStore, logStore, checker, events)

	bind := ws.Server.Bind
	if bind == "" {
		bind = "127.0.0.1:18080"
	}

	log.Printf("onespace-server: listening on %s", bind)
	if err := http.ListenAndServe(bind, server.Handler()); err != nil {
		log.Fatalf("onespace-server: %v", err)
	}
}
