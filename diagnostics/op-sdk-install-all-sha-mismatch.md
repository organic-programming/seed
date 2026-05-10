# op sdk install all validation blocker

Date: 2026-05-10
Branch: feature/op-sdk-build-all

## Command

```bash
env OPPATH=/tmp/op-sdk-install-all.7xByTY OPBIN=/tmp/op-sdk-install-all.7xByTY/bin op sdk install all
```

I used a fresh temporary `OPPATH` so `$OPPATH/sdk` started empty without
deleting the operator's real SDK cache.

## Result

The batch runner behaved correctly: it continued after the first failure,
installed the remaining SDKs, wrote per-SDK logs, wrote `summary.txt`, and
returned a non-zero exit code.

Run directory:

```text
/tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/
```

Summary:

```text
FAIL | go | 21s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/go.log | sha256 mismatch for https://github.com/organic-programming/seed/releases/download/go-holons-v0.1.0/go-holons-v0.1.0-aarch64-apple-darwin.tar.gz: got 0e7556a498c1a819457fec7437524ed00c62fef858087d6f864dc39a4705ba3e, want 0a329903501e7e14d2b4977a6df8fbce03315a257be3be6c024dbb0f47341891
OK | java | 4s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/java.log
OK | kotlin | 5s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/kotlin.log
OK | dart | 2s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/dart.log
OK | swift | 1s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/swift.log
OK | python | 2s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/python.log
OK | csharp | 2s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/csharp.log
OK | js | 2s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/js.log
OK | js-web | 1s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/js-web.log
OK | rust | 1s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/rust.log
OK | ruby | 11s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/ruby.log
OK | zig | 9s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/zig.log
OK | c | 10s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/c.log
OK | cpp | 9s | /tmp/op-sdk-install-all.7xByTY/logs/sdk-install/20260510T090809Z/cpp.log
```

## Blocker

The requested validation requires `op sdk install all` from a fresh SDK root to
populate all 14 SDKs from current GitHub Releases. Current release integrity
metadata blocks that: the `go-holons-v0.1.0` aarch64 macOS archive downloads
with SHA-256 `0e7556a498c1a819457fec7437524ed00c62fef858087d6f864dc39a4705ba3e`,
but the release metadata used by the existing install path expects
`0a329903501e7e14d2b4977a6df8fbce03315a257be3be6c024dbb0f47341891`.

Per the brief, I did not push or open a PR after this failed local validation.
