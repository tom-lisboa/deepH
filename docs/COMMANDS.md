# deepH Command Dictionary

Reference for all CLI commands currently available in `deeph`.

Tips:
- Use `deeph command list` to browse by category.
- Use `deeph command explain "<command path>"` for examples and notes.

## Meta

### `help`
- Purpose: Show CLI usage and top-level commands.
- Usage:
  - `deeph help`
  - `deeph --help`
  - `deeph -h`

### `command list`
- Purpose: List commands grouped by category.
- Usage:
  - `deeph command list [--category CAT] [--json]`

### `command explain`
- Purpose: Explain one command path from the command dictionary.
- Usage:
  - `deeph command explain [--json] "<command path>"`
- Examples:
  - `deeph command explain "provider add"`
  - `deeph command explain "trace"`

## Workspace

### `init`
- Purpose: Initialize a `deepH` workspace (`deeph.yaml`, `agents/`, `skills/`, `examples/`).
- Usage:
  - `deeph init [--workspace DIR]`

### `quickstart`
- Purpose: One-command starter setup (init workspace + starter agent + optional echo skill/provider).
- Usage:
  - `deeph quickstart [--workspace DIR] [--agent NAME] [--provider NAME] [--model MODEL] [--with-echo] [--deepseek] [--force]`
- Examples:
  - `deeph quickstart`
  - `deeph quickstart --workspace ./myproj --deepseek`
  - `deeph quickstart --agent planner --provider deepseek --model deepseek-chat`
- Notes:
  - Creates a starter agent template and validates the workspace immediately.
  - With `--deepseek`, scaffolds provider config and sets it as `default_provider`.

### `studio`
- Purpose: Interactive menu mode for onboarding and common workflows.
- Usage:
  - `deeph studio [--workspace DIR]`
- Examples:
  - `deeph studio`
  - `deeph studio --workspace ./myproj`
- Notes:
  - Guides users through quickstart, provider setup, agent creation, run and chat.

### `update`
- Purpose: Download and install latest (or specific) GitHub release binary for this platform.
- Usage:
  - `deeph update [--owner NAME] [--repo NAME] [--tag latest|vX.Y.Z] [--check]`
- Examples:
  - `deeph update`
  - `deeph update --check`
  - `deeph update --tag v0.1.0`
- Notes:
  - Defaults to `tom-lisboa/deepH`.
  - On Windows, update is downloaded as `deeph.new.exe` and replacement steps are printed.

### `validate`
- Purpose: Validate `deeph.yaml`, agents and skills YAML files.
- Usage:
  - `deeph validate [--workspace DIR]`

## Execution

### `review`
- Purpose: Review the current git diff with a compact, Go-aware working set.
- Usage:
  - `deeph review [--workspace DIR] [--spec SPEC] [--base REF|auto] [--trace] [--coach=false] [--checks=true|false] [--check-timeout 45s] [--json] [focus]`
- Examples:
  - `deeph review`
  - `deeph review --base auto`
  - `deeph review --trace "focus on regressions and missing tests"`
  - `deeph review --spec @reviewflow`
  - `deeph review --spec reviewer`
  - `deeph review --checks=false`
  - `deeph review --json`
- Notes:
  - Builds a compact review brief from the current git diff plus a Go-aware working set (same package, tests, local imports, reverse imports).
  - `--json` prints the generated scope and review input payload instead of running the agent.
  - `--base auto` (default) tries `HEAD`, `HEAD~1`, upstream merge-base and last-commit patch, reducing "no local diff" failures.
  - `--checks` runs deterministic pre-review checks (`go test ./...` and `go vet ./...`) and injects a compact summary into the review context.
  - When `crews/reviewflow.yaml` exists, defaults to `@reviewflow`; otherwise falls back to a builtin multiverse review flow rooted at `reviewer` or `guide`.
  - Passing `--spec SPEC` keeps the review on that explicit agent or crew instead of auto-selecting the builtin flow.

### `trace`
- Purpose: Show the execution plan (stages, channels, handoffs) before running.
- Usage:
  - `deeph trace [--workspace DIR] [--json] [--multiverse N] [--daemon=true|false] [--daemon-target HOST:PORT] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`
- Examples:
  - `deeph trace guide "teste"`
  - `deeph trace "planner+reader>coder>reviewer" "implemente X"`
  - `deeph trace --json "planner+coder>reviewer" "debug"`
  - `deeph trace --multiverse 0 @reviewpack "task"`
  - `deeph trace --daemon @reviewpack "task"`
  - `deeph trace --daemon=false guide "task"`
- Notes:
  - `--multiverse N` traces N universes; with `@crew`, `--multiverse 0` means all crew universes.
  - Crew universes can declare `depends_on` to create multiverse channels (`u1.result -> u2.context`) shown in the trace.
  - `--daemon` defaults to `true` and forwards the request to a local `deephd` gRPC daemon.
  - If daemon is unavailable, deepH tries to start it and falls back to local execution when needed.
  - Use `--daemon=false` to force local in-process execution.
  - Set `DEEPH_DAEMON_DEBUG=1` to print daemon connection-pool stats (`hits/misses/dials/drops`) to stderr.

### `run`
- Purpose: Execute one or more agents with `dag_channels` orchestration.
- Usage:
  - `deeph run [--workspace DIR] [--trace] [--coach=false] [--multiverse N] [--judge-agent SPEC] [--judge-max-output-chars N] [--daemon=true|false] [--daemon-target HOST:PORT] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`
- Examples:
  - `deeph run guide "teste"`
  - `deeph run "planner+reader>coder>reviewer" "crie feature X"`
  - `deeph run --trace "a+b>c" "task"`
  - `deeph run --multiverse 0 @reviewpack "task"`
  - `deeph run --multiverse 0 --judge-agent guide @reviewpack "task"`
  - `deeph run --daemon guide "task"`
  - `deeph run --daemon=false guide "task"`
- Notes:
  - `--multiverse` runs branch universes and prints a sink-output fingerprint consensus.
  - Crew universes with `depends_on` run with a multiverse DAG/channels scheduler and can contribute compact handoffs to downstream universes.
  - `--judge-agent` runs a follow-up comparison agent over branch summaries (reconcile step).
  - Judge output is parsed when possible (JSON or labeled sections) to show `winner`, `rationale`, `risks` and `follow_up` clearly.
  - `--daemon` defaults to `true` and forwards the run to a local `deephd` gRPC daemon.
  - If daemon is unavailable, deepH tries to start it and falls back to local execution when needed.
  - Use `--daemon=false` to force local in-process execution.
  - Set `DEEPH_DAEMON_DEBUG=1` to print daemon connection-pool stats (`hits/misses/dials/drops`) to stderr.
  - Shows occasional local semantic hints while waiting (disable with `--coach=false` or `DEEPH_COACH=0`).
  - The coach learns local command transitions (ex.: `run -> trace`) to suggest likely next steps without extra LLM tokens.

### `daemon serve`
- Purpose: Run the local `deephd` gRPC daemon in foreground.
- Usage:
  - `deeph daemon serve [--target HOST:PORT]`
- Examples:
  - `deeph daemon serve`
  - `deeph daemon serve --target 127.0.0.1:7788`
- Notes:
  - Serves Ping/Trace/Run/Shutdown gRPC methods over HTTP/2 with protobuf structs.
  - Use `deeph run --daemon ...` or `deeph trace --daemon ...` to send requests to this process.

### `daemon start`
- Purpose: Start `deephd` in background and wait until reachable.
- Usage:
  - `deeph daemon start [--target HOST:PORT]`
- Examples:
  - `deeph daemon start`
  - `deeph daemon start --target 127.0.0.1:7788`
- Notes:
  - Starts a detached `deeph daemon serve` process and writes logs to a temp file.

### `daemon status`
- Purpose: Check whether `deephd` is reachable.
- Usage:
  - `deeph daemon status [--target HOST:PORT]`
- Examples:
  - `deeph daemon status`

### `daemon stop`
- Purpose: Request graceful shutdown of `deephd`.
- Usage:
  - `deeph daemon stop [--target HOST:PORT]`
- Examples:
  - `deeph daemon stop`

### `chat`
- Purpose: Start a fluid terminal chat session with one agent or multi-agent spec.
- Usage:
  - `deeph chat [--workspace DIR] [--session ID] [--history-turns N] [--history-tokens N] [--trace] [--coach=false] "<agent|a+b|a>b|a+b>c>"`
  - `deeph chat [--workspace DIR] --session ID`
- Examples:
  - `deeph chat guide`
  - `deeph chat "planner+reader>coder>reviewer"`
  - `deeph chat --session feat-login`
  - `deeph chat --session feat-login guide`
- Notes:
  - Persists history under `sessions/<id>.jsonl` and `sessions/<id>.meta.json`.
  - Slash commands: `/help`, `/history`, `/trace`, `/exit`.
  - Shows occasional local semantic hints while waiting (disable with `--coach=false` or `DEEPH_COACH=0`).
  - The coach can learn local follow-up patterns (ex.: `chat -> session show`) in the workspace.

### `gws`
- Purpose: Execute Google Workspace CLI (`gws`) through a safe `deeph` wrapper.
- Usage:
  - `deeph gws [--yes|--allow-mutate] [--json] [--timeout 30s] [--max-output-bytes N] [--bin gws] [--allow-any-root] <gws args...>`
  - `deeph gws -- <raw gws args...>`
- Examples:
  - `deeph gws drive files list --page-size 5`
  - `deeph gws --json gmail users.messages.list --user me --max-results 10`
  - `deeph gws --allow-mutate drive files delete --file-id 123`
- Notes:
  - Runs `gws` via `exec.Command` (no shell expansion/pipes).
  - Blocks unknown root groups by default; use `--allow-any-root` only when needed.
  - Commands that look mutating require `--allow-mutate` (or `--yes`).
  - Use `--` when raw `gws` flags conflict with wrapper flags like `--timeout`.

## Agents

### `agent create`
- Purpose: Create a user-defined agent template in `agents/`.
- Usage:
  - `deeph agent create [--workspace DIR] [--force] [--provider NAME] [--model MODEL] <name>`
- Examples:
  - `deeph agent create analyst`
  - `deeph agent create --provider deepseek --model deepseek-chat reviewer`

## Crews

### `crew list`
- Purpose: List crew presets in `crews/` (agent-spec aliases with optional multiverse universes).
- Usage:
  - `deeph crew list [--workspace DIR]`

### `crew show`
- Purpose: Show one crew preset, base spec and universe variants.
- Usage:
  - `deeph crew show [--workspace DIR] <name>`
- Examples:
  - `deeph crew show reviewpack`
- Notes:
  - Use `@name` or `crew:name` in `run/trace`.
  - If `universes` are defined, combine with `--multiverse` for branch presets.
  - Each universe can declare `depends_on`, `input_port`, `output_port`, `output_kind`, `merge_policy` and `handoff_max_chars`.

## Skills

### `skill list`
- Purpose: List built-in skill templates available to install.
- Usage:
  - `deeph skill list`

### `skill add`
- Purpose: Install a skill template YAML into `skills/`.
- Usage:
  - `deeph skill add [--workspace DIR] [--force] <name>`
- Examples:
  - `deeph skill add echo`
  - `deeph skill add file_read_range`

## Providers

### `provider list`
- Purpose: List providers configured in `deeph.yaml`.
- Usage:
  - `deeph provider list [--workspace DIR]`

### `provider add`
- Purpose: Add/update provider scaffold in `deeph.yaml` (DeepSeek-focused).
- Usage:
  - `deeph provider add [--workspace DIR] [--name NAME] [--model MODEL] [--set-default] [--force] deepseek`
- Examples:
  - `deeph provider add --set-default deepseek`
  - `deeph provider add --name deepseek_prod --model deepseek-chat --timeout-ms 30000 deepseek`
  - `deeph provider add --force --api-key-env DEEPSEEK_API_KEY deepseek`

## Kits

### `kit list`
- Purpose: List installable starter kits (agents + crews + skills templates).
- Usage:
  - `deeph kit list [--workspace DIR]`
- Examples:
  - `deeph kit list`

### `kit add`
- Purpose: Install a starter kit by name (skills, agents, crews and provider scaffold).
- Usage:
  - `deeph kit add [--workspace DIR] [--force] [--provider-name NAME] [--model MODEL] [--set-default-provider] [--skip-provider] <name|git-url[#manifest.yaml]>`
- Examples:
  - `deeph kit add hello-next-tailwind`
  - `deeph kit add hello-next-shadcn`
  - `deeph kit add crud-next-multiverse`
  - `deeph kit add crud-next-multiverse --provider-name deepseek --model deepseek-chat`
  - `deeph kit add https://github.com/acme/deeph-kits.git#kits/next/kit.yaml`
- Notes:
  - Required skills are installed automatically from the built-in skill catalog.
  - Existing files are preserved by default; use `--force` to overwrite changed templates.
  - DeepSeek provider config is scaffolded by default for DeepSeek-focused kits.
  - Git URL mode expects `deeph-kit.yaml` or `kit.yaml` in repo root unless `#path/to/manifest.yaml` is provided.

## CRUD

### `crud init`
- Purpose: Initialize the opinionated CRUD starter and save a workspace CRUD profile.
- Usage:
  - `deeph crud init [--workspace DIR] [--force] [--provider-name NAME] [--model MODEL] [--set-default-provider] [--skip-provider] [--mode backend|fullstack] [--db-kind relational|document] [--db postgres|mongodb] [--entity NAME] [--fields nome:text,cidade:text] [--containers=true|false]`
- Examples:
  - `deeph crud init`
  - `deeph crud init --workspace ./futebol`
  - `deeph crud init --workspace ./futebol --mode fullstack --db-kind relational --db postgres --entity players --fields nome:text,posicao:text,time_id:int`
- Notes:
  - Bootstraps the workspace if needed and installs the `crud-next-multiverse` kit.
  - Defaults to `Go` + `Next.js` + `Postgres` as the recommended CRUD stack.
  - In interactive terminals, asks about backend-only vs fullstack, relational vs document, entity, fields and local containers, then saves the answers in `.deeph/crud.json`.

### `crud prompt`
- Purpose: Print the opinionated CRUD prompt that will be sent to the multiverse crew.
- Usage:
  - `deeph crud prompt [--workspace DIR] [--entity NAME] [--fields nome:text,cidade:text] [--db-kind relational|document] [--db postgres|mongodb] [--backend go] [--frontend next] [--backend-only] [--mode backend|fullstack] [--containers=true|false]`
- Examples:
  - `deeph crud prompt --entity players --fields nome:text,cidade:text`
  - `deeph crud prompt --entity teams --fields nome:text,cidade:text,pais:text --backend-only`

### `crud trace`
- Purpose: Trace the CRUD multiverse run using the opinionated `Go` + `Next.js` + `Postgres` defaults.
- Usage:
  - `deeph crud trace [--workspace DIR] [--entity NAME] [--fields nome:text,cidade:text] [--db-kind relational|document] [--db postgres|mongodb] [--backend go] [--frontend next] [--backend-only] [--mode backend|fullstack] [--containers=true|false]`
- Examples:
  - `deeph crud trace --entity people --fields nome:text,cidade:text`
  - `deeph crud trace --workspace ./futebol --entity players --fields nome:text,posicao:text,time_id:int`
- Notes:
  - Chooses the CRUD crew from the saved CRUD profile or from explicit flags (`crud_backend_relational`, `crud_fullstack_relational`, `crud_backend_document`, `crud_fullstack_document`).
  - `--mode` lets the user switch between backend-only and fullstack without editing `.deeph/crud.json`.

### `crud run`
- Purpose: Run the CRUD multiverse crew with the opinionated `Go` + `Next.js` + `Postgres` defaults.
- Usage:
  - `deeph crud run [--workspace DIR] [--entity NAME] [--fields nome:text,cidade:text] [--db-kind relational|document] [--db postgres|mongodb] [--backend go] [--frontend next] [--backend-only] [--mode backend|fullstack] [--containers=true|false]`
- Examples:
  - `deeph crud run --entity people --fields nome:text,cidade:text`
  - `deeph crud run --workspace ./futebol --entity players --fields nome:text,posicao:text,time_id:int`
- Notes:
  - Chooses the CRUD crew from the saved CRUD profile or from explicit flags (`crud_backend_relational`, `crud_fullstack_relational`, `crud_backend_document`, `crud_fullstack_document`).
  - Use `--backend-only` when you want just the `Go` API and infra without generating `Next.js` pages.
  - `--containers=false` lets the user ask for local startup without Docker Compose in the generated prompt.

### `crud up`
- Purpose: Start the generated CRUD containers with Docker Compose.
- Usage:
  - `deeph crud up [--workspace DIR] [--compose-file FILE] [--build=true|false] [--detach=true|false] [--wait 45s] [--base-url URL]`
- Examples:
  - `deeph crud up`
  - `deeph crud up --workspace ./futebol`
  - `deeph crud up --workspace ./futebol --wait 60s`
- Notes:
  - Auto-detects `docker-compose.yml`, `docker-compose.yaml`, `compose.yml` or `compose.yaml` inside the workspace unless `--compose-file` is provided.
  - Prefers `docker compose`, then falls back to `docker-compose` when available.

### `crud smoke`
- Purpose: Run the generated CRUD smoke test or fall back to a built-in HTTP CRUD probe.
- Usage:
  - `deeph crud smoke [--workspace DIR] [--compose-file FILE] [--base-url URL] [--route-base /people] [--entity NAME] [--fields nome:text,cidade:text] [--no-script] [--timeout 45s]`
- Examples:
  - `deeph crud smoke`
  - `deeph crud smoke --workspace ./futebol --base-url http://127.0.0.1:8080`
  - `deeph crud smoke --workspace ./futebol --no-script --entity players --fields nome:text,posicao:text`
- Notes:
  - Runs `scripts/smoke.sh` or `scripts/smoke.ps1` when the generated project provides them.
  - When no script is found, tries a built-in create/list/read/update/delete flow using the saved CRUD profile.

### `crud down`
- Purpose: Stop the generated CRUD containers with Docker Compose.
- Usage:
  - `deeph crud down [--workspace DIR] [--compose-file FILE] [--volumes]`
- Examples:
  - `deeph crud down`
  - `deeph crud down --workspace ./futebol`
  - `deeph crud down --workspace ./futebol --volumes`
- Notes:
  - Stops the detected compose stack and removes orphan containers.
  - Use `--volumes` only when you intentionally want to drop local database volumes.

## Coach

### `coach stats`
- Purpose: Inspect local coach learning state (hints, command counts, next-step transitions, port hotspots).
- Usage:
  - `deeph coach stats [--workspace DIR] [--top N] [--scope SPEC] [--kind KIND] [--json]`
- Examples:
  - `deeph coach stats`
  - `deeph coach stats --top 20`
  - `deeph coach stats --scope "planner+reader>coder>reviewer"`
  - `deeph coach stats --kind handoff_drop`
  - `deeph coach stats --json`
- Notes:
  - Reads workspace-local `.deeph/coach_state.json`.
  - Purely local (no LLM calls) and used to drive semantic onboarding hints.
  - Includes `port_signals` counters used by post-run optimization hints.
  - `--scope` inspects workflow-specific transitions/port signals keyed by agent spec.
  - `--kind` filters `port_signals` (`handoff_drop`, `context_channel_drop`, `context_drop`).

### `coach reset`
- Purpose: Reset local coach learning state for the workspace (full or partial).
- Usage:
  - `deeph coach reset [--workspace DIR] [--all] [--hints] [--transitions] [--commands] [--ports] --yes`
- Examples:
  - `deeph coach reset --yes`
  - `deeph coach reset --all --yes`
  - `deeph coach reset --ports --yes`
  - `deeph coach reset --hints --transitions --yes`
- Notes:
  - Without partial flags, removes `.deeph/coach_state.json`.
  - With partial flags, only the selected sections are cleared.

## Sessions

### `session list`
- Purpose: List saved chat sessions in the workspace.
- Usage:
  - `deeph session list [--workspace DIR]`

### `session show`
- Purpose: Show persisted session metadata and recent entries.
- Usage:
  - `deeph session show [--workspace DIR] [--tail N] <id>`
- Examples:
  - `deeph session show feat-login`
  - `deeph session show --tail 50 feat-login`

## Types

### `type list`
- Purpose: List semantic runtime types (for typed ports/context).
- Usage:
  - `deeph type list [--category CAT] [--json]`
- Examples:
  - `deeph type list`
  - `deeph type list --category code`
  - `deeph type list --json`

### `type explain`
- Purpose: Explain one semantic type or alias.
- Usage:
  - `deeph type explain [--json] <kind|alias>`
- Examples:
  - `deeph type explain code/go`
  - `deeph type explain CODE.GO`
  - `deeph type explain string`
