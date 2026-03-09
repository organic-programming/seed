# TASK002 — Package Manager Distribution for `op`

## Objective

Prepare the `op` CLI (Grace) for installation via mainstream package managers on macOS, Windows, and Linux. Currently `op` is only installable via `go install` or from source — this limits adoption to developers with a Go toolchain. Package manager distribution makes `op` accessible to any user with a single command.

## Target Package Managers

| Platform | Package Manager | Command | Priority |
|---|---|---|---|
| **macOS** | Homebrew | `brew install op` | 🔴 High |
| **Windows** | WinGet | `winget install op` | 🔴 High |
| **Windows** | Chocolatey | `choco install op` | 🟡 Medium |
| **Windows** | Scoop | `scoop install op` | 🟢 Low |
| **Linux** | APT (Debian/Ubuntu) | `apt install op` | 🟡 Medium |
| **Linux** | Snap | `snap install op` | 🟢 Low |
| **Cross-platform** | NPM | `npm install -g @organic-programming/op` | 🔴 High |
| **Cross-platform** | Go install (existing) | `go install ...` | ✅ Done |

---

## Prerequisites

### 1. Release Script (`scripts/releaser.go`)

Before any package manager can distribute `op`, a **pure Go release script** must produce platform-specific binaries. No GoReleaser, no proprietary YAML — just Go.

The script handles: **cross-compilation**, **checksums (SHA256)**, and **archiving (tar.gz / zip)**.

```go
// scripts/releaser.go — run with: go run scripts/releaser.go
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	appName   = "op"
	version   = "v0.1.0" // or read from git tag
	distDir   = "dist"
	platforms = []string{
		"darwin/amd64",
		"darwin/arm64",
		"linux/amd64",
		"linux/arm64",
		"windows/amd64",
	}
)

func main() {
	os.RemoveAll(distDir)
	os.MkdirAll(distDir, 0755)
	fmt.Printf("🚀 Building %s %s...\n", appName, version)

	for _, plat := range platforms {
		parts := strings.SplitN(plat, "/", 2)
		goos, goarch := parts[0], parts[1]

		binaryName := fmt.Sprintf("%s-%s-%s", appName, goos, goarch)
		if goos == "windows" {
			binaryName += ".exe"
		}
		binaryPath := filepath.Join(distDir, binaryName)

		// Cross-compile with stripped debug info
		fmt.Printf("📦 %s/%s...\n", goos, goarch)
		cmd := exec.Command("go", "build",
			"-ldflags", fmt.Sprintf("-s -w -X main.Version=%s", version),
			"-o", binaryPath, "./cmd/op")
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
		if err := cmd.Run(); err != nil {
			fmt.Printf("❌ Failed %s: %v\n", plat, err)
			continue
		}

		// Archive: tar.gz for Unix, zip for Windows
		if goos == "windows" {
			archiveZip(binaryPath)
		} else {
			archiveTarGz(binaryPath)
		}

		// SHA256 checksum
		writeChecksum(binaryPath)
	}

	fmt.Println("\n✅ Release artifacts in ./dist")
}

func archiveTarGz(path string) { /* tar.gz the binary */ }
func archiveZip(path string)   { /* zip the binary */ }
func writeChecksum(path string) {
	f, _ := os.Open(path)
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	os.WriteFile(path+".sha256", []byte(fmt.Sprintf("%x", h.Sum(nil))), 0644)
}
```

**Why pure Go instead of GoReleaser:**
- No limits — add S3 upload, npm publish, Slack ping with standard Go libraries
- No dependency — anyone with Go can run `go run scripts/releaser.go`
- No proprietary config — just Go code you can read and debug
- Consistent with OP philosophy: *"Go is the scripting language"*

**CI/CD integration** (GitHub Actions):

```yaml
- name: Build release
  run: go run scripts/releaser.go
- name: Upload to GitHub Release
  uses: softprops/action-gh-release@v1
  with:
    files: dist/*
```

- [ ] Write `scripts/releaser.go` with cross-compilation + checksum + archive
- [ ] Add `-ldflags` version injection (read from git tag)
- [ ] Add GitHub Actions workflow (`.github/workflows/release.yml`)
- [ ] Test: `go run scripts/releaser.go` produces binaries in `dist/`
- [ ] Verify: `dist/` contains binaries + `.tar.gz`/`.zip` + `.sha256` for all 5 platforms

- [ ] Check `brew search op`
- [ ] Check WinGet manifest repository for `op`
- [ ] Check `choco search op`
- [ ] Check APT repositories for `op`

---

## Per-Package Manager Steps

### Homebrew (macOS)

1. **Create a Homebrew Tap**: `organic-programming/homebrew-tap`
2. **Write the Formula** (`op.rb`):

```ruby
class Op < Formula
  desc "Organic Programming CLI — discover, build, and run holons"
  homepage "https://github.com/organic-programming/grace-op"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/organic-programming/grace-op/releases/download/v0.1.0/op-darwin-arm64.tar.gz"
      sha256 "PLACEHOLDER"
    else
      url "https://github.com/organic-programming/grace-op/releases/download/v0.1.0/op-darwin-amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  def install
    bin.install "op"
  end

  test do
    system "#{bin}/op", "version"
  end
end
```

3. **Install command**: `brew tap organic-programming/tap && brew install op`
4. **Automation**: release script outputs to `dist/`, CI publishes formula on tag

- [ ] Create `organic-programming/homebrew-tap` repo
- [ ] Write and test `op.rb` formula
- [ ] Integrate with release script + GitHub Actions for auto-update
- [ ] Test: `brew tap ... && brew install op && op version`

### WinGet (Windows)

1. **Create a WinGet manifest** (YAML):

```yaml
PackageIdentifier: OrganicProgramming.Op
PackageVersion: 0.1.0
PackageName: op
Publisher: Organic Programming
License: MIT
ShortDescription: Organic Programming CLI
Installers:
  - Architecture: x64
    InstallerUrl: https://github.com/organic-programming/grace-op/releases/download/v0.1.0/op-windows-amd64.zip
    InstallerSha256: PLACEHOLDER
    InstallerType: zip
```

2. **Submit PR** to [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs)
3. **Install command**: `winget install OrganicProgramming.Op`

- [ ] Write WinGet manifest
- [ ] Submit to `microsoft/winget-pkgs` and pass validation
- [ ] Test: `winget install OrganicProgramming.Op && op version`

### Chocolatey (Windows)

1. **Create a Chocolatey package** (`.nuspec` + install script)
2. **Publish** to [community.chocolatey.org](https://community.chocolatey.org/)
3. **Install command**: `choco install op`

- [ ] Create `.nuspec` package definition
- [ ] Write `chocolateyinstall.ps1` (download binary, place in PATH)
- [ ] Submit to Chocolatey community repo (requires moderation ~1-2 weeks)
- [ ] Test: `choco install op && op version`

### APT (Debian/Ubuntu/Linux)

1. **Create a PPA** or use GitHub as an APT repository
2. **Build `.deb` package** (GoReleaser supports nfpm for deb/rpm)
3. **Install command**: `sudo apt install op` (after adding repo)

- [ ] Configure nfpm (or pure Go `.deb` builder) for `.deb` output
- [ ] Set up APT repository (GitHub Pages or Cloudflare R2)
- [ ] Write installation instructions (add repo + apt install)
- [ ] Test: `apt install op && op version`

### NPM (Cross-platform)

Follow the **esbuild pattern**: publish a meta-package that depends on platform-specific sub-packages containing the actual binary. No `postinstall` download — binaries are in the npm registry itself.

1. **Create platform sub-packages**:
   - `@organic-programming/op-darwin-arm64`
   - `@organic-programming/op-darwin-x64`
   - `@organic-programming/op-linux-x64`
   - `@organic-programming/op-linux-arm64`
   - `@organic-programming/op-win32-x64`

2. **Create meta-package** `@organic-programming/op`:
   - Uses `optionalDependencies` to pull the right platform sub-package
   - Exports a bin entry that resolves to the platform binary

3. **Install commands**:
   - Global: `npm install -g @organic-programming/op`
   - One-shot: `npx @organic-programming/op version`

4. **Automation**: release script builds binaries, a publish function packages them into npm tarballs and publishes

- [ ] Create npm organization `@organic-programming`
- [ ] Write platform sub-package scaffolding (5 packages)
- [ ] Write meta-package with `optionalDependencies`
- [ ] Write publish script (triggered by GoReleaser on GitHub Release)
- [ ] Test: `npm install -g @organic-programming/op && op version`
- [ ] Test: `npx @organic-programming/op version`

---

## Deliverable: `INSTALL.md`

Create an `INSTALL.md` file in the grace-op holon root (`organic-programming/holons/grace-op/INSTALL.md`) that covers **every installation method** in a single user-facing document:

```markdown
# Installing op

## Quick Install (recommended)

### macOS
  brew tap organic-programming/tap && brew install op

### Windows
  winget install OrganicProgramming.Op

### Any platform (with Node.js)
  npm install -g @organic-programming/op

### Any platform (with Go)
  go install github.com/organic-programming/grace-op/cmd/op@latest

## Verify
  op version

## Other methods
  - Chocolatey (Windows): choco install op
  - APT (Debian/Ubuntu): instructions for adding repo
  - From source: git clone + go build

## Uninstall
  Per-manager uninstall commands
```

- [ ] Write `INSTALL.md` in `organic-programming/holons/grace-op/`
- [ ] Link from the main `grace-op/README.md`
- [ ] Link from `organic-programming/README.md`

---

## Recommended Execution Order

1. **GoReleaser setup** — prerequisite for everything else
2. **Homebrew** — quickest to ship (own tap, no external review)
3. **NPM** — cross-platform, reaches web developers immediately
4. **WinGet** — submission-based but straightforward
5. **Chocolatey** — moderation queue, start early
6. **APT** — more infrastructure, defer until demand warrants
7. **INSTALL.md** — write once all methods are confirmed working

---

## Verification

For each package manager, verify:

- [ ] Clean install from scratch (no Go toolchain present)
- [ ] `op version` outputs correct version
- [ ] `op env` shows correct `OPPATH`/`OPBIN`
- [ ] Upgrade path works when a new version is released
- [ ] Uninstall leaves no residual files
