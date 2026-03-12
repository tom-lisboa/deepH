package commanddoc

import (
	"sort"
	"strings"
)

type Doc struct {
	Path     string   `json:"path"`
	Category string   `json:"category"`
	Summary  string   `json:"summary"`
	Usage    []string `json:"usage,omitempty"`
	Examples []string `json:"examples,omitempty"`
	Notes    []string `json:"notes,omitempty"`
}

func NormalizePath(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

func Dictionary() []Doc {
	out := make([]Doc, 0, len(docs))
	for _, d := range docs {
		out = append(out, d)
	}
	return out
}

func Lookup(path string) (Doc, bool) {
	path = NormalizePath(path)
	for _, d := range docs {
		if NormalizePath(d.Path) == path {
			return d, true
		}
	}
	return Doc{}, false
}

func Search(query, category string, limit int) []Doc {
	query = NormalizePath(query)
	category = NormalizePath(category)
	if limit <= 0 {
		limit = 5
	}

	type match struct {
		doc   Doc
		score int
	}
	matches := make([]match, 0, len(docs))
	for _, d := range docs {
		docCat := NormalizePath(d.Category)
		if category != "" && docCat != category {
			continue
		}
		score := 0
		docPath := NormalizePath(d.Path)
		docSummary := NormalizePath(d.Summary)
		switch {
		case query == "":
			score = 1
		case docPath == query:
			score = 100
		case strings.HasPrefix(docPath, query):
			score = 80
		case strings.Contains(docPath, query):
			score = 60
		case strings.Contains(docSummary, query):
			score = 40
		case strings.Contains(docCat, query):
			score = 20
		}
		if score == 0 {
			continue
		}
		matches = append(matches, match{doc: d, score: score})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].doc.Path < matches[j].doc.Path
		}
		return matches[i].score > matches[j].score
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}
	out := make([]Doc, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.doc)
	}
	return out
}

var docs = []Doc{
	{
		Path:     "help",
		Category: "meta",
		Summary:  "Show CLI usage and available top-level commands",
		Usage: []string{
			"deeph help",
			"deeph --help",
			"deeph -h",
		},
		Notes: []string{
			"Also shown when no command is provided.",
		},
	},
	{
		Path:     "version",
		Category: "meta",
		Summary:  "Show installed deepH version/build details",
		Usage: []string{
			"deeph version [--json]",
			"deeph --version",
			"deeph -v",
		},
		Examples: []string{
			"deeph version",
			"deeph version --json",
		},
		Notes: []string{
			"Release binaries embed tag/commit/build date during CI.",
		},
	},
	{
		Path:     "init",
		Category: "workspace",
		Summary:  "Initialize a deepH workspace (deeph.yaml, folders, examples)",
		Usage: []string{
			"deeph init [--workspace DIR]",
		},
		Examples: []string{
			"deeph init",
			"deeph init --workspace /tmp/my-deeph",
		},
	},
	{
		Path:     "quickstart",
		Category: "workspace",
		Summary:  "One-command starter setup (init workspace + starter agent + optional echo skill/provider)",
		Usage: []string{
			"deeph quickstart [--workspace DIR] [--agent NAME] [--provider NAME] [--model MODEL] [--with-echo] [--deepseek] [--force]",
		},
		Examples: []string{
			"deeph quickstart",
			"deeph quickstart --workspace ./myproj --deepseek",
			"deeph quickstart --agent planner --provider deepseek --model deepseek-chat",
		},
		Notes: []string{
			"Creates a starter agent template and validates the workspace immediately.",
			"With `--deepseek`, scaffolds provider config and sets it as default.",
		},
	},
	{
		Path:     "studio",
		Category: "workspace",
		Summary:  "Interactive menu mode for onboarding and common workflows",
		Usage: []string{
			"deeph studio [--workspace DIR]",
		},
		Examples: []string{
			"deeph studio",
			"deeph studio --workspace ./myproj",
		},
		Notes: []string{
			"Guides users through quickstart, provider setup, agent creation, run and chat.",
		},
	},
	{
		Path:     "update",
		Category: "workspace",
		Summary:  "Download and install the latest (or specific) GitHub release binary for this platform",
		Usage: []string{
			"deeph update [--owner NAME] [--repo NAME] [--tag latest|vX.Y.Z] [--check]",
		},
		Examples: []string{
			"deeph update",
			"deeph update --check",
			"deeph update --tag v0.1.0",
		},
		Notes: []string{
			"Defaults to GitHub release owner/repo tom-lisboa/deepH.",
			"On Windows, update is downloaded as deeph.new.exe and replacement steps are printed.",
		},
	},
	{
		Path:     "validate",
		Category: "workspace",
		Summary:  "Validate deeph.yaml, agents and skills YAML files",
		Usage: []string{
			"deeph validate [--workspace DIR]",
		},
		Examples: []string{
			"deeph validate",
			"deeph validate --workspace ./myproj",
		},
	},
	{
		Path:     "review",
		Category: "execution",
		Summary:  "Review the current git diff with a compact, Go-aware working set",
		Usage: []string{
			`deeph review [--workspace DIR] [--spec SPEC] [--base REF] [--trace] [--coach=false] [--json] [focus]`,
		},
		Examples: []string{
			"deeph review",
			`deeph review --trace "focus on regressions and missing tests"`,
			"deeph review --spec @reviewflow",
			"deeph review --spec reviewer",
			"deeph review --json",
		},
		Notes: []string{
			"Builds a compact review brief from the current git diff plus a Go-aware working set (same package, tests, local imports, reverse imports).",
			"`--json` prints the generated scope and review input payload instead of running the agent.",
			"When `crews/reviewflow.yaml` exists, defaults to `@reviewflow`; otherwise falls back to a builtin multiverse review flow rooted at `reviewer` or `guide`.",
			"Passing `--spec SPEC` keeps the review on that explicit agent or crew instead of auto-selecting the builtin flow.",
		},
	},
	{
		Path:     "diagnose",
		Category: "execution",
		Summary:  "Analyze an error, panic, stack trace, or failing output against a compact workspace scope",
		Usage: []string{
			`deeph diagnose [--workspace DIR] [--spec SPEC] [--base REF] [--trace] [--coach=false] [--fix] [--yes] [--json] [--file PATH] [issue]`,
		},
		Examples: []string{
			`deeph diagnose "panic: nil pointer dereference in cmd/main.go:42"`,
			`go test ./... 2>&1 | deeph diagnose`,
			`deeph diagnose --file /tmp/build.log`,
			`deeph diagnose --fix "panic: nil pointer dereference in cmd/main.go:42"`,
		},
		Notes: []string{
			"Builds a compact workspace scope from referenced files in the error text plus a small same-package expansion.",
			"If `diagnoser` exists, it is the default agent; otherwise falls back to `reviewer` and then `guide`.",
			"`--json` prints the generated diagnose scope and payload instead of running the agent.",
			"`--fix` proposes a follow-up `deeph edit`; add `--yes` to run that edit immediately after diagnosis.",
		},
	},
	{
		Path:     "edit",
		Category: "execution",
		Summary:  "Run the default `coder` agent with a focused code-editing task",
		Usage: []string{
			`deeph edit [--workspace DIR] [--trace] [--coach=false] [task]`,
		},
		Examples: []string{
			`deeph edit "analyze cmd/main.go and add two helper functions"`,
			`deeph edit --trace "refactor the handler and keep behavior unchanged"`,
		},
		Notes: []string{
			"Thin shortcut over `deeph run coder ...` for the common editing path.",
			"Best used after `deeph quickstart`, which scaffolds the default `coder` agent and file skills.",
		},
	},
	{
		Path:     "trace",
		Category: "execution",
		Summary:  "Show execution plan, stages, channels and handoffs before running",
		Usage: []string{
			`deeph trace [--workspace DIR] [--json] [--multiverse N] [--daemon=true|false] [--daemon-target HOST:PORT] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`,
		},
		Examples: []string{
			`deeph trace guide "teste"`,
			`deeph trace "planner+reader>coder>reviewer" "implemente X"`,
			`deeph trace --json "planner+coder>reviewer" "debug"`,
			`deeph trace --multiverse 0 @reviewpack "task"`,
			`deeph trace --daemon @reviewpack "task"`,
			`deeph trace --daemon=false guide "task"`,
		},
		Notes: []string{
			"`--json` is useful for automation, UI integration and logs.",
			"`--multiverse N` traces N universes (or all crew universes with `--multiverse 0`).",
			"Crew universes can declare `depends_on` to create multiverse channels (`u1.result -> u2.context`) shown in the trace.",
			"`--daemon` defaults to `true` and forwards the request to a local `deephd` gRPC daemon.",
			"If daemon is unavailable, deepH tries to start it and falls back to local execution when needed.",
			"Use `--daemon=false` to force local in-process execution.",
		},
	},
	{
		Path:     "run",
		Category: "execution",
		Summary:  "Run one or more agents with DAG/channels orchestration",
		Usage: []string{
			`deeph run [--workspace DIR] [--trace] [--coach=false] [--multiverse N] [--judge-agent SPEC] [--judge-max-output-chars N] [--daemon=true|false] [--daemon-target HOST:PORT] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`,
		},
		Examples: []string{
			`deeph run guide "teste"`,
			`deeph run "planner+reader>coder>reviewer" "crie feature X"`,
			`deeph run --trace "a+b>c" "task"`,
			`deeph run --multiverse 0 @reviewpack "task"`,
			`deeph run --multiverse 0 --judge-agent guide @reviewpack "task"`,
			`deeph run --daemon guide "task"`,
			`deeph run --daemon=false guide "task"`,
		},
		Notes: []string{
			"Prints context, channel, handoff and tool budget metrics per agent.",
			"`--multiverse` runs multiple universes and prints branch outputs plus a sink-output fingerprint consensus.",
			"Crew universes with `depends_on` run with a multiverse DAG/channels scheduler and can contribute compact handoffs to downstream universes.",
			"`--judge-agent` runs a follow-up comparison agent over multiverse branch summaries (reconcile/judge step).",
			"Judge output is parsed when possible (JSON or labeled sections) to show `winner`, `rationale`, `risks` and `follow_up` clearly.",
			"`--daemon` defaults to `true` and forwards the run to a local `deephd` gRPC daemon.",
			"If daemon is unavailable, deepH tries to start it and falls back to local execution when needed.",
			"Use `--daemon=false` to force local in-process execution.",
			"Shows occasional local semantic hints while waiting (disable with `--coach=false` or `DEEPH_COACH=0`).",
			"Coach also learns local command transitions (ex.: run -> trace) to suggest likely next steps without using LLM tokens.",
		},
	},
	{
		Path:     "daemon serve",
		Category: "execution",
		Summary:  "Run the local `deephd` gRPC daemon in foreground",
		Usage: []string{
			"deeph daemon serve [--target HOST:PORT]",
		},
		Examples: []string{
			"deeph daemon serve",
			"deeph daemon serve --target 127.0.0.1:7788",
		},
		Notes: []string{
			"Serves Ping/Trace/Run/Shutdown gRPC methods over HTTP/2 with protobuf structs.",
			"Use `deeph run --daemon ...` or `deeph trace --daemon ...` to send requests to this process.",
		},
	},
	{
		Path:     "daemon start",
		Category: "execution",
		Summary:  "Start `deephd` in background and wait until reachable",
		Usage: []string{
			"deeph daemon start [--target HOST:PORT]",
		},
		Examples: []string{
			"deeph daemon start",
			"deeph daemon start --target 127.0.0.1:7788",
		},
		Notes: []string{
			"Starts a detached `deeph daemon serve` process and writes logs to a temp file.",
		},
	},
	{
		Path:     "daemon status",
		Category: "execution",
		Summary:  "Check whether `deephd` is reachable",
		Usage: []string{
			"deeph daemon status [--target HOST:PORT]",
		},
		Examples: []string{
			"deeph daemon status",
		},
	},
	{
		Path:     "daemon stop",
		Category: "execution",
		Summary:  "Request graceful shutdown of `deephd`",
		Usage: []string{
			"deeph daemon stop [--target HOST:PORT]",
		},
		Examples: []string{
			"deeph daemon stop",
		},
	},
	{
		Path:     "chat",
		Category: "execution",
		Summary:  "Start a fluid terminal chat session with one agent or a multi-agent spec",
		Usage: []string{
			`deeph chat [--workspace DIR] [--session ID] [--history-turns N] [--history-tokens N] [--trace] [--coach=false] "<agent|a+b|a>b|a+b>c>"`,
			"deeph chat [--workspace DIR] --session ID",
		},
		Examples: []string{
			"deeph chat guide",
			`deeph chat "planner+reader>coder>reviewer"`,
			"deeph chat --session feat-login",
			"deeph chat --session feat-login guide",
		},
		Notes: []string{
			"Persists chat history in sessions/<id>.jsonl and sessions/<id>.meta.json.",
			"Supports slash commands: /help, /history, /trace, /exit.",
			"Shows occasional local hints while waiting (disable with `--coach=false` or `DEEPH_COACH=0`).",
			"Coach can learn local follow-up patterns (ex.: chat -> session show) from your usage in the workspace.",
		},
	},
	{
		Path:     "gws",
		Category: "integrations",
		Summary:  "Run Google Workspace CLI (`gws`) via a safe deeph wrapper (no shell, timeout and output cap)",
		Usage: []string{
			"deeph gws [--yes|--allow-mutate] [--json] [--timeout 30s] [--max-output-bytes N] [--bin gws] [--allow-any-root] <gws args...>",
			"deeph gws -- <raw gws args...>",
		},
		Examples: []string{
			"deeph gws drive files list --page-size 5",
			"deeph gws --json gmail users.messages.list --user me --max-results 10",
			"deeph gws --allow-mutate drive files delete --file-id 123",
		},
		Notes: []string{
			"Executes `gws` with `exec.Command` (no shell expansion).",
			"Unknown root groups are blocked by default; use `--allow-any-root` to bypass.",
			"Commands that look mutating require `--allow-mutate` (or `--yes`).",
			"Use `--` when raw gws flags conflict with wrapper flags like `--timeout`.",
		},
	},
	{
		Path:     "crew list",
		Category: "crews",
		Summary:  "List crew presets in crews/ (agent-spec aliases, optional multiverse universes)",
		Usage: []string{
			"deeph crew list [--workspace DIR]",
		},
		Examples: []string{
			"deeph crew list",
		},
	},
	{
		Path:     "crew show",
		Category: "crews",
		Summary:  "Show one crew preset, base spec and universe variants",
		Usage: []string{
			"deeph crew show [--workspace DIR] <name>",
		},
		Examples: []string{
			"deeph crew show reviewpack",
		},
		Notes: []string{
			"Use `@name` or `crew:name` in `run/trace` to execute the crew.",
			"If `universes` are defined, combine with `--multiverse` to run branch presets.",
			"Each universe can declare `depends_on`, `input_port`, `output_port`, `output_kind`, `merge_policy` and `handoff_max_chars`.",
		},
	},
	{
		Path:     "agent create",
		Category: "agents",
		Summary:  "Create a user-defined agent YAML template in agents/",
		Usage: []string{
			"deeph agent create [--workspace DIR] [--force] [--provider NAME] [--model MODEL] <name>",
		},
		Examples: []string{
			"deeph agent create analyst",
			"deeph agent create --provider deepseek --model deepseek-chat reviewer",
		},
		Notes: []string{
			"Template includes examples of typed ports, routing, budgets and tool settings.",
		},
	},
	{
		Path:     "skill list",
		Category: "skills",
		Summary:  "List built-in skill templates available to install",
		Usage: []string{
			"deeph skill list",
		},
		Examples: []string{
			"deeph skill list",
		},
	},
	{
		Path:     "skill add",
		Category: "skills",
		Summary:  "Install a skill template YAML into skills/",
		Usage: []string{
			"deeph skill add [--workspace DIR] [--force] <name>",
		},
		Examples: []string{
			"deeph skill add echo",
			"deeph skill add file_read_range",
		},
	},
	{
		Path:     "provider list",
		Category: "providers",
		Summary:  "List providers configured in deeph.yaml",
		Usage: []string{
			"deeph provider list [--workspace DIR]",
		},
		Examples: []string{
			"deeph provider list",
		},
	},
	{
		Path:     "provider add",
		Category: "providers",
		Summary:  "Add or update a provider scaffold in deeph.yaml (DeepSeek-focused)",
		Usage: []string{
			"deeph provider add [--workspace DIR] [--name NAME] [--model MODEL] [--set-default] [--force] deepseek",
		},
		Examples: []string{
			"deeph provider add --set-default deepseek",
			"deeph provider add --name deepseek_prod --model deepseek-chat --timeout-ms 30000 deepseek",
			"deeph provider add --force --api-key-env DEEPSEEK_API_KEY deepseek",
		},
		Notes: []string{
			"Scaffolds OpenAI-compatible DeepSeek config with sane defaults.",
		},
	},
	{
		Path:     "kit list",
		Category: "kits",
		Summary:  "List installable starter kits (agents + crews + skills templates)",
		Usage: []string{
			"deeph kit list [--workspace DIR]",
		},
		Examples: []string{
			"deeph kit list",
		},
		Notes: []string{
			"Kits are local starter bundles that accelerate setup by name.",
		},
	},
	{
		Path:     "kit add",
		Category: "kits",
		Summary:  "Install a starter kit by name (skills, agents, crews and provider scaffold)",
		Usage: []string{
			"deeph kit add [--workspace DIR] [--force] [--provider-name NAME] [--model MODEL] [--set-default-provider] [--skip-provider] <name|git-url[#manifest.yaml]>",
		},
		Examples: []string{
			"deeph kit add hello-next-tailwind",
			"deeph kit add hello-next-shadcn",
			"deeph kit add crud-next-multiverse",
			"deeph kit add crud-next-multiverse --provider-name deepseek --model deepseek-chat",
			"deeph kit add https://github.com/acme/deeph-kits.git#kits/next/kit.yaml",
		},
		Notes: []string{
			"Installs required skill templates automatically and writes agents/crews YAML files.",
			"For DeepSeek-focused kits, provider config is scaffolded unless `--skip-provider` is used.",
			"Existing files are preserved by default; use `--force` to overwrite changed templates.",
			"Git URL mode expects a manifest file (`deeph-kit.yaml` or `kit.yaml`) in the repo root unless `#path/to/manifest.yaml` is provided.",
		},
	},
	{
		Path:     "crud init",
		Category: "crud",
		Summary:  "Initialize the opinionated CRUD starter and save a workspace CRUD profile",
		Usage: []string{
			"deeph crud init [--workspace DIR] [--force] [--provider-name NAME] [--model MODEL] [--set-default-provider] [--skip-provider] [--mode backend|fullstack] [--db-kind relational|document] [--db postgres|mongodb] [--entity NAME] [--fields nome:text,cidade:text] [--containers=true|false]",
		},
		Examples: []string{
			"deeph crud init",
			"deeph crud init --workspace ./futebol",
			"deeph crud init --workspace ./futebol --mode fullstack --db-kind relational --db postgres --entity players --fields nome:text,posicao:text,time_id:int",
		},
		Notes: []string{
			"Bootstraps the workspace if needed and installs the `crud-next-multiverse` kit.",
			"Defaults to Go + Next.js + Postgres as the recommended CRUD stack.",
			"In interactive terminals, asks about backend-only vs fullstack, relational vs document, entity, fields and local containers, then saves the answers in `.deeph/crud.json`.",
		},
	},
	{
		Path:     "crud prompt",
		Category: "crud",
		Summary:  "Print the opinionated CRUD prompt that will be sent to the multiverse crew",
		Usage: []string{
			"deeph crud prompt [--workspace DIR] [--entity NAME] [--fields nome:text,cidade:text] [--db-kind relational|document] [--db postgres|mongodb] [--backend go] [--frontend next] [--backend-only] [--mode backend|fullstack] [--containers=true|false]",
		},
		Examples: []string{
			"deeph crud prompt --entity players --fields nome:text,cidade:text",
			"deeph crud prompt --entity teams --fields nome:text,cidade:text,pais:text --backend-only",
		},
	},
	{
		Path:     "crud trace",
		Category: "crud",
		Summary:  "Trace the CRUD multiverse run using the opinionated Go + Next + Postgres defaults",
		Usage: []string{
			"deeph crud trace [--workspace DIR] [--entity NAME] [--fields nome:text,cidade:text] [--db-kind relational|document] [--db postgres|mongodb] [--backend go] [--frontend next] [--backend-only] [--mode backend|fullstack] [--containers=true|false]",
		},
		Examples: []string{
			"deeph crud trace --entity people --fields nome:text,cidade:text",
			"deeph crud trace --workspace ./futebol --entity players --fields nome:text,posicao:text,time_id:int",
		},
		Notes: []string{
			"Chooses the CRUD crew from the saved CRUD profile or from explicit flags (`crud_backend_relational`, `crud_fullstack_relational`, `crud_backend_document`, `crud_fullstack_document`).",
			"`--mode` lets the user switch between backend-only and fullstack without rewriting the saved profile by hand.",
		},
	},
	{
		Path:     "crud run",
		Category: "crud",
		Summary:  "Run the CRUD multiverse crew with the opinionated Go + Next + Postgres defaults",
		Usage: []string{
			"deeph crud run [--workspace DIR] [--entity NAME] [--fields nome:text,cidade:text] [--db-kind relational|document] [--db postgres|mongodb] [--backend go] [--frontend next] [--backend-only] [--mode backend|fullstack] [--containers=true|false]",
		},
		Examples: []string{
			"deeph crud run --entity people --fields nome:text,cidade:text",
			"deeph crud run --workspace ./futebol --entity players --fields nome:text,posicao:text,time_id:int",
		},
		Notes: []string{
			"Chooses the CRUD crew from the saved CRUD profile or from explicit flags (`crud_backend_relational`, `crud_fullstack_relational`, `crud_backend_document`, `crud_fullstack_document`).",
			"Use `--backend-only` when you want just the Go API and infra without generating Next.js pages.",
			"`--containers=false` lets the user ask for local startup without Docker Compose in the generated prompt.",
		},
	},
	{
		Path:     "crud up",
		Category: "crud",
		Summary:  "Start the generated CRUD containers with Docker Compose",
		Usage: []string{
			"deeph crud up [--workspace DIR] [--compose-file FILE] [--build=true|false] [--detach=true|false] [--wait 45s] [--base-url URL]",
		},
		Examples: []string{
			"deeph crud up",
			"deeph crud up --workspace ./futebol",
			"deeph crud up --workspace ./futebol --wait 60s",
		},
		Notes: []string{
			"Auto-detects `docker-compose.yml`, `docker-compose.yaml`, `compose.yml` or `compose.yaml` inside the workspace unless `--compose-file` is provided.",
			"Prefers `docker compose`, then falls back to `docker-compose` when available.",
		},
	},
	{
		Path:     "crud smoke",
		Category: "crud",
		Summary:  "Run the generated CRUD smoke test or fall back to a built-in HTTP CRUD probe",
		Usage: []string{
			"deeph crud smoke [--workspace DIR] [--compose-file FILE] [--base-url URL] [--route-base /people] [--entity NAME] [--fields nome:text,cidade:text] [--no-script] [--timeout 45s]",
		},
		Examples: []string{
			"deeph crud smoke",
			"deeph crud smoke --workspace ./futebol --base-url http://127.0.0.1:8080",
			"deeph crud smoke --workspace ./futebol --no-script --entity players --fields nome:text,posicao:text",
		},
		Notes: []string{
			"Runs `scripts/smoke.sh` or `scripts/smoke.ps1` when the generated project provides them.",
			"When no script is found, tries a built-in create/list/read/update/delete flow using the saved CRUD profile.",
		},
	},
	{
		Path:     "crud down",
		Category: "crud",
		Summary:  "Stop the generated CRUD containers with Docker Compose",
		Usage: []string{
			"deeph crud down [--workspace DIR] [--compose-file FILE] [--volumes]",
		},
		Examples: []string{
			"deeph crud down",
			"deeph crud down --workspace ./futebol",
			"deeph crud down --workspace ./futebol --volumes",
		},
		Notes: []string{
			"Stops the detected compose stack and removes orphan containers.",
			"Use `--volumes` only when you intentionally want to drop local database volumes.",
		},
	},
	{
		Path:     "coach stats",
		Category: "coach",
		Summary:  "Inspect local coach learning state (hints, command counts, transitions, port hotspots)",
		Usage: []string{
			"deeph coach stats [--workspace DIR] [--top N] [--scope SPEC] [--kind KIND] [--json]",
		},
		Examples: []string{
			"deeph coach stats",
			"deeph coach stats --top 20",
			`deeph coach stats --scope "planner+reader>coder>reviewer"`,
			"deeph coach stats --kind handoff_drop",
			"deeph coach stats --json",
		},
		Notes: []string{
			"Reads workspace-local .deeph/coach_state.json.",
			"No LLM calls are used; this is purely local usage telemetry for hints.",
			"Includes `port_signals` counters used by post-run optimization hints.",
			"`--scope` inspects workflow-specific transitions/port signals keyed by agent spec.",
			"`--kind` filters `port_signals` (ex.: handoff_drop, context_channel_drop, context_drop).",
		},
	},
	{
		Path:     "coach reset",
		Category: "coach",
		Summary:  "Reset local coach learning state for the workspace (full or partial)",
		Usage: []string{
			"deeph coach reset [--workspace DIR] [--all] [--hints] [--transitions] [--commands] [--ports] --yes",
		},
		Examples: []string{
			"deeph coach reset --yes",
			"deeph coach reset --all --yes",
			"deeph coach reset --ports --yes",
			"deeph coach reset --hints --transitions --yes",
		},
		Notes: []string{
			"Without partial flags, deletes .deeph/coach_state.json in the selected workspace.",
			"With partial flags, preserves the file and clears only selected sections.",
		},
	},
	{
		Path:     "session list",
		Category: "sessions",
		Summary:  "List saved chat sessions in the workspace",
		Usage: []string{
			"deeph session list [--workspace DIR]",
		},
		Examples: []string{
			"deeph session list",
		},
	},
	{
		Path:     "session show",
		Category: "sessions",
		Summary:  "Show persisted chat session metadata and recent entries",
		Usage: []string{
			"deeph session show [--workspace DIR] [--tail N] <id>",
		},
		Examples: []string{
			"deeph session show feat-login",
			"deeph session show --tail 50 feat-login",
		},
		Notes: []string{
			"Reads sessions/<id>.meta.json and sessions/<id>.jsonl.",
		},
	},
	{
		Path:     "type list",
		Category: "types",
		Summary:  "List semantic runtime types (code/go, summary/code, artifact/ref, ...)",
		Usage: []string{
			"deeph type list [--category CAT] [--json]",
		},
		Examples: []string{
			"deeph type list",
			"deeph type list --category code",
			"deeph type list --json",
		},
		Notes: []string{
			"`--json` exports the type dictionary for tooling/docs.",
		},
	},
	{
		Path:     "type explain",
		Category: "types",
		Summary:  "Explain one semantic type or alias",
		Usage: []string{
			"deeph type explain [--json] <kind|alias>",
		},
		Examples: []string{
			"deeph type explain code/go",
			"deeph type explain CODE.GO",
			"deeph type explain string",
			"deeph type explain --json code/go",
		},
	},
	{
		Path:     "command list",
		Category: "meta",
		Summary:  "List all deeph commands by category (command dictionary index)",
		Usage: []string{
			"deeph command list [--category CAT] [--json]",
		},
		Examples: []string{
			"deeph command list",
			"deeph command list --category execution",
			"deeph command list --json",
		},
		Notes: []string{
			"`--json` emits the command dictionary for docs/automation tooling.",
		},
	},
	{
		Path:     "command explain",
		Category: "meta",
		Summary:  "Explain one command path from the command dictionary",
		Usage: []string{
			`deeph command explain [--json] "<command path>"`,
		},
		Examples: []string{
			`deeph command explain "provider add"`,
			`deeph command explain "trace"`,
			`deeph command explain --json "provider add"`,
		},
		Notes: []string{
			"`--json` emits a single command entry object.",
		},
	},
}
