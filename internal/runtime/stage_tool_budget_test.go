package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"deeph/internal/project"
)

func TestBuildStageToolBudgetsUsesStrictestPositiveLimits(t *testing.T) {
	tasks := []Task{
		{StageIndex: 0, Agent: project.AgentConfig{Name: "a", Metadata: map[string]string{"stage_tool_max_calls": "10", "stage_tool_max_exec_ms": "1200"}}},
		{StageIndex: 0, Agent: project.AgentConfig{Name: "b", Metadata: map[string]string{"stage_tool_max_calls": "5", "stage_tool_max_exec_ms": "900"}}},
		{StageIndex: 1, Agent: project.AgentConfig{Name: "c", Metadata: map[string]string{"stage_tool_max_calls": "7"}}},
	}
	got := buildStageToolBudgets(tasks)
	if len(got) != 2 {
		t.Fatalf("expected budgets for 2 stages, got %d", len(got))
	}
	stage0 := got[0]
	if stage0 == nil {
		t.Fatalf("missing stage 0 budget")
	}
	if stage0.MaxCalls != 5 {
		t.Fatalf("stage0 MaxCalls=%d want=5", stage0.MaxCalls)
	}
	if stage0.MaxExec != 900*time.Millisecond {
		t.Fatalf("stage0 MaxExec=%s want=900ms", stage0.MaxExec)
	}
	stage1 := got[1]
	if stage1 == nil || stage1.MaxCalls != 7 || stage1.MaxExec != 0 {
		t.Fatalf("unexpected stage1 budget: %+v", stage1)
	}
}

func TestRunSpecSharedStageToolBudgetBlocksParallelTools(t *testing.T) {
	p := &project.Project{
		Root: project.RootConfig{
			Version:         1,
			DefaultProvider: "mockp",
			Providers: []project.ProviderConfig{
				{Name: "mockp", Type: "mock", Model: "mock-small"},
			},
		},
		Agents: []project.AgentConfig{
			{
				Name: "a",
				Metadata: map[string]string{
					"stage_tool_max_calls": "1",
				},
				StartupCalls: []project.SkillCall{{Skill: "echo", Args: map[string]any{"message": "a"}}},
			},
			{
				Name: "b",
				Metadata: map[string]string{
					"stage_tool_max_calls": "1",
				},
				StartupCalls: []project.SkillCall{{Skill: "echo", Args: map[string]any{"message": "b"}}},
			},
		},
		Skills: []project.SkillConfig{
			{Name: "echo", Type: "echo"},
		},
		AgentFiles: map[string]string{},
	}
	eng, err := New(t.TempDir(), p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	report, err := eng.RunSpec(context.Background(), "a+b", "hello")
	if err != nil {
		t.Fatalf("RunSpec returned err: %v", err)
	}
	if len(report.Results) != 2 {
		t.Fatalf("results=%d want=2", len(report.Results))
	}
	var blocked, succeeded int
	for _, r := range report.Results {
		if r.StageToolBudgetCallsLimit != 1 {
			t.Fatalf("%s stage budget calls limit=%d want=1", r.Agent, r.StageToolBudgetCallsLimit)
		}
		if r.Error != "" && strings.Contains(r.Error, "stage_tool_max_calls exceeded") {
			blocked++
			continue
		}
		if r.Error == "" {
			succeeded++
		}
	}
	if blocked != 1 || succeeded != 1 {
		t.Fatalf("expected one blocked and one succeeded, got blocked=%d succeeded=%d results=%+v", blocked, succeeded, report.Results)
	}
}
