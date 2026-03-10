# TASK12 — MVS Transitive Dependency Resolution

## Context

OP.md §11 specifies Minimum Version Selection (the Go modules
algorithm) for `op mod`. Today `op mod` only manages direct
dependencies — it does not follow transitive `holon.mod` files.

## Objective

Implement transitive dependency resolution using MVS so that
`op mod pull` fetches the full graph and `op mod graph` displays it.

## Algorithm

MVS is simple: for each dependency in the graph, select the
maximum of all minimum versions required by any path.

```
buildList(root):
    visited = {}
    queue = [root]
    for each module in queue:
        for each require in module.holon.mod:
            if require not in visited or visited[require] < require.version:
                visited[require] = require.version
                queue.append(require@version)
    return visited
```

No SAT solver, no backtracking. Deterministic.

## Changes

### 1. Graph traversal (`internal/mod/mod.go`)

Add a `resolve` function that:
1. Reads the root `holon.mod`
2. For each dependency, fetches it to cache (if not present)
3. Reads each dependency's `holon.mod` (if it exists)
4. Recursively collects all transitive requirements
5. Applies MVS: keep the maximum version for each module path

### 2. `op mod pull`

After MVS resolution, fetch all selected versions to cache.
Report both direct and transitive dependencies.

### 3. `op mod tidy`

After MVS resolution, ensure `holon.sum` contains hashes for
all transitive dependencies. Prune entries no longer in the graph.

### 4. `op mod graph`

Output the full transitive graph, not just direct edges.

### 5. `op mod update`

When updating a module, re-resolve the full graph with the new
minimum version and fetch any new transitive dependencies.

## Acceptance Criteria

- [ ] `op mod pull` fetches transitive dependencies
- [ ] MVS picks the maximum minimum version when conflicts exist
- [ ] `op mod graph` shows full transitive graph
- [ ] `op mod tidy` includes transitive hashes in `holon.sum`
- [ ] Circular dependencies are detected and reported as errors
- [ ] Dependencies with no `holon.mod` are treated as leaf nodes
- [ ] `go test ./...` — zero failures

## Dependencies

None. Independent of TASK00–10.
