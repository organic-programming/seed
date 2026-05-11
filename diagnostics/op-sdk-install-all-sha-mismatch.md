# op sdk install all release metadata remediation

Date: 2026-05-11
Branch: feature/sdk-versioning-and-unified-pipeline

## Initial failure

`op sdk install all` correctly continued past a failed Go install, installed the
remaining SDKs, wrote per-SDK logs plus `summary.txt`, and returned non-zero.
The failed entry was:

```text
FAIL | go | sha256 mismatch for go-holons-v0.1.0-aarch64-apple-darwin.tar.gz:
got  0e7556a498c1a819457fec7437524ed00c62fef858087d6f864dc39a4705ba3e
want 0a329903501e7e14d2b4977a6df8fbce03315a257be3be6c024dbb0f47341891
```

## Audit result

All present non-debug SDK tarball `.sha256` sidecar assets initially matched
the tarballs downloaded from GitHub Releases. No pre-existing `.sha256` sidecar
mismatch was found.

The mismatch was in `go-holons-v0.1.0` `release-manifest.json`: it had a stale
archive digest for `go-holons-v0.1.0-aarch64-apple-darwin.tar.gz`. The manifest
was regenerated from the current GitHub release assets and uploaded with
`gh release upload --clobber`.

The same manifest audit was run across the current SDK release lines. These
manifests were repaired because they contained stale, missing, or incomplete
archive entries:

```text
go-holons-v0.1.0      aarch64-apple-darwin: 0a329903501e7e14d2b4977a6df8fbce03315a257be3be6c024dbb0f47341891 -> 0e7556a498c1a819457fec7437524ed00c62fef858087d6f864dc39a4705ba3e
cpp-holons-v0.7.0    manifest entries regenerated from current assets
c-holons-v0.7.0      manifest entries regenerated from current assets
csharp-holons-v0.7.0 manifest entries regenerated from current assets
dart-holons-v0.7.0   manifest entries regenerated from current assets
cpp-holons-v1.80.0   x86_64-apple-darwin entry added from current asset
c-holons-v1.80.0     x86_64-apple-darwin entry added from current asset
zig-holons-v0.1.0    x86_64-apple-darwin and x86_64-windows-gnu entries added from current assets
```

The `go-holons-v0.1.0` release only contained the host
`aarch64-apple-darwin` tarball at audit time; the other expected target tarballs
were absent, so there was no sidecar or manifest entry to repair for those
targets.

A full `op sdk install all` then exposed two additional metadata repairs in the
`v0.7.0` release line: csharp and dart had `v0.7.0` release tags containing
host archives still named and internally declared as `v0.1.0`, even though their
embedded `seed_release` was already `0.7.0`. Those host archives were repacked
with SDK version `0.7.0`, new sidecars were generated, and the release manifests
were regenerated:

```text
csharp-holons-v0.7.0:
  old csharp-holons-v0.1.0-aarch64-apple-darwin.tar.gz sha256 4d0f7f01174b7b6b4545f145395dc8a48d75bf63804589e2dd19ea5a18f59f37
  new csharp-holons-v0.7.0-aarch64-apple-darwin.tar.gz sha256 dc4dc3e968d96615f7a6562d7b953200dc5f7140205be082a37d2ea049984b39

dart-holons-v0.7.0:
  old dart-holons-v0.1.0-aarch64-apple-darwin.tar.gz sha256 00ad36351ca04603916da3fe7c2af59655986dd1ca803f1ec7acf8bf40f07fd4
  new dart-holons-v0.7.0-aarch64-apple-darwin.tar.gz sha256 466ccd7d357265ebc5d8ab4861152d7edbec802f3de30f1d2502321027e8ce12
```

## Verification

After the Go manifest repair, a fresh `OPPATH` Go install succeeded:

```bash
tmp="$(mktemp -d)"
env OPPATH="$tmp/op" OPBIN="$tmp/op/bin" go run ./holons/grace-op/cmd/op sdk install go
```

Result:

```text
installed go 0.1.0 aarch64-apple-darwin
archive sha256 0e7556a498c1a819457fec7437524ed00c62fef858087d6f864dc39a4705ba3e
tree sha256    95bb1ca690dfefa8f15015b464793afd08db0ed522274f8811ce85a935e4d24c
```

After the csharp/dart archive repairs, a fresh `OPPATH` install-all run
succeeded for all 14 SDKs:

```text
OK | go
OK | java
OK | kotlin
OK | dart
OK | swift
OK | python
OK | csharp
OK | js
OK | js-web
OK | rust
OK | ruby
OK | zig
OK | c
OK | cpp
```
