package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultModel  = "gpt-5.4"
	stateFileName = ".codex_orchestrator_state.json"
)

type Config struct {
	Sets      []string
	Model     string
	Root      string
	StateFile string // default: <root>/.codex_orchestrator_state.json
}

type setFlags []string

func (s *setFlags) String() string {
	return strings.Join(*s, ",")
}

func (s *setFlags) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("--set cannot be empty")
	}

	*s = append(*s, value)
	return nil
}

func Parse() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	cfg := &Config{
		Model: defaultModel,
		Root:  cwd,
	}

	var sets setFlags

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Var(&sets, "set", "task set to run (repeatable)")
	fs.StringVar(&cfg.Model, "model", cfg.Model, "codex model to use")
	fs.StringVar(&cfg.Root, "root", cfg.Root, "repository root")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	if len(fs.Args()) > 0 {
		return nil, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("--model cannot be empty")
	}

	if strings.TrimSpace(cfg.Root) == "" {
		return nil, errors.New("--root cannot be empty")
	}

	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	cfg.Root = filepath.Clean(root)
	cfg.Sets = append(cfg.Sets, sets...)
	cfg.StateFile = filepath.Join(cfg.Root, stateFileName)

	return cfg, nil
}
