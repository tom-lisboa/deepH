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

### `validate`
- Purpose: Validate `deeph.yaml`, agents and skills YAML files.
- Usage:
  - `deeph validate [--workspace DIR]`

## Execution

### `trace`
- Purpose: Show the execution plan (stages, channels, handoffs) before running.
- Usage:
  - `deeph trace [--workspace DIR] [--json] [--multiverse N] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`
- Examples:
  - `deeph trace guide "teste"`
  - `deeph trace "planner+reader>coder>reviewer" "implemente X"`
  - `deeph trace --json "planner+coder>reviewer" "debug"`
  - `deeph trace --multiverse 0 @reviewpack "task"`
- Notes:
  - `--multiverse N` traces N universes; with `@crew`, `--multiverse 0` means all crew universes.
  - Crew universes can declare `depends_on` to create multiverse channels (`u1.result -> u2.context`) shown in the trace.

### `run`
- Purpose: Execute one or more agents with `dag_channels` orchestration.
- Usage:
  - `deeph run [--workspace DIR] [--trace] [--coach=false] [--multiverse N] [--judge-agent SPEC] [--judge-max-output-chars N] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`
- Examples:
  - `deeph run guide "teste"`
  - `deeph run "planner+reader>coder>reviewer" "crie feature X"`
  - `deeph run --trace "a+b>c" "task"`
  - `deeph run --multiverse 0 @reviewpack "task"`
  - `deeph run --multiverse 0 --judge-agent guide @reviewpack "task"`
- Notes:
  - `--multiverse` runs branch universes and prints a sink-output fingerprint consensus.
  - Crew universes with `depends_on` run with a multiverse DAG/channels scheduler and can contribute compact handoffs to downstream universes.
  - `--judge-agent` runs a follow-up comparison agent over branch summaries (reconcile step).
  - Judge output is parsed when possible (JSON or labeled sections) to show `winner`, `rationale`, `risks` and `follow_up` clearly.
  - Shows occasional local semantic hints while waiting (disable with `--coach=false` or `DEEPH_COACH=0`).
  - The coach learns local command transitions (ex.: `run -> trace`) to suggest likely next steps without extra LLM tokens.

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

## Agents

### `agent create`
- Purpose: Create a user-defined agent template in `agents/`.
- Usage:
  - `deeph agent create [--workspace DIR] [--force] [--provider NAME] [--model MODEL] <name>`
- Examples:
  - `deeph agent create analyst`
  - `deeph agent create reviewer --provider deepseek --model deepseek-chat`

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
  - `deeph provider add deepseek --set-default`
  - `deeph provider add deepseek --name deepseek_prod --model deepseek-chat --timeout-ms 30000`
  - `deeph provider add deepseek --force --api-key-env DEEPSEEK_API_KEY`

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
