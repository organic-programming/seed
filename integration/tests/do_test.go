// Sequence tests exercise op do in dry-run and live modes, including multi-step
// execution and expected failure cases.
package integration

import "testing"

func TestDo_DryRun(t *testing.T) {
	sb := newSandbox(t)
	result := sb.runOP(t, "do", "gabriel-greeting-go", "multilingual-greeting", "--name=Alice", "--lang_code=fr", "--dry-run")
	requireSuccess(t, result)
	requireContains(t, result.Stdout, "ListLanguages")
	requireContains(t, result.Stdout, "SayHello")
}

func TestDo_Live(t *testing.T) {
	sb := newSandbox(t)
	result := sb.runOP(t, "do", "gabriel-greeting-go", "multilingual-greeting", "--name=Alice", "--lang_code=fr")
	requireSuccess(t, result)
	requireContains(t, result.Stdout, "Bonjour")
}

func TestDo_MultiStep(t *testing.T) {
	sb := newSandbox(t)
	result := sb.runOP(t, "do", "gabriel-greeting-go", "greeting-fr-ja-ru-en", "--name=Bob")
	requireSuccess(t, result)
	requireContains(t, result.Stdout, "Bonjour")
	requireContains(t, result.Stdout, "Hello")
}

func TestDo_Errors(t *testing.T) {
	sb := newSandbox(t)

	missingSequence := sb.runOP(t, "do", "gabriel-greeting-go", "nonexistent", "--name=Bob")
	requireFailure(t, missingSequence)
	requireContains(t, missingSequence.Stderr, "sequence")

	missingParam := sb.runOP(t, "do", "gabriel-greeting-go", "multilingual-greeting", "--name=Bob")
	requireFailure(t, missingParam)
	requireContains(t, missingParam.Stderr, "missing required param")

	continueOnError := sb.runOP(t, "do", "gabriel-greeting-go", "multilingual-greeting", "--continue-on-error", "--name=Bob")
	requireFailure(t, continueOnError)
	requireContains(t, continueOnError.Stderr, "missing required param")
}
