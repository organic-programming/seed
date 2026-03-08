# CODEX_SOPHIA_REMOVAL — Remove sophia-who dependency from grace-op

## Context

`protos/op/v1/op.proto` has been rewritten to inline all sophia-who
types (Clade, Status, ReproductionMode, HolonIdentity, all identity
request/response messages). The generated Go code in `gen/go/op/v1/`
has already been regenerated from this new proto. The sophia-who
import line is gone.

**Do NOT modify `op.proto` or regenerate `gen/go/op/v1/` — they are
already correct.**

## What to do

### 1. Copy `pkg/identity/` from sophia-who

The domain model for holon identity lives in sophia-who's
`pkg/identity/` package. Copy it into grace-op:

```
pkg/identity/
├── identity.go    # Identity struct, Clades, Statuses, New()
├── registry.go    # Scan/load logic
└── writer.go      # holon.yaml writer
```

Source: `$(go env GOMODCACHE)/github.com/organic-programming/sophia-who@v0.0.0-20260307110200-0cbfa5acdf7f/pkg/identity/`

After copying, update the package-internal import path if any files
import `github.com/organic-programming/sophia-who/...`. The new root
is `github.com/organic-programming/grace-op/pkg/identity`.

### 2. Replace all sophia-who proto imports

In every `.go` file that imports:
```go
sophiapb "github.com/organic-programming/sophia-who/gen/go/sophia_who/v1"
```

Change to:
```go
opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
```

Then rename all `sophiapb.X` references to `opv1.X`.

Files to update (15 files):
- `internal/who/who.go`
- `internal/who/who_test.go`
- `internal/server/server.go`
- `internal/server/server_test.go`
- `internal/cli/formatter.go`
- `internal/cli/formatter_test.go`
- `internal/cli/mem_compose.go`
- `internal/cli/who.go`
- `internal/cli/commands_test.go`
- `internal/cli/transport_chain_test.go`
- `internal/holons/discovery.go`
- `internal/holons/discovery_test.go`
- `internal/holons/lifecycle_test.go`
- `internal/mod/mod.go`
- `internal/mod/mod_test.go`

### 3. Replace `pkg/identity` import

Change:
```go
"github.com/organic-programming/sophia-who/pkg/identity"
```
To:
```go
"github.com/organic-programming/grace-op/pkg/identity"
```

### 4. Remove sophia-who from go.mod

```bash
go mod edit -droprequire github.com/organic-programming/sophia-who
go mod tidy
```

### 5. Delete `protos/sophia_who/`

This was a temporary copy. Remove the entire directory:
```bash
rm -rf protos/sophia_who/
```

### 6. Verify

```bash
go build ./...
go test ./...
git diff --check
```

All must pass with zero failures.

## Rules

- Do NOT modify `protos/op/v1/op.proto` — already correct.
- Do NOT regenerate `gen/go/op/v1/` — already correct.
- Do NOT touch `protos/holonmeta/` — it will be moved to go-holons SDK in TASK004.
- The `HolonEntry` message in op.proto now includes `relative_path` (field 3) — make sure Go code using HolonEntry populates this field where applicable.
- Run `go test ./...` and confirm zero failures before considering this done.
