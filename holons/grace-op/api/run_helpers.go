package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/progress"
	"github.com/organic-programming/grace-op/internal/runpolicy"
)

type runIO struct {
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
	forwardSignal bool
	progress      progress.Reporter
}

func Run(req *opv1.RunRequest) (*opv1.RunResponse, error) {
	return runWithIO(req, runIO{})
}

func runWithIO(req *opv1.RunRequest, ioCfg runIO) (*opv1.RunResponse, error) {
	if req == nil || strings.TrimSpace(req.GetHolon()) == "" {
		return nil, fmt.Errorf("holon is required")
	}

	holonName := strings.TrimSpace(req.GetHolon())
	response := &opv1.RunResponse{
		Holon:     holonName,
		ListenUri: strings.TrimSpace(req.GetListenUri()),
	}
	listenURI, err := runpolicy.NormalizeRunListenURI(req.GetListenUri(), strings.TrimSpace(req.GetListenUri()) != "")
	if err != nil {
		return response, err
	}
	response.ListenUri = listenURI
	reporter := ioCfg.progress
	if reporter == nil {
		reporter = progress.Silence()
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	runCommand := func(cmd *exec.Cmd) error {
		if ioCfg.stdin != nil {
			cmd.Stdin = ioCfg.stdin
		}
		cmd.Stdout = multiWriter(&stdout, ioCfg.stdout)
		cmd.Stderr = multiWriter(&stderr, ioCfg.stderr)
		response.Command = cmd.Path
		response.Args = append([]string(nil), cmd.Args[1:]...)

		var err error
		if ioCfg.forwardSignal {
			err = runForeground(cmd)
		} else {
			err = cmd.Run()
		}
		response.Stdout = stdout.String()
		response.Stderr = stderr.String()
		if code, ok := commandExitCode(err); ok {
			response.ExitCode = int32(code)
			return nil
		}
		response.ExitCode = 0
		return err
	}

	var resolvedTarget *holons.Target
	reporter.Step("resolving " + holonName + "...")
	if target, resolveErr := holons.ResolveTarget(holonName); resolveErr == nil && target != nil && target.ManifestErr == nil && target.Manifest != nil {
		resolvedTarget = target
	}

	if binary := holons.ResolveInstalledBinary(holonName); binary != "" {
		response.ResolvedTarget = binary
		reporter.Step("launching " + holonName + "...")
		cmd, err := commandForInstalledArtifact(binary, resolvedTarget, listenURI)
		if err != nil {
			return response, err
		}
		err = runCommand(cmd)
		return response, err
	}

	target, err := holons.ResolveTarget(holonName)
	if err != nil {
		return response, err
	}
	if target.ManifestErr != nil {
		return response, target.ManifestErr
	}
	if target.Manifest == nil {
		return response, fmt.Errorf("no %s found in %s", identity.ProtoManifestFileName, target.RelativePath)
	}

	response.ResolvedTarget = target.RelativePath
	ctx, err := holons.ResolveBuildContext(target.Manifest, holons.BuildOptions{
		Target: req.GetTarget(),
		Mode:   req.GetMode(),
	})
	if err != nil {
		return response, err
	}
	if ctx.Target == "all" {
		return response, fmt.Errorf("target %q cannot be launched", ctx.Target)
	}

	if target.Manifest.Manifest.Kind == holons.KindComposite && listenURI != "stdio://" && strings.TrimSpace(req.GetListenUri()) != "" {
		return response, fmt.Errorf("--listen is only supported for service holons")
	}

	artifactPath := target.Manifest.ArtifactPath(ctx)
	if artifactPath == "" {
		return response, fmt.Errorf("no artifact declared for target %q mode %q", ctx.Target, ctx.Mode)
	}
	response.Artifact = artifactPath

	if _, err := os.Stat(artifactPath); err != nil {
		if !os.IsNotExist(err) {
			return response, err
		}
		if req.GetNoBuild() {
			return response, fmt.Errorf("artifact missing: %s", artifactPath)
		}
		if _, err := holons.ExecuteLifecycle(holons.OperationBuild, holonName, holons.BuildOptions{
			Target:   req.GetTarget(),
			Mode:     req.GetMode(),
			Progress: reporter,
		}); err != nil {
			return response, err
		}
	}

	cmd, err := commandForArtifact(target.Manifest, ctx, listenURI)
	if err != nil {
		return response, err
	}
	reporter.Step("launching " + holonName + "...")
	if err := runCommand(cmd); err != nil {
		return response, err
	}
	return response, nil
}

func commandForInstalledArtifact(path string, target *holons.Target, listenURI string) (*exec.Cmd, error) {
	var manifest *holons.LoadedManifest
	if target != nil {
		manifest = target.Manifest
	}
	path = holons.LaunchableArtifactPath(path, manifest)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		if isMacAppBundle(path) && runtime.GOOS == "darwin" {
			return exec.Command("open", "-W", path), nil
		}
		return nil, fmt.Errorf("artifact is not directly launchable: %s", path)
	}
	if isHTMLFile(path) {
		switch runtime.GOOS {
		case "darwin":
			return exec.Command("open", "-W", path), nil
		case "linux":
			return exec.Command("xdg-open", path), nil
		case "windows":
			return exec.Command("cmd", "/c", "start", "", path), nil
		default:
			return nil, fmt.Errorf("cannot open %s on %s", path, runtime.GOOS)
		}
	}
	if target != nil && manifest != nil && manifest.Manifest.Kind == holons.KindComposite {
		return exec.Command(path), nil
	}
	cmd := exec.Command(path, serveArgs(listenURI)...)
	cmd.Dir = runCommandDir(target, manifest, path)
	return cmd, nil
}

func commandForArtifact(manifest *holons.LoadedManifest, ctx holons.BuildContext, listenURI string) (*exec.Cmd, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest required")
	}
	if manifest.Manifest.Kind == holons.KindComposite {
		artifactPath := manifest.ArtifactPath(ctx)
		info, err := os.Stat(artifactPath)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			if isMacAppBundle(artifactPath) && runtime.GOOS == "darwin" {
				return exec.Command("open", "-W", artifactPath), nil
			}
			return nil, fmt.Errorf("artifact is not directly launchable: %s", artifactPath)
		}
		if isHTMLFile(artifactPath) {
			switch runtime.GOOS {
			case "darwin":
				return exec.Command("open", "-W", artifactPath), nil
			case "linux":
				return exec.Command("xdg-open", artifactPath), nil
			case "windows":
				return exec.Command("cmd", "/c", "start", "", artifactPath), nil
			default:
				return nil, fmt.Errorf("cannot open %s on %s", artifactPath, runtime.GOOS)
			}
		}
		return exec.Command(artifactPath), nil
	}

	binaryPath := manifest.BinaryPath()
	if strings.TrimSpace(binaryPath) == "" {
		return nil, fmt.Errorf("no binary declared for %s", manifest.Name)
	}
	cmd := exec.Command(binaryPath, serveArgs(listenURI)...)
	cmd.Dir = runCommandDir(nil, manifest, binaryPath)
	return cmd, nil
}

func serveArgs(listenURI string) []string {
	return []string{"serve", "--listen", listenURI}
}

func runCommandDir(target *holons.Target, manifest *holons.LoadedManifest, artifactPath string) string {
	if manifest != nil && strings.TrimSpace(manifest.Dir) != "" {
		return manifest.Dir
	}
	if target != nil && strings.TrimSpace(target.Dir) != "" {
		return target.Dir
	}
	if trimmed := strings.TrimSpace(artifactPath); trimmed != "" {
		return filepath.Dir(trimmed)
	}
	return ""
}

func isMacAppBundle(path string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(path)), ".app")
}

func isHTMLFile(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm")
}

func multiWriter(buffer *bytes.Buffer, target io.Writer) io.Writer {
	if target == nil {
		return buffer
	}
	return io.MultiWriter(buffer, target)
}

func runForeground(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	signals := []os.Signal{os.Interrupt}
	if runtime.GOOS != "windows" {
		signals = append(signals, syscall.SIGTERM)
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, signals...)
	defer signal.Stop(sigCh)

	for {
		select {
		case err := <-waitCh:
			return err
		case sig := <-sigCh:
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}
}

func commandExitCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}
	return 0, false
}

func parseLegacyRunTarget(value string) (string, string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsAny(trimmed, `/\`) {
		return "", "", false
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), "tcp://:" + strings.TrimSpace(parts[1]), true
}

func resolveRunRequest(holonName string, listenURI string, noBuild bool, target string, mode string) *opv1.RunRequest {
	if legacyName, legacyListen, ok := parseLegacyRunTarget(holonName); ok && strings.TrimSpace(listenURI) == "" {
		holonName = legacyName
		listenURI = legacyListen
	}
	return &opv1.RunRequest{
		Holon:     holonName,
		ListenUri: listenURI,
		NoBuild:   noBuild,
		Target:    target,
		Mode:      mode,
	}
}

func artifactLabel(path string) string {
	if path == "" {
		return ""
	}
	if rel, err := filepath.Rel(".", path); err == nil {
		return rel
	}
	return path
}
