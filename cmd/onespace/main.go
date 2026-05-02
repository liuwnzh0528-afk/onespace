package main

import (
	"os"

	"github.com/wnzhone/onespace/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}
