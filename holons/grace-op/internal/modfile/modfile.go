package modfile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ModFile struct {
	HolonPath string
	Require   []Require
	Replace   []Replace
}

type Require struct {
	Path    string
	Version string
}

type Replace struct {
	Old       string
	LocalPath string
}

func Parse(path string) (*ModFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	mod := &ModFile{}
	scanner := bufio.NewScanner(f)
	var inBlock string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if line == ")" {
			inBlock = ""
			continue
		}
		if line == "require (" {
			inBlock = "require"
			continue
		}
		if line == "replace (" {
			inBlock = "replace"
			continue
		}

		if strings.HasPrefix(line, "holon ") {
			mod.HolonPath = strings.TrimPrefix(line, "holon ")
			continue
		}

		switch inBlock {
		case "require":
			parts := strings.Fields(line)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid require line: %q", line)
			}
			mod.Require = append(mod.Require, Require{Path: parts[0], Version: parts[1]})
		case "replace":
			parts := strings.SplitN(line, " => ", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid replace line: %q", line)
			}
			mod.Replace = append(mod.Replace, Replace{
				Old:       strings.TrimSpace(parts[0]),
				LocalPath: strings.TrimSpace(parts[1]),
			})
		}
	}

	return mod, scanner.Err()
}

func (m *ModFile) Write(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "holon %s\n", m.HolonPath)

	if len(m.Require) > 0 {
		fmt.Fprintln(f)
		fmt.Fprintln(f, "require (")
		for _, r := range m.Require {
			fmt.Fprintf(f, "    %s %s\n", r.Path, r.Version)
		}
		fmt.Fprintln(f, ")")
	}

	if len(m.Replace) > 0 {
		fmt.Fprintln(f)
		fmt.Fprintln(f, "replace (")
		for _, r := range m.Replace {
			fmt.Fprintf(f, "    %s => %s\n", r.Old, r.LocalPath)
		}
		fmt.Fprintln(f, ")")
	}

	return nil
}

func (m *ModFile) AddRequire(path, version string) bool {
	for i, r := range m.Require {
		if r.Path == path {
			m.Require[i].Version = version
			return false
		}
	}
	m.Require = append(m.Require, Require{Path: path, Version: version})
	return true
}

func (m *ModFile) RemoveRequire(path string) bool {
	for i, r := range m.Require {
		if r.Path == path {
			m.Require = append(m.Require[:i], m.Require[i+1:]...)
			return true
		}
	}
	return false
}

func (m *ModFile) ResolvedPath(depPath string) string {
	for _, r := range m.Replace {
		if r.Old == depPath {
			return r.LocalPath
		}
	}
	return ""
}

type SumEntry struct {
	Path    string
	Version string
	Hash    string
}

type SumFile struct {
	Entries []SumEntry
}

func ParseSum(path string) (*SumFile, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SumFile{}, nil
		}
		return nil, err
	}
	defer f.Close()

	sum := &SumFile{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid holon.sum line: %q", line)
		}
		sum.Entries = append(sum.Entries, SumEntry{
			Path:    parts[0],
			Version: parts[1],
			Hash:    parts[2],
		})
	}
	return sum, scanner.Err()
}

func (s *SumFile) Write(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sort.Slice(s.Entries, func(i, j int) bool {
		if s.Entries[i].Path != s.Entries[j].Path {
			return s.Entries[i].Path < s.Entries[j].Path
		}
		return s.Entries[i].Version < s.Entries[j].Version
	})

	for _, e := range s.Entries {
		fmt.Fprintf(f, "%s %s %s\n", e.Path, e.Version, e.Hash)
	}
	return nil
}

func (s *SumFile) Set(path, version, hash string) {
	for i, e := range s.Entries {
		if e.Path == path && e.Version == version {
			s.Entries[i].Hash = hash
			return
		}
	}
	s.Entries = append(s.Entries, SumEntry{Path: path, Version: version, Hash: hash})
}

func (s *SumFile) Lookup(path, version string) string {
	for _, e := range s.Entries {
		if e.Path == path && e.Version == version {
			return e.Hash
		}
	}
	return ""
}
