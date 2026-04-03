package profile

import "fmt"

// DriverKind identifies the underlying CLI.
type DriverKind string

const (
	DriverCodex  DriverKind = "codex"
	DriverGemini DriverKind = "gemini"
	DriverOllama DriverKind = "ollama" // [EXPERIMENTAL]
)

// Profile configures a single AI CLI invocation.
type Profile struct {
	Name             string     `yaml:"name"`
	Driver           DriverKind `yaml:"driver"`
	Model            string     `yaml:"model,omitempty"`
	Tier             string     `yaml:"tier,omitempty"`
	ExtraArgs        []string   `yaml:"extra_args,omitempty"`
	QuotaProbePrompt string     `yaml:"quota_probe_prompt,omitempty"`
	QuotaPhrases     []string   `yaml:"quota_phrases,omitempty"`
}

// Validate ensures the profile can drive a supported CLI.
func (p Profile) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	switch p.Driver {
	case DriverCodex, DriverGemini:
		return nil
	case DriverOllama:
		if p.Model == "" {
			return fmt.Errorf("ollama profile %q requires model", p.Name)
		}
		return nil
	default:
		return fmt.Errorf("unknown driver %q", p.Driver)
	}
}

// DriverBinary returns the command name that must exist in PATH.
func (p Profile) DriverBinary() string {
	switch p.Driver {
	case DriverCodex:
		return "codex"
	case DriverGemini:
		return "gemini"
	case DriverOllama:
		return "ollama"
	default:
		return ""
	}
}
