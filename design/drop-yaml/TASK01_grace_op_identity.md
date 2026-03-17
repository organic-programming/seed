# TASK01 — grace-op: identity model → proto-only

## Scope

`holons/grace-op/internal/identity/`

## Changes

### `identity.go`

- Update package doc: "lives in holon.yaml" → "lives in holon.proto".
- Remove all `yaml:"..."` struct tags from `Identity`. The struct is populated from proto parsing only.

### `registry.go`

- Change `ManifestFileName = "holon.yaml"` → `"holon.proto"`.
- Remove `import "gopkg.in/yaml.v3"`.
- Remove `ParseHolonYAML()` and `ReadHolonYAML()` entirely.
- Replace callers with proto-based equivalents (see `identity.ResolveFromProtoFile`).
- `FindAll`, `FindAllWithPaths`, `ScanAllWithPaths`, `FindByUUID`: scan for `holon.proto` instead of `holon.yaml`.

### `writer.go`

- Remove `WriteHolonYAML()` and the YAML template function `holonTemplate()`.
- Replace with a `WriteHolonProto()` that writes a `.proto` file with the identity as proto options.

## Verification

```bash
cd holons/grace-op && go build ./...
cd holons/grace-op && go test ./internal/identity/...
```
