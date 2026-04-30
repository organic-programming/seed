# Codex prompt — implement RFC `op build` proto code generation

> Fresh Codex session. The spec is **[`.bpds/op_build_proto_generation.md`](../.bpds/op_build_proto_generation.md)** — read it fully. This wrapper adds operational instructions only.

---

## Mission

Implement the accepted RFC. Multi-PR chantier against `master`. Marathon mode — overnight expected. Composer admin-merges fast on green CI + DoD met.

## Context discipline

The previous Codex session ran out of context. Do not repeat:
- Read each file once with focused offset+limit when possible.
- Do not paste large logs back into context — extract the diagnostic line, discard the rest.
- Drop spec/file content from working memory once a phase is done.

## PR sequencing (RFC §11 with operational ordering)

1. **Library swap** — replace `jhump/protoreflect` with `bufbuild/protocompile` across `proto_stage.go`, `internal/inspect/parser.go`, `internal/holons/doc_gen.go`. Ship as standalone PR; gen/ unchanged; tests stay green.
2. **Codegen driver + manifest schema** — add `build.codegen.languages` to manifest proto, plug the dispatch phase between descriptor write and preflight (RFC §4 placement), wire plugin discovery from `$OPPATH/sdk/<sdk>/<version>/<target>/manifest.json` (RFC §6.1, 6.2). Behind a feature gate or unused-yet field — no holon opts in this PR.
3. **Distribution layer extension to 14 languages** (RFC §11.0). Land per-language; order independent. Each adds the codegen plugin binary + extends `manifest.json` with the `codegen` block (RFC §6.2). Heavy SDKs (c/cpp/ruby/zig) gain `codegen.plugins` next to existing native libs; light SDKs (go/rust/swift/dart/java/kotlin/csharp/python/js/js-web) get a minimal distribution.
4. **Holon migration** (RFC §11.2) — opt holons into `build.codegen`, verify gen/ diff is byte-clean (or only deterministic header noise), remove `before_commands` + `tools/generate` + `requires.commands: ["protoc", ...]`. One PR per holon family or a small batch — composer's call.
5. **Cleanup** (RFC §11.3) — delete `holons/grace-op/tools/generate` and per-example `scripts/generate_proto.sh`. Update `OP_BUILD.md` to mark `before_commands` as legacy.

## Open question codex resolves (RFC §12, second paragraph)

For `op` itself, the RFC explicitly leaves "regenerate `op`'s stubs" out of scope. **Codex picks one** in PR 4 (or a sibling PR), justified in the PR body:
- A `make regen-stubs` target invoking `op build op --regen-stubs`-style codepath
- A standalone `holons/grace-op/scripts/regen-stubs.sh`

Either is acceptable; pick the one that better fits the existing repo conventions.

## Reporting per PR

After each merge, reply with: PR URL, commit range, `git diff --stat`, RFC section addressed, DoD bullet ticked, any RFC open question now resolved. Then start the next PR immediately.

## Definition of done (chantier-level)

- All 14 distributions ship a codegen block.
- All reference holons under `examples/` and `holons/` use `build.codegen` (or have no codegen needs).
- `holons/grace-op/tools/generate` and per-example `scripts/generate_proto.sh` deleted.
- `op build` of a fresh checkout on a machine with **only Go installed** succeeds for grace-op (gen/ committed, RFC §12).
- `op build` of any holon after editing its `.proto` regenerates correctly given the right distribution installed; CI gates with `op build && git diff --exit-code gen/`.
- `OP_BUILD.md` updated; `OP_SDK.md` mentions the `codegen` block.

## Constraints

- PRs target `master`.
- No regression on existing `ader smoke` profile at any commit pushed.
- Do not commit transient files in `.codex/` or `.bpds/`.
- Halt and report only on RFC ambiguity that materially changes implementation, OR a hidden invariant in fundamental tension with §6/§7/§11 (per CLAUDE.md "doubt is the method").

Go.
