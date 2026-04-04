package test_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestTest_CLI_Matrix(t *testing.T) {
	for _, spec := range integration.NativeTestHolons(t) {
		if spec.Slug == "gabriel-greeting-go" || !integration.SupportsOPTest(spec) {
			continue
		}
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			result := sb.RunOP(t, "test", spec.Slug)
			integration.RequireSuccess(t, result)
			integration.RequireContains(t, result.Stdout, "Operation: test")
		})
	}
}
