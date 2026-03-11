# Gudule Greeting Flutter Go

Thin macOS assembly for the extracted Go daemon and Flutter HostUI.

## Build

```sh
op build recipes/assemblies/gudule-greeting-flutter-go
```

## Run

```sh
op run recipes/assemblies/gudule-greeting-flutter-go
```

## Notes

- The assembly is macOS-only in v0.4.1 because `grace-op` still has a single
  `artifacts.primary` path.
- Linux and Windows source trees remain in the extracted HostUI, but their
  manifest-driven `op run` path is deferred.
