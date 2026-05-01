package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/wnzhone/onespace/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		if err := json.NewEncoder(os.Stdout).Encode(version.Info()); err != nil {
			fmt.Fprintf(os.Stderr, "onespace: write version: %v\n", err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "onespace: no command specified")
	os.Exit(2)
}
