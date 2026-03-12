# TASK03 — Embedded C Toolchain Provisioning

## Context

Stage 2 of CGO support: instead of relying on the host's
C compiler, Rob provisions and manages its own C toolchain
(candidate: Zig CC) alongside the Go distribution.

## Objective

Implement download, verification, and caching of an embedded
C cross-compiler under `$OPPATH/toolchains/cc/`.

## Changes

### `internal/toolchain/cc.go` [NEW]

```go
// CCToolchain manages an embedded C cross-compiler.
type CCToolchain struct {
    Name    string // e.g. "zig"
    Version string // e.g. "0.13.0"
    Root    string // e.g. ~/.op/toolchains/cc/zig-0.13.0
}

// EnsureCC checks if the C toolchain is cached. If not,
// downloads and verifies it.
func EnsureCC(name, version string) (*CCToolchain, error)

// CCPath returns the absolute path to the C compiler binary.
func (c *CCToolchain) CCPath() string

// CXXPath returns the absolute path to the C++ compiler binary.
func (c *CCToolchain) CXXPath() string
```

Storage layout:
```
$OPPATH/toolchains/cc/
├── zig-0.13.0/
│   ├── zig          ← compiler binary
│   └── lib/         ← sysroot and headers
└── current -> zig-0.13.0
```

### `internal/toolchain/cc_test.go` [NEW]

- `TestEnsureCCDownloads` — mock HTTP, verify extraction
- `TestEnsureCCCached` — verify no-download when present
- `TestCCPath` — verify correct binary path

## Acceptance Criteria

- [ ] `EnsureCC("zig", "0.13.0")` downloads and extracts Zig
- [ ] Subsequent calls skip download
- [ ] Checksum verified
- [ ] Cross-platform: correct archive for OS/arch

## Dependencies

None (parallel to TASK01/TASK02, but logically Stage 2).
