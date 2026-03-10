---
description: Version-bump, build, deploy, and push any holon during dev
---

# Deploy Holon

Every time a holon is modified and deployed during dev,
the **patch version must increment** (e.g., `0.2.0` → `0.2.1`).
Minor version increments are reserved for feature milestones.

## Prerequisites

`$OPPATH` and `$OPBIN` must be set and `$OPBIN` on `$PATH`.
If not done yet, add the output of `op env --shell` to your shell profile.

## Steps

1. Increment the **patch** version in `holon.yaml`.

// turbo
2. Run tests:
   ```
   op test <holon>
   ```

3. Build:
   ```
   op build <holon>
   ```

4. Commit and push:
   ```
   cd <holon-dir> && \
     git add -A && \
     git commit -m "<commit message>" && \
     git push
   ```

5. Install:
   ```
   op install <holon>
   ```

6. Verify:
   ```
   <holon-binary> --version   # or equivalent
   ```

## Rules

- Always bump the patch version **before** building.
- Always commit and push **after** a successful build.
- Always update parent submodule pointers (organic-programming → videosteno).
- `op install` copies the built artifact into `$OPBIN` — no manual
  path needed.
