package main

import (
	"fmt"
	"os"

	"github.com/organic-programming/codex-orchestrator/internal/cli"
)

func main() {
	cfg, err := cli.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("orchestrator: sets=%v model=%s root=%s\n", cfg.Sets, cfg.Model, cfg.Root)
}
