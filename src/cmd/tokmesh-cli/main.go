// Package main provides the entry point for tokmesh-cli.
//
// tokmesh-cli is the command-line management tool for TokMesh,
// supporting both single-command mode and interactive REPL mode.
package main

import (
	"fmt"
	"os"

	"github.com/yndnr/tokmesh-go/internal/cli/command"
)

func main() {
	app := command.App()

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
