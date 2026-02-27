package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"deeph/internal/catalog"
	"deeph/internal/project"
	"deeph/internal/runtime"
	"deeph/internal/scaffold"
	"deeph/internal/typesys"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		if isInteractiveTerminal(os.Stdin) {
			return cmdStudio([]string{})
		}
		printUsage()
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "init":
		return cmdInit(args[1:])
	case "quickstart":
		return cmdQuickstart(args[1:])
	case "studio":
		return cmdStudio(args[1:])
	case "update":
		return cmdUpdate(args[1:])
	case "validate":
		return cmdValidate(args[1:])
	case "trace":
		return cmdTrace(args[1:])
	case "run":
		return cmdRun(args[1:])
	case "chat":
		return cmdChat(args[1:])
	case "session":
		return cmdSession(args[1:])
	case "crew":
		return cmdCrew(args[1:])
	case "skill":
		return cmdSkill(args[1:])
	case "agent":
		return cmdAgent(args[1:])
	case "provider":
		return cmdProvider(args[1:])
	case "kit":
		return cmdKit(args[1:])
	case "coach":
		return cmdCoach(args[1:])
	case "command":
		return cmdCommand(args[1:])
	case "type":
		return cmdType(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Println("deepH - lightweight agent runtime in Go")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  deeph init [--workspace DIR]")
	fmt.Println("  deeph quickstart [--workspace DIR] [--agent NAME] [--provider NAME] [--model MODEL] [--with-echo] [--deepseek] [--force]")
	fmt.Println("  deeph studio [--workspace DIR]")
	fmt.Println("  deeph update [--owner NAME] [--repo NAME] [--tag latest|vX.Y.Z] [--check]")
	fmt.Println("  deeph validate [--workspace DIR]")
	fmt.Println(`  deeph trace [--workspace DIR] [--json] [--multiverse N] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`)
	fmt.Println(`  deeph run [--workspace DIR] [--trace] [--coach=false] [--multiverse N] [--judge-agent SPEC] [--judge-max-output-chars N] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`)
	fmt.Println(`  deeph chat [--workspace DIR] [--session ID] [--history-turns N] [--history-tokens N] [--trace] [--coach=false] "<agent|a+b|a>b|a+b>c>"`)
	fmt.Println("  deeph session list [--workspace DIR]")
	fmt.Println("  deeph session show [--workspace DIR] [--tail N] <id>")
	fmt.Println("  deeph crew list [--workspace DIR]")
	fmt.Println("  deeph crew show [--workspace DIR] <name>")
	fmt.Println("  deeph agent create [--workspace DIR] [--force] [--provider NAME] [--model MODEL] <name>")
	fmt.Println("  deeph provider list [--workspace DIR]")
	fmt.Println("  deeph provider add [--workspace DIR] [--name NAME] [--model MODEL] [--set-default] [--force] deepseek")
	fmt.Println("  deeph kit list [--workspace DIR]")
	fmt.Println("  deeph kit add [--workspace DIR] [--force] [--provider-name NAME] [--model MODEL] [--set-default-provider] [--skip-provider] <name|git-url[#manifest.yaml]>")
	fmt.Println("  deeph coach stats [--workspace DIR] [--top N] [--scope SPEC] [--kind KIND] [--json]")
	fmt.Println("  deeph coach reset [--workspace DIR] [--all] [--hints] [--transitions] [--commands] [--ports] --yes")
	fmt.Println("  deeph command list [--category CAT] [--json]")
	fmt.Println(`  deeph command explain [--json] "<command path>"`)
	fmt.Println("  deeph skill list")
	fmt.Println("  deeph skill add [--workspace DIR] [--force] <name>")
	fmt.Println("  deeph type list [--category CAT] [--json]")
	fmt.Println("  deeph type explain [--json] <kind|alias>")
}

func isInteractiveTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	abs, _ := filepath.Abs(*workspace)
	if err := scaffold.InitWorkspace(abs); err != nil {
		return err
	}
	fmt.Printf("Initialized deepH workspace at %s\n", abs)
	fmt.Println("Next steps:")
	fmt.Println("  1. deeph quickstart --workspace . --deepseek   (recommended)")
	fmt.Println("  2. set/export DEEPSEEK_API_KEY=\"sk-...\"")
	fmt.Println("  3. deeph run guide \"teste\"")
	fmt.Println("  4. or manual mode: skill add + agent create + validate")
	fmt.Println("  5. deeph kit list   (optional prebuilt starter kits)")
	return nil
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, abs, verr, err := loadAndValidate(*workspace)
	if err != nil {
		return err
	}
	printValidation(verr)
	if verr != nil && verr.HasErrors() {
		return verr
	}
	fmt.Printf("Validation OK (%d agent(s), %d skill(s), %d provider(s)) in %s\n", len(p.Agents), len(p.Skills), len(p.Root.Providers), abs)
	return nil
}

func cmdTrace(args []string) error {
	fs := flag.NewFlagSet("trace", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	jsonOut := fs.Bool("json", false, "print execution plan trace as JSON")
	multiverse := fs.Int("multiverse", 1, "number of universes to trace (0 = all crew universes)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return errors.New("trace requires <agent|a+b|@crew|crew:name>")
	}
	agentSpecArg := rest[0]
	input := strings.Join(rest[1:], " ")

	p, abs, verr, err := loadAndValidate(*workspace)
	if err != nil {
		return err
	}
	printValidation(verr)
	if verr != nil && verr.HasErrors() {
		return verr
	}
	eng, err := runtime.New(abs, p)
	if err != nil {
		return err
	}
	resolvedSpec, crew, err := resolveAgentSpecOrCrew(abs, agentSpecArg)
	if err != nil {
		return err
	}
	universes, err := buildMultiverseUniverses(abs, agentSpecArg, resolvedSpec, input, *multiverse, crew)
	if err != nil {
		return err
	}
	if len(universes) > 1 {
		recordCoachCommandTransition(abs, "trace", resolvedSpec)
		branches, mvPlan, err := traceMultiverse(context.Background(), abs, p, universes)
		if err != nil {
			return err
		}
		if *jsonOut {
			payload := struct {
				Workspace        string                      `json:"workspace"`
				Scheduler        string                      `json:"scheduler"`
				Source           string                      `json:"source"`
				UniverseHandoffs []multiverseUniverseHandoff `json:"universe_handoffs,omitempty"`
				Branches         []multiverseTraceBranch     `json:"branches"`
			}{
				Workspace:        abs,
				Scheduler:        mvPlan.Scheduler,
				Source:           agentSpecArg,
				UniverseHandoffs: mvPlan.Handoffs,
				Branches:         branches,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}
		printMultiverseTraceText(abs, agentSpecArg, mvPlan, branches)
		return nil
	}
	agentSpec := resolvedSpec
	plan, _, err := eng.PlanSpec(context.Background(), agentSpec, input)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "trace", agentSpec)
	if *jsonOut {
		payload := struct {
			Workspace string                `json:"workspace"`
			Scheduler string                `json:"scheduler"`
			Plan      runtime.ExecutionPlan `json:"plan"`
		}{
			Workspace: abs,
			Scheduler: "dag_channels",
			Plan:      plan,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	fmt.Printf("Trace (%s)\n", abs)
	fmt.Printf("  created_at: %s\n", plan.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  parallel: %v\n", plan.Parallel)
	fmt.Println("  scheduler: dag_channels (dependency-driven, selective stage wait)")
	if strings.TrimSpace(plan.Spec) != "" {
		fmt.Printf("  spec: %q\n", plan.Spec)
	}
	fmt.Printf("  input: %q\n", plan.Input)
	if len(plan.Stages) > 1 {
		for _, s := range plan.Stages {
			fmt.Printf("  stage[%d]: agents=%v\n", s.Index, s.Agents)
		}
		if len(plan.Handoffs) > 0 {
			fmt.Println("  handoffs:")
			for _, h := range plan.Handoffs {
				req := ""
				if h.Required {
					req = " required=true"
				}
				ch := ""
				if strings.TrimSpace(h.Channel) != "" {
					ch = " channel=" + h.Channel
				}
				merge := ""
				if strings.TrimSpace(h.MergePolicy) != "" && h.MergePolicy != "auto" {
					merge = " merge=" + h.MergePolicy
				}
				chPrio := ""
				if h.ChannelPriority > 0 {
					chPrio = fmt.Sprintf(" channel_priority=%.2f", h.ChannelPriority)
				}
				maxTok := ""
				if h.TargetMaxTokens > 0 {
					maxTok = fmt.Sprintf(" max_tokens=%d", h.TargetMaxTokens)
				}
				fmt.Printf("    - %s.%s -> %s.%s kind=%s%s%s%s%s%s\n", h.FromAgent, h.FromPort, h.ToAgent, h.ToPort, h.Kind, ch, merge, chPrio, maxTok, req)
			}
		}
	}
	for i, t := range plan.Tasks {
		fmt.Printf("  task[%d]: stage=%d agent=%s provider=%s(%s) model=%s skills=%v startup_calls=%d context_budget=%dt context_moment=%s\n", i, t.StageIndex, t.Agent, t.Provider, t.ProviderType, t.Model, t.Skills, t.StartupCalls, t.ContextBudget, t.ContextMoment)
		if len(t.DependsOn) > 0 {
			fmt.Printf("           depends_on=%v\n", t.DependsOn)
		}
		if len(t.IO.Inputs) > 0 || len(t.IO.Outputs) > 0 {
			fmt.Printf("           io.inputs=%d io.outputs=%d\n", len(t.IO.Inputs), len(t.IO.Outputs))
			for _, in := range t.IO.Inputs {
				if (in.MergePolicy != "" && in.MergePolicy != "auto") || in.ChannelPriority > 0 || in.MaxTokens > 0 {
					fmt.Printf("           input[%s] kinds=%v", in.Name, in.Kinds)
					if in.MergePolicy != "" && in.MergePolicy != "auto" {
						fmt.Printf(" merge=%s", in.MergePolicy)
					}
					if in.ChannelPriority > 0 {
						fmt.Printf(" channel_priority=%.2f", in.ChannelPriority)
					}
					if in.MaxTokens > 0 {
						fmt.Printf(" max_tokens=%d", in.MaxTokens)
					}
					fmt.Println()
				}
			}
		}
		if t.ProviderType == "deepseek" && len(t.Skills) > 0 {
			fmt.Println("           tool_loop=enabled (deepseek chat completions -> skills)")
		}
		if t.AgentFile != "" {
			fmt.Printf("           source=%s\n", t.AgentFile)
		}
	}
	return nil
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	showTrace := fs.Bool("trace", false, "print execution trace summary")
	showCoach := fs.Bool("coach", true, "show occasional semantic tips while waiting")
	multiverse := fs.Int("multiverse", 1, "number of universes to run (0 = all crew universes)")
	judgeAgent := fs.String("judge-agent", "", "agent spec (or @crew) used to compare multiverse branches and recommend a result")
	judgeMaxOutputChars := fs.Int("judge-max-output-chars", 700, "max chars per branch sink output sent to the judge agent")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return errors.New("run requires <agent|a+b|@crew|crew:name>")
	}
	agentSpecArg := rest[0]
	input := strings.Join(rest[1:], " ")

	p, abs, verr, err := loadAndValidate(*workspace)
	if err != nil {
		return err
	}
	printValidation(verr)
	if verr != nil && verr.HasErrors() {
		return verr
	}
	eng, err := runtime.New(abs, p)
	if err != nil {
		return err
	}
	resolvedSpec, crew, err := resolveAgentSpecOrCrew(abs, agentSpecArg)
	if err != nil {
		return err
	}
	ctx := context.Background()
	universes, err := buildMultiverseUniverses(abs, agentSpecArg, resolvedSpec, input, *multiverse, crew)
	if err != nil {
		return err
	}
	if len(universes) > 1 {
		// Plan the first universe for coach heuristics and trace summary.
		plan, tasks, err := eng.PlanSpec(ctx, universes[0].Spec, universes[0].Input)
		if err != nil {
			return err
		}
		recordCoachCommandTransition(abs, "run", universes[0].Spec)
		stopCoach := func() {}
		if *showCoach {
			stopCoach = startCoachHint(ctx, coachHintRequest{
				Workspace:   abs,
				CommandPath: "run",
				AgentSpec:   universes[0].Spec,
				Input:       universes[0].Input,
				Plan:        &plan,
				Tasks:       tasks,
				ShowTrace:   *showTrace,
			})
		}
		branches, mvPlan, err := runMultiverse(ctx, abs, p, universes)
		if err != nil {
			return err
		}
		stopCoach()
		printMultiverseRunText(abs, agentSpecArg, mvPlan, branches)
		if strings.TrimSpace(*judgeAgent) != "" {
			judgeSpec, _, jerr := resolveAgentSpecOrCrew(abs, strings.TrimSpace(*judgeAgent))
			judge := multiverseJudgeRun{}
			if jerr != nil {
				judge = multiverseJudgeRun{Spec: strings.TrimSpace(*judgeAgent), Error: jerr.Error()}
			} else {
				judge = runMultiverseJudge(ctx, abs, p, judgeSpec, agentSpecArg, input, branches, *judgeMaxOutputChars)
			}
			printMultiverseJudgeText(judge)
		}
		// Feed coach with branch reports for post-run hints and learning.
		for _, b := range branches {
			if b.Error == "" {
				branchPlan, _, perr := eng.PlanSpec(ctx, b.Universe.Spec, b.Universe.Input)
				if perr == nil {
					recordCoachRunSignals(abs, &branchPlan, b.Report)
					if *showCoach {
						maybePrintCoachPostRunHint(abs, "run", &branchPlan, b.Report)
					}
				}
			}
		}
		if *showTrace {
			fmt.Printf("Trace summary: multiverse_branches=%d source=%q scheduler=%s\n", len(branches), agentSpecArg, mvPlan.Scheduler)
		}
		return nil
	}
	agentSpec := resolvedSpec
	plan, tasks, err := eng.PlanSpec(ctx, agentSpec, input)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "run", agentSpec)
	stopCoach := func() {}
	if *showCoach {
		stopCoach = startCoachHint(ctx, coachHintRequest{
			Workspace:   abs,
			CommandPath: "run",
			AgentSpec:   agentSpec,
			Input:       input,
			Plan:        &plan,
			Tasks:       tasks,
			ShowTrace:   *showTrace,
		})
	}
	report, err := eng.RunSpec(ctx, agentSpec, input)
	stopCoach()
	if err != nil {
		return err
	}
	recordCoachRunSignals(abs, &plan, report)
	fmt.Printf("Run started=%s parallel=%v scheduler=dag_channels input=%q\n", report.StartedAt.Format(time.RFC3339), report.Parallel, report.Input)
	for _, r := range report.Results {
		fmt.Printf("\n[%s] stage=%d provider=%s(%s) model=%s duration=%s context=%d/%dt dropped=%d version=%d moment=%s\n", r.Agent, r.StageIndex, r.Provider, r.ProviderType, r.Model, r.Duration.Round(time.Millisecond), r.ContextTokens, r.ContextBudget, r.ContextDropped, r.ContextVersion, r.ContextMoment)
		if len(r.DependsOn) > 0 {
			fmt.Printf("  depends_on=%v\n", r.DependsOn)
		}
		if r.ContextChannelsTotal > 0 {
			fmt.Printf("  context_channels=%d/%d dropped=%d\n", r.ContextChannelsUsed, r.ContextChannelsTotal, r.ContextChannelsDropped)
		}
		if r.ToolCacheHits > 0 || r.ToolCacheMisses > 0 {
			fmt.Printf("  tool_cache hits=%d misses=%d\n", r.ToolCacheHits, r.ToolCacheMisses)
		}
		if r.ToolBudgetCallsLimit > 0 || r.ToolBudgetExecMSLimit > 0 {
			callLimit := "unlimited"
			if r.ToolBudgetCallsLimit > 0 {
				callLimit = fmt.Sprintf("%d", r.ToolBudgetCallsLimit)
			}
			execLimit := "unlimited"
			if r.ToolBudgetExecMSLimit > 0 {
				execLimit = fmt.Sprintf("%dms", r.ToolBudgetExecMSLimit)
			}
			fmt.Printf("  tool_budget calls=%d/%s exec_ms=%d/%s\n", r.ToolBudgetCallsUsed, callLimit, r.ToolBudgetExecMSUsed, execLimit)
		}
		if r.StageToolBudgetCallsLimit > 0 || r.StageToolBudgetExecMSLimit > 0 {
			callLimit := "unlimited"
			if r.StageToolBudgetCallsLimit > 0 {
				callLimit = fmt.Sprintf("%d", r.StageToolBudgetCallsLimit)
			}
			execLimit := "unlimited"
			if r.StageToolBudgetExecMSLimit > 0 {
				execLimit = fmt.Sprintf("%dms", r.StageToolBudgetExecMSLimit)
			}
			fmt.Printf("  stage_tool_budget calls=%d/%s exec_ms=%d/%s\n", r.StageToolBudgetCallsUsed, callLimit, r.StageToolBudgetExecMSUsed, execLimit)
		}
		if r.SentHandoffs > 0 {
			fmt.Printf("  handoffs_sent=%d\n", r.SentHandoffs)
		}
		if r.HandoffTokens > 0 || r.DroppedHandoffs > 0 {
			fmt.Printf("  handoff_publish tokens=%d dropped=%d\n", r.HandoffTokens, r.DroppedHandoffs)
		}
		if r.SkippedOutputPublish {
			fmt.Println("  handoff_publish skipped_unconsumed_output=true")
		}
		if len(r.StartupCalls) > 0 {
			for _, c := range r.StartupCalls {
				if c.Error != "" {
					fmt.Printf("  startup_call %s failed (%s): %s\n", c.Skill, c.Duration.Round(time.Millisecond), c.Error)
				} else {
					fmt.Printf("  startup_call %s ok (%s)\n", c.Skill, c.Duration.Round(time.Millisecond))
				}
			}
		}
		if len(r.ToolCalls) > 0 {
			for _, c := range r.ToolCalls {
				if c.Error != "" {
					fmt.Printf("  tool_call %s", c.Skill)
					if c.CallID != "" {
						fmt.Printf(" id=%s", c.CallID)
					}
					fmt.Printf(" failed (%s): %s\n", c.Duration.Round(time.Millisecond), c.Error)
					continue
				}
				fmt.Printf("  tool_call %s", c.Skill)
				if c.CallID != "" {
					fmt.Printf(" id=%s", c.CallID)
				}
				if len(c.Args) > 0 {
					fmt.Printf(" args=%v", c.Args)
				}
				if c.Cached {
					fmt.Printf(" cached=true")
				}
				if c.Cacheable && !c.Cached {
					fmt.Printf(" cacheable=true")
				}
				fmt.Printf(" ok (%s)\n", c.Duration.Round(time.Millisecond))
			}
		}
		if r.Error != "" {
			fmt.Printf("  error: %s\n", r.Error)
			continue
		}
		fmt.Println(r.Output)
	}
	fmt.Printf("\nFinished in %s\n", report.EndedAt.Sub(report.StartedAt).Round(time.Millisecond))
	if *showCoach {
		maybePrintCoachPostRunHint(abs, "run", &plan, report)
	}
	if *showTrace {
		fmt.Printf("Trace summary: tasks=%d stages=%d handoffs=%d parallel=%v\n", len(plan.Tasks), len(plan.Stages), len(plan.Handoffs), plan.Parallel)
	}
	return nil
}

func cmdSkill(args []string) error {
	if len(args) == 0 {
		return errors.New("skill requires a subcommand: list or add")
	}
	switch args[0] {
	case "list":
		for _, s := range catalog.List() {
			fmt.Printf("- %s: %s\n", s.Name, s.Description)
		}
		return nil
	case "add":
		return cmdSkillAdd(args[1:])
	default:
		return fmt.Errorf("unknown skill subcommand %q", args[0])
	}
}

func cmdAgent(args []string) error {
	if len(args) == 0 {
		return errors.New("agent requires a subcommand: create")
	}
	switch args[0] {
	case "create":
		return cmdAgentCreate(args[1:])
	default:
		return fmt.Errorf("unknown agent subcommand %q", args[0])
	}
}

func cmdProvider(args []string) error {
	if len(args) == 0 {
		return errors.New("provider requires a subcommand: list or add")
	}
	switch args[0] {
	case "list":
		return cmdProviderList(args[1:])
	case "add":
		return cmdProviderAdd(args[1:])
	default:
		return fmt.Errorf("unknown provider subcommand %q", args[0])
	}
}

func cmdProviderList(args []string) error {
	fs := flag.NewFlagSet("provider list", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("provider list does not accept positional arguments")
	}
	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	p, err := project.Load(abs)
	if err != nil {
		return err
	}
	if len(p.Root.Providers) == 0 {
		fmt.Printf("No providers configured in %s\n", filepath.Join(abs, project.RootConfigFile))
		return nil
	}
	for _, pr := range p.Root.Providers {
		mark := ""
		if strings.TrimSpace(p.Root.DefaultProvider) == pr.Name {
			mark = " (default)"
		}
		fmt.Printf("- %s: type=%s model=%s timeout_ms=%d%s\n", pr.Name, pr.Type, pr.Model, pr.TimeoutMS, mark)
	}
	return nil
}

func cmdProviderAdd(args []string) error {
	fs := flag.NewFlagSet("provider add", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	name := fs.String("name", "deepseek", "provider instance name")
	model := fs.String("model", "deepseek-chat", "default model")
	apiKeyEnv := fs.String("api-key-env", "DEEPSEEK_API_KEY", "environment variable containing API key")
	baseURL := fs.String("base-url", "https://api.deepseek.com", "provider base URL")
	timeoutMS := fs.Int("timeout-ms", 30000, "request timeout in milliseconds")
	setDefault := fs.Bool("set-default", false, "set this provider as default_provider")
	force := fs.Bool("force", false, "replace provider with the same name if it exists")
	rest, err := parseFlagsLoose(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return errors.New("provider add requires a provider type (currently: deepseek)")
	}
	providerType := strings.ToLower(strings.TrimSpace(rest[0]))
	if providerType != "deepseek" {
		return fmt.Errorf("unsupported provider type %q (currently only deepseek is scaffolded)", providerType)
	}
	if strings.TrimSpace(*name) == "" {
		return errors.New("--name cannot be empty")
	}
	if *timeoutMS < 0 {
		return errors.New("--timeout-ms must be >= 0")
	}

	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	p, err := project.Load(abs)
	if err != nil {
		return err
	}

	cfg := project.ProviderConfig{
		Name:      strings.TrimSpace(*name),
		Type:      "deepseek",
		BaseURL:   strings.TrimSpace(*baseURL),
		APIKeyEnv: strings.TrimSpace(*apiKeyEnv),
		Model:     strings.TrimSpace(*model),
		TimeoutMS: *timeoutMS,
	}
	if cfg.TimeoutMS == 0 {
		cfg.TimeoutMS = 30000
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com"
	}
	if cfg.APIKeyEnv == "" {
		cfg.APIKeyEnv = "DEEPSEEK_API_KEY"
	}

	replaced := false
	for i := range p.Root.Providers {
		if p.Root.Providers[i].Name != cfg.Name {
			continue
		}
		if !*force {
			return fmt.Errorf("provider %q already exists (use --force to replace)", cfg.Name)
		}
		p.Root.Providers[i] = cfg
		replaced = true
		break
	}
	if !replaced {
		p.Root.Providers = append(p.Root.Providers, cfg)
	}
	if *setDefault || strings.TrimSpace(p.Root.DefaultProvider) == "" {
		p.Root.DefaultProvider = cfg.Name
	}

	if verr := project.Validate(&project.Project{Root: p.Root}); verr != nil && verr.HasErrors() {
		return verr
	}
	if err := project.SaveRootConfig(abs, p.Root); err != nil {
		return err
	}

	action := "Added"
	if replaced {
		action = "Updated"
	}
	fmt.Printf("%s provider %q (type=deepseek) in %s\n", action, cfg.Name, filepath.Join(abs, project.RootConfigFile))
	if *setDefault || strings.TrimSpace(p.Root.DefaultProvider) == cfg.Name {
		fmt.Printf("default_provider=%s\n", p.Root.DefaultProvider)
	}
	fmt.Printf("Next steps:\n")
	fmt.Printf("  1. set/export %s=\"<your_key>\"\n", cfg.APIKeyEnv)
	fmt.Printf("  2. deeph provider list\n")
	fmt.Printf("  3. deeph agent create --provider %s --model %s analyst\n", cfg.Name, cfg.Model)
	return nil
}

func cmdAgentCreate(args []string) error {
	fs := flag.NewFlagSet("agent create", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	force := fs.Bool("force", false, "overwrite if file exists")
	provider := fs.String("provider", "", "provider name (defaults to deeph.yaml default_provider when available)")
	model := fs.String("model", "mock-small", "model name")
	rest, err := parseFlagsLoose(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return errors.New("agent create requires <name>")
	}

	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}

	selectedProvider := strings.TrimSpace(*provider)
	if selectedProvider == "" {
		if p, err := project.Load(abs); err == nil {
			selectedProvider = strings.TrimSpace(p.Root.DefaultProvider)
		}
	}

	outPath, err := scaffold.CreateAgentFile(abs, scaffold.AgentTemplateOptions{
		Name:        rest[0],
		Provider:    selectedProvider,
		Model:       strings.TrimSpace(*model),
		Description: "User-defined agent",
		Force:       *force,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Created agent template at %s\n", outPath)
	if selectedProvider == "" {
		fmt.Println("Tip: set `provider:` in the agent YAML or configure `default_provider` in deeph.yaml.")
	}
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit the system_prompt in the new agent file")
	fmt.Println("  2. Optionally install skills: deeph skill list && deeph skill add echo")
	fmt.Println("  3. deeph validate")
	fmt.Println("  4. deeph run " + rest[0] + " \"teste\"")
	return nil
}

func cmdType(args []string) error {
	if len(args) == 0 {
		return errors.New("type requires a subcommand: list or explain")
	}
	switch args[0] {
	case "list":
		return cmdTypeList(args[1:])
	case "explain":
		return cmdTypeExplain(args[1:])
	default:
		return fmt.Errorf("unknown type subcommand %q", args[0])
	}
}

func cmdTypeList(args []string) error {
	fs := flag.NewFlagSet("type list", flag.ContinueOnError)
	category := fs.String("category", "", "filter by category (code, text, json, ...)")
	jsonOut := fs.Bool("json", false, "print types as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	catFilter := strings.TrimSpace(strings.ToLower(*category))

	defs := typesys.List()
	if *jsonOut {
		filtered := make([]typesys.TypeDef, 0, len(defs))
		for _, d := range defs {
			if catFilter != "" && d.Category != catFilter {
				continue
			}
			filtered = append(filtered, d)
		}
		if len(filtered) == 0 && catFilter != "" {
			return fmt.Errorf("no types found for category %q", catFilter)
		}
		payload := struct {
			Category string            `json:"category,omitempty"`
			Types    []typesys.TypeDef `json:"types"`
		}{
			Category: catFilter,
			Types:    filtered,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	lastCategory := ""
	for _, d := range defs {
		if catFilter != "" && d.Category != catFilter {
			continue
		}
		if d.Category != lastCategory {
			if lastCategory != "" {
				fmt.Println("")
			}
			fmt.Printf("[%s]\n", d.Category)
			lastCategory = d.Category
		}
		fmt.Printf("- %s", d.Kind)
		if len(d.Aliases) > 0 {
			fmt.Printf(" (aliases: %s)", strings.Join(d.Aliases, ", "))
		}
		fmt.Printf(": %s\n", d.Description)
	}
	if lastCategory == "" {
		if catFilter == "" {
			fmt.Println("No types registered.")
		} else {
			return fmt.Errorf("no types found for category %q", catFilter)
		}
	}
	return nil
}

func cmdTypeExplain(args []string) error {
	fs := flag.NewFlagSet("type explain", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print type entry as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return errors.New("type explain requires <kind|alias>")
	}
	def, ok := typesys.Lookup(rest[0])
	if !ok {
		return fmt.Errorf("unknown type %q", rest[0])
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(def)
	}
	fmt.Printf("kind: %s\n", def.Kind)
	fmt.Printf("category: %s\n", def.Category)
	fmt.Printf("description: %s\n", def.Description)
	if len(def.Aliases) > 0 {
		fmt.Printf("aliases: %s\n", strings.Join(def.Aliases, ", "))
	}
	return nil
}

func cmdSkillAdd(args []string) error {
	fs := flag.NewFlagSet("skill add", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	force := fs.Bool("force", false, "overwrite if file exists")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return errors.New("skill add requires <name>")
	}
	tmpl, err := catalog.Get(rest[0])
	if err != nil {
		return err
	}
	abs, _ := filepath.Abs(*workspace)
	skillsDir := filepath.Join(abs, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return err
	}
	outPath := filepath.Join(skillsDir, tmpl.Filename)
	if !*force {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", outPath)
		}
	}
	if err := os.WriteFile(outPath, []byte(tmpl.Content), 0o644); err != nil {
		return err
	}
	fmt.Printf("Installed skill template %q into %s\n", tmpl.Name, outPath)
	return nil
}

func loadAndValidate(workspace string) (*project.Project, string, *project.ValidationError, error) {
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return nil, "", nil, err
	}
	p, err := project.Load(abs)
	if err != nil {
		return nil, abs, nil, err
	}
	verr := project.Validate(p)
	return p, abs, verr, nil
}

func printValidation(verr *project.ValidationError) {
	if verr == nil || len(verr.Issues) == 0 {
		return
	}
	for _, issue := range verr.Issues {
		fmt.Println(issue.String())
	}
}
