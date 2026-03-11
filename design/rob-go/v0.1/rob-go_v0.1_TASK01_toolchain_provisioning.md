# TASK01 — Toolchain Provisioning

## Context

Rob-Go must embed and manage the Go toolchain rather than
relying on `go` being on `$PATH`. The toolchain is stored
under `$OPPATH/toolchains/go/`.

## Objective

Implement a provisioning module that downloads, verifies, and
caches Go distributions.

## Changes

### `internal/toolchain/toolchain.go` [NEW]

Public API:

```go
// Toolchain manages an embedded Go distribution.
type Toolchain struct {
    Version string // e.g. "1.24.0"
    Root    string // e.g. ~/.op/toolchains/go/versions/go1.24.0
}

// Ensure checks if the pinned version is cached. If not, downloads
// and verifies it. Returns the path to the go binary.
func Ensure(version string) (*Toolchain, error)

// GoBinary returns the absolute path to the embedded go binary.
func (t *Toolchain) GoBinary() string
```

Implementation:
- Storage path: `$OPPATH/toolchains/go/versions/go<version>/`
- Download URL: `https://go.dev/dl/go<version>.<os>-<arch>.tar.gz`
- Verify SHA-256 checksum from `https://go.dev/dl/?mode=json`
- Extract with `archive/tar` + `compress/gzip`
- Create `current` symlink after success
- Never fall back to system `go`

### `internal/toolchain/toolchain_test.go` [NEW]

- `TestEnsureDownloadsToolchain` — mock HTTP, verify extraction
- `TestEnsureCached` — verify no-download when version exists
- `TestGoBinaryPath` — verify correct binary path

## Acceptance Criteria

- [ ] `Ensure("1.24.0")` downloads and extracts Go toolchain
- [ ] Subsequent calls skip download
- [ ] SHA-256 checksum verified
- [ ] Cross-platform: correct archive format (`.tar.gz` on Unix, `.zip` on Windows)
- [ ] Fails clearly when network is unavailable and version is not cached

## Dependencies

None.
