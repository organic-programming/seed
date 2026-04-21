package integration

import (
	"math"
	"testing"
)

type InvocationExample struct {
	Method  string
	Payload string
}

func InvokeExamplesFor(spec HolonSpec) []InvocationExample {
	switch spec.Slug {
	case "matt-calculator-go":
		return []InvocationExample{
			{Method: "Set", Payload: `{"value":20.0}`},
			{Method: "Add", Payload: `{"value":1.0}`},
		}
	default:
		return []InvocationExample{
			{Method: "SayHello", Payload: `{"name":"World","lang_code":"en"}`},
			{Method: "ListLanguages"},
		}
	}
}

func PrimaryInvokeExample(spec HolonSpec) InvocationExample {
	return InvokeExamplesFor(spec)[0]
}

func ExpectedServiceName(spec HolonSpec) string {
	switch spec.Slug {
	case "matt-calculator-go":
		return "CalculatorService"
	default:
		return "GreetingService"
	}
}

func AssertInvokePayload(t *testing.T, spec HolonSpec, method string, payload map[string]any) {
	t.Helper()

	switch spec.Slug {
	case "matt-calculator-go":
		assertCalculatorPayload(t, method, payload)
	default:
		assertGreetingPayload(t, method, payload)
	}
}

func assertGreetingPayload(t *testing.T, method string, payload map[string]any) {
	t.Helper()

	switch method {
	case "SayHello":
		if payload["greeting"] == "" {
			t.Fatalf("empty greeting payload: %#v", payload)
		}
		if payload["langCode"] != "en" && payload["lang_code"] != "en" {
			t.Fatalf("lang code = %#v / %#v, want en", payload["langCode"], payload["lang_code"])
		}
	case "ListLanguages":
		languages, ok := payload["languages"].([]any)
		if !ok || len(languages) == 0 {
			t.Fatalf("languages = %#v, want non-empty array", payload["languages"])
		}
	default:
		t.Fatalf("no greeting assertion registered for %s", method)
	}
}

func assertCalculatorPayload(t *testing.T, method string, payload map[string]any) {
	t.Helper()

	value, ok := payload["result"].(float64)
	if !ok {
		t.Fatalf("result = %#v, want float64", payload["result"])
	}
	expression, _ := payload["expression"].(string)

	switch method {
	case "Set":
		if math.Abs(value-20.0) > 1e-9 {
			t.Fatalf("result = %v, want 20", value)
		}
		if expression != "set → 20" {
			t.Fatalf("expression = %q, want %q", expression, "set → 20")
		}
	case "Add":
		if math.Abs(value-1.0) > 1e-9 {
			t.Fatalf("result = %v, want 1", value)
		}
		if expression != "0 + 1 = 1" {
			t.Fatalf("expression = %q, want %q", expression, "0 + 1 = 1")
		}
	default:
		t.Fatalf("no calculator assertion registered for %s", method)
	}
}
