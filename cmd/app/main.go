package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lucasmellof/ct/internal/config"
	"github.com/lucasmellof/ct/internal/terminal"
)

func main() {
	if err := terminal.EnableVirtualTerminal(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to enable virtual terminal: %v\n", err)
		os.Exit(1)
	}

	if err := config.LoadConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <program> [arguments...]\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	program := os.Args[1]
	args := os.Args[2:]

	err := terminal.Run(program, args)
	if err == nil {
		os.Exit(0)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}

	fmt.Fprintf(os.Stderr, "program exited with error: %v\n", err)
	os.Exit(1)
}
