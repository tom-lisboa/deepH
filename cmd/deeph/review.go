package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
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
	Input        string            `json:"input"`
}

func cmdReview(args []string) error {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	spec := fs.String("spec", "", "agent spec or crew used for the review")
	baseRef := fs.String("base", "auto", "git base ref used for diff-aware review (default: auto)")
	showTrace := fs.Bool("trace", false, "print review scope summary before running")
	showCoach := fs.Bool("coach", true, "show occasional semantic tips while waiting")
	jsonOut := fs.Bool("json", false, "print diff-aware review payload as JSON instead of running")
	if err := fs.Parse(args); err != nil {
		return err
	}
	focus := strings.TrimSpace(strings.Join(fs.Args(), " "))

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
	input := reviewscope.BuildInput(scope, focus, cfg)
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
			Input:        input,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

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
		universes = buildBuiltinReviewUniverses(baseSpec, synthSpec, input, scope)
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

func buildBuiltinReviewUniverses(baseSpec string, synthSpec string, input string, scope reviewscope.Scope) []multiverseUniverse {
	baseSpec = strings.TrimSpace(baseSpec)
	synthSpec = strings.TrimSpace(synthSpec)
	if baseSpec == "" {
		return nil
	}
	if synthSpec == "" {
		synthSpec = baseSpec
	}
	langLabel := "go_focus"
	langMode := "go-specific"
	langMessage := "Focus on Go semantics: context cancellation, goroutine leaks, channel misuse, nil/pointer handling, interface traps, error wrapping, sync/race hazards and io/resource cleanup."
	langInputNote := "builtin review universe go-specific"
	if scope.GoChanged == 0 {
		langLabel = "impl_focus"
		langMode = "implementation-specific"
		langMessage = "Focus on implementation hazards for the changed stack: async/concurrency races, null or undefined mistakes, error propagation gaps, resource lifecycle issues, boundary validation misses, and contract drift."
		langInputNote = "builtin review universe implementation-specific"
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
			Label:           langLabel,
			Spec:            baseSpec,
			Input:           strings.TrimSpace("[review_mode]\nmode: " + langMode + "\n" + langMessage + "\n\n" + input),
			Source:          "builtin.reviewflow",
			Index:           2,
			InputPort:       "context",
			OutputPort:      "result",
			OutputKind:      string(typesys.KindDiagnosticBuild),
			MergePolicy:     "append",
			HandoffMaxChars: 260,
			InputNote:       langInputNote,
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
