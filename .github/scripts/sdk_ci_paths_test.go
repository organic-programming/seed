package scripts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDKCIPathsCommands(t *testing.T) {
	files := filepath.Join(t.TempDir(), "files.txt")
	if err := os.WriteFile(files, []byte("sdk/cpp-holons/src/runtime.cpp\nsdk/python-holons/holons/module\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runScript(t, "sdk_ci_paths.go", "classify", "--files", files)
	if !strings.Contains(out, "sdk_source=true\n") || !strings.Contains(out, "sdk_source_json=true\n") {
		t.Fatalf("classify output = %q", out)
	}
	out = runScript(t, "sdk_ci_paths.go", "publish-set", "--files", files)
	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("publish-set JSON = %q: %v", out, err)
	}
	want := []string{"cpp", "c", "ruby", "python", "csharp", "kotlin", "java", "js"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("publish-set = %#v, want %#v", got, want)
	}
}
