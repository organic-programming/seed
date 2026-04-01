// Dispatch tests call real holon RPC methods through the direct CLI target
// syntax across the local transport matrix.
package integration

import (
	"testing"
)

func TestDispatch_SayHelloAcrossTransports(t *testing.T) {
	for _, spec := range nativeTestHolons(t) {
		spec := spec
		t.Run(spec.Slug, func(t *testing.T) {
			for _, transport := range transportMatrix() {
				transport := transport
				t.Run(transport.Name, func(t *testing.T) {
					sb := newSandbox(t)
					target := spec.Slug
					if transport.URIPrefix != "" {
						target = transport.URIPrefix + spec.Slug
					}

					result := sb.runOP(t, target, "SayHello", `{"name":"World","lang_code":"en"}`)
					requireSuccess(t, result)

					payload := decodeJSON[map[string]any](t, result.Stdout)
					if payload["greeting"] == "" {
						t.Fatalf("empty greeting payload: %#v", payload)
					}
				})
			}
		})
	}
}

func TestDispatch_JSONOutput(t *testing.T) {
	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)
			result := sb.runOP(t, "--format", "json", spec.Slug, "SayHello", `{"name":"World","lang_code":"en"}`)
			requireSuccess(t, result)
			payload := decodeJSON[map[string]any](t, result.Stdout)
			if payload["langCode"] != "en" {
				t.Fatalf("langCode = %#v, want en", payload["langCode"])
			}
		})
	}
}

func TestDispatch_ListLanguages(t *testing.T) {
	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)
			result := sb.runOP(t, spec.Slug, "ListLanguages")
			requireSuccess(t, result)
			payload := decodeJSON[map[string]any](t, result.Stdout)
			languages, ok := payload["languages"].([]any)
			if !ok || len(languages) == 0 {
				t.Fatalf("languages = %#v, want non-empty array", payload["languages"])
			}
		})
	}
}

func TestDispatch_AutoBuild(t *testing.T) {
	skipIfShort(t, shortTestReason)

	slug := "gabriel-greeting-go"
	if toolsAvailable("cmake") {
		slug = "gabriel-greeting-c"
	}

	sb := newSandbox(t)
	removeArtifactFor(t, sb, slug)

	result := sb.runOP(t, slug, "SayHello", `{"name":"World","lang_code":"en"}`)
	requireSuccess(t, result)
	requirePathExists(t, artifactPathFor(t, sb, slug))
}

func TestDispatch_NoBuildDoesNotBuild(t *testing.T) {
	sb := newSandbox(t)
	removeArtifactFor(t, sb, "gabriel-greeting-go")
	artifactPath := artifactPathFor(t, sb, "gabriel-greeting-go")

	result := sb.runOP(t, "gabriel-greeting-go", "SayHello", "--no-build", `{"name":"World","lang_code":"en"}`)
	if result.TimedOut {
		t.Fatalf("--no-build timed out\nstdout:\n%s\nstderr:\n%s", result.Stdout, result.Stderr)
	}
	requirePathMissing(t, artifactPath)
}

func TestDispatch_CleanThenCall(t *testing.T) {
	sb := newSandbox(t)
	buildReportFor(t, sb, "gabriel-greeting-go")

	result := sb.runOP(t, "gabriel-greeting-go", "--clean", "SayHello", `{"name":"Test","lang_code":"en"}`)
	requireSuccess(t, result)
	payload := decodeJSON[map[string]any](t, result.Stdout)
	if payload["greeting"] == "" {
		t.Fatalf("clean call payload = %#v", payload)
	}
}
