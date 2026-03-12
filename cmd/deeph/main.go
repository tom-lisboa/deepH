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
	case "version", "-v", "--version":
		return cmdVersion(args[1:])
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
	case "review":
		return cmdReview(args[1:])
	case "diagnose":
		return cmdDiagnose(args[1:])
	case "edit":
		return cmdEdit(args[1:])
	case "trace":
		return cmdTrace(args[1:])
	case "run":
		return cmdRun(args[1:])
	case "chat":
		return cmdChat(args[1:])
	case "gws":
		return cmdGWS(args[1:])
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
	case "daemon":
		return cmdDaemon(args[1:])
	case "kit":
		return cmdKit(args[1:])
	case "coach":
		return cmdCoach(args[1:])
	case "command":
		return cmdCommand(args[1:])
	case "type":
		return cmdType(args[1:])
	case "crud":
		return cmdCRUD(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Println("deepH - lightweight agent runtime in Go")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  deeph version [--json]")
	fmt.Println("  deeph init [--workspace DIR]")
	fmt.Println("  deeph quickstart [--workspace DIR] [--agent NAME] [--provider NAME] [--model MODEL] [--with-echo] [--deepseek] [--force]")
	fmt.Println("  deeph studio [--workspace DIR]")
	fmt.Println("  deeph update [--owner NAME] [--repo NAME] [--tag latest|vX.Y.Z] [--check]")
	fmt.Println("  deeph validate [--workspace DIR]")
	fmt.Println(`  deeph review [--workspace DIR] [--spec SPEC] [--base REF] [--trace] [--coach=false] [--json] [focus]`)
	fmt.Println(`  deeph diagnose [--workspace DIR] [--spec SPEC] [--base REF] [--trace] [--coach=false] [--fix] [--yes] [--json] [--file PATH] [issue]`)
	fmt.Println(`  deeph edit [--workspace DIR] [--trace] [--coach=false] [task]`)
	fmt.Println(`  deeph trace [--workspace DIR] [--json] [--multiverse N] [--daemon=true|false] [--daemon-target HOST:PORT] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`)
	fmt.Println(`  deeph run [--workspace DIR] [--trace] [--coach=false] [--multiverse N] [--judge-agent SPEC] [--judge-max-output-chars N] [--daemon=true|false] [--daemon-target HOST:PORT] "<agent|a+b|a>b|a+b>c|@crew|crew:name>" [input]`)
	fmt.Println(`  deeph chat [--workspace DIR] [--session ID] [--history-turns N] [--history-tokens N] [--trace] [--coach=false] "<agent|a+b|a>b|a+b>c>"`)
	fmt.Println("  deeph gws [--yes|--allow-mutate] [--json] [--timeout 30s] [--max-output-bytes N] [--bin gws] [--allow-any-root] <gws args...>")
	fmt.Println("  deeph session list [--workspace DIR]")
	fmt.Println("  deeph session show [--workspace DIR] [--tail N] <id>")
	fmt.Println("  deeph crew list [--workspace DIR]")
	fmt.Println("  deeph crew show [--workspace DIR] <name>")
	fmt.Println("  deeph agent create [--workspace DIR] [--force] [--provider NAME] [--model MODEL] <name>")
	fmt.Println("  deeph provider list [--workspace DIR]")
	fmt.Println("  deeph provider add [--workspace DIR] [--name NAME] [--model MODEL] [--set-default] [--force] deepseek")
	fmt.Println("  deeph daemon serve [--target HOST:PORT]")
	fmt.Println("  deeph daemon start [--target HOST:PORT]")
	fmt.Println("  deeph daemon status [--target HOST:PORT]")
	fmt.Println("  deeph daemon stop [--target HOST:PORT]")
	fmt.Println("  deeph kit list [--workspace DIR]")
	fmt.Println("  deeph kit add [--workspace DIR] [--force] [--provider-name NAME] [--model MODEL] [--set-default-provider] [--skip-provider] <name|git-url[#manifest.yaml]>")
	fmt.Println("  deeph crud init [--workspace DIR] [--force] [--provider-name NAME] [--model MODEL] [--set-default-provider] [--skip-provider] [--mode backend|fullstack] [--db-kind relational|document] [--db postgres|mongodb] [--entity NAME] [--fields nome:text,cidade:text] [--containers=true|false]")
	fmt.Println("  deeph crud prompt [--workspace DIR] [--entity NAME] [--fields nome:text,cidade:text] [--db-kind relational|document] [--db postgres|mongodb] [--backend go] [--frontend next] [--backend-only] [--mode backend|fullstack] [--containers=true|false]")
	fmt.Println("  deeph crud trace [--workspace DIR] [--entity NAME] [--fields nome:text,cidade:text] [--db-kind relational|document] [--db postgres|mongodb] [--backend go] [--frontend next] [--backend-only] [--mode backend|fullstack] [--containers=true|false]")
	fmt.Println("  deeph crud run [--workspace DIR] [--entity NAME] [--fields nome:text,cidade:text] [--db-kind relational|document] [--db postgres|mongodb] [--backend go] [--frontend next] [--backend-only] [--mode backend|fullstack] [--containers=true|false]")
	fmt.Println("  deeph crud up [--workspace DIR] [--compose-file FILE] [--build=true|false] [--detach=true|false] [--wait 45s] [--base-url URL]")
	fmt.Println("  deeph crud smoke [--workspace DIR] [--compose-file FILE] [--base-url URL] [--route-base /people] [--entity NAME] [--fields nome:text,cidade:text] [--no-script] [--timeout 45s]")
	fmt.Println("  deeph crud down [--workspace DIR] [--compose-file FILE] [--volumes]")
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
	useDaemon := fs.Bool("daemon", true, "execute via deephd (local daemon, default=true)")
	daemonTarget := fs.String("daemon-target", deephDaemonDefaultTarget(), "deephd target (host:port)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return errors.New("trace requires <agent|a+b|@crew|crew:name>")
	}
	agentSpecArg := rest[0]
	input := strings.Join(rest[1:], " ")
	target := strings.TrimSpace(*daemonTarget)
	if target == "" {
		target = deephDaemonDefaultTarget()
	}
	if *useDaemon {
		req := daemonTraceRequest{
			Workspace:    *workspace,
			AgentSpecArg: agentSpecArg,
			Input:        input,
			Multiverse:   *multiverse,
		}
		if err := cmdTraceViaDaemon(target, req, *jsonOut); err == nil {
			return nil
		} else if !isDaemonUnavailableError(err) {
			return err
		}
		if _, _, err := startDaemonBackground(target); err == nil {
			if err := cmdTraceViaDaemon(target, req, *jsonOut); err == nil {
				return nil
			} else if !isDaemonUnavailableError(err) {
				return err
			}
		}
		fmt.Fprintf(os.Stderr, "warn: deephd unavailable at %s; falling back to local trace (use --daemon=false to silence)\n", target)
	}

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
	printTracePlanText(abs, plan)
	return nil
}

func cmdEdit(args []string) error {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	showTrace := fs.Bool("trace", false, "print execution trace summary")
	showCoach := fs.Bool("coach", true, "show occasional semantic tips while waiting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	input := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if input == "" {
		return errors.New("edit requires [task] describing the requested code change")
	}
	return cmdRun(buildEditRunArgs(*workspace, *showTrace, *showCoach, input))
}

func buildEditRunArgs(workspace string, showTrace, showCoach bool, input string) []string {
	args := []string{"--workspace", workspace}
	if showTrace {
		args = append(args, "--trace")
	}
	if !showCoach {
		args = append(args, "--coach=false")
	}
	args = append(args, "coder", input)
	return args
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	showTrace := fs.Bool("trace", false, "print execution trace summary")
	showCoach := fs.Bool("coach", true, "show occasional semantic tips while waiting")
	multiverse := fs.Int("multiverse", 1, "number of universes to run (0 = all crew universes)")
	judgeAgent := fs.String("judge-agent", "", "agent spec (or @crew) used to compare multiverse branches and recommend a result")
	judgeMaxOutputChars := fs.Int("judge-max-output-chars", 700, "max chars per branch sink output sent to the judge agent")
	useDaemon := fs.Bool("daemon", true, "execute via deephd (local daemon, default=true)")
	daemonTarget := fs.String("daemon-target", deephDaemonDefaultTarget(), "deephd target (host:port)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return errors.New("run requires <agent|a+b|@crew|crew:name>")
	}
	agentSpecArg := rest[0]
	input := strings.Join(rest[1:], " ")
	target := strings.TrimSpace(*daemonTarget)
	if target == "" {
		target = deephDaemonDefaultTarget()
	}
	if *useDaemon {
		req := daemonRunRequest{
			Workspace:           *workspace,
			AgentSpecArg:        agentSpecArg,
			Input:               input,
			Multiverse:          *multiverse,
			JudgeAgent:          strings.TrimSpace(*judgeAgent),
			JudgeMaxOutputChars: *judgeMaxOutputChars,
		}
		if err := cmdRunViaDaemon(target, req, *showTrace); err == nil {
			return nil
		} else if !isDaemonUnavailableError(err) {
			return err
		}
		if _, _, err := startDaemonBackground(target); err == nil {
			if err := cmdRunViaDaemon(target, req, *showTrace); err == nil {
				return nil
			} else if !isDaemonUnavailableError(err) {
				return err
			}
		}
		fmt.Fprintf(os.Stderr, "warn: deephd unavailable at %s; falling back to local run (use --daemon=false to silence)\n", target)
	}

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
		saveStudioRecent(abs, agentSpecArg, "")
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
	printRunReportText(plan, report)
	if *showCoach {
		maybePrintCoachPostRunHint(abs, "run", &plan, report)
	}
	if *showTrace {
		fmt.Printf("Trace summary: tasks=%d stages=%d handoffs=%d parallel=%v\n", len(plan.Tasks), len(plan.Stages), len(plan.Handoffs), plan.Parallel)
	}
	saveStudioRecent(abs, agentSpecArg, "")
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
	rootPath := filepath.Join(abs, project.RootConfigFile)
	if _, err := os.Stat(rootPath); err != nil {
		if os.IsNotExist(err) {
			return nil, abs, nil, fmt.Errorf("workspace not initialized: %s not found (run `deeph quickstart --workspace %s --deepseek` or `deeph init --workspace %s` first)", rootPath, abs, abs)
		}
		return nil, abs, nil, err
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
