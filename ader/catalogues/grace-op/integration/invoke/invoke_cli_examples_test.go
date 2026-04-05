package invoke_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestInvoke_CLI_ExamplesAcrossTransports(t *testing.T) {
	methods := []struct {
		name    string
		payload string
		assert  func(*testing.T, map[string]any)
	}{
		{
			name:    "SayHello",
			payload: `{"name":"World","lang_code":"en"}`,
			assert: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if payload["greeting"] == "" {
					t.Fatalf("empty greeting payload: %#v", payload)
				}
			},
		},
		{
			name:    "ListLanguages",
			payload: "",
			assert: func(t *testing.T, payload map[string]any) {
				t.Helper()
				languages, ok := payload["languages"].([]any)
				if !ok || len(languages) == 0 {
					t.Fatalf("languages = %#v, want non-empty array", payload["languages"])
				}
			},
		},
	}

	for _, spec := range integration.NativeTestHolons(t) {
		spec := spec
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			integration.BuildReportFor(t, sb, spec.Slug)
			for _, transport := range exampleInvokeTransports() {
				transport := transport
				t.Run(transport.Name, func(t *testing.T) {
					target, cleanup := startExampleTransportTarget(t, sb, spec.Slug, transport)
					defer cleanup()

					for _, method := range methods {
						t.Run(method.name, func(t *testing.T) {
							payload := invokeCLIJSON(t, sb, integration.RunOptions{}, target, method.name, method.payload)
							method.assert(t, payload)
						})
					}
				})
			}
		})
	}
}

func TestInvoke_CLI_CompositeAcrossTransports(t *testing.T) {
	composites := integration.CompositeTestHolons(t)
	methods := []struct {
		name    string
		payload string
		assert  func(*testing.T, map[string]any)
	}{
		{
			name:    "SelectHolon",
			payload: `{"slug":"gabriel-greeting-go"}`,
			assert: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if payload["slug"] != "gabriel-greeting-go" {
					t.Fatalf("slug = %#v, want gabriel-greeting-go", payload["slug"])
				}
			},
		},
		{
			name:    "SelectLanguage",
			payload: `{"code":"fr"}`,
			assert: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if payload["code"] != "fr" {
					t.Fatalf("code = %#v, want fr", payload["code"])
				}
			},
		},
		{
			name:    "Greet",
			payload: `{"name":"World","lang_code":"en"}`,
			assert: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if payload["greeting"] == "" {
					t.Fatalf("greeting = %#v, want non-empty", payload["greeting"])
				}
			},
		},
	}

	for _, spec := range composites {
		spec := spec
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			integration.BuildReportFor(t, sb, spec.Slug)
			for _, transport := range exampleInvokeTransports() {
				transport := transport
				t.Run(transport.Name, func(t *testing.T) {
					target, cleanup := startExampleTransportTarget(t, sb, spec.Slug, transport)
					defer cleanup()

					for _, method := range methods {
						t.Run(method.name, func(t *testing.T) {
							payload := invokeCLIJSON(t, sb, integration.RunOptions{}, target, method.name, method.payload)
							method.assert(t, payload)
						})
					}
				})
			}
		})
	}
}
