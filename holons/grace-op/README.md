# OP — The Organic Programming CLI

`op` is the organic programming tool, like `go` for **go programming language**.
It is a rich tool. 

## Windows builds

Use `op build op --install` to rebuild and install OP. On Windows, self-install
updates both OPBIN and the OP executable that is currently running, if its name
is `op.exe`. It does not change the shell profile or persistent PATH, and does
not replace other executables merely because they are also found on PATH.

A project can declare its installed build tools in `seed-toolchain.yaml`:

```yaml
build_environments:
  windows:
    path:
      - "${OPPATH}/tools/my-tool/bin"
    env:
      MY_TOOL_CONFIG: "${CONFIG_DIR}/build/config.json"
    required_files:
      - "${MY_TOOL_CONFIG}"
```

OP selects the nearest ancestor file containing this section, including when a
nested seed checkout only declares SDK versions. It prepares a private child
environment for lifecycle commands and codegen plugins without changing the
environment of its caller or concurrent API/RPC requests. Relative paths use
the configuration directory. `${OPPATH}`, `${OPBIN}`, `${CONFIG_DIR}` and other
environment variables are available; undefined variables and cycles are errors.
`${PROTOC_VERSION}` uses the file's `protoc.version`, or that of its nested
`seed/seed-toolchain.yaml`.

The optional `visual_studio` mapping accepts `version`, `msvc`, `architecture`
and `host_architecture`. OP discovers the installed C++ tools with `vswhere`
and imports Microsoft's developer environment privately. Version constraints
must match the installed versions. OP validates PATH directories and
`required_files` before lifecycle operations, including clean, so an invalid
toolchain fails before artifacts are removed. It does not download tools from
this configuration. Hosts other than Windows ignore the section entirely.

Before cleaning or rebuilding Windows artifacts, OP checks whether application
or worker executables are running from the holon's outputs or those of its
dependencies. If so, it reports their paths and PIDs before changing any build
output. This protection also applies to macOS, as described below.

## Running applications during a rebuild

On macOS and Windows, build and clean inventory executable paths before changing
outputs, including every holon dependency of a composite. Protected locations
include `.op`, the holon package and the declared primary artifact. An application
or an orphan worker running from one of these locations blocks the operation.
Paths are canonicalized, including symlinks and Windows junctions: a process in
another checkout or another installed copy does not match merely by name.

When `op run` needs to build an artifact, or when `op run --clean` is requested,
the CLI can resolve this conflict before any cleanup begins. With terminal input
and terminal error output, it displays the affected PIDs and paths, warns that
closing them can interrupt work, and asks:

```text
This application is using files that need to be rebuilt. Close it and continue? [y/N]
```

Only `y` or `yes` (case-insensitive) permits a normal shutdown request. OP
requests application shutdown first and waits up to 10 seconds for applications
and their workers to exit. On macOS an orphan worker receives `SIGTERM`.
If processes survive, OP lists them again and asks separately before
forcing their termination; it then waits up to 5 seconds. Process identity
(canonical executable path, PID and start time) is checked again before each
stop request. A fresh inventory must be clear before clean/build/run resumes,
and the lifecycle checks remain enabled after consent.

A refusal, empty answer, end of input or interruption cancels the operation.
Artifacts remain untouched; a process already asked to exit may nevertheless
have stopped. A failed shutdown, a changed process identity or a remaining
conflict prevents rebuilding. Launching an existing artifact without modifying
it does not trigger the dialog. OP never interprets a recipe's error text as
permission to stop a process.

With redirected standard input or standard error, JSON formatting, or an API/RPC call, OP returns
the conflict immediately without prompting or stopping anything. Go API callers
can use `errors.As` with `*api.ActiveArtifactConflict` to read `Operation`,
`Holon` and `Processes` (`PID`, `Path`, `Started`, `BundlePath`). RPC Run, Build,
Clean and build-through-Install return `FailedPrecondition` with an
`google.rpc.ErrorInfo` detail: reason `ACTIVE_ARTIFACTS`, domain
`organic-programming.org/op`, and metadata `holon`, `operation`, `processes`.
The latter is a JSON array with `pid`, `path`, `started`, and optional
`bundle_path` fields. Close the reported processes and retry the request.

Detection concerns executable images in declared output locations. It does not
enumerate every mapped library or open data file, nor arbitrary outputs written
by recipe commands outside those locations. Other platforms retain their
existing runner-specific protections. The inventory is a snapshot, not a lock:
another process can start after a check. Application-specific build safety
checks must therefore remain in place. No application-specific name or path is
required. No application change is needed when its executables live in these
declared outputs. Keep existing recipe safety checks (including Dart build-safety
scripts) unchanged. Applications should declare custom output locations as
artifacts and should stop their embedded workers during normal shutdown.

On macOS the inventory uses native process information and works without cgo.
Normal application shutdown uses `NSRunningApplication.terminate` through the
system `osascript`, addressed to the verified PID; it does not target every app
with the same name or bundle identifier. Protected processes that the current
user cannot inspect are outside the inventory. macOS does not provide an atomic
check-and-signal operation, so a narrow race remains between the final identity
check and a signal. On Windows normal shutdown sends `WM_CLOSE` to the verified
process's windows. Console workers without windows have no corresponding safe
normal shutdown request and may remain for the separate forced-stop decision.

<!--todo write  a synthetic quick start 

# Quickstart

- How to [install **op**?](./INSTALL.md)
- How to build a holon? 
- How to send message to an holon? 
- Auto-completion.

...  Select the topic that are really useful to discover the core concepts.
-->
