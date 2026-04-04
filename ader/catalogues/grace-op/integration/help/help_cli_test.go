package help_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestHelp_CLI_RootTopics(t *testing.T) {
	sb := integration.NewSandbox(t)
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		t.Run(args[0], func(t *testing.T) {
			result := sb.RunOP(t, args...)
			integration.RequireSuccess(t, result)
			integration.RequireContains(t, result.Stdout, "Available Commands:")
			integration.RequireContains(t, result.Stdout, "build")
			integration.RequireContains(t, result.Stdout, "version")
		})
	}
}

func TestHelp_CLI_IncludesCanonicalExamples(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "help")
	integration.RequireSuccess(t, result)
	for _, want := range []string{
		"op <holon> --clean <method> [--no-build] [json]",
		"op build [<holon-or-path>] --clean",
		"op run <holon>:<port>",
	} {
		integration.RequireContains(t, result.Stdout, want)
	}
}

func TestHelp_CLI_BuildAndRunTopics(t *testing.T) {
	sb := integration.NewSandbox(t)

	build := sb.RunOP(t, "help", "build")
	integration.RequireSuccess(t, build)
	integration.RequireContains(t, build.Stdout, "--clean")
	integration.RequireContains(t, build.Stdout, "cannot be combined with --dry-run")

	run := sb.RunOP(t, "help", "run")
	integration.RequireSuccess(t, run)
	integration.RequireContains(t, run.Stdout, "--clean")
	integration.RequireContains(t, run.Stdout, "cannot be combined with --no-build")
}
