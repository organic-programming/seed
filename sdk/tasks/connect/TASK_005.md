# TASK_005 — End-to-End Runtime Verification

## Context

Depends on: `TASK_004` (all recipes use SDK connect).

Full QA pass: build, launch, and verify every recipe and every
hello-world example.

## What to do

### 1. Verify all 12 recipes

For each recipe (Build → Launch → UI → RPC → Daemon lifecycle):

#### Go-backend

- [ ] `go-swift-holons`
- [ ] `go-web-holons`
- [ ] `go-qt-holons`
- [ ] `go-dart-holons`
- [ ] `go-kotlin-holons`
- [ ] `go-dotnet-holons`

#### Rust-backend

- [ ] `rust-swift-holons`
- [ ] `rust-web-holons`
- [ ] `rust-qt-holons`
- [ ] `rust-dart-holons`
- [ ] `rust-kotlin-holons`
- [ ] `rust-dotnet-holons`

### 2. Verify 14 hello-world examples

- [ ] Build, run tests, run connect example if present.

### 3. Update compliance matrix

- [ ] Fill in every cell in `CONNECT.md` compliance matrix.
- [ ] Mark failures with `BLOCKED.md` per the 3-attempt rule.

## Rules

- Commit per-recipe.
- 3-attempt rule: if stuck after 3 attempts, write `BLOCKED.md`.
- Task complete when every row is ✅ or has `BLOCKED.md`.
