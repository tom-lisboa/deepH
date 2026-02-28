# deepH

`deepH` is a lightweight agent runtime in Go for user-defined agents and installable skills.

The core idea is simple:

- the runtime knows contracts (`agent`, `skill`, `provider`)
- users define agents in `YAML`
- skills are optional and installable
- providers are swappable

This is not a locked app with baked-in agents.
It is an engine that loads your agents.

## Why this is different

- Lightweight Go binary and low dependency surface
- Agents belong to the user (`agents/*.yaml`)
- Skills are optional (`deeph skill add ...`)
- Provider-agnostic runtime (`mock`, `http`, future DeepSeek/OpenAI/etc.)
- Built-in `validate` and `trace` for observability and debugging

## Status

MVP scaffold (working CLI) with:

- `init`
- `run`
- `validate`
- `trace`
- `agent create`
- `skill list`
- `skill add`
- `type list`
- `type explain`

## Project Layout

```text
.
├── deeph.yaml
├── agents/
├── skills/
└── examples/
    └── agents/
```

## Quick Start (2 minutes)

### Install from GitHub Releases (recommended)

macOS / Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/tom-lisboa/deepH/main/scripts/install.sh | bash
```

Windows PowerShell:

```powershell
$tmp = Join-Path $env:TEMP "deeph-install.ps1"
iwr https://raw.githubusercontent.com/tom-lisboa/deepH/main/scripts/install.ps1 -UseBasicParsing -OutFile $tmp
Set-ExecutionPolicy -Scope Process Bypass -Force
& $tmp
```

Detailed Windows guide:

- [Windows PowerShell step by step](docs/WINDOWS_POWERSHELL.md)

Then run:

```bash
deeph
```

`deeph` opens `studio` by default in interactive terminals (guided menu).

For quick non-interactive start:

```bash
mkdir -p ~/deeph-workspace && cd ~/deeph-workspace
deeph quickstart --deepseek
export DEEPSEEK_API_KEY="sk-...your_real_key..."
deeph run guide "hello"
```

Windows CMD key setup:

```bat
set DEEPSEEK_API_KEY=sk-...your_real_key...
```

Notes:

- `quickstart` creates workspace + starter agent + `echo` skill + validation in one flow.
- Use a real DeepSeek key; placeholders like `sk-CHAVE_NOVA_REAL` will return 401.
- Update binary any time with `deeph update`.

## Guided Walkthroughs

- [Agents e Agent Spec no deepH](docs/AGENTS_AND_SPECS.md)
- [O que e um agent](docs/AGENTS_AND_SPECS.md#o-que-e-um-agent)
- [Anatomia do arquivo do agent](docs/AGENTS_AND_SPECS.md#anatomia-do-arquivo-do-agent)
- [O que e um agent spec](docs/AGENTS_AND_SPECS.md#o-que-e-um-agent-spec)
- [Como funciona o `+`](docs/AGENTS_AND_SPECS.md#como-funciona-o-operador-plus-)
- [Como funciona o `>`](docs/AGENTS_AND_SPECS.md#como-funciona-o-operador-maior-que-)
- [Combinar `+` e `>`](docs/AGENTS_AND_SPECS.md#combinar-plus--e-maior-que-)
- [Quando usar cada formato](docs/AGENTS_AND_SPECS.md#quando-usar-cada-formato)
- [Conversa com agents no chat](docs/AGENTS_AND_SPECS.md#conversa-com-agents-no-chat)
- [Quando usar `@crew` ou `crew:nome`](docs/AGENTS_AND_SPECS.md#quando-usar-crew-ou-crewnome)
- [Erros comuns](docs/AGENTS_AND_SPECS.md#erros-comuns)
- [Workflows, Universos e Comunicacao no deepH](docs/WORKFLOWS_AND_UNIVERSES.md)
- [O que e um workflow](docs/WORKFLOWS_AND_UNIVERSES.md#o-que-e-um-workflow)
- [Workflow simples vs workflow nomeado](docs/WORKFLOWS_AND_UNIVERSES.md#workflow-simples-vs-workflow-nomeado)
- [O que e um universo](docs/WORKFLOWS_AND_UNIVERSES.md#o-que-e-um-universo)
- [Exemplo de crew com universos](docs/WORKFLOWS_AND_UNIVERSES.md#exemplo-de-crew-com-universos)
- [Como os universos se comunicam](docs/WORKFLOWS_AND_UNIVERSES.md#como-os-universos-se-comunicam)
- [Campos que controlam a comunicacao](docs/WORKFLOWS_AND_UNIVERSES.md#campos-que-controlam-a-comunicacao)
- [Quando os universos nao se comunicam](docs/WORKFLOWS_AND_UNIVERSES.md#quando-os-universos-nao-se-comunicam)
- [Como inspecionar isso na CLI](docs/WORKFLOWS_AND_UNIVERSES.md#como-inspecionar-isso-na-cli)
- [Quando usar workflow simples, crew ou multiverse](docs/WORKFLOWS_AND_UNIVERSES.md#quando-usar-workflow-simples-crew-ou-multiverse)
- [Erros comuns em workflows/universos](docs/WORKFLOWS_AND_UNIVERSES.md#erros-comuns)
- [Windows PowerShell com deepH](docs/WINDOWS_POWERSHELL.md)
- [Passo 1: instalar o deeph no PowerShell](docs/WINDOWS_POWERSHELL.md#passo-1-instalar-o-deeph)
- [Passo 2: confirmar o binario](docs/WINDOWS_POWERSHELL.md#passo-2-confirmar-o-binario)
- [Passo 3: salvar e importar a API key](docs/WINDOWS_POWERSHELL.md#passo-3-salvar-e-importar-a-api-key)
- [Passo 4: criar o workspace](docs/WINDOWS_POWERSHELL.md#passo-4-criar-o-workspace)
- [Passo 5: configurar o provider DeepSeek](docs/WINDOWS_POWERSHELL.md#passo-5-configurar-o-provider-deepseek)
- [Passo 6: abrir o studio e conversar](docs/WINDOWS_POWERSHELL.md#passo-6-abrir-o-studio-e-conversar)
- [Passo 7: troubleshooting](docs/WINDOWS_POWERSHELL.md#passo-7-troubleshooting)
- [Calculadora estilo iPhone com deepH](docs/IPHONE_CALCULATOR.md)
- [Passo 1: instalar o deeph](docs/IPHONE_CALCULATOR.md#passo-1-instalar-o-deeph)
- [Passo 2: criar workspace e configurar a API key](docs/IPHONE_CALCULATOR.md#passo-2-criar-workspace-e-configurar-a-api-key)
- [Passo 3: bootstrap do workspace e aumento de timeout](docs/IPHONE_CALCULATOR.md#passo-3-bootstrap-do-workspace-e-aumento-de-timeout)
- [Passo 4: gerar a base fullstack da calculadora](docs/IPHONE_CALCULATOR.md#passo-4-gerar-a-base-fullstack-da-calculadora)
- [Passo 5: criar os agents de frontend iPhone](docs/IPHONE_CALCULATOR.md#passo-5-criar-os-agents-de-frontend-iphone)
- [Passo 6: colar os prompts completos dos agents](docs/IPHONE_CALCULATOR.md#passo-6-colar-os-prompts-completos-dos-agents)
- [Passo 7: criar o crew de multiverso front-only](docs/IPHONE_CALCULATOR.md#passo-7-criar-o-crew-de-multiverso-front-only)
- [Passo 8: rodar o multiverso](docs/IPHONE_CALCULATOR.md#passo-8-rodar-o-multiverso)
- [Passo 9: recovery se algum universo falhar](docs/IPHONE_CALCULATOR.md#passo-9-recovery-se-algum-universo-falhar)
- [Passo 10: pedir para o deeph configurar Tailwind e a paleta iPhone](docs/IPHONE_CALCULATOR.md#passo-10-pedir-para-o-deeph-configurar-tailwind-e-a-paleta-iphone)
- [Passo 11: subir backend e frontend](docs/IPHONE_CALCULATOR.md#passo-11-subir-backend-e-frontend)
- [Passo 12: conferir o que foi gerado](docs/IPHONE_CALCULATOR.md#passo-12-conferir-o-que-foi-gerado)

GitHub renders these links as clickable Markdown links on the repository homepage. Opening in a new tab is controlled by the browser/UI; use `Cmd`/`Ctrl` + click if needed.

### 3) Daily commands

```bash
deeph trace guide "analyze this"
deeph run guide "analyze this"
deeph update --check

# DAG/staged orchestration (quote it because '>' is a shell redirect)
deeph trace "planner+reader>coder>reviewer" "build a concise plan and review"
deeph run "planner+reader>coder>reviewer" "build a concise plan and review"
```

### Release flow (maintainer)

1. Bump version/tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

2. GitHub Action `release-binaries` builds and uploads platform assets + `checksums.txt`.
3. Users install/upgrade with `scripts/install.sh`, `scripts/install.ps1`, or `deeph update`.

### Dev mode (without installing binary)

```bash
go run ./cmd/deeph init
go run ./cmd/deeph validate
go run ./cmd/deeph run guide "teste"
```

## Config Example (`deeph.yaml`)

```yaml
version: 1
default_provider: local_mock
providers:
  - name: local_mock
    type: mock
    timeout_ms: 3000
```

## DeepSeek Provider (Docs-First)

`deepH` uses DeepSeek through the official Chat Completions API shape (`/chat/completions`).

Example provider config:

```yaml
version: 1
default_provider: deepseek
providers:
  - name: deepseek
    type: deepseek
    api_key_env: DEEPSEEK_API_KEY
    # optional (defaults to https://api.deepseek.com)
    # base_url: https://api.deepseek.com
    model: deepseek-chat
    timeout_ms: 30000
```

Set your key:

```bash
export DEEPSEEK_API_KEY="your_key_here"
```

Then create an agent using DeepSeek:

```bash
go run ./cmd/deeph agent create --provider deepseek --model deepseek-chat analyst
go run ./cmd/deeph run analyst "faça uma análise rápida"
```

Notes:

- `deepseek-chat` is the best default to start
- `deepseek-reasoner` can be used later for reasoning-heavy agents
- Function/tool-calling support varies by DeepSeek model/mode and is evolving; this MVP supports plain text completions and a basic local tool-calling loop (including replay of `reasoning_content` during tool loops)

## DeepSeek Tool Calling (MVP)

`deepH` now supports a basic tool-calling loop for provider type `deepseek`:

1. `deepH` sends `tools` to `/chat/completions`
2. DeepSeek may return `tool_calls`
3. `deepH` executes matching local skills
4. `deepH` sends `tool` messages back to DeepSeek
5. DeepSeek returns the final answer

Quick test:

```bash
go run ./cmd/deeph skill add file_read_range
go run ./cmd/deeph agent create --provider deepseek --model deepseek-chat reader
```

Edit `agents/reader.yaml` and set:

```yaml
skills:
  - file_read_range
```

Then run:

```bash
go run ./cmd/deeph trace reader "Use file_read_range to inspect only the relevant lines in README.md and summarize."
go run ./cmd/deeph run reader "Use file_read_range to inspect only the relevant lines in README.md and summarize."
```

The `run` output will print each executed `tool_call` (skill name, args, duration, and errors if any).

## Low-Token Context Strategy (MVP)

`deepH` now uses an internal **ContextBus + ContextCompiler** to reduce token waste:

- shared context state (`goal`, facts, events, artifacts)
- large tool outputs are summarized and stored as `artifacts`
- context is compiled with a token budget (heuristic token estimate)
- greedy selection by `score / token_cost`
- anti-loop guards for repeated tool calls
- weighted selection by `type + moment`
- range-first file reads (`file_read_range`) to avoid full-file replay

This keeps prompts cleaner than replaying full transcripts.

### Type + Moment Weighting

The `ContextCompiler` now scores candidates using:

- semantic type weight (`memory/fact`, `tool/error`, `summary/code`, `code/go`, etc.)
- current execution moment (`plan`, `discovery`, `tool_loop`, `synthesis`, `validate`)
- item moment (when/why that item was produced)
- recency and confidence
- token cost

Practical effect:

- `tool_loop` boosts tool errors/results and fresh artifacts
- `synthesis` boosts summaries and facts, reduces raw code priority
- `validate` boosts diagnostics

### Agent-Level Budget Tuning

You can tune budgets and anti-loop behavior using `metadata` in an agent config:

```yaml
metadata:
  context_max_input_tokens: "900"
  context_max_recent_events: "8"
  context_max_artifacts: "6"
  context_max_facts: "12"
  max_tool_rounds: "4"
  max_repeated_tool_calls: "2"
  # optional phase override (otherwise inferred by runtime):
  # context_moment: "tool_loop"
```

`trace` shows `context_budget`, and `run` shows estimated context token usage and dropped items.

`trace` also shows staged orchestration (`stage[n]`) and inferred typed handoffs between agents when you use a DAG-like spec such as `"a+b>c>d"`.
Execution uses a dependency-driven scheduler (`dag_channels`) so a downstream task can start as soon as its required handoff channels are ready (selective stage wait), instead of waiting for every task in the prior stage.

You can also add explicit non-adjacent dependencies in agent YAML (`depends_on`) to create a richer DAG without replaying raw outputs:

```yaml
name: reviewer
depends_on: [planner]
```

If multiple upstream agents write to the same target input port, `deepH` now merges handoff facts using a **type-aware merge policy** (e.g. summaries/diagnostics append a few recent items, text stays short, artifacts keep compact refs).

For tighter routing, you can constrain a target input port to specific upstream agents/ports with `depends_on_ports`:

```yaml
name: reviewer
depends_on_ports:
  brief: [planner.summary, coder.summary]
```

Selector format:

- `agent` (any output port from that agent that matches the input type)
- `agent.port` (specific upstream output port)

## Typed Runtime (Type Is Life)

`deepH` now includes a canonical runtime type system for **messages, code, tool results, artifacts, diagnostics and memory**.

Goals:

- typed agent-to-agent communication
- cleaner context compilation
- better cache keys (`kind + hash`)
- less token waste from raw text replay

### UX Philosophy

- beginner: inference/defaults
- advanced: explicit types via aliases (e.g. `go`, `ts`, `string`, `tools`)
- production: optional strict contracts in agent `io`

### Agent IO (Optional, Recommended)

```yaml
io:
  inputs:
    - name: source
      accepts: [code/go, code/ts, text/plain]
      required: true
      merge_policy: latest   # keep only the latest handoff for this port
      max_tokens: 120        # cap merged handoff fact size (approx)
    - name: ask
      accepts: [text/plain]
      merge_policy: append2  # keep up to 2 recent upstream items
  outputs:
    - name: answer
      produces: [text/markdown]
```

When `io.inputs`/`io.outputs` are present, `deepH` infers typed handoffs between stages (`"a+b>c"`), publishes compact `message/agent` + `artifact/ref` summaries into the shared context, and avoids replaying large raw outputs.

`merge_policy` is evaluated per target input port (`io.inputs[*]`) so different ports can use different token strategies:

- `latest`: overwrite with the most recent upstream handoff
- `append2` / `append3` / `append4`: keep a short merged list of recent upstreams
- `auto` (default): runtime picks a policy based on semantic type (`summary/*`, `diagnostic/*`, `artifact/*`, etc.)

`trace` will show the inferred handoffs after applying both `depends_on` / `depends_on_ports` and port merge settings.
Each handoff includes a logical `channel` id (`fromAgent.fromPort->toAgent.toPort#kind`) for easier debugging and future routing/scheduling policies.

### Channel Publish Budget (Low-Token Orchestration)

`deepH` publishes agent outputs to downstreams as **typed channels** and now supports a per-agent publish budget:

```yaml
metadata:
  publish_max_channels: "4"
  publish_max_channel_tokens: "240"
  # default is selective publish: if no downstream consumes this agent output,
  # it is not pushed into shared context (saves tokens)
  # publish_unconsumed_output: "true"
```

Behavior:

- channels are prioritized (required ports first, then semantic type value)
- budget is applied per agent run
- unconsumed outputs are skipped by default (still visible in `run` output)
- `run` shows `handoff_publish tokens=... dropped=...`

Validate contracts:

```bash
go run ./cmd/deeph validate
```

Explore all registered types:

```bash
go run ./cmd/deeph type list
go run ./cmd/deeph type list --category message
go run ./cmd/deeph type explain tools
go run ./cmd/deeph type explain code.go
```

## Agent Example (`agents/analyst.yaml`)

```yaml
name: analyst
description: User-defined agent example
provider: local_mock
model: mock-small
system_prompt: |
  You are an analyst. Be concise and explicit.
skills:
  - echo
```

## Built-in Skill Templates (catalog)

- `echo`
- `file_read`
- `file_read_range`
- `http_request`

These are templates the user can install into `skills/`.

## Roadmap (next)

- Real provider adapters (DeepSeek/OpenAI/Anthropic/Ollama)
- LLM tool-calling loop (today MVP supports static startup calls)
- Memory backends (sqlite first)
- Structured tracing output (JSON)
- Permission model for risky skills
