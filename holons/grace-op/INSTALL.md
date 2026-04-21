## Install

### macOS / Linux

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/organic-programming/grace-op/dev/scripts/install.sh)"
```

Or via Homebrew:

```bash
brew tap organic-programming/tap && brew install op
```

### Windows

```powershell
irm https://raw.githubusercontent.com/organic-programming/grace-op/dev/scripts/install.ps1 | iex
```

Or via Chocolatey:

```powershell
choco install op
```

### Go (any platform)

```bash
go install github.com/organic-programming/grace-op/cmd/op@latest
op env --init
op install .
```

Then activate (or restart your terminal):

```bash
eval "$(op env --shell)"                                    # macOS / Linux
op completion install zsh                                  # zsh
op completion install bash                                 # bash
```
```powershell
op env --init   # Windows (OPBIN is added to PATH by op install)
```

### From source

```bash
git clone https://github.com/organic-programming/grace-op.git
cd grace-op
go run ./cmd/op env --init
go run ./cmd/op install .
```

Then activate as above, or restart your terminal.

### Shell completion

`op` can install shell completion directly into the active shell profile:

```bash
op completion install zsh
op completion install bash
```

These commands are idempotent. They append one of:

```zsh
eval "$(op completion zsh)"
```

```bash
source <(op completion bash)
```
