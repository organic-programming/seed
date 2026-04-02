package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const statusFile = "status.yaml"

func ReadStatus(dir string) (*Status, error) {
	path := filepath.Join(dir, statusFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var status Status
	if err := yaml.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	if status.Steps == nil {
		status.Steps = make(map[string]StepStatus)
	}
	return &status, nil
}

func WriteStatus(dir string, status *Status) error {
	if status == nil {
		return errors.New("status is nil")
	}
	if status.Steps == nil {
		status.Steps = make(map[string]StepStatus)
	}
	data, err := yaml.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	path := filepath.Join(dir, statusFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
