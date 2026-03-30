package holons

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/progress"
)

type InstallOptions struct {
	Build            bool
	LinkApplications bool
	Progress         progress.Reporter
}

type InstallReport struct {
	Operation   string   `json:"operation"`
	Target      string   `json:"target"`
	Holon       string   `json:"holon"`
	Dir         string   `json:"dir,omitempty"`
	Manifest    string   `json:"manifest,omitempty"`
	Binary      string   `json:"binary,omitempty"`
	BuildTarget string   `json:"build_target,omitempty"`
	BuildMode   string   `json:"build_mode,omitempty"`
	Artifact    string   `json:"artifact,omitempty"`
	Installed   string   `json:"installed,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

func Install(ref string, opts InstallOptions) (InstallReport, error) {
	reporter := opts.Progress
	if reporter == nil {
		reporter = progress.Silence()
	}
	reporter.Step("resolving " + normalizedTarget(ref) + "...")
	target, err := ResolveTarget(ref)
	if err != nil {
		return InstallReport{
			Operation: "install",
			Target:    normalizedTarget(ref),
		}, err
	}
	if target.ManifestErr != nil {
		return baseInstallReport("install", target, BuildContext{}), target.ManifestErr
	}
	if target.Manifest == nil {
		return baseInstallReport("install", target, BuildContext{}), fmt.Errorf("no %s found in %s", identity.ProtoManifestFileName, target.RelativePath)
	}

	ctx, err := resolveBuildContext(target.Manifest, BuildOptions{})
	if err != nil {
		return baseInstallReport("install", target, BuildContext{}), err
	}

	report := baseInstallReport("install", target, ctx)
	artifactPath := target.Manifest.ArtifactPath(ctx)
	report.Artifact = workspaceRelativePath(artifactPath)
	report.Binary = target.Manifest.BinaryName()
	installName := installNameForArtifact(target, artifactPath)
	if installName == "" {
		return report, fmt.Errorf("cannot resolve install name for %q", report.Holon)
	}
	if report.Binary == "" {
		report.Binary = installName
	}

	artifactExisted := true
	if _, err := os.Stat(artifactPath); err != nil {
		if !os.IsNotExist(err) {
			return report, err
		}
		artifactExisted = false
	}

	if opts.Build {
		if artifactExisted {
			if err := os.RemoveAll(artifactPath); err != nil {
				return report, fmt.Errorf("remove stale artifact %s: %w", report.Artifact, err)
			}
		}
		_, buildErr := ExecuteLifecycle(OperationBuild, ref, BuildOptions{Progress: reporter})
		if buildErr != nil {
			report.Notes = append(report.Notes, "build failed before install")
			return report, buildErr
		}
		if artifactExisted {
			report.Notes = append(report.Notes, "rebuilt before install")
		} else {
			report.Notes = append(report.Notes, "artifact missing; built before install")
		}

		artifactPath = target.Manifest.ArtifactPath(ctx)
		report.Artifact = workspaceRelativePath(artifactPath)
	} else if !artifactExisted {
		return report, fmt.Errorf("artifact not found at %s; run op build first", report.Artifact)
	}

	if _, err := os.Stat(artifactPath); err != nil {
		if os.IsNotExist(err) {
			return report, fmt.Errorf("artifact not found at %s; run op build first", report.Artifact)
		}
		return report, err
	}

	if err := openv.Init(); err != nil {
		return report, fmt.Errorf("prepare %s: %w", openv.OPBIN(), err)
	}

	installedPath := filepath.Join(openv.OPBIN(), installName)

	// Self-install: install as a proper .holon package (like every other
	// holon), then create a symlink $OPBIN/<binary> → <pkg>/bin/<arch>/<binary>
	// so that `op` remains directly callable from PATH.
	if isSelfInstall(target.Manifest) {
		reporter.Step("copying to " + installedPath + "...")
		if err := copyArtifact(artifactPath, installedPath); err != nil {
			return report, fmt.Errorf("install %s: %w", installName, err)
		}
		report.Installed = installedPath
		report.Notes = append(report.Notes, "installed into "+installedPath)

		binaryName := target.Manifest.BinaryName()
		// Relative symlink: e.g. grace-op.holon/bin/darwin_arm64/op
		symlinkRel := filepath.Join(installName, "bin", runtimeArchitecture(), binaryName)
		symlinkPath := filepath.Join(openv.OPBIN(), binaryName)
		_ = os.Remove(symlinkPath)
		if err := os.Symlink(symlinkRel, symlinkPath); err != nil {
			return report, fmt.Errorf("symlink %s: %w", symlinkPath, err)
		}
		report.Notes = append(report.Notes, "symlinked "+symlinkPath+" → "+symlinkRel)
		return report, nil
	}

	// App bundles: wrap in a .holon package so the .app lives under
	// bin/<arch>/ — consistent with the package spec and enabling
	// multi-target/multi-platform installs.
	if isMacAppBundlePath(artifactPath) {
		slug := filepath.Base(target.Dir)
		pkgName := slug + ".holon"
		pkgPath := filepath.Join(openv.OPBIN(), pkgName)
		archDir := filepath.Join(pkgPath, "bin", runtimeArchitecture())
		appName := filepath.Base(artifactPath)

		reporter.Step("copying to " + pkgPath + "...")
		_ = os.RemoveAll(pkgPath)
		if err := os.MkdirAll(archDir, 0o755); err != nil {
			return report, fmt.Errorf("create %s: %w", archDir, err)
		}
		if err := copyArtifact(artifactPath, filepath.Join(archDir, appName)); err != nil {
			return report, fmt.Errorf("install %s: %w", pkgName, err)
		}
		if err := writeHolonJSONForInstall(target.Manifest, pkgPath); err != nil {
			return report, fmt.Errorf("write .holon.json: %w", err)
		}

		report.Installed = pkgPath
		report.Notes = append(report.Notes, "installed into "+pkgPath)
		if opts.LinkApplications {
			linkPath, err := linkBundleIntoApplications(filepath.Join(archDir, appName))
			if err != nil {
				return report, fmt.Errorf("link %s into Applications: %w", appName, err)
			}
			report.Notes = append(report.Notes, "linked into "+linkPath)
		}
		return report, nil
	}

	reporter.Step("copying to " + installedPath + "...")
	if err := copyArtifact(artifactPath, installedPath); err != nil {
		return report, fmt.Errorf("install %s: %w", installName, err)
	}
	report.Installed = installedPath
	report.Notes = append(report.Notes, "installed into "+installedPath)
	return report, nil
}

func Uninstall(ref string) (InstallReport, error) {
	return UninstallWithOptions(ref, InstallOptions{})
}

func UninstallWithOptions(ref string, opts InstallOptions) (InstallReport, error) {
	reporter := opts.Progress
	if reporter == nil {
		reporter = progress.Silence()
	}
	reporter.Step("resolving " + normalizedTarget(ref) + "...")
	target, err := ResolveTarget(ref)
	if err == nil && target.ManifestErr != nil {
		return baseInstallReport("uninstall", target, BuildContext{}), target.ManifestErr
	}

	report := InstallReport{
		Operation: "uninstall",
		Target:    normalizedTarget(ref),
	}
	if err == nil {
		report = baseInstallReport("uninstall", target, BuildContext{})
	}

	binaryName := strings.TrimSpace(ref)
	if err == nil {
		ctx, ctxErr := resolveBuildContext(target.Manifest, BuildOptions{})
		if ctxErr == nil {
			binaryName = installNameForArtifact(target, target.Manifest.ArtifactPath(ctx))
		} else {
			binaryName = installNameForArtifact(target, target.Manifest.ArtifactPath(BuildContext{}))
		}
	}
	if binaryName == "" && err == nil {
		return report, fmt.Errorf("cannot resolve install name for %q", ref)
	}
	if installedPath := lookupInstalledArtifactInOPBIN(binaryName); installedPath != "" {
		report.Installed = installedPath
		report.Binary = installedBinaryName(target, installedPath)
	} else {
		report.Binary = installedBinaryName(target, binaryName)
		report.Installed = filepath.Join(openv.OPBIN(), binaryName)
	}
	installedPath := report.Installed
	report.Installed = installedPath
	reporter.Step("removing " + installedPath + "...")
	if removeErr := os.RemoveAll(installedPath); removeErr != nil {
		if os.IsNotExist(removeErr) {
			report.Notes = append(report.Notes, "not installed")
			return report, nil
		}
		return report, fmt.Errorf("remove %s: %w", installedPath, removeErr)
	}
	if isMacAppBundlePath(installedPath) && runtime.GOOS == "darwin" {
		linkPath := filepath.Join(applicationsDir, filepath.Base(installedPath))
		if _, err := os.Lstat(linkPath); err == nil {
			_ = os.Remove(linkPath)
		}
	}
	// Self-install holons leave a symlink ($OPBIN/<binary>) alongside the
	// .holon package; clean it up.
	if target != nil && target.Manifest != nil && isSelfInstall(target.Manifest) {
		symlinkPath := filepath.Join(openv.OPBIN(), target.Manifest.BinaryName())
		if fi, lErr := os.Lstat(symlinkPath); lErr == nil && fi.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(symlinkPath)
		}
	}
	report.Notes = append(report.Notes, "removed "+installedPath)
	return report, nil
}

func installedBinaryName(target *Target, path string) string {
	if target != nil && target.Manifest != nil {
		if binary := target.Manifest.BinaryName(); binary != "" {
			return binary
		}
	}
	base := filepath.Base(strings.TrimSpace(path))
	if isHolonPackagePath(base) {
		return strings.TrimSuffix(base, ".holon")
	}
	return base
}

func baseInstallReport(operation string, target *Target, ctx BuildContext) InstallReport {
	report := InstallReport{
		Operation:   operation,
		Target:      normalizedTarget(target.Ref),
		Holon:       filepath.Base(target.Dir),
		Dir:         target.RelativePath,
		BuildTarget: ctx.Target,
		BuildMode:   ctx.Mode,
	}
	if ref := strings.TrimSpace(target.Ref); ref != "" && ref != "." && !strings.ContainsAny(ref, `/\`) {
		report.Holon = ref
	}
	if target.Manifest != nil {
		report.Manifest = workspaceRelativePath(target.Manifest.Path)
	}
	return report
}

func binaryNameForTarget(target *Target, artifactPath string) string {
	if target != nil && target.Manifest != nil {
		if binary := target.Manifest.BinaryName(); binary != "" {
			return binary
		}
	}
	if trimmed := strings.TrimSpace(artifactPath); trimmed != "" {
		return filepath.Base(trimmed)
	}
	if target != nil {
		return filepath.Base(target.Dir)
	}
	return ""
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", src)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Chmod(tmp, info.Mode()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// isSelfInstall returns true when the holon being installed is op itself.
// In that case, only the binary is installed (no .holon package directory).
func isSelfInstall(manifest *LoadedManifest) bool {
	return manifest != nil && manifest.BinaryName() == "op"
}
