# CONVENTIONS.md — Cache Directories (Draft)

> These entries are designed to be added to the project's
> `CONVENTIONS.md` or standard directories reference.

---

## Standard Directories — Setup Cache

| Directory | Purpose |
|---|---|
| `~/.op/cache/` | Root cache directory |
| `~/.op/cache/sources/` | Cloned source dependencies (from `requires.sources`) |
| `~/.op/cache/sources/<name>/` | Single source dependency (git clone) |
| `~/.op/cache/artifacts/` | Downloaded pre-compiled binaries (from registry) |
| `~/.op/cache/builds/` | Locally built artifacts (fallback source builds) |
