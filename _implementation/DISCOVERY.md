# FIX:Holon Discovery

## BPdS decisions 

**Pseudo API** = `Discover(holon:string!,root:string?, specifiers:enum{source,built,installed,cached}...)` specifiers parameter is variadic or can be combined by logic value --source --installed. no specifier is a special case, it means all the specifiers.

**<holon>** can be slug, alias, uuid, package path, binary path, in any context.

## BPdS tasks

- Work directly on the go Discover function public & implementation. 
- [] --root modifier is a global flag and can be used with any command to constraint to a root for discovery
- [] when --root is set we search only in this root. 
- [] reject --root with void or any invalid path.
- [] We need just one command `op list` to list holons (no `op discover`) remove it, from code, code api , documentations.
- [] any command that needs to resolve a holon should use the same discovery algorithm
- Some command like `op build` always use --source specifier and reject any other specifier.


### Flags for `op` command that uses discovery : 

| Flag | Scope | Notes|
|------|-------|-------|    
| *(none)* | same as `--all` | |
| `--all` | everything across all layers | |
| `--siblings` | e.g. bundle for apple app's bundles | |
| `--cwd` | the execution | |
| `--source` | source holons in workspace | |
| `--built` | `.op/build/` packages | |
| `--installed` | `$OPBIN` packages | | 
| `--cached` | `$OPPATH/cache` packages | |
| `--root <path>` | override anything as scan root | **unique preempt any other flag** | 

Resolution order when `--all` or undefined : 

1. siblings
2. cwd
3. source
4. built
5. installed
6. cached

#### What happens we provide a set of specifiers ? e.g : --installed --source

The resolution should be the intersection of the specifiers with the default resolution order (--source --installed  in our case we don't want to give a meaning to flag positions)

####  Op command special cases

1. By design `op build <holon>` requires source code and ignore any specifier flags it uses the Discovery code api with `--source` specifier in any case (immutable behavior).  **IMPORTANT1** note <holon> can be the path to a source holon in this case the build is done in the directory of the source holon. **IMPORTANT2** if `--root` is set, the build is done in the root directory (if it contains sources or .holon with sources recursively)
2. `op install <holon> --build` is a composition of `op build <holon> --source` followed by `op install <holon> --installed`. Note the that `op install` without build uses the already built binary.
3. `op run <holon>` uses the installed binary if available, otherwise it uses the built binary if available, otherwise fails.  **IMPORTANT**: you can add `--build` to force to build , you can give the path to the source holon as <holon> to scope a specific .holon.  When `op run <holon>` just find a source holon it it equivalent to run `--build` this mechanism is auto-build.

```shell
op build    → Discover(holon, root, --source)
op install  → Discover(holon, root, --built)
op run      → Discover(holon, root, --installed, --built, --siblings)
#               ↳ if only source found → auto-build, then run

```


### Using binary path.

- `op <holon> <command> [args]` can be a binary path.
- Using a binary path bypasses discover and is faster.
- When possible for example in autocompletion the op internal logic should use the resolved binary path to avoid discover and even cache the result of the description.

- NEW feature `op <binary-path> Describe` should enable to get the description of the holon it is faster because it bypasses discover. 

### The --bin flag should be renamed --origin 

**VERY IMPORTANT**
- `op <holon> <command> --origin` should show the origin in stderr. (operationnal in build)


## Usage contexts

### A. Scan — enumerate all holons under a root

| Command | Default root | `--root` |
|---|---|---|
| `op list [root]` | cwd | overrides cwd |$

### B. Resolve one holon by name — find it, then act

| Command | Accepts |
|---|---|
| `op <holon> <command> [args]` | slug, alias |
| `op run <holon>` | slug, alias |
| `op inspect <slug>` | slug, alias, host:port |
| `op do <holon> <sequence>` | slug, alias |
| `op tools <slug>` | slug, alias |
| `op mcp <slug>` | slug, alias, URI |
| `op uninstall <holon>` | slug, alias |

### C. Resolve by slug-or-path — local-first

| Command | Accepts |
|---|---|
| `op build [<holon-or-path>]` | slug, alias, `.`, `./path` |
| `op check [<holon-or-path>]` | slug, alias, `.`, `./path` |
| `op test [<holon-or-path>]` | slug, alias, `.`, `./path` |
| `op clean [<holon-or-path>]` | slug, alias, `.`, `./path` |
| `op install [<holon-or-path>]` | slug, alias, `.`, `./path` |

### D. Resolve by UUID

| Command | Accepts |
|---|---|
| `op show <uuid-or-prefix>` | full UUID, prefix |

### E. No discovery

| Command | Notes |
|---|---|
| `op <binary-path> <method>` | direct file, no resolution |
| `op grpc://...` | direct URI |
| `op serve`, `op version`, `op new`, `op env`, `op mod` | self or scaffolding |

### F. SDK runtime — holon-to-holon

A running holon discovers peers to connect to them. Uses the same algorithm but from the holon's own process context, not the CLI.

---

## Holon identity keys

A holon can be found by any of these:

| Key | Example | Source |
|---|---|---|
| **Slug** | `gabriel-greeting-go` | `given-family` lowercased |
| **Alias** | `op` | explicit `aliases` list in identity |
| **Dir name** | `gabriel-greeting-go.holon` | directory basename (`.holon` stripped) |
| **UUID** | `3f08b5c3` | identity, full or prefix |
| **Path** | `./holons/foo`, `.` | filesystem, no discovery walk |
| **Binary** | `gabriel-greeting-go` | installed in `$OPBIN` or `$PATH` |

---

## What discovery finds

| Kind | Marker | Has binary? | Has `holon.proto`? |
|---|---|---|---|
| **Source holon** | `holon.proto` in tree | after build | yes |
| **`.holon` package** | `*.holon/` dir + `.holon.json` | yes (prebuilt) | no |

