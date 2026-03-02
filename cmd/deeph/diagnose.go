package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"deeph/internal/diagnosescope"
	"deeph/internal/project"
	"deeph/internal/runtime"
)

type diagnoseJSONPayload struct {
	Spec         string              `json:"spec"`
	PromptTokens int                 `json:"prompt_tokens_estimate"`
	Scope        diagnosescope.Scope `json:"scope"`
	Input        string              `json:"input"`
}

func cmdDiagnose(args []string) error {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	spec := fs.String("spec", "", "agent spec used for the diagnosis")
	baseRef := fs.String("base", "HEAD", "git base ref used for lightweight workspace context")
	showTrace := fs.Bool("trace", false, "print diagnose scope summary before running")
	showCoach := fs.Bool("coach", true, "show occasional semantic tips while waiting")
	fix := fs.Bool("fix", false, "propose or run a follow-up deeph edit using the diagnosis result")
	yes := fs.Bool("yes", false, "with --fix, run the follow-up edit without asking for confirmation")
	jsonOut := fs.Bool("json", false, "print diagnose payload as JSON instead of running")
	inputFile := fs.String("file", "", "read the failing output or error text from a file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	issue, err := readDiagnoseIssue(fs.Args(), *inputFile)
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

	cfg := diagnosescope.DefaultConfig()
	scope, err := diagnosescope.BuildScope(abs, strings.TrimSpace(*baseRef), issue, cfg)
	if err != nil {
		return err
	}
	input := diagnosescope.BuildInput(scope, issue, cfg)
	promptTokens := diagnosescope.EstimateTokens(input)
	selectedSpec := defaultDiagnoseAgentSpec(p, strings.TrimSpace(*spec))

	if *jsonOut {
		payload := diagnoseJSONPayload{
			Spec:         selectedSpec,
			PromptTokens: promptTokens,
			Scope:        scope,
			Input:        input,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	resolvedSpec, _, err := resolveAgentSpecOrCrew(abs, selectedSpec)
	if err != nil {
		return err
	}
	eng, err := runtime.New(abs, p)
	if err != nil {
		return err
	}
	ctx := context.Background()
	recordCoachCommandTransition(abs, "diagnose", selectedSpec)
	plan, tasks, err := eng.PlanSpec(ctx, resolvedSpec, input)
	if err != nil {
		return err
	}
	if *showTrace {
		printDiagnoseScope(scope, selectedSpec, promptTokens)
		printCompactChatPlan(plan, chatSinkTaskIndexes(tasks))
	}
	stopCoach := func() {}
	if *showCoach {
		stopCoach = startCoachHint(ctx, coachHintRequest{
			Workspace:   abs,
			CommandPath: "diagnose",
			AgentSpec:   selectedSpec,
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
	fmt.Printf("Diagnose started=%s refs=%d working_set=%d prompt=%dt spec=%q\n", report.StartedAt.Format(time.RFC3339), len(scope.References), len(scope.WorkingSet), promptTokens, selectedSpec)
	printExecutionReport(report)
	fmt.Printf("\nFinished in %s\n", report.EndedAt.Sub(report.StartedAt).Round(time.Millisecond))
	if *showCoach {
		maybePrintCoachPostRunHint(abs, "diagnose", &plan, report)
	}
	if *fix {
		editTask := buildDiagnoseFixTask(issue, diagnoseLastOutput(report))
		if !*yes {
			if shouldRun, askErr := confirmDiagnoseFix(abs, editTask); askErr != nil {
				return askErr
			} else if !shouldRun {
				saveStudioRecent(abs, selectedSpec, "")
				return nil
			}
		}
		fmt.Println("\n[follow-up] running deeph edit with diagnosis context")
		if err := cmdEdit(buildDiagnoseFixEditArgs(abs, *showTrace, *showCoach, editTask)); err != nil {
			return err
		}
	}
	saveStudioRecent(abs, selectedSpec, "")
	return nil
}

func readDiagnoseIssue(args []string, filePath string) (string, error) {
	if strings.TrimSpace(filePath) != "" {
		b, err := os.ReadFile(strings.TrimSpace(filePath))
		if err != nil {
			return "", err
		}
		issue := strings.TrimSpace(string(b))
		if issue == "" {
			return "", errors.New("diagnose input file is empty")
		}
		return issue, nil
	}
	if len(args) > 0 {
		issue := strings.TrimSpace(strings.Join(args, " "))
		if issue != "" {
			return issue, nil
		}
	}
	if !isInteractiveTerminal(os.Stdin) {
		b, err := io.ReadAll(io.LimitReader(os.Stdin, 128*1024))
		if err != nil {
			return "", err
		}
		issue := strings.TrimSpace(string(b))
		if issue != "" {
			return issue, nil
		}
	}
	return "", errors.New("diagnose requires an error, stack trace, failing output, or --file PATH")
}

func defaultDiagnoseAgentSpec(p *project.Project, requested string) string {
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested)
	}
	for _, candidate := range []string{"diagnoser", "reviewer", "guide"} {
		for _, agent := range p.Agents {
			if strings.EqualFold(strings.TrimSpace(agent.Name), candidate) {
				return agent.Name
			}
		}
	}
	return "diagnoser"
}

func printDiagnoseScope(scope diagnosescope.Scope, spec string, promptTokens int) {
	fmt.Printf("Diagnose scope (%s)\n", scope.Workspace)
	fmt.Printf("  spec: %q\n", spec)
	if strings.TrimSpace(scope.BaseRef) != "" {
		fmt.Printf("  base_ref: %s\n", scope.BaseRef)
	}
	fmt.Printf("  references: %d working_set=%d same_package=%d tests=%d\n", len(scope.References), len(scope.WorkingSet), scope.SamePackage, scope.TestFiles)
	fmt.Printf("  prompt_estimate: %dt\n", promptTokens)
	for _, ref := range scope.References {
		if ref.Line > 0 {
			fmt.Printf("  ref: %s:%d\n", ref.Path, ref.Line)
			continue
		}
		fmt.Printf("  ref: %s\n", ref.Path)
	}
	for _, file := range scope.WorkingSet {
		fmt.Printf("  file: %s reason=%s\n", file.Path, file.Reason)
	}
}

func diagnoseLastOutput(report runtime.ExecutionReport) string {
	for i := len(report.Results) - 1; i >= 0; i-- {
		out := strings.TrimSpace(report.Results[i].Output)
		if out != "" {
			return out
		}
	}
	return ""
}

func buildDiagnoseFixTask(issue, diagnosis string) string {
	issue = clipLine(strings.TrimSpace(issue), 420)
	diagnosis = clipLine(strings.TrimSpace(diagnosis), 900)
	lines := []string{
		"Use the diagnosis below to implement the minimum safe code fix.",
		"Issue:",
		issue,
	}
	if diagnosis != "" {
		lines = append(lines, "", "Diagnosis:", diagnosis)
	}
	lines = append(lines,
		"",
		"Edit only the necessary files, keep the change focused, and summarize changed files plus residual risks.",
	)
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func buildDiagnoseFixEditArgs(workspace string, showTrace, showCoach bool, task string) []string {
	return buildEditRunArgs(workspace, showTrace, showCoach, task)
}

func confirmDiagnoseFix(workspace, task string) (bool, error) {
	command := "deeph edit --workspace " + workspace + " " + strconv.Quote(task)
	fmt.Println("\nSuggested follow-up:")
	fmt.Printf("  %s\n", command)
	if !isInteractiveTerminal(os.Stdin) {
		fmt.Println("Run with `--fix --yes` to execute automatically.")
		return false, nil
	}
	fmt.Print("Run this edit now? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return chatLooksAffirmative(strings.TrimSpace(line)), nil
}
