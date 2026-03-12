package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "gudule-greeting-testmatrix: %v\n", err)
		os.Exit(2)
	}

	report, err := runMatrix(context.Background(), cfg, shellRunner{}, defaultRuntimeEnv())
	if err != nil {
		fmt.Fprintf(os.Stderr, "gudule-greeting-testmatrix: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(renderReport(report, cfg.Format))
	if report.HasFailures() {
		os.Exit(1)
	}
}

func parseConfig(args []string) (MatrixConfig, error) {
	fs := flag.NewFlagSet("gudule-greeting-testmatrix", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		filterExpr string
		skipExpr   string
		timeout    time.Duration
		format     string
		dryRun     bool
	)

	fs.StringVar(&filterExpr, "filter", "", "regex applied to target names and paths")
	fs.StringVar(&skipExpr, "skip", "", "regex for targets to mark as skipped")
	fs.DurationVar(&timeout, "timeout", 10*time.Second, "timeout for op run --no-build")
	fs.StringVar(&format, "format", "text", "output format: text or json")
	fs.BoolVar(&dryRun, "dry-run", false, "discover and classify targets without calling op")

	if err := fs.Parse(args); err != nil {
		return MatrixConfig{}, err
	}
	if fs.NArg() != 0 {
		return MatrixConfig{}, fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}

	root, err := locateRepoRootFrom(".")
	if err != nil {
		return MatrixConfig{}, err
	}

	filter, err := compilePattern(filterExpr)
	if err != nil {
		return MatrixConfig{}, fmt.Errorf("invalid --filter: %w", err)
	}
	skip, err := compilePattern(skipExpr)
	if err != nil {
		return MatrixConfig{}, fmt.Errorf("invalid --skip: %w", err)
	}

	switch format {
	case "text", "json":
	default:
		return MatrixConfig{}, fmt.Errorf("unsupported --format %q", format)
	}

	return MatrixConfig{
		RepoRoot:   root,
		OpPath:     resolvedPath("op"),
		FilterExpr: filterExpr,
		Filter:     filter,
		SkipExpr:   skipExpr,
		Skip:       skip,
		Timeout:    timeout,
		Format:     format,
		DryRun:     dryRun,
	}, nil
}

func compilePattern(expr string) (*regexp.Regexp, error) {
	if expr == "" {
		return nil, nil
	}
	return regexp.Compile(expr)
}

func resolvedPath(name string) string {
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".op", "bin", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return name
	}
	return path
}
