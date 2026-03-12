package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"deeph/internal/project"
	"deeph/internal/reviewscope"
	"deeph/internal/runtime"
	"deeph/internal/typesys"
)

type reviewJSONPayload struct {
	Spec         string            `json:"spec"`
	PromptTokens int               `json:"prompt_tokens_estimate"`
	Scope        reviewscope.Scope `json:"scope"`
	Preflight    reviewPreflight   `json:"preflight"`
	Input        string            `json:"input"`
}

type reviewPreflight struct {
	Enabled      bool                   `json:"enabled"`
	Ran          bool                   `json:"ran"`
	Skipped      string                 `json:"skipped,omitempty"`
	CheckTimeout string                 `json:"check_timeout,omitempty"`
	Results      []reviewPreflightCheck `json:"results,omitempty"`
}

type reviewPreflightCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Summary    string `json:"summary,omitempty"`
}

func cmdReview(args []string) error {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	spec := fs.String("spec", "", "agent spec or crew used for the review")
	baseRef := fs.String("base", "auto", "git base ref used for diff-aware review (`auto` tries HEAD, HEAD~1 and last commit)")
	showTrace := fs.Bool("trace", false, "print review scope summary before running")
	showCoach := fs.Bool("coach", true, "show occasional semantic tips while waiting")
	checks := fs.Bool("checks", true, "run deterministic Go checks (`go test ./...` and `go vet ./...`) before review")
	checkTimeout := fs.String("check-timeout", "45s", "timeout per deterministic check when --checks is true")
	jsonOut := fs.Bool("json", false, "print diff-aware review payload as JSON instead of running")
	if err := fs.Parse(args); err != nil {
		return err
	}
	focus := strings.TrimSpace(strings.Join(fs.Args(), " "))
	parsedCheckTimeout, err := parseReviewCheckTimeout(*checkTimeout)
	if err != nil {
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

	cfg := reviewscope.DefaultConfig()
	scope, err := reviewscope.BuildScope(abs, strings.TrimSpace(*baseRef), cfg)
	if err != nil {
		return err
	}
	preflight, preflightBlock := buildReviewPreflight(abs, *checks, parsedCheckTimeout)
	input := reviewscope.BuildInput(scope, focus, cfg)
	input = appendReviewPreflight(input, preflightBlock, cfg.MaxInputChars)
	promptTokens := reviewscope.EstimateTokens(input)

	baseSpec := defaultReviewAgentSpec(p)
	synthSpec := defaultReviewSynthSpec(p)
	selectedSpecArg, useBuiltinFlow := resolveDefaultReviewTarget(abs, p, strings.TrimSpace(*spec))
	displaySpec := reviewDisplaySpec(selectedSpecArg, baseSpec, synthSpec, useBuiltinFlow)

	if *jsonOut {
		payload := reviewJSONPayload{
			Spec:         displaySpec,
			PromptTokens: promptTokens,
			Scope:        scope,
			Preflight:    preflight,
			Input:        input,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	printReviewPreflight(preflight)

	resolvedSpec, crew, err := resolveAgentSpecOrCrew(abs, selectedSpecArg)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if branches, mvPlan, plan, tasks, err := maybeRunReviewMultiverse(ctx, abs, p, input, selectedSpecArg, displaySpec, baseSpec, synthSpec, crew, useBuiltinFlow, *showTrace, *showCoach, scope, promptTokens); err != nil {
		return err
	} else if len(branches) > 0 {
		if *showCoach && plan.Spec != "" {
			_ = tasks
		}
		fmt.Printf("Review started=%s base=%q changed=%d working_set=%d prompt=%dt spec=%q branches=%d\n", time.Now().Format(time.RFC3339), scope.BaseRef, len(scope.DiffFiles), len(scope.WorkingSet), promptTokens, displaySpec, len(branches))
		printMultiverseRunText(abs, displaySpec, mvPlan, branches)
		eng, engErr := runtime.New(abs, p)
		if engErr == nil {
			for _, b := range branches {
				if b.Error != "" {
					continue
				}
				branchInput := strings.TrimSpace(b.Report.Input)
				if branchInput == "" {
					branchInput = b.Universe.Input
				}
				branchPlan, _, perr := eng.PlanSpec(ctx, b.Universe.Spec, branchInput)
				if perr != nil {
					continue
				}
				recordCoachRunSignals(abs, &branchPlan, b.Report)
				if *showCoach {
					maybePrintCoachPostRunHint(abs, "review", &branchPlan, b.Report)
				}
			}
		}
		saveStudioRecent(abs, displaySpec, "")
		return nil
	}

	eng, err := runtime.New(abs, p)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "review", displaySpec)
	plan, tasks, err := eng.PlanSpec(ctx, resolvedSpec, input)
	if err != nil {
		return err
	}
	if *showTrace {
		printReviewScope(scope, displaySpec, promptTokens)
		printCompactChatPlan(plan, chatSinkTaskIndexes(tasks))
	}
	stopCoach := func() {}
	if *showCoach {
		stopCoach = startCoachHint(ctx, coachHintRequest{
			Workspace:   abs,
			CommandPath: "review",
			AgentSpec:   displaySpec,
			Input:       input,
			Plan:        &plan,
			Tasks:       tasks,
			ShowTrace:   *showTrace,
		})
	}
	report, err := eng.RunSpec(ctx, resolvedSpec, input)
	stopCoach()
	if err != nil {
		return err
	}
	recordCoachRunSignals(abs, &plan, report)
	fmt.Printf("Review started=%s base=%q changed=%d working_set=%d prompt=%dt spec=%q\n", report.StartedAt.Format(time.RFC3339), scope.BaseRef, len(scope.DiffFiles), len(scope.WorkingSet), promptTokens, displaySpec)
	printExecutionReport(report)
	fmt.Printf("\nFinished in %s\n", report.EndedAt.Sub(report.StartedAt).Round(time.Millisecond))
	if *showCoach {
		maybePrintCoachPostRunHint(abs, "review", &plan, report)
	}
	saveStudioRecent(abs, displaySpec, "")
	return nil
}

func parseReviewCheckTimeout(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 45 * time.Second, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid --check-timeout %q: %w", raw, err)
	}
	if d <= 0 {
		return 0, errors.New("--check-timeout must be greater than zero")
	}
	return d, nil
}

func buildReviewPreflight(workspace string, enabled bool, timeout time.Duration) (reviewPreflight, string) {
	report := reviewPreflight{
		Enabled:      enabled,
		CheckTimeout: timeout.String(),
	}
	if !enabled {
		report.Skipped = "disabled by flag"
		return report, ""
	}
	if !workspaceHasGoModule(workspace) {
		report.Skipped = "workspace has no go.mod"
		return report, formatReviewPreflightBlock(report)
	}
	report.Ran = true
	specs := []struct {
		name string
		args []string
	}{
		{name: "go_test", args: []string{"test", "./..."}},
		{name: "go_vet", args: []string{"vet", "./..."}},
	}
	for _, spec := range specs {
		report.Results = append(report.Results, runReviewPreflightCheck(workspace, timeout, spec.name, spec.args...))
	}
	return report, formatReviewPreflightBlock(report)
}

func workspaceHasGoModule(workspace string) bool {
	info, err := os.Stat(filepath.Join(workspace, "go.mod"))
	return err == nil && !info.IsDir()
}

func runReviewPreflightCheck(workspace string, timeout time.Duration, name string, args ...string) reviewPreflightCheck {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = workspace
	out, err := cmd.CombinedOutput()
	duration := time.Since(start).Milliseconds()
	summary := summarizeReviewCheckOutput(string(out))
	result := reviewPreflightCheck{
		Name:       name,
		DurationMS: duration,
	}
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		result.Status = "timeout"
		if summary == "" {
			summary = "command timed out"
		}
	case err == nil:
		result.Status = "pass"
	default:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.Status = "fail"
		} else {
			result.Status = "error"
		}
		if summary == "" {
			summary = clipLine(err.Error(), 220)
		}
	}
	result.Summary = summary
	return result
}

func summarizeReviewCheckOutput(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lines := strings.Split(raw, "\n")
	picked := make([]string, 0, 8)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		picked = append(picked, clipLine(line, 160))
		if len(picked) >= 8 {
			break
		}
	}
	if len(picked) == 0 {
		return ""
	}
	return clipLine(strings.Join(picked, " | "), 340)
}

func formatReviewPreflightBlock(report reviewPreflight) string {
	if !report.Enabled {
		return ""
	}
	lines := []string{"[deterministic_checks]"}
	if !report.Ran {
		lines = append(lines, "status: skipped")
		if strings.TrimSpace(report.Skipped) != "" {
			lines = append(lines, "reason: "+clipLine(report.Skipped, 180))
		}
		lines = append(lines, "instruction: checks were skipped. keep residual risks explicit.")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "status: ran")
	if strings.TrimSpace(report.CheckTimeout) != "" {
		lines = append(lines, "timeout_per_check: "+report.CheckTimeout)
	}
	overall := "pass"
	for _, check := range report.Results {
		if check.Status != "pass" {
			overall = "attention"
			break
		}
	}
	lines = append(lines, "overall: "+overall)
	lines = append(lines, "checks:")
	for _, check := range report.Results {
		lines = append(lines, fmt.Sprintf("- name: %s status: %s duration_ms: %d", check.Name, check.Status, check.DurationMS))
		if strings.TrimSpace(check.Summary) != "" {
			lines = append(lines, "  summary: "+clipLine(check.Summary, 220))
		}
	}
	lines = append(lines, "instruction: treat fail/timeout checks as strong regression evidence and still inspect code-level risks.")
	return strings.Join(lines, "\n")
}

func appendReviewPreflight(input string, block string, maxChars int) string {
	input = strings.TrimSpace(input)
	block = strings.TrimSpace(block)
	if block == "" {
		return input
	}
	joined := strings.TrimSpace(input + "\n\n" + block)
	if maxChars <= 0 || len(joined) <= maxChars {
		return joined
	}
	remaining := maxChars - len(input) - 2
	if remaining <= 16 {
		return input
	}
	block = trimReviewBlock(block, remaining)
	return strings.TrimSpace(input + "\n\n" + block)
}

func trimReviewBlock(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	if maxChars <= 3 {
		return s[:maxChars]
	}
	return strings.TrimSpace(s[:maxChars-3]) + "..."
}

func printReviewPreflight(report reviewPreflight) {
	if !report.Enabled {
		return
	}
	if !report.Ran {
		if strings.TrimSpace(report.Skipped) == "" {
			fmt.Println("Deterministic checks: skipped")
			return
		}
		fmt.Printf("Deterministic checks: skipped (%s)\n", report.Skipped)
		return
	}
	parts := make([]string, 0, len(report.Results))
	overall := "pass"
	for _, check := range report.Results {
		parts = append(parts, check.Name+"="+check.Status)
		if check.Status != "pass" {
			overall = "attention"
		}
	}
	fmt.Printf("Deterministic checks: %s [%s]\n", overall, strings.Join(parts, ", "))
}

func maybeRunReviewMultiverse(ctx context.Context, workspace string, p *project.Project, input string, selectedSpecArg string, displaySpec string, baseSpec string, synthSpec string, crew *crewConfig, useBuiltinFlow bool, showTrace bool, showCoach bool, scope reviewscope.Scope, promptTokens int) ([]multiverseRunBranch, *multiverseOrchestrationPlan, runtime.ExecutionPlan, []runtime.Task, error) {
	var universes []multiverseUniverse
	var err error
	switch {
	case crew != nil && len(crew.Universes) > 1:
		universes, err = buildMultiverseUniverses(workspace, selectedSpecArg, strings.TrimSpace(crew.Spec), input, 0, crew)
		if err != nil {
			return nil, nil, runtime.ExecutionPlan{}, nil, err
		}
	case useBuiltinFlow:
		universes = buildBuiltinReviewUniverses(baseSpec, synthSpec, input)
	}
	if len(universes) <= 1 {
		return nil, nil, runtime.ExecutionPlan{}, nil, nil
	}

	recordCoachCommandTransition(workspace, "review", displaySpec)
	var coachPlan runtime.ExecutionPlan
	var coachTasks []runtime.Task
	stopCoach := func() {}
	if showCoach {
		eng, engErr := runtime.New(workspace, p)
		if engErr == nil {
			coachPlan, coachTasks, _ = eng.PlanSpec(ctx, universes[0].Spec, universes[0].Input)
			if coachPlan.Spec != "" {
				stopCoach = startCoachHint(ctx, coachHintRequest{
					Workspace:   workspace,
					CommandPath: "review",
					AgentSpec:   displaySpec,
					Input:       input,
					Plan:        &coachPlan,
					Tasks:       coachTasks,
					ShowTrace:   showTrace,
				})
			}
		}
	}
	if showTrace {
		printReviewScope(scope, displaySpec, promptTokens)
		traceBranches, mvPlan, err := traceMultiverse(ctx, workspace, p, universes)
		if err != nil {
			stopCoach()
			return nil, nil, runtime.ExecutionPlan{}, nil, err
		}
		printMultiverseTraceText(workspace, displaySpec, mvPlan, traceBranches)
	}
	branches, mvPlan, err := runMultiverse(ctx, workspace, p, universes)
	stopCoach()
	if err != nil {
		return nil, nil, runtime.ExecutionPlan{}, nil, err
	}
	return branches, mvPlan, coachPlan, coachTasks, nil
}

func resolveDefaultReviewTarget(workspace string, p *project.Project, requested string) (selected string, useBuiltinFlow bool) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return requested, false
	}
	if _, _, err := loadCrewConfig(workspace, "reviewflow"); err == nil {
		return "@reviewflow", false
	}
	return defaultReviewAgentSpec(p), true
}

func defaultReviewAgentSpec(p *project.Project) string {
	for _, candidate := range []string{"reviewer", "guide"} {
		for _, agent := range p.Agents {
			if strings.EqualFold(strings.TrimSpace(agent.Name), candidate) {
				return agent.Name
			}
		}
	}
	return "reviewer"
}

func defaultReviewSynthSpec(p *project.Project) string {
	for _, candidate := range []string{"review_synth", "reviewer", "guide"} {
		for _, agent := range p.Agents {
			if strings.EqualFold(strings.TrimSpace(agent.Name), candidate) {
				return agent.Name
			}
		}
	}
	return "review_synth"
}

func reviewDisplaySpec(selected string, baseSpec string, synthSpec string, builtin bool) string {
	selected = strings.TrimSpace(selected)
	if !builtin {
		return selected
	}
	return fmt.Sprintf("builtin:reviewflow(%s>%s)", strings.TrimSpace(baseSpec), strings.TrimSpace(synthSpec))
}

func buildBuiltinReviewUniverses(baseSpec string, synthSpec string, input string) []multiverseUniverse {
	baseSpec = strings.TrimSpace(baseSpec)
	synthSpec = strings.TrimSpace(synthSpec)
	if baseSpec == "" {
		return nil
	}
	if synthSpec == "" {
		synthSpec = baseSpec
	}
	return []multiverseUniverse{
		{
			ID:              "u1",
			Label:           "baseline",
			Spec:            baseSpec,
			Input:           input,
			Source:          "builtin.reviewflow",
			Index:           0,
			InputPort:       "context",
			OutputPort:      "result",
			OutputKind:      string(typesys.KindSummaryText),
			MergePolicy:     "append",
			HandoffMaxChars: 240,
			InputNote:       "builtin review universe baseline",
		},
		{
			ID:              "u2",
			Label:           "strict",
			Spec:            baseSpec,
			Input:           strings.TrimSpace("[review_mode]\nmode: strict\nFocus on regressions, edge cases, missing tests, unsafe assumptions and overconfident conclusions.\n\n" + input),
			Source:          "builtin.reviewflow",
			Index:           1,
			InputPort:       "context",
			OutputPort:      "result",
			OutputKind:      string(typesys.KindDiagnosticTest),
			MergePolicy:     "append",
			HandoffMaxChars: 240,
			InputNote:       "builtin review universe strict",
		},
		{
			ID:              "u3",
			Label:           "go_focus",
			Spec:            baseSpec,
			Input:           strings.TrimSpace("[review_mode]\nmode: go-specific\nFocus on Go semantics: context cancellation, goroutine leaks, channel misuse, nil/pointer handling, interface traps, error wrapping, sync/race hazards and io/resource cleanup.\n\n" + input),
			Source:          "builtin.reviewflow",
			Index:           2,
			InputPort:       "context",
			OutputPort:      "result",
			OutputKind:      string(typesys.KindDiagnosticBuild),
			MergePolicy:     "append",
			HandoffMaxChars: 260,
			InputNote:       "builtin review universe go-specific",
		},
		{
			ID:              "u4",
			Label:           "tests_focus",
			Spec:            baseSpec,
			Input:           strings.TrimSpace("[review_mode]\nmode: tests\nFocus on missing tests, brittle assertions, integration gaps, fixture blind spots and why the bug could escape current coverage.\n\n" + input),
			Source:          "builtin.reviewflow",
			Index:           3,
			InputPort:       "context",
			OutputPort:      "result",
			OutputKind:      string(typesys.KindTestUnit),
			MergePolicy:     "append",
			HandoffMaxChars: 240,
			InputNote:       "builtin review universe tests",
		},
		{
			ID:              "u5",
			Label:           "synth",
			Spec:            synthSpec,
			Input:           strings.TrimSpace("[review_mode]\nmode: synth\nMerge upstream review findings, deduplicate overlaps, rank severity, keep file references, and say explicitly when no convincing bug was found.\n\n" + input),
			Source:          "builtin.reviewflow",
			Index:           4,
			DependsOn:       []string{"u1", "u2", "u3", "u4"},
			InputPort:       "context",
			OutputPort:      "result",
			OutputKind:      string(typesys.KindPlanSummary),
			MergePolicy:     "append",
			HandoffMaxChars: 280,
			InputNote:       "builtin review universe synth",
		},
	}
}

func printReviewScope(scope reviewscope.Scope, spec string, promptTokens int) {
	fmt.Printf("Review scope (%s)\n", scope.Workspace)
	fmt.Printf("  spec: %q\n", spec)
	fmt.Printf("  base_ref: %s\n", scope.BaseRef)
	fmt.Printf("  changed_files: %d (+%d -%d) go=%d\n", len(scope.DiffFiles), scope.AddedLines, scope.DeletedLines, scope.GoChanged)
	fmt.Printf("  working_set: %d (same_package=%d tests=%d imports=%d reverse_imports=%d)\n", len(scope.WorkingSet), scope.SamePackage, scope.TestFiles, scope.Imports, scope.ReverseImports)
	fmt.Printf("  prompt_estimate: %dt\n", promptTokens)
	for _, file := range scope.DiffFiles {
		fmt.Printf("  diff: %s %s +%d -%d hunks=%d\n", file.Status, file.Path, file.Added, file.Deleted, len(file.Hunks))
	}
}

func printExecutionReport(report runtime.ExecutionReport) {
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
}
