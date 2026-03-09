# windows_TASK001 — Build WinUI Recipes on Windows

## Context

Depends on: mac_TASK001 (Go recipes verified on macOS).

These two recipes are **Windows-only** — WinUI 3 cannot build on
macOS. This is the primary reason to run on Windows.

See `IMPLEMENTATION_ON_WINDOWS.md` for full reference.

## Recipes

| # | Recipe | Frontend | Priority |
|---|--------|----------|----------|
| 1 | `go-dotnet-holons` | WinUI 3 (.NET 8) | **primary** |
| 2 | `rust-dotnet-holons` | WinUI 3 (.NET 8) | secondary |

## go-dotnet-holons — WinUI target

The recipe already has a Mac Catalyst target. Add the Windows target:

```yaml
targets:
  windows:
    steps:
      - build_member: daemon
      - exec:
          cwd: greeting-godotnet
          argv: ["dotnet", "build", "-c", "Debug"]
      - copy:
          from: greeting-daemon/gudule-daemon-greeting-godotnet.exe
          to: greeting-godotnet/bin/Debug/net8.0-windows10.0.19041.0/
      - assert_file:
          path: greeting-godotnet/bin/Debug/net8.0-windows10.0.19041.0/GreetingDotnet.exe
```

**Project type:** WinUI 3 (Windows App SDK).
**Target framework:** `net8.0-windows10.0.19041.0`.

**NuGet dependencies:**
- `Grpc.Net.Client`, `Google.Protobuf`, `Grpc.Tools`
- `Microsoft.WindowsAppSDK`

## rust-dotnet-holons — WinUI target

Same as above but with `cargo build` for the daemon.
Binary: `gudule-daemon-greeting-rustdotnet.exe`.

## Required tools

- .NET 8 SDK
- Visual Studio 2022 (Windows App SDK workload)
- Go (for go-dotnet daemon)
- Cargo (for rust-dotnet daemon)

## Verification

```powershell
op check recipes\go-dotnet-holons\examples\greeting
op build recipes\go-dotnet-holons\examples\greeting --target windows
```

## Rules

- Build go-dotnet first, then clone for rust-dotnet.
- Test that the app launches and shows the greeting UI.
- Commit and push from Windows.
