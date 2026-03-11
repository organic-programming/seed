package verify

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var commandPatterns = []string{
	"go test",
	"go build",
	"go vet",
	"op build",
	"op check",
	"op run",
	"cargo test",
	"swift test",
	"flutter test",
}

var backtickCommandRE = regexp.MustCompile("`([^`]+)`")

func ExtractCommands(taskFile string) ([]string, error) {
	content, err := os.ReadFile(taskFile)
	if err != nil {
		return nil, fmt.Errorf("read task file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	inSection := false
	seen := make(map[string]struct{})
	var commands []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inSection = trimmed == "## Acceptance Criteria" || trimmed == "## Checklist"
			continue
		}
		if !inSection || trimmed == "" {
			continue
		}

		for _, match := range backtickCommandRE.FindAllStringSubmatch(trimmed, -1) {
			command := strings.TrimSpace(match[1])
			if !looksLikeCommand(command) || seenCommand(seen, command) {
				continue
			}
			commands = append(commands, command)
		}

		candidate := stripListPrefix(trimmed)
		if !looksLikeCommand(candidate) || seenCommand(seen, candidate) {
			continue
		}
		commands = append(commands, candidate)
	}

	return commands, nil
}

func stripListPrefix(line string) string {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"- [ ] ", "- ", "* [ ] ", "* ", "1. ", "2. ", "3. "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return line
}

func looksLikeCommand(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, pattern := range commandPatterns {
		if strings.HasPrefix(value, pattern) {
			return true
		}
	}
	return false
}

func seenCommand(seen map[string]struct{}, command string) bool {
	if _, ok := seen[command]; ok {
		return true
	}
	seen[command] = struct{}{}
	return false
}
