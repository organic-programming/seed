# Codex Orchestrator v1 Tasks

These tasks build the orchestrator from scratch following the Effective Go layout
defined in [DESIGN.md §4](./DESIGN.md).

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./orchestrator_v1_TASK01_skeleton.md) | Project skeleton: `go.mod`, `cmd/`, `internal/` tree | — |
| 02 | [TASK02](./orchestrator_v1_TASK02_logging_engine.md) | Timestamped tee logging + JSONL capture (§3.4) | TASK01 |
| 03 | [TASK03](./orchestrator_v1_TASK03_dependency_graph.md) | Parse `_TASKS.md`, build DAG, topological sort (§3.6) | TASK01 |
| 04 | [TASK04](./orchestrator_v1_TASK04_prompt_builder.md) | 4-layer prompt assembly + context compression (§3.8) | TASK01 |
| 05 | [TASK05](./orchestrator_v1_TASK05_execution_loop.md) | Create → Verify → Fix loop with session resume (§3.9) | TASK02 |
| 06 | [TASK06](./orchestrator_v1_TASK06_error_handling.md) | Error classification + retry policy (§3.7) | TASK02, TASK05 |
| 07 | [TASK07](./orchestrator_v1_TASK07_lifecycle_engine.md) | Task lifecycle doctrine: start/complete/status (§3.5) | TASK02, TASK03 |
| 08 | [TASK08](./orchestrator_v1_TASK08_preflight.md) | Pre-flight checks + concurrency lock (§3.10, §3.13) | TASK01 |
| 09 | [TASK09](./orchestrator_v1_TASK09_submodule_access.md) | Submodule write access via `--add-dir` (§3.11) | TASK01 |
| 10 | [TASK10](./orchestrator_v1_TASK10_token_accounting.md) | Token usage tracking + post-run summary (§3.12, §3.15) | TASK02 |
| 11 | [TASK11](./orchestrator_v1_TASK11_signal_handling.md) | Graceful shutdown on SIGINT/SIGTERM (§3.14) | TASK02, TASK08 |
| 12 | [TASK12](./orchestrator_v1_TASK12_integration.md) | Wire all packages into `cmd/orchestrator/main.go` | TASK02–11 |

Design document: [DESIGN.md](./DESIGN.md)
