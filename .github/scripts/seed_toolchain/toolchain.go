package seedtoolchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const FileName = "seed-toolchain.yaml"

type ToolchainEntry struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Target  string `json:"target,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
}

type SeedToolchain struct {
	SeedRelease string `yaml:"seed_release"`
	Protoc      struct {
		UpstreamTag     string            `yaml:"upstream_tag"`
		Version         string            `yaml:"version"`
		RequiredBy      map[string]bool   `yaml:"required_by"`
		SHA256PerTarget map[string]string `yaml:"sha256_per_target"`
	} `yaml:"protoc"`
	CPPRuntime struct {
		ProtobufSubmoduleTag string `yaml:"protobuf_submodule_tag"`
	} `yaml:"cpp_runtime"`
	Plugins map[string]map[string]any `yaml:"plugins"`
}

func Load(repoRoot string) (SeedToolchain, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, FileName))
	if err != nil {
		return SeedToolchain{}, err
	}
	var seed SeedToolchain
	if err := yaml.Unmarshal(data, &seed); err != nil {
		return SeedToolchain{}, fmt.Errorf("parse %s: %w", FileName, err)
	}
	return seed, nil
}

func SeedRelease(seed SeedToolchain) string {
	return strings.TrimSpace(seed.SeedRelease)
}

func ProtocVersion(seed SeedToolchain) string {
	if version := strings.TrimSpace(seed.Protoc.Version); version != "" {
		return strings.TrimPrefix(version, "v")
	}
	return strings.TrimPrefix(strings.TrimSpace(seed.Protoc.UpstreamTag), "v")
}

func CPPProtobufTag(seed SeedToolchain) string {
	return strings.TrimSpace(seed.CPPRuntime.ProtobufSubmoduleTag)
}

func SDKRequiresProtoc(seed SeedToolchain, lang string) bool {
	if seed.Protoc.RequiredBy == nil {
		return false
	}
	return seed.Protoc.RequiredBy[strings.TrimSpace(lang)]
}

func ToolchainManifest(seed SeedToolchain, lang, target string) ([]ToolchainEntry, error) {
	lang = strings.TrimSpace(lang)
	target = strings.TrimSpace(target)
	entries := []ToolchainEntry{}
	if SDKRequiresProtoc(seed, lang) {
		version := ProtocVersion(seed)
		if version == "" {
			return nil, fmt.Errorf("%s missing protoc version", FileName)
		}
		sha := strings.TrimSpace(seed.Protoc.SHA256PerTarget[target])
		if sha == "" {
			return nil, fmt.Errorf("%s missing protoc sha256 for %s", FileName, target)
		}
		entries = append(entries, ToolchainEntry{
			Name:    "protoc",
			Version: version,
			Target:  target,
			SHA256:  sha,
		})
	}
	plugins := seed.Plugins[lang]
	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := ToolchainEntry{Name: strings.TrimSpace(name)}
		switch value := plugins[name].(type) {
		case string:
			entry.Version = strings.TrimSpace(value)
		case map[string]any:
			if version, ok := value["version"].(string); ok {
				entry.Version = strings.TrimSpace(version)
			}
			if sha, ok := value["sha256"].(string); ok {
				entry.SHA256 = strings.TrimSpace(sha)
			}
			if perTarget, ok := value["sha256_per_target"].(map[string]any); ok {
				if sha, ok := perTarget[target].(string); ok {
					entry.Target = target
					entry.SHA256 = strings.TrimSpace(sha)
				}
			}
		case nil:
		default:
			entry.Version = strings.TrimSpace(fmt.Sprint(value))
		}
		if entry.Name != "" {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func PluginVersion(seed SeedToolchain, lang, name string) string {
	raw := seed.Plugins[strings.TrimSpace(lang)][strings.TrimSpace(name)]
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		if version, ok := value["version"].(string); ok {
			return strings.TrimSpace(version)
		}
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}

func PluginSHA256(seed SeedToolchain, lang, name, target string) string {
	raw := seed.Plugins[strings.TrimSpace(lang)][strings.TrimSpace(name)]
	value, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	if perTarget, ok := value["sha256_per_target"].(map[string]any); ok {
		if sha, ok := perTarget[strings.TrimSpace(target)].(string); ok {
			return strings.TrimSpace(sha)
		}
	}
	if sha, ok := value["sha256"].(string); ok {
		return strings.TrimSpace(sha)
	}
	return ""
}

func ManifestJSON(seed SeedToolchain, lang, target string) ([]byte, error) {
	entries, err := ToolchainManifest(seed, lang, target)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(entries, "", "    ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
