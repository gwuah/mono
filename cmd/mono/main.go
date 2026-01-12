package main

import (
	"os"

	"github.com/anthropics/mono/internal/cli"
)

func main() {
	cmd := cli.NewRootCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
