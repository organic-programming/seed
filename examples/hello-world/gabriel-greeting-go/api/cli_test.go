package api_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"gabriel-greeting-go/api"
)

func TestRunCLIVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := api.RunCLI([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCLI returned %d, want 0", code)
	}
	// The version is resolved from the manifest at runtime;
	// exact value is tested in integration tests.
	if got := strings.TrimSpace(stdout.String()); got == "" {
		t.Fatal("version output is empty")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCLIListLanguagesJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := api.RunCLI([]string{"listLanguages", "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCLI returned %d, want 0", code)
	}

	var payload struct {
		Languages []struct {
			Code   string `json:"code"`
			Name   string `json:"name"`
			Native string `json:"native"`
		} `json:"languages"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json output: %v\noutput=%s", err, stdout.String())
	}
	if len(payload.Languages) == 0 {
		t.Fatal("expected languages in output")
	}
	first := payload.Languages[0]
	if first.Code != "en" || first.Name != "English" {
		t.Fatalf("unexpected first language: %+v", first)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCLISayHelloText(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := api.RunCLI([]string{"sayHello", "Alice", "fr"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCLI returned %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "Bonjour Alice" {
		t.Fatalf("greeting = %q, want %q", got, "Bonjour Alice")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCLISayHelloDefaultsToEnglishJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := api.RunCLI([]string{"sayHello", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCLI returned %d, want 0", code)
	}

	var payload struct {
		Greeting string `json:"greeting"`
		Language string `json:"language"`
		LangCode string `json:"langCode"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json output: %v\noutput=%s", err, stdout.String())
	}
	if payload.Greeting != "Hello Mary" {
		t.Fatalf("greeting = %q, want %q", payload.Greeting, "Hello Mary")
	}
	if payload.LangCode != "en" {
		t.Fatalf("langCode = %q, want %q", payload.LangCode, "en")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
