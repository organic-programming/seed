//go:build e2e

package dispatch_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestDispatch_CLI_SayHelloAcrossTransports(t *testing.T) {
	for _, spec := range integration.NativeTestHolons(t) {
		spec := spec
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			integration.BuildReportFor(t, sb, spec.Slug)
			example := integration.PrimaryInvokeExample(spec)
			for _, transport := range integration.TransportMatrix() {
				transport := transport
				t.Run(transport.Name, func(t *testing.T) {
					target := spec.Slug
					if transport.URIPrefix != "" {
						target = transport.URIPrefix + spec.Slug
					}

					result := sb.RunOP(t, target, example.Method, example.Payload)
					integration.RequireSuccess(t, result)

					payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
					integration.AssertInvokePayload(t, spec, example.Method, payload)
				})
			}
		})
	}
}

func TestDispatch_CLI_JSONOutput(t *testing.T) {
	for _, spec := range integration.NativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			integration.BuildReportFor(t, sb, spec.Slug)
			example := integration.PrimaryInvokeExample(spec)
			result := sb.RunOP(t, "--format", "json", spec.Slug, example.Method, example.Payload)
			integration.RequireSuccess(t, result)
			payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
			integration.AssertInvokePayload(t, spec, example.Method, payload)
		})
	}
}

func TestDispatch_CLI_ListLanguages(t *testing.T) {
	for _, spec := range integration.NativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			integration.BuildReportFor(t, sb, spec.Slug)
			examples := integration.InvokeExamplesFor(spec)
			example := examples[len(examples)-1]
			args := []string{spec.Slug, example.Method}
			if example.Payload != "" {
				args = append(args, example.Payload)
			}
			result := sb.RunOP(t, args...)
			integration.RequireSuccess(t, result)
			payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
			integration.AssertInvokePayload(t, spec, example.Method, payload)
		})
	}
}

func TestDispatch_CLI_AutoBuild(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)

	slug := "gabriel-greeting-go"
	sb := integration.NewSandbox(t)
	integration.RemoveArtifactFor(t, sb, slug)

	example := integration.PrimaryInvokeExample(integration.HolonSpec{Slug: slug})
	result := sb.RunOP(t, slug, example.Method, example.Payload)
	integration.RequireSuccess(t, result)
	integration.RequirePathExists(t, integration.ArtifactPathFor(t, sb, slug))
}

func TestDispatch_CLI_NoBuildDoesNotBuild(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.RemoveArtifactFor(t, sb, "gabriel-greeting-go")
	artifactPath := integration.ArtifactPathFor(t, sb, "gabriel-greeting-go")

	example := integration.PrimaryInvokeExample(integration.HolonSpec{Slug: "gabriel-greeting-go"})
	result := sb.RunOP(t, "gabriel-greeting-go", example.Method, "--no-build", example.Payload)
	if result.TimedOut {
		t.Fatalf("--no-build timed out\nstdout:\n%s\nstderr:\n%s", result.Stdout, result.Stderr)
	}
	integration.RequirePathMissing(t, artifactPath)
}

func TestDispatch_CLI_CleanThenCall(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.BuildReportFor(t, sb, "gabriel-greeting-go")

	result := sb.RunOP(t, "gabriel-greeting-go", "--clean", "SayHello", `{"name":"Test","lang_code":"en"}`)
	integration.RequireSuccess(t, result)
	payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
	if payload["greeting"] == "" {
		t.Fatalf("clean call payload = %#v", payload)
	}
}
