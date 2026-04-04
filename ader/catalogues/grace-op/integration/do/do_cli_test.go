package do_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestDo_CLI_DryRun(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "do", "gabriel-greeting-go", "multilingual-greeting", "--name=Alice", "--lang_code=fr", "--dry-run")
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "ListLanguages")
	integration.RequireContains(t, result.Stdout, "SayHello")
}

func TestDo_CLI_Live(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "do", "gabriel-greeting-go", "multilingual-greeting", "--name=Alice", "--lang_code=fr")
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "Bonjour")
}

func TestDo_CLI_MultiStep(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "do", "gabriel-greeting-go", "greeting-fr-ja-ru-en", "--name=Bob")
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "Bonjour")
	integration.RequireContains(t, result.Stdout, "Hello")
}

func TestDo_CLI_Errors(t *testing.T) {
	sb := integration.NewSandbox(t)

	missingSequence := sb.RunOP(t, "do", "gabriel-greeting-go", "nonexistent", "--name=Bob")
	integration.RequireFailure(t, missingSequence)
	integration.RequireContains(t, missingSequence.Stderr, "sequence")

	missingParam := sb.RunOP(t, "do", "gabriel-greeting-go", "multilingual-greeting", "--name=Bob")
	integration.RequireFailure(t, missingParam)
	integration.RequireContains(t, missingParam.Stderr, "missing required param")

	continueOnError := sb.RunOP(t, "do", "gabriel-greeting-go", "multilingual-greeting", "--continue-on-error", "--name=Bob")
	integration.RequireFailure(t, continueOnError)
	integration.RequireContains(t, continueOnError.Stderr, "missing required param")
}
