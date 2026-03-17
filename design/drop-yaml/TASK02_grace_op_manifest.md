# TASK02 — grace-op: manifest loading → proto-only

## Scope

`holons/grace-op/internal/holons/manifest.go`

## Changes

- Remove `ManifestFileName = "holon.yaml"` constant.
- Remove `ManifestSourceLabel()`.
- Remove `loadYAMLManifest()` function entirely.
- `LoadManifest()`: remove the YAML fallback path (lines reading `holon.yaml` after proto fails). If no `holon.proto` found, return an error directly.
- Remove `import "gopkg.in/yaml.v3"`.
- Remove all `yaml:"..."` struct tags from: `Manifest`, `BuildConfig`, `RecipeDefaults`, `RecipeMember`, `RecipeTarget`, `RecipeStep`, `RecipeStepExec`, `RecipeStepCopy`, `RecipeStepFile`, `RecipeStepCopyArtifact`, `Requires`, `Delegates`, `ArtifactPaths`, `Skill`, `Sequence`, `SequenceParam`.

These structs are populated exclusively via `manifestFromResolved()` from proto data.

## Depends on

TASK01 (identity changes referenced by manifest).

## Verification

```bash
cd holons/grace-op && go build ./...
cd holons/grace-op && go test ./internal/holons/...
```
