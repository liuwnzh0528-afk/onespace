package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/wnzhone/onespace/internal/serviceops"
)

func WriteJSON(w io.Writer, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}

func WriteDeployText(w io.Writer, result serviceops.Result) error {
	fmt.Fprintf(w, "service: %s\n", result.Service)
	fmt.Fprintf(w, "status:  %s\n", result.Status)
	if result.Stage != "" {
		fmt.Fprintf(w, "stage:   %s\n", result.Stage)
	}
	if result.Commit != "" {
		fmt.Fprintf(w, "commit:  %s\n", result.Commit)
	}
	if result.Health != "" {
		fmt.Fprintf(w, "health:  %s\n", result.Health)
	}
	if result.Debug != nil {
		fmt.Fprintf(w, "debug:   %s (%s)\n", result.Debug.Debugger, result.Debug.Address)
	}
	return nil
}

func WriteServicesTable(w io.Writer, services []ServiceSummary) error {
	if len(services) == 0 {
		fmt.Fprintln(w, "No services found.")
		return nil
	}

	maxName := 4
	for _, s := range services {
		if len(s.Name) > maxName {
			maxName = len(s.Name)
		}
	}

	fmt.Fprintf(w, "%-*s  %-12s  %s\n", maxName, "NAME", "LANGUAGE", "HEALTH")
	fmt.Fprintf(w, "%s  %s  %s\n", strings.Repeat("-", maxName), strings.Repeat("-", 12), strings.Repeat("-", 7))

	for _, s := range services {
		fmt.Fprintf(w, "%-*s  %-12s  %s\n", maxName, s.Name, s.Language, s.Health)
	}
	return nil
}

func WriteConfigText(w io.Writer, cfg ServiceConfig) error {
	fmt.Fprintf(w, "SERVICE %s\n\n", cfg.Service)
	fmt.Fprintln(w, "ENV")
	for _, entry := range cfg.Env {
		secret := ""
		if entry.Secret {
			secret = " secret"
		}
		fmt.Fprintf(w, "%-24s %-16s %s%s\n", entry.Name, entry.Value, entry.Source, secret)
	}
	fmt.Fprintln(w, "\nFILES")
	for _, file := range cfg.Files {
		secret := ""
		if file.Secret {
			secret = " secret"
		}
		fmt.Fprintf(w, "%-32s %-32s %s%s\n", file.Target, file.Source, file.Mode, secret)
	}
	fmt.Fprintln(w, "\nVOLUMES")
	for _, volume := range cfg.Volumes {
		fmt.Fprintf(w, "%-32s %-32s %s\n", volume.Target, volume.Source, volume.Type)
	}
	if len(cfg.DependsOn) > 0 {
		fmt.Fprintln(w, "\nDEPENDS ON")
		for _, dep := range cfg.DependsOn {
			fmt.Fprintln(w, dep)
		}
	}
	return nil
}
