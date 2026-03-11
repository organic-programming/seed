package tasks

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var expectedTasksHeader = []string{"#", "File", "Summary", "Depends on", "Status"}

type Entry struct {
	Number    string
	FilePath  string
	Summary   string
	DependsOn []string
}

// Parse reads a _TASKS.md file and returns all task entries.
func Parse(tasksFile string) ([]Entry, error) {
	absTasksFile, err := filepath.Abs(tasksFile)
	if err != nil {
		return nil, fmt.Errorf("resolve tasks file %q: %w", tasksFile, err)
	}

	file, err := os.Open(absTasksFile)
	if err != nil {
		return nil, fmt.Errorf("open tasks file %q: %w", absTasksFile, err)
	}
	defer file.Close()

	baseDir := filepath.Dir(absTasksFile)
	scanner := bufio.NewScanner(file)

	var (
		lineNo          int
		headerSeen      bool
		separatorSeen   bool
		entries         []Entry
		startedDataRows bool
	)

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		if !headerSeen {
			cells, ok := parseTableRow(line)
			if !ok {
				continue
			}
			if sameCells(cells, expectedTasksHeader) {
				headerSeen = true
			}
			continue
		}

		if !separatorSeen {
			if line == "" {
				continue
			}
			cells, ok := parseTableRow(line)
			if !ok || !isSeparatorRow(cells) {
				return nil, fmt.Errorf("parse %s:%d: expected markdown table separator", absTasksFile, lineNo)
			}
			separatorSeen = true
			continue
		}

		if line == "" {
			if startedDataRows {
				continue
			}
			continue
		}

		cells, ok := parseTableRow(line)
		if !ok {
			if startedDataRows {
				break
			}
			continue
		}

		startedDataRows = true

		if len(cells) != len(expectedTasksHeader) {
			return nil, fmt.Errorf(
				"parse %s:%d: expected %d columns, got %d",
				absTasksFile,
				lineNo,
				len(expectedTasksHeader),
				len(cells),
			)
		}

		filePath, err := parseLinkedPath(baseDir, cells[1])
		if err != nil {
			return nil, fmt.Errorf("parse %s:%d: %w", absTasksFile, lineNo, err)
		}

		entries = append(entries, Entry{
			Number:    strings.TrimSpace(cells[0]),
			FilePath:  filePath,
			Summary:   strings.TrimSpace(cells[2]),
			DependsOn: parseDependencies(cells[3]),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read tasks file %q: %w", absTasksFile, err)
	}

	if !headerSeen {
		return nil, fmt.Errorf("parse %s: _TASKS.md table header not found", absTasksFile)
	}
	if !separatorSeen {
		return nil, fmt.Errorf("parse %s: markdown table separator not found", absTasksFile)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("parse %s: no task entries found", absTasksFile)
	}

	return entries, nil
}

// FindSetDir locates the version folder for a given set name (e.g. "v0.4")
// by scanning <root>/design/<project>/ for a matching directory.
// Returns the absolute path and the project name.
func FindSetDir(root, setName string) (setDir, project string, err error) {
	root = strings.TrimSpace(root)
	setName = strings.TrimSpace(setName)
	if root == "" {
		return "", "", fmt.Errorf("root cannot be empty")
	}
	if setName == "" {
		return "", "", fmt.Errorf("set name cannot be empty")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve root %q: %w", root, err)
	}
	absRoot = filepath.Clean(absRoot)

	// Support passing a specific project directory directly.
	if directSetDir, ok := findMatchingSetDir(absRoot, setName); ok {
		return directSetDir, filepath.Base(absRoot), nil
	}

	type match struct {
		setDir  string
		project string
	}

	var matches []match
	seen := make(map[string]struct{})

	for _, designRoot := range uniqueStrings([]string{
		filepath.Join(absRoot, "design"),
		absRoot,
	}) {
		found, err := scanDesignRoot(designRoot, setName)
		if err != nil {
			return "", "", err
		}
		for _, item := range found {
			if _, ok := seen[item.setDir]; ok {
				continue
			}
			seen[item.setDir] = struct{}{}
			matches = append(matches, item)
		}
	}

	switch len(matches) {
	case 0:
		return "", "", fmt.Errorf("set directory %s not found", setName)
	case 1:
		return matches[0].setDir, matches[0].project, nil
	default:
		projects := make([]string, 0, len(matches))
		for _, item := range matches {
			projects = append(projects, item.project)
		}
		sort.Strings(projects)
		return "", "", fmt.Errorf(
			"set directory %s is ambiguous across projects: %s",
			setName,
			strings.Join(projects, ", "),
		)
	}
}

func parseTableRow(line string) ([]string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil, false
	}

	parts := strings.Split(line[1:len(line)-1], "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}

	return cells, true
}

func sameCells(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func isSeparatorRow(cells []string) bool {
	if len(cells) != len(expectedTasksHeader) {
		return false
	}
	for _, cell := range cells {
		if cell == "" {
			return false
		}
		for _, r := range cell {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return true
}

func parseLinkedPath(baseDir, cell string) (string, error) {
	open := strings.Index(cell, "(")
	close := strings.LastIndex(cell, ")")
	if open == -1 || close == -1 || close <= open+1 {
		return "", fmt.Errorf("invalid file link %q", cell)
	}

	linkTarget := strings.TrimSpace(cell[open+1 : close])
	if linkTarget == "" {
		return "", fmt.Errorf("invalid file link %q", cell)
	}

	if !filepath.IsAbs(linkTarget) {
		linkTarget = filepath.Join(baseDir, filepath.FromSlash(linkTarget))
	}

	absPath, err := filepath.Abs(linkTarget)
	if err != nil {
		return "", fmt.Errorf("resolve file link %q: %w", cell, err)
	}

	return filepath.Clean(absPath), nil
}

func parseDependencies(cell string) []string {
	value := strings.TrimSpace(cell)
	if value == "" || value == "—" {
		return nil
	}

	parts := strings.Split(value, ",")
	deps := make([]string, 0, len(parts))
	for _, part := range parts {
		dep := strings.TrimSpace(part)
		if dep == "" || dep == "—" {
			continue
		}
		deps = append(deps, dep)
	}

	return deps
}

func scanDesignRoot(designRoot, setName string) ([]struct {
	setDir  string
	project string
}, error) {
	entries, err := os.ReadDir(designRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan design root %q: %w", designRoot, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var matches []struct {
		setDir  string
		project string
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		candidate, ok := findMatchingSetDir(filepath.Join(designRoot, entry.Name()), setName)
		if !ok {
			continue
		}

		absCandidate, err := filepath.Abs(candidate)
		if err != nil {
			return nil, fmt.Errorf("resolve set directory %q: %w", candidate, err)
		}

		matches = append(matches, struct {
			setDir  string
			project string
		}{
			setDir:  filepath.Clean(absCandidate),
			project: entry.Name(),
		})
	}

	return matches, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func findMatchingSetDir(baseDir, setName string) (string, bool) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !matchesSetDirName(entry.Name(), setName) {
			continue
		}
		return filepath.Join(baseDir, entry.Name()), true
	}

	return "", false
}

func matchesSetDirName(dirName, setName string) bool {
	return stripSetDirPrefix(strings.TrimSpace(dirName)) == strings.TrimSpace(setName)
}

func stripSetDirPrefix(dirName string) string {
	for _, prefix := range []string{"✅ ", "⚠️ ", "⚠ ", "💭 "} {
		if strings.HasPrefix(dirName, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(dirName, prefix))
		}
	}
	return dirName
}
