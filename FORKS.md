# OP-maintained third-party forks

Organic Programming maintains forks of two foundational libraries:

| Fork | Upstream | Role |
|---|---|---|
| `organic-programming/protobuf` | `protocolbuffers/protobuf` | Long-term mirror; base for OP-specific patches and upstream PRs. |
| `organic-programming/grpc` | `grpc/grpc` | Long-term mirror; same role. Currently also hosts a `.gitmodules` repointer to consume protobuf v32.0 while we stay on grpc 1.80.x. |

Both libraries sit at the gravitational center of OP's transport and IDL stack, so we keep direct authoring access on them rather than depending on upstream cadence.

## Conventions

- Default branch of each fork = exact mirror of upstream `master` (or release branch).
- OP-specific work lives on dedicated branches, e.g. `op/<topic>` or `chore/<task>`.
- When an OP patch is mergeable upstream, open a PR from the fork branch to upstream. Once merged, drop the local branch.
- Periodically (quarterly) rebase or merge upstream into the OP branches that we still consume.

## Current consumers

- `sdk/zig-holons/third_party/grpc` -> `organic-programming/grpc` at `chore/pipeline-redesign-and-protobuf-v32` (pinned).
- That submodule's own `third_party/protobuf` -> `protocolbuffers/protobuf` at `v32.0` directly (the patch we needed is upstream now; no OP-protobuf branch in the chain at the moment).

## When to retire a fork

Never proactively. The mirrors are cheap and protect us against blockers we can't yet predict. Only consider retirement if a fork has had no OP-specific commits for several years and we have full confidence in upstream's responsiveness for the surfaces we depend on.
