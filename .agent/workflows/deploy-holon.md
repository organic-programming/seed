---
description: Version-bump, build, deploy, and push any holon during dev
---

# Deploy Holon

Every time any holon (including `op`) is modified and deployed during dev,
the **patch version must increment** (e.g., `0.2.0` → `0.2.1` → `0.2.2`).
Minor version increments are reserved for feature milestones.

For `op` specifically, the version lives in `cmd/op/main.go` as
`var version = "X.Y.Z"`. Other holons store their version in `holon.yaml`.

## Steps

1. Increment the **patch** version.

// turbo
2. Run tests:
   ```
   cd holons/grace-op && go test ./...
   ```

3. Build and deploy with commit hash:
   ```
   cd holons/grace-op && \
     git add -A && \
     git commit -m "<commit message>" && \
     COMMIT=$(git rev-parse HEAD) && \
     go build -ldflags "-X main.commit=$COMMIT" -o /usr/local/bin/op ./cmd/op && \
     git push
   ```

4. Verify:
   ```
   op version
   ```
   Expected output: `op X.Y.Z (<commit>)`

## Rules

- The commit hash is injected via `-ldflags "-X main.commit=<hash>"`.
- Always deploy to `/usr/local/bin/op`.
- Always push after deploying.
- Always update parent submodule pointers (organic-programming → videosteno).
- **Patch version increments on every deployment** — this rule applies to
  any holon during dev, not just `op`.
