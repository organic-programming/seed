# TASK06 — go-holons SDK → proto-only

## Scope

`sdk/go-holons/`

## Changes

### `pkg/identity/identity.go`
- Remove `ParseHolon(yamlPath)` and its YAML unmarshalling.
- Remove `import "gopkg.in/yaml.v3"`.

### `pkg/identity/resolve.go`
- Remove `holon.yaml` fallback in `ResolveManifest()`. Error if no `holon.proto` found.
- Remove `TestResolveManifest_YAMLFallback` test.

### `pkg/describe/describe.go`
- Remove `case "holon.yaml":` branch. Only accept proto paths.

### `pkg/discover/discover.go`
- Remove `manifestFileName = "holon.yaml"` constant.
- Remove `addYAMLEntry()` and YAML-based parsing (`parseManifest`).
- Discovery scans for `holon.proto` only.

### `pkg/serve/serve.go`
- Remove `"holon.yaml"` from the filename check.

### Tests
- `describe_test.go`, `discover_test.go`, `serve_test.go`, `connect_test.go`, `identity_test.go`: update all to use `holon.proto` instead of `holon.yaml`.

### Dependencies
- Remove `gopkg.in/yaml.v3` from `go.mod` if identity and discover were the only consumers. Run `go mod tidy`.

### `README.md`
- Update identity module description.

## Verification

```bash
cd sdk/go-holons && go build ./... && go test ./...
```
