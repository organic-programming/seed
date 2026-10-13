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
output. Close those processes normally, then retry. OP does not stop them.

<!--todo write  a synthetic quick start 

# Quickstart

- How to [install **op**?](./INSTALL.md)
- How to build a holon? 
- How to send message to an holon? 
- Auto-completion.

...  Select the topic that are really useful to discover the core concepts.
-->
