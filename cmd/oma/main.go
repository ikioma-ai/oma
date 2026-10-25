package main

import (
	"fmt"
	"os"

	"github.com/ikioma-ai/oma/internal/command"
	"github.com/ikioma-ai/oma/internal/docker"
	"github.com/ikioma-ai/oma/internal/logging"
)

func main() {
	logging.SetupStderr()

	if err := command.NewRootCommand().Execute(); err != nil {
		if code, ok := command.ExitCode(err); ok {
			os.Exit(code)
		}
		fmt.Fprintln(os.Stderr, "Error:", docker.ErrorMessage(err))
		os.Exit(1)
	}
}
