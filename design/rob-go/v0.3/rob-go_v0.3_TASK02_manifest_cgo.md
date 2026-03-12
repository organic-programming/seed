# TASK02 — Manifest CGO Declaration

## Context

The `holon.yaml` manifest needs a way to declare that
Rob-Go should enable CGO passthrough by default, without
requiring every caller to send `CGO_ENABLED=1` per-RPC.

## Objective

Parse and honor a `delegates.toolchain.cgo` manifest section.

## Changes

### `holon.yaml` (schema extension)

```yaml
delegates:
  toolchain:
    name: go
    version: "1.24.0"
    source: https://go.dev/dl/
    cgo:
      enabled: true
      passthrough: [CC, CXX, AR, PKG_CONFIG_PATH]
```

### `internal/toolchain/config.go` [NEW]

```go
// CGOConfig holds CGO-related configuration from holon.yaml.
type CGOConfig struct {
    Enabled     bool
    Passthrough []string // host variable names to inherit
}

// LoadConfig reads delegates.toolchain from holon.yaml.
func LoadConfig(manifestPath string) (*ToolchainConfig, error)
```

### `internal/service/service.go`

At startup, read CGO config and pass `cgoPassthrough` to
`HermeticEnv` based on manifest or per-RPC override.

### `cmd/rob-go/main.go`

Load manifest config before constructing the toolchain.

## Acceptance Criteria

- [ ] `cgo.enabled: true` in manifest activates passthrough by default
- [ ] `cgo.passthrough` list controls which host variables are inherited
- [ ] Per-RPC `CGO_ENABLED=1` override still works regardless of manifest
- [ ] Missing `cgo` section defaults to `enabled: false`

## Dependencies

TASK01 (passthrough mechanism).
