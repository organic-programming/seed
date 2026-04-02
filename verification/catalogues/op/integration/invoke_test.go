// Invoke tests exercise the explicit op invoke command across local transports
// and flag combinations against the real hello-world holons.
package integration

import "testing"

func TestInvoke_AcrossTransports(t *testing.T) {
	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			for _, transport := range transportMatrix() {
				t.Run(transport.Name, func(t *testing.T) {
					sb := newSandbox(t)
					target := spec.Slug
					if transport.URIPrefix != "" {
						target = transport.URIPrefix + spec.Slug
					}

					result := sb.runOP(t, "invoke", target, "SayHello", `{"name":"World","lang_code":"en"}`)
					requireSuccess(t, result)
					payload := decodeJSON[map[string]any](t, result.Stdout)
					if payload["greeting"] == "" {
						t.Fatalf("empty invoke payload: %#v", payload)
					}
				})
			}
		})
	}
}

func TestInvoke_CleanFlag(t *testing.T) {
	sb := newSandbox(t)
	buildReportFor(t, sb, "gabriel-greeting-go")

	result := sb.runOP(t, "invoke", "--clean", "gabriel-greeting-go", "SayHello", `{"name":"World","lang_code":"en"}`)
	requireSuccess(t, result)
	payload := decodeJSON[map[string]any](t, result.Stdout)
	if payload["greeting"] == "" {
		t.Fatalf("empty invoke payload after clean: %#v", payload)
	}
}

func TestInvoke_NoBuildDoesNotBuild(t *testing.T) {
	sb := newSandbox(t)
	removeArtifactFor(t, sb, "gabriel-greeting-go")
	artifactPath := artifactPathFor(t, sb, "gabriel-greeting-go")

	result := sb.runOP(t, "invoke", "gabriel-greeting-go", "SayHello", "--no-build", `{"name":"World","lang_code":"en"}`)
	if result.TimedOut {
		t.Fatalf("--no-build timed out\nstdout:\n%s\nstderr:\n%s", result.Stdout, result.Stderr)
	}
	requirePathMissing(t, artifactPath)
}

func TestInvoke_CleanNoBuildConflict(t *testing.T) {
	sb := newSandbox(t)
	result := sb.runOP(t, "invoke", "--clean", "--no-build", "gabriel-greeting-go", "SayHello", `{"name":"World","lang_code":"en"}`)
	requireFailure(t, result)
	requireContains(t, result.Stderr, "--clean cannot be combined with --no-build")
}
