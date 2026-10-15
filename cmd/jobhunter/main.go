package main

import (
	"os"

	"github.com/cryk/jobhunter/internal/cli"
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
