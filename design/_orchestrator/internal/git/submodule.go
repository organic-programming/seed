package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var repoRefRE = regexp.MustCompile(`github\.com/organic-programming/([A-Za-z0-9._-]+)`)

func ListSubmodulePaths(root string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "submodule", "foreach", "--quiet", "--recursive", "pwd")
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if strings.Contains(text, "No submodule mapping found") {
			return nil, nil
		}
		return nil, fmt.Errorf("list submodules: %w: %s", err, text)
	}

	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		absPath, err := filepath.Abs(line)
		if err != nil {
			return nil, err
		}
		paths = append(paths, filepath.Clean(absPath))
	}
	return paths, nil
}

func DetectRefs(taskContent string, submodules []string) []string {
	known := make(map[string]string, len(submodules))
	for _, submodule := range submodules {
		known[filepath.Base(submodule)] = submodule
	}

	var refs []string
	seen := make(map[string]struct{})
	for _, match := range repoRefRE.FindAllStringSubmatch(taskContent, -1) {
		name := match[1]
		path, ok := known[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: referenced submodule %s not found locally\n", name)
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		refs = append(refs, path)
	}

	for _, name := range parseRepositorySection(taskContent) {
		path, ok := known[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: referenced submodule %s not found locally\n", name)
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		refs = append(refs, path)
	}

	slices.Sort(refs)
	return refs
}

func parseRepositorySection(taskContent string) []string {
	lines := strings.Split(taskContent, "\n")
	inSection := false
	seen := make(map[string]struct{})
	var names []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inSection = trimmed == "## Repository"
			continue
		}
		if !inSection || trimmed == "" {
			continue
		}

		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "* ")
		for _, token := range splitRepositoryTokens(trimmed) {
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			names = append(names, token)
		}
	}

	return names
}

func splitRepositoryTokens(line string) []string {
	line = strings.NewReplacer(",", " ", ":", " ", "`", " ").Replace(line)
	fields := strings.Fields(line)
	var names []string
	for _, field := range fields {
		if bytes.ContainsRune([]byte(field), '/') {
			continue
		}
		names = append(names, field)
	}
	return names
}
