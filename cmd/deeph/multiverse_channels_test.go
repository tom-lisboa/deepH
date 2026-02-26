package main

import (
	"strings"
	"testing"
	"time"

	"deeph/internal/runtime"
)

func TestPlanMultiverseOrchestrationBuildsChannels(t *testing.T) {
	universes := []multiverseUniverse{
		{ID: "u1", Label: "baseline", Spec: "guide", Index: 0, OutputPort: "result", OutputKind: "summary/code"},
		{ID: "u2", Label: "strict", Spec: "guide", Index: 1, OutputPort: "result", OutputKind: "diagnostic/test"},
		{ID: "u3", Label: "synth", Spec: "guide", Index: 2, DependsOn: []string{"baseline", "u2"}, InputPort: "context", MergePolicy: "append", HandoffMaxChars: 180},
	}
	mv, err := planMultiverseOrchestration(universes)
	if err != nil {
		t.Fatalf("planMultiverseOrchestration error: %v", err)
	}
	if mv.Scheduler != "dag_channels" {
		t.Fatalf("expected dag_channels scheduler, got %q", mv.Scheduler)
	}
	if len(mv.Handoffs) != 2 {
		t.Fatalf("expected 2 handoffs, got %d", len(mv.Handoffs))
	}
	if mv.Handoffs[0].Channel == "" || !strings.Contains(mv.Handoffs[0].Channel, "->u3.context") {
		t.Fatalf("unexpected channel: %+v", mv.Handoffs[0])
	}
	if mv.Handoffs[0].Kind == "" || (!strings.Contains(mv.Handoffs[0].Kind, "summary/") && !strings.Contains(mv.Handoffs[0].Kind, "diagnostic/")) {
		t.Fatalf("expected typed handoff kind, got %+v", mv.Handoffs[0])
	}
	if got := mv.indegree[2]; got != 2 {
		t.Fatalf("expected indegree[2]=2, got %d", got)
	}
}

func TestPlanMultiverseOrchestrationDetectsCycle(t *testing.T) {
	universes := []multiverseUniverse{
		{ID: "u1", Label: "a", Spec: "guide", Index: 0, DependsOn: []string{"b"}},
		{ID: "u2", Label: "b", Spec: "guide", Index: 1, DependsOn: []string{"a"}},
	}
	_, err := planMultiverseOrchestration(universes)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestBuildMultiverseUniverseInputIncludesUpstreamChannels(t *testing.T) {
	universes := []multiverseUniverse{
		{ID: "u1", Label: "baseline", Spec: "guide", Index: 0, OutputPort: "result"},
		{ID: "u2", Label: "strict", Spec: "guide", Index: 1, OutputPort: "result"},
		{ID: "u3", Label: "synth", Spec: "guide", Index: 2, Input: "Solve task", DependsOn: []string{"u1", "u2"}, InputPort: "context", MergePolicy: "append", HandoffMaxChars: 80},
	}
	mv, err := planMultiverseOrchestration(universes)
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	done := []bool{true, true, false}
	branches := []multiverseRunBranch{
		{
			Universe: universes[0],
			Report: runtime.ExecutionReport{
				StartedAt: time.Now(), EndedAt: time.Now(),
				Results: []runtime.AgentRunResult{{Agent: "reviewer", StageIndex: 0, Output: "baseline output"}},
			},
		},
		{
			Universe: universes[1],
			Report: runtime.ExecutionReport{
				StartedAt: time.Now(), EndedAt: time.Now(),
				Results: []runtime.AgentRunResult{{Agent: "reviewer", StageIndex: 0, Output: "strict output"}},
			},
		},
	}
	input, note, channels, contribs := buildMultiverseUniverseInput(universes[2], mv, done, branches)
	if contribs != 2 {
		t.Fatalf("expected 2 contributions, got %d", contribs)
	}
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
	if !strings.Contains(input, "[multiverse_handoffs]") || !strings.Contains(input, "baseline output") || !strings.Contains(input, "strict output") {
		t.Fatalf("compiled input missing handoff content:\n%s", input)
	}
	if !strings.Contains(input, "kind: summary/text") {
		t.Fatalf("expected typed handoff metadata in compiled input, got:\n%s", input)
	}
	if !strings.Contains(note, "channels=2") {
		t.Fatalf("unexpected note: %q", note)
	}
}

func TestBuildMultiverseUniverseInputLatestMergeKeepsOne(t *testing.T) {
	universes := []multiverseUniverse{
		{ID: "u1", Label: "baseline", Spec: "guide", Index: 0},
		{ID: "u2", Label: "strict", Spec: "guide", Index: 1},
		{ID: "u3", Label: "synth", Spec: "guide", Index: 2, DependsOn: []string{"u1", "u2"}, MergePolicy: "latest"},
	}
	mv, err := planMultiverseOrchestration(universes)
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	done := []bool{true, true, false}
	branches := []multiverseRunBranch{
		{Universe: universes[0], Report: runtime.ExecutionReport{Results: []runtime.AgentRunResult{{Agent: "a", StageIndex: 0, Output: "from u1"}}}},
		{Universe: universes[1], Report: runtime.ExecutionReport{Results: []runtime.AgentRunResult{{Agent: "b", StageIndex: 0, Output: "from u2"}}}},
	}
	input, _, channels, contribs := buildMultiverseUniverseInput(universes[2], mv, done, branches)
	if contribs != 1 || len(channels) != 1 {
		t.Fatalf("expected 1 contribution/channel, got contribs=%d channels=%d", contribs, len(channels))
	}
	if strings.Contains(input, "from u1") || !strings.Contains(input, "from u2") {
		t.Fatalf("latest merge should keep only last dependency contribution, got:\n%s", input)
	}
}
