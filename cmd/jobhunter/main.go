package main

import (
	"os"

	"github.com/Crykk/JobHunter/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := cli.Execute(version, commit, date); err != nil {
		os.Exit(1)
	}
}
