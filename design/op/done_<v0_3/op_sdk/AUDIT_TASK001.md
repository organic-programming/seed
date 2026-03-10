# TODO: Documentation Audit & SDK Guide Revision

Depends on: `TODO_DISCOVER.md`, `TODO_CONNECT.md`,
`TODO_MIGRATE_RECIPES.md`.

Run this **after** discover/connect are implemented and recipes are
migrated. Its purpose is to ensure all documentation across the
project reflects the real state of the code.

## 1. Audit `SDK_GUIDE.md`

- [ ] Verify every ✅/❌ in the hello-world table matches reality
  (does the example actually import the SDK?).
- [ ] Verify every ✅/❌ in the recipe table matches reality (does
  the daemon/frontend use its SDK?).
- [ ] Update the maturity matrix: replace 🔜 with ✅ for discover
  and connect once implemented.
- [ ] Update code examples if APIs changed during implementation.
- [ ] Add "Getting started" examples for Dart, Swift, Python,
  Kotlin, C# (currently only Go and Rust shown).

## 2. Audit `README.md` (sdk/)

- [ ] Update the Fleet Overview table versions if SDKs were bumped.
- [ ] Update the gRPC Transport Matrix if new transports were added.
- [ ] Update the Holon-RPC table if capabilities changed.
- [ ] Add `discover` and `connect` support columns.
- [ ] Update the Recipes table — currently only lists `go-dart-holons`,
  should list all 12.

## 3. Audit per-SDK READMEs

For each of the 13 SDK repos:

- [ ] `go-holons/README.md` — document `pkg/discover`, `pkg/connect`
- [ ] `rust-holons/README.md` — document `discover`, `connect`
- [ ] `swift-holons/README.md` — document `Discover.swift`, `Connect.swift`
- [ ] `dart-holons/README.md` — document discover, connect
- [ ] `js-holons/README.md` — document discover, connect
- [ ] `js-web-holons/README.md` — document connect (no discover in browser)
- [ ] `kotlin-holons/README.md` — document discover, connect
- [ ] `java-holons/README.md` — document discover, connect
- [ ] `csharp-holons/README.md` — document discover, connect
- [ ] `cpp-holons/README.md` — document discover, connect
- [ ] `c-holons/README.md` — document discover, connect
- [ ] `python-holons/README.md` — document discover, connect
- [ ] `ruby-holons/README.md` — document discover, connect

## 4. Audit hello-world examples

For each of the 13 examples:

- [ ] Verify it builds and runs.
- [ ] Verify it uses its SDK (not raw gRPC).
- [ ] Add a connect example (calling another hello-world) if missing.
- [ ] Verify `holon.yaml` exists and is valid.
- [ ] Update local README if it references outdated instructions.

## 5. Audit recipe documentation

For each of the 12 recipes:

- [ ] Verify README.md describes the actual stack (no Godart leftovers).
- [ ] Verify `holon.yaml` for all members (daemon + frontend + composite).
- [ ] Verify BLOCKED.md is accurate (or removed if unblocked).
- [ ] Verify build instructions work on macOS.
- [ ] Update build instructions for Windows/Linux if applicable.

## 6. Audit top-level docs

- [ ] `organic-programming/README.md` — ensure SDK section links are
  current and describe the 5-module architecture.
- [ ] `organic-programming/PROTOCOL.md` — verify it reflects the
  current transport matrix.
- [ ] `organic-programming/OP.md` — verify runner list matches
  implemented runners.
- [ ] `organic-programming/CONVENTIONS.md` — verify holon structure
  rules match SDK expectations.
- [ ] `organic-programming/recipes/README.md` — verify all 12
  recipes are listed with correct links and status.
- [ ] `organic-programming/recipes/IMPLEMENTATION_ON_MAC_OS.md` —
  verify instructions match current build steps.

## 7. Cross-reference consistency

- [ ] Every SDK listed in `sdk/README.md` has a matching
  `examples/<lang>-hello-world`.
- [ ] Every recipe listed in `sdk/SDK_GUIDE.md` exists in
  `recipes/` and has a working `holon.yaml`.
- [ ] Every `build.runner` value used in recipe manifests is
  documented in `OP.md`.
- [ ] The slug naming convention is consistent across all
  `holon.yaml` files, directory names, and `artifacts.binary`.

## Rules

- Do not change code — this is a documentation-only pass.
- If documentation and code disagree, update the documentation
  to match the code (code is truth).
- Commit documentation fixes per-repo, not as one giant commit.
