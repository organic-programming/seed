//go:build ignore

package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var seedReleaseRE = regexp.MustCompile(`^(seed_release:\s*)"?([^"#\s]+)"?(.*)$`)

func bumpPatch(version string) (string, error) {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("seed_release is not a major.minor.patch version: %s", version)
	}
	values := make([]int, 3)
	for i, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return "", fmt.Errorf("seed_release is not a major.minor.patch version: %s", version)
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return "", fmt.Errorf("seed_release is not a major.minor.patch version: %s", version)
		}
		values[i] = value
	}
	return fmt.Sprintf("%d.%d.%d", values[0], values[1], values[2]+1), nil
}

func readSeedRelease(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		match := seedReleaseRE.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
		if match != nil {
			return match[2], nil
		}
	}
	return "", fmt.Errorf("%s does not contain seed_release", path)
}

func bumpFile(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	lines := strings.SplitAfter(string(data), "\n")
	out := make([]string, 0, len(lines))
	current := ""
	next := ""
	replaced := false
	for _, line := range lines {
		if line == "" {
			continue
		}
		newline := ""
		body := line
		if strings.HasSuffix(body, "\n") {
			newline = "\n"
			body = strings.TrimSuffix(body, "\n")
		}
		match := seedReleaseRE.FindStringSubmatch(strings.TrimSuffix(body, "\r"))
		if match != nil && !replaced {
			current = match[2]
			next, err = bumpPatch(current)
			if err != nil {
				return "", "", err
			}
			out = append(out, fmt.Sprintf(`%s"%s"%s%s`, match[1], next, match[3], newline))
			replaced = true
			continue
		}
		out = append(out, line)
	}
	if !replaced {
		return "", "", fmt.Errorf("%s does not contain seed_release", path)
	}
	if err := os.WriteFile(path, []byte(strings.Join(out, "")), 0o644); err != nil {
		return "", "", err
	}
	return current, next, nil
}

func setFile(path, version string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.SplitAfter(string(data), "\n")
	out := make([]string, 0, len(lines))
	current := ""
	replaced := false
	for _, line := range lines {
		if line == "" {
			continue
		}
		newline := ""
		body := line
		if strings.HasSuffix(body, "\n") {
			newline = "\n"
			body = strings.TrimSuffix(body, "\n")
		}
		match := seedReleaseRE.FindStringSubmatch(strings.TrimSuffix(body, "\r"))
		if match != nil && !replaced {
			current = match[2]
			out = append(out, fmt.Sprintf(`%s"%s"%s%s`, match[1], version, match[3], newline))
			replaced = true
			continue
		}
		out = append(out, line)
	}
	if !replaced {
		return "", fmt.Errorf("%s does not contain seed_release", path)
	}
	if err := os.WriteFile(path, []byte(strings.Join(out, "")), 0o644); err != nil {
		return "", err
	}
	return current, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: seed_release_bump <next-patch|read|bump-file|set-file> [...]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "next-patch":
		if len(os.Args) != 3 {
			err = fmt.Errorf("usage: seed_release_bump next-patch <version>")
			break
		}
		var next string
		next, err = bumpPatch(os.Args[2])
		if err == nil {
			fmt.Println(next)
		}
	case "read":
		if len(os.Args) != 3 {
			err = fmt.Errorf("usage: seed_release_bump read <path>")
			break
		}
		var version string
		version, err = readSeedRelease(os.Args[2])
		if err == nil {
			fmt.Println(version)
		}
	case "bump-file":
		if len(os.Args) != 3 {
			err = fmt.Errorf("usage: seed_release_bump bump-file <path>")
			break
		}
		var current, next string
		current, next, err = bumpFile(os.Args[2])
		if err == nil {
			fmt.Printf("current=%s\nnext=%s\n", current, next)
		}
	case "set-file":
		if len(os.Args) != 4 {
			err = fmt.Errorf("usage: seed_release_bump set-file <path> <version>")
			break
		}
		var current string
		current, err = setFile(os.Args[2], os.Args[3])
		if err == nil {
			fmt.Printf("current=%s\nnext=%s\n", current, os.Args[3])
		}
	default:
		err = fmt.Errorf("unknown command: %s", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
