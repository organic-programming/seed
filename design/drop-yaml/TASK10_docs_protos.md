# TASK10 — Top-level docs + proto comments

## Scope

- Top-level markdown: `OP.md`, `CONVENTIONS.md`, `PROTOCOL.md`, `HOLON_PACKAGE.md`, `AGENT.md`
- SDK-level docs: `sdk/SDK_GUIDE.md`, `sdk/README.md`
- Proto definition: `sdk/go-holons/protos/holonmeta/v1/holonmeta.proto`
- Generated stubs (after proto comment update)
- `_IN_PROGRESS.md`: update the `holon.yaml` row from "superseded" to "**removed**"

## Changes

### Documentation

| File | What to update |
|------|---------------|
| `OP.md` | §4 "Holon YAML Manifest" → rewrite for proto. All discovery/manifest references. |
| `CONVENTIONS.md` | Directory tree, "holon.yaml is always at the holon root" rule. |
| `PROTOCOL.md` | Comment "Holon identity from holon.yaml" → "from holon.proto". |
| `HOLON_PACKAGE.md` | Remove migration fallback mentions, update discovery table. |
| `AGENT.md` | Discovery scan mention. |
| `sdk/SDK_GUIDE.md` | Identity module → "Read holon.proto". |
| `sdk/README.md` | Identity module → "read holon.proto". |

### Proto comment

`sdk/go-holons/protos/holonmeta/v1/holonmeta.proto` L18: "Holon identity from holon.yaml" → "Holon identity from holon.proto".

**Regenerate** stubs after:
- `sdk/go-holons/gen/go/holonmeta/v1/holonmeta.pb.go`
- `sdk/swift-holons/Sources/Holons/Gen/holonmeta/v1/holonmeta.pb.swift`
- `sdk/java-holons/src/main/java/gen/holonmeta/v1/Holonmeta.java`
- Other generated files as needed.

## Verification

```bash
grep -rn "holon\.yaml\|holonYAML\|holon_yaml\|HolonYaml" \
  --include='*.go' --include='*.swift' --include='*.java' --include='*.kt' \
  --include='*.rb' --include='*.py' --include='*.dart' --include='*.js' \
  --include='*.ts' --include='*.c' --include='*.cpp' --include='*.hpp' \
  --include='*.cs' --include='*.rs' --include='*.md' --include='*.proto' \
  . | grep -v legacy/
# Must return zero results.

find . -name "holon.yaml" -not -path "*/legacy/*"
# Must return zero results.
```
