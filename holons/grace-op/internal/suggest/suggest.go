package suggest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/holons"
)

type Context struct {
	Command     string
	Holon       string
	Manifest    *holons.LoadedManifest
	BuildTarget string
	Artifact    string
	Installed   string
}

type item struct {
	Command     string
	Description string
}

var currentGOOS = func() string { return runtime.GOOS }

func Suggest(w io.Writer, command string, holon string, manifest *holons.Manifest) {
	if manifest == nil {
		Print(w, Context{Command: command, Holon: holon})
		return
	}
	Print(w, Context{
		Command: command,
		Holon:   holon,
		Manifest: &holons.LoadedManifest{
			Manifest: *manifest,
			Name:     holon,
		},
	})
}

func Print(w io.Writer, ctx Context) {
	items, note := buildSuggestions(ctx)
	if len(items) == 0 && note == "" {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Next steps:")
	for _, entry := range items {
		description := strings.TrimSpace(entry.Description)
		command := strings.TrimSpace(entry.Command)
		if description != "" {
			fmt.Fprintf(w, "    - %s\n", description)
		}
		if command != "" {
			if description != "" {
				fmt.Fprintf(w, "      %s\n", command)
			} else {
				fmt.Fprintf(w, "    %s\n", command)
			}
		}
	}
	if note != "" {
		fmt.Fprintf(w, "  Note: %s\n", note)
	}
}

func buildSuggestions(ctx Context) ([]item, string) {
	command := strings.TrimSpace(ctx.Command)
	holon := strings.TrimSpace(ctx.Holon)
	manifest := ctx.Manifest
	buildTarget := strings.TrimSpace(ctx.BuildTarget)

	switch command {
	case "build":
		items := []item{
			{Command: fmt.Sprintf("op test %s", holon), Description: "run tests"},
		}
		if hasBinary(manifest) {
			items = append(items, item{
				Command:     fmt.Sprintf("op install %s", holon),
				Description: "install to ~/.op/bin/",
			})
		}
		if hasContract(manifest) {
			items = append(items, item{
				Command:     fmt.Sprintf("op run %s:9090", holon),
				Description: "start gRPC server",
			})
		}
		run, note := launchSuggestion(manifest, buildTarget, ctx.Artifact, "build")
		if run != "" {
			items = append(items, item{Command: run, Description: "run directly"})
		}
		return items, note
	case "test":
		items := make([]item, 0, 2)
		if !artifactExists(manifest, buildTarget) {
			items = append(items, item{
				Command:     fmt.Sprintf("op build %s", holon),
				Description: "build the holon",
			})
		}
		if hasBinary(manifest) {
			items = append(items, item{
				Command:     fmt.Sprintf("op install %s", holon),
				Description: "install to ~/.op/bin/",
			})
		}
		return items, ""
	case "install":
		items := make([]item, 0, 2)
		if hasContract(manifest) {
			items = append(items, item{
				Command:     fmt.Sprintf("op run %s:9090", holon),
				Description: "start gRPC server",
			})
		}
		run, note := launchSuggestion(manifest, buildTarget, ctx.Installed, "install")
		if run != "" {
			items = append(items, item{Command: run, Description: "run directly"})
		}
		return items, note
	case "clean":
		if holon == "" {
			return nil, ""
		}
		return []item{{
			Command:     fmt.Sprintf("op build %s", holon),
			Description: "build again",
		}}, ""
	case "mod init":
		return []item{{
			Command:     "op mod add <module>",
			Description: "add a dependency",
		}}, ""
	case "mod pull":
		return []item{
			{Command: "op mod list", Description: "see all dependencies"},
			{Command: "op mod graph", Description: "visualize dependency graph"},
			{Command: "op build", Description: "build the project"},
		}, ""
	case "mod add":
		return []item{
			{Command: "op mod pull", Description: "fetch dependencies"},
			{Command: "op build", Description: "build the project"},
		}, ""
	case "mod tidy":
		return []item{
			{Command: "op mod pull", Description: "fetch dependencies"},
			{Command: "op build", Description: "build the project"},
		}, ""
	case "new":
		if holon == "" {
			return nil, ""
		}
		return []item{
			{Command: fmt.Sprintf("op check %s", holon), Description: "validate the manifest"},
			{Command: fmt.Sprintf("op build %s", holon), Description: "build the holon"},
		}, ""
	default:
		return nil, ""
	}
}

func hasBinary(manifest *holons.LoadedManifest) bool {
	return manifest != nil && manifest.BinaryName() != ""
}

func hasContract(manifest *holons.LoadedManifest) bool {
	if manifest == nil {
		return false
	}
	return manifest.Manifest.Contract != nil
}

func artifactExists(manifest *holons.LoadedManifest, buildTarget string) bool {
	if manifest == nil {
		return false
	}
	if manifest.Manifest.Kind == holons.KindNative || manifest.Manifest.Kind == holons.KindWrapper {
		info, err := os.Stat(manifest.BinaryPath())
		return err == nil && !info.IsDir()
	}
	ctx := holons.BuildContext{Target: buildTarget}
	if ctx.Target == "" {
		ctx.Target = runtimeTarget()
	}
	path := manifest.PrimaryArtifactPath(ctx)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func launchSuggestion(manifest *holons.LoadedManifest, buildTarget, explicitPath, phase string) (string, string) {
	if manifest == nil {
		return "", ""
	}
	if manifest.Manifest.Kind != holons.KindNative && manifest.Manifest.Kind != holons.KindWrapper {
		return "", ""
	}
	if !hasBinary(manifest) {
		return "", ""
	}

	target := strings.TrimSpace(buildTarget)
	if target == "" {
		target = runtimeTarget()
	}
	if !platformMatches(target, currentGOOS()) {
		return "", fmt.Sprintf("built for %s, current platform is %s — no launch command available", target, currentGOOS())
	}

	path := strings.TrimSpace(explicitPath)
	if path == "" {
		switch phase {
		case "install":
			path = filepath.Join(openv.OPBIN(), manifest.Name+".holon")
		default:
			path = relativeToCWD(manifest.BinaryPath())
		}
	}
	path = holons.LaunchableArtifactPath(path, manifest)
	if path == "" {
		return "", ""
	}

	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".app") && currentGOOS() == "darwin":
		return "open " + shellQuote(path), ""
	case strings.HasSuffix(lower, ".desktop") && currentGOOS() == "linux":
		return "xdg-open " + shellQuote(path), ""
	case currentGOOS() == "windows":
		if !strings.HasSuffix(lower, ".exe") {
			path += ".exe"
		}
		return "start " + shellQuote(path), ""
	default:
		return shellQuote(path) + " --help", ""
	}
}

func relativeToCWD(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func shellQuote(path string) string {
	if !strings.ContainsAny(path, " \t") {
		return path
	}
	return fmt.Sprintf("%q", path)
}

func runtimeTarget() string {
	switch currentGOOS() {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	case "linux":
		return "linux"
	default:
		return currentGOOS()
	}
}

func platformMatches(target, goos string) bool {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", "all":
		return true
	case "macos":
		return goos == "darwin"
	case "windows":
		return goos == "windows"
	case "linux":
		return goos == "linux"
	case "android":
		return goos == "android"
	default:
		return false
	}
}
