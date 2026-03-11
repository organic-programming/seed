package verify

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExtractCommands(t *testing.T) {
	t.Parallel()

	taskFile := filepath.Join(t.TempDir(), "task.md")
	content := `# TASK

## Acceptance Criteria

- [ ] ` + "`go test ./...`" + `
- [ ] ` + "`go build ./...`" + `
- [ ] ` + "`go test ./...`" + `

## Checklist

- [ ] go vet ./...

## Notes

- [ ] ` + "`go test ./ignored`" + `
`
	if err := os.WriteFile(taskFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	commands, err := ExtractCommands(taskFile)
	if err != nil {
		t.Fatalf("ExtractCommands returned error: %v", err)
	}

	want := []string{"go test ./...", "go build ./...", "go vet ./..."}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %v, want %v", commands, want)
	}
}
