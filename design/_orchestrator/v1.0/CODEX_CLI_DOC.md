# CODEX CLI Documentation

Generated from local `codex help` output for `codex-cli 0.104.0` on 2026-03-10.

This file was built by recursively calling `codex help` and `codex help <subcommand>` for every discovered command path in the installed CLI.

## Top-Level Overview

| Command | Summary |
| --- | --- |
| `codex exec` | Run Codex non-interactively [aliases: e] |
| `codex review` | Run a code review non-interactively |
| `codex login` | Manage login |
| `codex logout` | Remove stored authentication credentials |
| `codex mcp` | Manage external MCP servers for Codex |
| `codex mcp-server` | Start Codex as an MCP server (stdio) |
| `codex app-server` | [experimental] Run the app server or related tooling |
| `codex app` | Launch the Codex desktop app (downloads the macOS installer if missing) |
| `codex completion` | Generate shell completion scripts |
| `codex sandbox` | Run commands within a Codex-provided sandbox |
| `codex debug` | Debugging tools |
| `codex apply` | Apply the latest diff produced by Codex agent as a `git apply` to your local working tree [aliases: a] |
| `codex resume` | Resume a previous interactive session (picker by default; use --last to continue the most recent) |
| `codex fork` | Fork a previous interactive session (picker by default; use --last to fork the most recent) |
| `codex cloud` | [EXPERIMENTAL] Browse tasks from Codex Cloud and apply changes locally |
| `codex features` | Inspect feature flags |
| `codex help` | Print this message or the help of the given subcommand(s) |

## Command Tree

```text
codex
- codex exec
  - codex exec resume
  - codex exec review
  - codex exec help
- codex review
- codex login
  - codex login status
  - codex login help
- codex logout
- codex mcp
  - codex mcp list
  - codex mcp get
  - codex mcp add
  - codex mcp remove
  - codex mcp login
  - codex mcp logout
  - codex mcp help
- codex mcp-server
- codex app-server
  - codex app-server generate-ts
  - codex app-server generate-json-schema
  - codex app-server help
- codex app
- codex completion
- codex sandbox
  - codex sandbox macos
  - codex sandbox linux
  - codex sandbox windows
  - codex sandbox help
- codex debug
  - codex debug app-server
    - codex debug app-server send-message-v2
    - codex debug app-server help
  - codex debug help
- codex apply
- codex resume
- codex fork
- codex cloud
  - codex cloud exec
  - codex cloud status
  - codex cloud list
  - codex cloud apply
  - codex cloud diff
  - codex cloud help
- codex features
  - codex features list
  - codex features enable
  - codex features disable
  - codex features help
- codex help
```

## Full Help Reference

## `codex`

Invocation: `codex help`

```text
Codex CLI

If no subcommand is specified, options will be forwarded to the interactive CLI.

Usage: codex [OPTIONS] [PROMPT]
       codex [OPTIONS] <COMMAND> [ARGS]

Commands:
  exec        Run Codex non-interactively [aliases: e]
  review      Run a code review non-interactively
  login       Manage login
  logout      Remove stored authentication credentials
  mcp         Manage external MCP servers for Codex
  mcp-server  Start Codex as an MCP server (stdio)
  app-server  [experimental] Run the app server or related tooling
  app         Launch the Codex desktop app (downloads the macOS installer if missing)
  completion  Generate shell completion scripts
  sandbox     Run commands within a Codex-provided sandbox
  debug       Debugging tools
  apply       Apply the latest diff produced by Codex agent as a `git apply` to your local working
              tree [aliases: a]
  resume      Resume a previous interactive session (picker by default; use --last to continue the
              most recent)
  fork        Fork a previous interactive session (picker by default; use --last to fork the most
              recent)
  cloud       [EXPERIMENTAL] Browse tasks from Codex Cloud and apply changes locally
  features    Inspect feature flags
  help        Print this message or the help of the given subcommand(s)

Arguments:
  [PROMPT]
          Optional user prompt to start the session

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -i, --image <FILE>...
          Optional image(s) to attach to the initial prompt

  -m, --model <MODEL>
          Model the agent should use

      --oss
          Convenience flag to select the local open source model provider. Equivalent to -c
          model_provider=oss; verifies a local LM Studio or Ollama server is running

      --local-provider <OSS_PROVIDER>
          Specify which local provider to use (lmstudio or ollama). If not specified with --oss,
          will use config default or show selection

  -p, --profile <CONFIG_PROFILE>
          Configuration profile from config.toml to specify default options

  -s, --sandbox <SANDBOX_MODE>
          Select the sandbox policy to use when executing model-generated shell commands
          
          [possible values: read-only, workspace-write, danger-full-access]

  -a, --ask-for-approval <APPROVAL_POLICY>
          Configure when the model requires human approval before executing a command

          Possible values:
          - untrusted:  Only run "trusted" commands (e.g. ls, cat, sed) without asking for user
            approval. Will escalate to the user if the model proposes a command that is not in the
            "trusted" set
          - on-failure: DEPRECATED: Run all commands without asking for user approval. Only asks for
            approval if a command fails to execute, in which case it will escalate to the user to
            ask for un-sandboxed execution. Prefer `on-request` for interactive runs or `never` for
            non-interactive runs
          - on-request: The model decides when to ask the user for approval
          - never:      Never ask for user approval Execution failures are immediately returned to
            the model

      --full-auto
          Convenience alias for low-friction sandboxed automatic execution (-a on-request, --sandbox
          workspace-write)

      --dangerously-bypass-approvals-and-sandbox
          Skip all confirmation prompts and execute commands without sandboxing. EXTREMELY
          DANGEROUS. Intended solely for running in environments that are externally sandboxed

  -C, --cd <DIR>
          Tell the agent to use the specified directory as its working root

      --search
          Enable live web search. When enabled, the native Responses `web_search` tool is available
          to the model (no per‑call approval)

      --add-dir <DIR>
          Additional directories that should be writable alongside the primary workspace

      --no-alt-screen
          Disable alternate screen mode
          
          Runs the TUI in inline mode, preserving terminal scrollback history. This is useful in
          terminal multiplexers like Zellij that follow the xterm spec strictly and disable
          scrollback in alternate screen buffers.

  -h, --help
          Print help (see a summary with '-h')

  -V, --version
          Print version
```

### `codex exec`

Invocation: `codex help exec`

```text
Run Codex non-interactively

Usage: codex exec [OPTIONS] [PROMPT] [COMMAND]

Commands:
  resume  Resume a previous session by id or pick the most recent with --last
  review  Run a code review against the current repository
  help    Print this message or the help of the given subcommand(s)

Arguments:
  [PROMPT]
          Initial instructions for the agent. If not provided as an argument (or if `-` is used),
          instructions are read from stdin

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -i, --image <FILE>...
          Optional image(s) to attach to the initial prompt

  -m, --model <MODEL>
          Model the agent should use

      --oss
          Use open-source provider

      --local-provider <OSS_PROVIDER>
          Specify which local provider to use (lmstudio or ollama). If not specified with --oss,
          will use config default or show selection

  -s, --sandbox <SANDBOX_MODE>
          Select the sandbox policy to use when executing model-generated shell commands
          
          [possible values: read-only, workspace-write, danger-full-access]

  -p, --profile <CONFIG_PROFILE>
          Configuration profile from config.toml to specify default options

      --full-auto
          Convenience alias for low-friction sandboxed automatic execution (-a on-request, --sandbox
          workspace-write)

      --dangerously-bypass-approvals-and-sandbox
          Skip all confirmation prompts and execute commands without sandboxing. EXTREMELY
          DANGEROUS. Intended solely for running in environments that are externally sandboxed

  -C, --cd <DIR>
          Tell the agent to use the specified directory as its working root

      --skip-git-repo-check
          Allow running Codex outside a Git repository

      --add-dir <DIR>
          Additional directories that should be writable alongside the primary workspace

      --ephemeral
          Run without persisting session files to disk

      --output-schema <FILE>
          Path to a JSON Schema file describing the model's final response shape

      --color <COLOR>
          Specifies color settings for use in the output
          
          [default: auto]
          [possible values: always, never, auto]

      --json
          Print events to stdout as JSONL

  -o, --output-last-message <FILE>
          Specifies file where the last message from the agent should be written

  -h, --help
          Print help (see a summary with '-h')

  -V, --version
          Print version
```

### `codex review`

Invocation: `codex help review`

```text
Run a code review non-interactively

Usage: codex review [OPTIONS] [PROMPT]

Arguments:
  [PROMPT]
          Custom review instructions. If `-` is used, read from stdin

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --uncommitted
          Review staged, unstaged, and untracked changes

      --base <BRANCH>
          Review changes against the given base branch

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --commit <SHA>
          Review the changes introduced by a commit

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

      --title <TITLE>
          Optional commit title to display in the review summary

  -h, --help
          Print help (see a summary with '-h')
```

### `codex login`

Invocation: `codex help login`

```text
Manage login

Usage: codex login [OPTIONS] [COMMAND]

Commands:
  status  Show login status
  help    Print this message or the help of the given subcommand(s)

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --with-api-key
          Read the API key from stdin (e.g. `printenv OPENAI_API_KEY | codex login --with-api-key`)

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --device-auth
          

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

### `codex logout`

Invocation: `codex help logout`

```text
Remove stored authentication credentials

Usage: codex logout [OPTIONS]

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

### `codex mcp`

Invocation: `codex help mcp`

```text
Manage external MCP servers for Codex

Usage: codex mcp [OPTIONS] <COMMAND>

Commands:
  list    
  get     
  add     
  remove  
  login   
  logout  
  help    Print this message or the help of the given subcommand(s)

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

### `codex mcp-server`

Invocation: `codex help mcp-server`

```text
Start Codex as an MCP server (stdio)

Usage: codex mcp-server [OPTIONS]

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

### `codex app-server`

Invocation: `codex help app-server`

```text
[experimental] Run the app server or related tooling

Usage: codex app-server [OPTIONS] [COMMAND]

Commands:
  generate-ts           [experimental] Generate TypeScript bindings for the app server protocol
  generate-json-schema  [experimental] Generate JSON Schema for the app server protocol
  help                  Print this message or the help of the given subcommand(s)

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

      --listen <URL>
          Transport endpoint URL. Supported values: `stdio://` (default), `ws://IP:PORT`
          
          [default: stdio://]

      --analytics-default-enabled
          Controls whether analytics are enabled by default.
          
          Analytics are disabled by default for app-server. Users have to explicitly opt in via the
          `analytics` section in the config.toml file.
          
          However, for first-party use cases like the VSCode IDE extension, we default analytics to
          be enabled by default by setting this flag. Users can still opt out by setting this in
          their config.toml:
          
          ```toml [analytics] enabled = false ```
          
          See https://developers.openai.com/codex/config-advanced/#metrics for more details.

  -h, --help
          Print help (see a summary with '-h')
```

### `codex app`

Invocation: `codex help app`

```text
Launch the Codex desktop app (downloads the macOS installer if missing)

Usage: codex app [OPTIONS] [PATH]

Arguments:
  [PATH]
          Workspace path to open in Codex Desktop
          
          [default: .]

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --download-url <DOWNLOAD_URL>
          Override the macOS DMG download URL (advanced)
          
          [default: https://persistent.oaistatic.com/codex-app-prod/Codex.dmg]

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

### `codex completion`

Invocation: `codex help completion`

```text
Generate shell completion scripts

Usage: codex completion [OPTIONS] [SHELL]

Arguments:
  [SHELL]
          Shell to generate completions for
          
          [default: bash]
          [possible values: bash, elvish, fish, powershell, zsh]

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

### `codex sandbox`

Invocation: `codex help sandbox`

```text
Run commands within a Codex-provided sandbox

Usage: codex sandbox [OPTIONS] <COMMAND>

Commands:
  macos    Run a command under Seatbelt (macOS only) [aliases: seatbelt]
  linux    Run a command under Landlock+seccomp (Linux only) [aliases: landlock]
  windows  Run a command under Windows restricted token (Windows only)
  help     Print this message or the help of the given subcommand(s)

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

### `codex debug`

Invocation: `codex help debug`

```text
Debugging tools

Usage: codex debug [OPTIONS] <COMMAND>

Commands:
  app-server  Tooling: helps debug the app server
  help        Print this message or the help of the given subcommand(s)

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

### `codex apply`

Invocation: `codex help apply`

```text
Apply the latest diff produced by Codex agent as a `git apply` to your local working tree

Usage: codex apply [OPTIONS] <TASK_ID>

Arguments:
  <TASK_ID>
          

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

### `codex resume`

Invocation: `codex help resume`

```text
Resume a previous interactive session (picker by default; use --last to continue the most recent)

Usage: codex resume [OPTIONS] [SESSION_ID] [PROMPT]

Arguments:
  [SESSION_ID]
          Conversation/session id (UUID) or thread name. UUIDs take precedence if it parses. If
          omitted, use --last to pick the most recent recorded session

  [PROMPT]
          Optional user prompt to start the session

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --last
          Continue the most recent session without showing the picker

      --all
          Show all sessions (disables cwd filtering and shows CWD column)

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -i, --image <FILE>...
          Optional image(s) to attach to the initial prompt

  -m, --model <MODEL>
          Model the agent should use

      --oss
          Convenience flag to select the local open source model provider. Equivalent to -c
          model_provider=oss; verifies a local LM Studio or Ollama server is running

      --local-provider <OSS_PROVIDER>
          Specify which local provider to use (lmstudio or ollama). If not specified with --oss,
          will use config default or show selection

  -p, --profile <CONFIG_PROFILE>
          Configuration profile from config.toml to specify default options

  -s, --sandbox <SANDBOX_MODE>
          Select the sandbox policy to use when executing model-generated shell commands
          
          [possible values: read-only, workspace-write, danger-full-access]

  -a, --ask-for-approval <APPROVAL_POLICY>
          Configure when the model requires human approval before executing a command

          Possible values:
          - untrusted:  Only run "trusted" commands (e.g. ls, cat, sed) without asking for user
            approval. Will escalate to the user if the model proposes a command that is not in the
            "trusted" set
          - on-failure: DEPRECATED: Run all commands without asking for user approval. Only asks for
            approval if a command fails to execute, in which case it will escalate to the user to
            ask for un-sandboxed execution. Prefer `on-request` for interactive runs or `never` for
            non-interactive runs
          - on-request: The model decides when to ask the user for approval
          - never:      Never ask for user approval Execution failures are immediately returned to
            the model

      --full-auto
          Convenience alias for low-friction sandboxed automatic execution (-a on-request, --sandbox
          workspace-write)

      --dangerously-bypass-approvals-and-sandbox
          Skip all confirmation prompts and execute commands without sandboxing. EXTREMELY
          DANGEROUS. Intended solely for running in environments that are externally sandboxed

  -C, --cd <DIR>
          Tell the agent to use the specified directory as its working root

      --search
          Enable live web search. When enabled, the native Responses `web_search` tool is available
          to the model (no per‑call approval)

      --add-dir <DIR>
          Additional directories that should be writable alongside the primary workspace

      --no-alt-screen
          Disable alternate screen mode
          
          Runs the TUI in inline mode, preserving terminal scrollback history. This is useful in
          terminal multiplexers like Zellij that follow the xterm spec strictly and disable
          scrollback in alternate screen buffers.

  -h, --help
          Print help (see a summary with '-h')

  -V, --version
          Print version
```

### `codex fork`

Invocation: `codex help fork`

```text
Fork a previous interactive session (picker by default; use --last to fork the most recent)

Usage: codex fork [OPTIONS] [SESSION_ID] [PROMPT]

Arguments:
  [SESSION_ID]
          Conversation/session id (UUID). When provided, forks this session. If omitted, use --last
          to pick the most recent recorded session

  [PROMPT]
          Optional user prompt to start the session

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --last
          Fork the most recent session without showing the picker

      --all
          Show all sessions (disables cwd filtering and shows CWD column)

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -i, --image <FILE>...
          Optional image(s) to attach to the initial prompt

  -m, --model <MODEL>
          Model the agent should use

      --oss
          Convenience flag to select the local open source model provider. Equivalent to -c
          model_provider=oss; verifies a local LM Studio or Ollama server is running

      --local-provider <OSS_PROVIDER>
          Specify which local provider to use (lmstudio or ollama). If not specified with --oss,
          will use config default or show selection

  -p, --profile <CONFIG_PROFILE>
          Configuration profile from config.toml to specify default options

  -s, --sandbox <SANDBOX_MODE>
          Select the sandbox policy to use when executing model-generated shell commands
          
          [possible values: read-only, workspace-write, danger-full-access]

  -a, --ask-for-approval <APPROVAL_POLICY>
          Configure when the model requires human approval before executing a command

          Possible values:
          - untrusted:  Only run "trusted" commands (e.g. ls, cat, sed) without asking for user
            approval. Will escalate to the user if the model proposes a command that is not in the
            "trusted" set
          - on-failure: DEPRECATED: Run all commands without asking for user approval. Only asks for
            approval if a command fails to execute, in which case it will escalate to the user to
            ask for un-sandboxed execution. Prefer `on-request` for interactive runs or `never` for
            non-interactive runs
          - on-request: The model decides when to ask the user for approval
          - never:      Never ask for user approval Execution failures are immediately returned to
            the model

      --full-auto
          Convenience alias for low-friction sandboxed automatic execution (-a on-request, --sandbox
          workspace-write)

      --dangerously-bypass-approvals-and-sandbox
          Skip all confirmation prompts and execute commands without sandboxing. EXTREMELY
          DANGEROUS. Intended solely for running in environments that are externally sandboxed

  -C, --cd <DIR>
          Tell the agent to use the specified directory as its working root

      --search
          Enable live web search. When enabled, the native Responses `web_search` tool is available
          to the model (no per‑call approval)

      --add-dir <DIR>
          Additional directories that should be writable alongside the primary workspace

      --no-alt-screen
          Disable alternate screen mode
          
          Runs the TUI in inline mode, preserving terminal scrollback history. This is useful in
          terminal multiplexers like Zellij that follow the xterm spec strictly and disable
          scrollback in alternate screen buffers.

  -h, --help
          Print help (see a summary with '-h')

  -V, --version
          Print version
```

### `codex cloud`

Invocation: `codex help cloud`

```text
[EXPERIMENTAL] Browse tasks from Codex Cloud and apply changes locally

Usage: codex cloud [OPTIONS] [COMMAND]

Commands:
  exec    Submit a new Codex Cloud task without launching the TUI
  status  Show the status of a Codex Cloud task
  list    List Codex Cloud tasks
  apply   Apply the diff for a Codex Cloud task locally
  diff    Show the unified diff for a Codex Cloud task
  help    Print this message or the help of the given subcommand(s)

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')

  -V, --version
          Print version
```

### `codex features`

Invocation: `codex help features`

```text
Inspect feature flags

Usage: codex features [OPTIONS] <COMMAND>

Commands:
  list     List known features with their stage and effective state
  enable   Enable a feature in config.toml
  disable  Disable a feature in config.toml
  help     Print this message or the help of the given subcommand(s)

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

### `codex help`

Invocation: `codex help help`

```text
Print this message or the help of the given subcommand(s)

Usage: codex help [COMMAND]...

Arguments:
  [COMMAND]...  Print help for the subcommand(s)
```

#### `codex exec resume`

Invocation: `codex help exec resume`

```text
Resume a previous session by id or pick the most recent with --last

Usage: codex exec resume [OPTIONS] [SESSION_ID] [PROMPT]

Arguments:
  [SESSION_ID]
          Conversation/session id (UUID) or thread name. UUIDs take precedence if it parses. If
          omitted, use --last to pick the most recent recorded session

  [PROMPT]
          Prompt to send after resuming the session. If `-` is used, read from stdin

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --last
          Resume the most recent recorded session (newest) without specifying an id

      --all
          Show all sessions (disables cwd filtering)

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -i, --image <FILE>
          Optional image(s) to attach to the prompt sent after resuming

  -m, --model <MODEL>
          Model the agent should use

      --full-auto
          Convenience alias for low-friction sandboxed automatic execution (-a on-request, --sandbox
          workspace-write)

      --dangerously-bypass-approvals-and-sandbox
          Skip all confirmation prompts and execute commands without sandboxing. EXTREMELY
          DANGEROUS. Intended solely for running in environments that are externally sandboxed

      --skip-git-repo-check
          Allow running Codex outside a Git repository

      --ephemeral
          Run without persisting session files to disk

      --json
          Print events to stdout as JSONL

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex exec review`

Invocation: `codex help exec review`

```text
Run a code review against the current repository

Usage: codex exec review [OPTIONS] [PROMPT]

Arguments:
  [PROMPT]
          Custom review instructions. If `-` is used, read from stdin

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --uncommitted
          Review staged, unstaged, and untracked changes

      --base <BRANCH>
          Review changes against the given base branch

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --commit <SHA>
          Review the changes introduced by a commit

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -m, --model <MODEL>
          Model the agent should use

      --title <TITLE>
          Optional commit title to display in the review summary

      --full-auto
          Convenience alias for low-friction sandboxed automatic execution (-a on-request, --sandbox
          workspace-write)

      --dangerously-bypass-approvals-and-sandbox
          Skip all confirmation prompts and execute commands without sandboxing. EXTREMELY
          DANGEROUS. Intended solely for running in environments that are externally sandboxed

      --skip-git-repo-check
          Allow running Codex outside a Git repository

      --ephemeral
          Run without persisting session files to disk

      --json
          Print events to stdout as JSONL

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex exec help`

Invocation: `codex help exec help`

```text
Print this message or the help of the given subcommand(s)

Usage: codex exec help [COMMAND]...

Arguments:
  [COMMAND]...  Print help for the subcommand(s)
```

#### `codex login status`

Invocation: `codex help login status`

```text
Show login status

Usage: codex login status [OPTIONS]

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex login help`

Invocation: `codex help login help`

```text
Print this message or the help of the given subcommand(s)

Usage: codex login help [COMMAND]...

Arguments:
  [COMMAND]...  Print help for the subcommand(s)
```

#### `codex mcp list`

Invocation: `codex help mcp list`

```text
Usage: codex mcp list [OPTIONS]

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --json
          Output the configured servers as JSON

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex mcp get`

Invocation: `codex help mcp get`

```text
Usage: codex mcp get [OPTIONS] <NAME>

Arguments:
  <NAME>
          Name of the MCP server to display

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --json
          Output the server configuration as JSON

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex mcp add`

Invocation: `codex help mcp add`

```text
Usage: codex mcp add [OPTIONS] <NAME> (--url <URL> | -- <COMMAND>...)

Arguments:
  <NAME>
          Name for the MCP server configuration

  [COMMAND]...
          Command to launch the MCP server. Use --url for a streamable HTTP server

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --env <KEY=VALUE>
          Environment variables to set when launching the server. Only valid with stdio servers

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --url <URL>
          URL for a streamable HTTP MCP server

      --bearer-token-env-var <ENV_VAR>
          Optional environment variable to read for a bearer token. Only valid with streamable HTTP
          servers

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex mcp remove`

Invocation: `codex help mcp remove`

```text
Usage: codex mcp remove [OPTIONS] <NAME>

Arguments:
  <NAME>
          Name of the MCP server configuration to remove

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex mcp login`

Invocation: `codex help mcp login`

```text
Usage: codex mcp login [OPTIONS] <NAME>

Arguments:
  <NAME>
          Name of the MCP server to authenticate with oauth

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --scopes <SCOPE,SCOPE>
          Comma-separated list of OAuth scopes to request

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex mcp logout`

Invocation: `codex help mcp logout`

```text
Usage: codex mcp logout [OPTIONS] <NAME>

Arguments:
  <NAME>
          Name of the MCP server to deauthenticate

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex mcp help`

Invocation: `codex help mcp help`

```text
Print this message or the help of the given subcommand(s)

Usage: codex mcp help [COMMAND]...

Arguments:
  [COMMAND]...  Print help for the subcommand(s)
```

#### `codex app-server generate-ts`

Invocation: `codex help app-server generate-ts`

```text
[experimental] Generate TypeScript bindings for the app server protocol

Usage: codex app-server generate-ts [OPTIONS] --out <DIR>

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

  -o, --out <DIR>
          Output directory where .ts files will be written

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

  -p, --prettier <PRETTIER_BIN>
          Optional path to the Prettier executable to format generated files

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

      --experimental
          Include experimental methods and fields in the generated output

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex app-server generate-json-schema`

Invocation: `codex help app-server generate-json-schema`

```text
[experimental] Generate JSON Schema for the app server protocol

Usage: codex app-server generate-json-schema [OPTIONS] --out <DIR>

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

  -o, --out <DIR>
          Output directory where the schema bundle will be written

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --experimental
          Include experimental methods and fields in the generated output

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex app-server help`

Invocation: `codex help app-server help`

```text
Print this message or the help of the given subcommand(s)

Usage: codex app-server help [COMMAND]...

Arguments:
  [COMMAND]...  Print help for the subcommand(s)
```

#### `codex sandbox macos`

Invocation: `codex help sandbox macos`

```text
Run a command under Seatbelt (macOS only)

Usage: codex sandbox macos [OPTIONS] [COMMAND]...

Arguments:
  [COMMAND]...
          Full command args to run under seatbelt

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --full-auto
          Convenience alias for low-friction sandboxed automatic execution (network-disabled sandbox
          that can write to cwd and TMPDIR)

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --log-denials
          While the command runs, capture macOS sandbox denials via `log stream` and print them
          after exit

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex sandbox linux`

Invocation: `codex help sandbox linux`

```text
Run a command under Landlock+seccomp (Linux only)

Usage: codex sandbox linux [OPTIONS] [COMMAND]...

Arguments:
  [COMMAND]...
          Full command args to run under landlock

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --full-auto
          Convenience alias for low-friction sandboxed automatic execution (network-disabled sandbox
          that can write to cwd and TMPDIR)

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex sandbox windows`

Invocation: `codex help sandbox windows`

```text
Run a command under Windows restricted token (Windows only)

Usage: codex sandbox windows [OPTIONS] [COMMAND]...

Arguments:
  [COMMAND]...
          Full command args to run under Windows restricted token sandbox

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --full-auto
          Convenience alias for low-friction sandboxed automatic execution (network-disabled sandbox
          that can write to cwd and TMPDIR)

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex sandbox help`

Invocation: `codex help sandbox help`

```text
Print this message or the help of the given subcommand(s)

Usage: codex sandbox help [COMMAND]...

Arguments:
  [COMMAND]...  Print help for the subcommand(s)
```

#### `codex debug app-server`

Invocation: `codex help debug app-server`

```text
Tooling: helps debug the app server

Usage: codex debug app-server [OPTIONS] <COMMAND>

Commands:
  send-message-v2  
  help             Print this message or the help of the given subcommand(s)

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex debug help`

Invocation: `codex help debug help`

```text
Print this message or the help of the given subcommand(s)

Usage: codex debug help [COMMAND]...

Arguments:
  [COMMAND]...  Print help for the subcommand(s)
```

#### `codex cloud exec`

Invocation: `codex help cloud exec`

```text
Submit a new Codex Cloud task without launching the TUI

Usage: codex cloud exec [OPTIONS] --env <ENV_ID> [QUERY]

Arguments:
  [QUERY]
          Task prompt to run in Codex Cloud

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --env <ENV_ID>
          Target environment identifier (see `codex cloud` to browse)

      --attempts <ATTEMPTS>
          Number of assistant attempts (best-of-N)
          
          [default: 1]

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --branch <BRANCH>
          Git branch to run in Codex Cloud (defaults to current branch)

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex cloud status`

Invocation: `codex help cloud status`

```text
Show the status of a Codex Cloud task

Usage: codex cloud status [OPTIONS] <TASK_ID>

Arguments:
  <TASK_ID>
          Codex Cloud task identifier to inspect

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex cloud list`

Invocation: `codex help cloud list`

```text
List Codex Cloud tasks

Usage: codex cloud list [OPTIONS]

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --env <ENV_ID>
          Filter tasks by environment identifier

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --limit <N>
          Maximum number of tasks to return (1-20)
          
          [default: 20]

      --cursor <CURSOR>
          Pagination cursor returned by a previous call

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

      --json
          Emit JSON instead of plain text

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex cloud apply`

Invocation: `codex help cloud apply`

```text
Apply the diff for a Codex Cloud task locally

Usage: codex cloud apply [OPTIONS] <TASK_ID>

Arguments:
  <TASK_ID>
          Codex Cloud task identifier to apply

Options:
      --attempt <N>
          Attempt number to apply (1-based)

  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex cloud diff`

Invocation: `codex help cloud diff`

```text
Show the unified diff for a Codex Cloud task

Usage: codex cloud diff [OPTIONS] <TASK_ID>

Arguments:
  <TASK_ID>
          Codex Cloud task identifier to display

Options:
      --attempt <N>
          Attempt number to display (1-based)

  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex cloud help`

Invocation: `codex help cloud help`

```text
Print this message or the help of the given subcommand(s)

Usage: codex cloud help [COMMAND]...

Arguments:
  [COMMAND]...  Print help for the subcommand(s)
```

#### `codex features list`

Invocation: `codex help features list`

```text
List known features with their stage and effective state

Usage: codex features list [OPTIONS]

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex features enable`

Invocation: `codex help features enable`

```text
Enable a feature in config.toml

Usage: codex features enable [OPTIONS] <FEATURE>

Arguments:
  <FEATURE>
          Feature key to update (for example: unified_exec)

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex features disable`

Invocation: `codex help features disable`

```text
Disable a feature in config.toml

Usage: codex features disable [OPTIONS] <FEATURE>

Arguments:
  <FEATURE>
          Feature key to update (for example: unified_exec)

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

#### `codex features help`

Invocation: `codex help features help`

```text
Print this message or the help of the given subcommand(s)

Usage: codex features help [COMMAND]...

Arguments:
  [COMMAND]...  Print help for the subcommand(s)
```

##### `codex debug app-server send-message-v2`

Invocation: `codex help debug app-server send-message-v2`

```text
Usage: codex debug app-server send-message-v2 [OPTIONS] <USER_MESSAGE>

Arguments:
  <USER_MESSAGE>
          

Options:
  -c, --config <key=value>
          Override a configuration value that would otherwise be loaded from `~/.codex/config.toml`.
          Use a dotted path (`foo.bar.baz`) to override nested values. The `value` portion is parsed
          as TOML. If it fails to parse as TOML, the raw string is used as a literal.
          
          Examples: - `-c model="o3"` - `-c 'sandbox_permissions=["disk-full-read-access"]'` - `-c
          shell_environment_policy.inherit=all`

      --enable <FEATURE>
          Enable a feature (repeatable). Equivalent to `-c features.<name>=true`

      --disable <FEATURE>
          Disable a feature (repeatable). Equivalent to `-c features.<name>=false`

  -h, --help
          Print help (see a summary with '-h')
```

##### `codex debug app-server help`

Invocation: `codex help debug app-server help`

```text
Print this message or the help of the given subcommand(s)

Usage: codex debug app-server help [COMMAND]...

Arguments:
  [COMMAND]...  Print help for the subcommand(s)
```
