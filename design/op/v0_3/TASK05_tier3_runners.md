# TASK05 — Tier 3 Runners (dotnet, qt-cmake)

## Context

Depends on TASK03 (runner registry must exist).

These runners require platform SDKs that may not be installed.
They must handle the missing SDK case gracefully with actionable errors.

**Repository**: `organic-programming/holons/grace-op`

---

### `dotnet` runner

For `build.runner: dotnet`.

| Op | Command |
|---|---|
| Check | verify `dotnet` on PATH, verify required workloads |
| Build | `dotnet build -c Release` |
| Test | `dotnet test` |
| Clean | `dotnet clean` |

**Workload check**: run `dotnet workload list` and verify required
workloads. For MAUI: `maui-maccatalyst` (macOS), `maui-android`, etc.

Missing workload error:
```
op check: dotnet runner requires workload "maui-maccatalyst"
  install with: dotnet workload install maui-maccatalyst
```

### `qt-cmake` runner

For `build.runner: qt-cmake`.

| Op | Command |
|---|---|
| Check | verify `cmake` on PATH, verify Qt6Config.cmake findable |
| Build | `cmake -B build -DCMAKE_PREFIX_PATH=<qt6>` then `cmake --build build` |
| Test | `ctest --test-dir build` |
| Clean | `rm -rf build/` |

**Qt6 detection**: try `brew --prefix qt6` on macOS, then `$Qt6_DIR`.

Missing Qt6 error:
```
op check: qt-cmake runner requires Qt6
  install with: brew install qt6
  then set: export Qt6_DIR=$(brew --prefix qt6)/lib/cmake/Qt6
```

## Checklist

- [ ] Implement `DotnetRunner` with workload detection
- [ ] Implement `QtCMakeRunner` with Qt6 detection
- [ ] Unit tests: Check with mocked `dotnet workload list`
- [ ] Unit tests: Check with mocked `brew --prefix qt6`
- [ ] Test actionable error messages
- [ ] `go test ./...` — zero failures

## Final registry after all TASK03–03

```go
var runners = map[string]Runner{
    "go-module":     &GoModuleRunner{},
    "cmake":         &CMakeRunner{},
    "cargo":         &CargoRunner{},
    "swift-package": &SwiftPackageRunner{},
    "flutter":       &FlutterRunner{},
    "npm":           &NPMRunner{},
    "gradle":        &GradleRunner{},
    "dotnet":        &DotnetRunner{},
    "qt-cmake":      &QtCMakeRunner{},
    "recipe":        &RecipeRunner{},
}
```

## Dependencies

- TASK03 (runner registry)
