package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

type commandDoc struct {
	Path     string
	Category string
	Summary  string
	Usage    []string
	Examples []string
	Notes    []string
}

func cmdCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("command requires a subcommand: list or explain")
	}
	switch args[0] {
	case "list":
		return cmdCommandList(args[1:])
	case "explain":
		return cmdCommandExplain(args[1:])
	default:
		return fmt.Errorf("unknown command subcommand %q", args[0])
	}
}

func cmdCommandList(args []string) error {
	fs := flag.NewFlagSet("command list", flag.ContinueOnError)
	category := fs.String("category", "", "filter by category (workspace, execution, providers, ...)")
	jsonOut := fs.Bool("json", false, "print command dictionary as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	filter := strings.TrimSpace(strings.ToLower(*category))

	docs := commandDictionary()
	grouped := map[string][]commandDoc{}
	cats := make([]string, 0, 8)
	for _, d := range docs {
		cat := strings.TrimSpace(strings.ToLower(d.Category))
		if filter != "" && cat != filter {
			continue
		}
		if _, ok := grouped[cat]; !ok {
			cats = append(cats, cat)
		}
		grouped[cat] = append(grouped[cat], d)
	}
	if len(cats) == 0 {
		if filter == "" {
			fmt.Println("No commands registered.")
			return nil
		}
		return fmt.Errorf("no commands found for category %q", filter)
	}
	if *jsonOut {
		type commandListPayload struct {
			Category string       `json:"category,omitempty"`
			Commands []commandDoc `json:"commands"`
		}
		payload := commandListPayload{Category: filter}
		for _, cat := range cats {
			items := grouped[cat]
			sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
			payload.Commands = append(payload.Commands, items...)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	sort.Strings(cats)
	for i, cat := range cats {
		if i > 0 {
			fmt.Println("")
		}
		fmt.Printf("[%s]\n", cat)
		items := grouped[cat]
		sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
		for _, d := range items {
			fmt.Printf("- %s: %s\n", d.Path, d.Summary)
		}
	}
	return nil
}

func cmdCommandExplain(args []string) error {
	fs := flag.NewFlagSet("command explain", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print command entry as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return errors.New(`command explain requires "<command path>" (ex.: "provider add")`)
	}
	path := normalizeCommandPath(strings.Join(rest, " "))
	doc, ok := lookupCommandDoc(path)
	if !ok {
		return fmt.Errorf("unknown command path %q (tip: use `deeph command list`)", path)
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	}
	fmt.Printf("command: %s\n", doc.Path)
	fmt.Printf("category: %s\n", doc.Category)
	fmt.Printf("summary: %s\n", doc.Summary)
	if len(doc.Usage) > 0 {
		fmt.Println("usage:")
		for _, u := range doc.Usage {
			fmt.Printf("  - %s\n", u)
		}
	}
	if len(doc.Examples) > 0 {
		fmt.Println("examples:")
		for _, ex := range doc.Examples {
			fmt.Printf("  - %s\n", ex)
		}
	}
	if len(doc.Notes) > 0 {
		fmt.Println("notes:")
		for _, n := range doc.Notes {
			fmt.Printf("  - %s\n", n)
		}
	}
	return nil
}

func lookupCommandDoc(path string) (commandDoc, bool) {
	path = normalizeCommandPath(path)
	for _, d := range commandDictionary() {
		if normalizeCommandPath(d.Path) == path {
			return d, true
		}
	}
	return commandDoc{}, false
}

func normalizeCommandPath(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

func commandDictionary() []commandDoc {
	return []commandDoc{
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
			Path:     "trace",
			Category: "execution",
			Summary:  "Show execution plan, stages, channels and handoffs before running",
			Usage: []string{
				`deeph trace [--workspace DIR] [--json] [--multiverse N] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`,
			},
			Examples: []string{
				`deeph trace guide "teste"`,
				`deeph trace "planner+reader>coder>reviewer" "implemente X"`,
				`deeph trace --json "planner+coder>reviewer" "debug"`,
				`deeph trace --multiverse 0 @reviewpack "task"`,
			},
			Notes: []string{
				"`--json` is useful for automation, UI integration and logs.",
				"`--multiverse N` traces N universes (or all crew universes with `--multiverse 0`).",
				"Crew universes can declare `depends_on` to create multiverse channels (`u1.result -> u2.context`) shown in the trace.",
			},
		},
		{
			Path:     "run",
			Category: "execution",
			Summary:  "Run one or more agents with DAG/channels orchestration",
			Usage: []string{
				`deeph run [--workspace DIR] [--trace] [--coach=false] [--multiverse N] [--judge-agent SPEC] [--judge-max-output-chars N] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`,
			},
			Examples: []string{
				`deeph run guide "teste"`,
				`deeph run "planner+reader>coder>reviewer" "crie feature X"`,
				`deeph run --trace "a+b>c" "task"`,
				`deeph run --multiverse 0 @reviewpack "task"`,
				`deeph run --multiverse 0 --judge-agent guide @reviewpack "task"`,
			},
			Notes: []string{
				"Prints context, channel, handoff and tool budget metrics per agent.",
				"`--multiverse` runs multiple universes and prints branch outputs plus a sink-output fingerprint consensus.",
				"Crew universes with `depends_on` run with a multiverse DAG/channels scheduler and can contribute compact handoffs to downstream universes.",
				"`--judge-agent` runs a follow-up comparison agent over multiverse branch summaries (reconcile/judge step).",
				"Judge output is parsed when possible (JSON or labeled sections) to show `winner`, `rationale`, `risks` and `follow_up` clearly.",
				"Shows occasional local semantic hints while waiting (disable with `--coach=false` or `DEEPH_COACH=0`).",
				"Coach also learns local command transitions (ex.: run -> trace) to suggest likely next steps without using LLM tokens.",
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
				"deeph agent create reviewer --provider deepseek --model deepseek-chat",
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
				"deeph provider add deepseek --set-default",
				"deeph provider add deepseek --name deepseek_prod --model deepseek-chat --timeout-ms 30000",
				"deeph provider add deepseek --force --api-key-env DEEPSEEK_API_KEY",
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
}
