# TASK04 — Remove Submodules

## Summary

Remove all 12 recipe submodule entries from `.gitmodules` and
archive the GitHub repos. The monorepo replaces them.

## Repos to Archive

| Submodule | GitHub repo |
|---|---|
| `recipes/go-dart-holons` | `organic-programming/go-dart-holons` |
| `recipes/go-swift-holons` | `organic-programming/go-swift-holons` |
| `recipes/go-kotlin-holons` | `organic-programming/go-kotlin-holons` |
| `recipes/go-web-holons` | `organic-programming/go-web-holons` |
| `recipes/go-dotnet-holons` | `organic-programming/go-dotnet-holons` |
| `recipes/go-qt-holons` | `organic-programming/go-qt-holons` |
| `recipes/rust-dart-holons` | `organic-programming/rust-dart-holons` |
| `recipes/rust-swift-holons` | `organic-programming/rust-swift-holons` |
| `recipes/rust-kotlin-holons` | `organic-programming/rust-kotlin-holons` |
| `recipes/rust-web-holons` | `organic-programming/rust-web-holons` |
| `recipes/rust-dotnet-holons` | `organic-programming/rust-dotnet-holons` |
| `recipes/rust-qt-holons` | `organic-programming/rust-qt-holons` |

## Steps

1. `git submodule deinit -f` each
2. `rm -rf .git/modules/recipes/<name>`
3. `git rm -f recipes/<name>`
4. Clean `.gitmodules`
5. Update external doc references (CONVENTIONS.md, SDK_GUIDE.md)
6. Archive repos on GitHub (manual, do not delete)

## Acceptance Criteria

- [ ] All 12 submodule entries removed from `.gitmodules`
- [ ] `.git/modules/recipes/` cleaned
- [ ] No broken references in docs
- [ ] GitHub repos archived (not deleted)

## Dependencies

TASK03 (assemblies must exist before removing originals).
