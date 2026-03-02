package main

import (
	"strings"
	"testing"

	"deeph/internal/project"
	"deeph/internal/runtime"
)

func TestDefaultDiagnoseAgentSpecPrefersDiagnoserThenReviewerThenGuide(t *testing.T) {
	p := &project.Project{
		Agents: []project.AgentConfig{{Name: "guide"}, {Name: "reviewer"}, {Name: "diagnoser"}},
	}
	if got := defaultDiagnoseAgentSpec(p, ""); got != "diagnoser" {
		t.Fatalf("spec=%q", got)
	}
	if got := defaultDiagnoseAgentSpec(&project.Project{Agents: []project.AgentConfig{{Name: "reviewer"}}}, ""); got != "reviewer" {
		t.Fatalf("fallback=%q", got)
	}
	if got := defaultDiagnoseAgentSpec(&project.Project{Agents: []project.AgentConfig{{Name: "guide"}}}, ""); got != "guide" {
		t.Fatalf("guide fallback=%q", got)
	}
	if got := defaultDiagnoseAgentSpec(&project.Project{}, "custom"); got != "custom" {
		t.Fatalf("requested=%q", got)
	}
}

func TestReadDiagnoseIssueUsesArgsFirst(t *testing.T) {
	got, err := readDiagnoseIssue([]string{"panic:", "nil", "pointer"}, "")
	if err != nil {
		t.Fatalf("read issue: %v", err)
	}
	if got != "panic: nil pointer" {
		t.Fatalf("got=%q", got)
	}
}

func TestDiagnoseLastOutputPrefersLastNonEmptyResult(t *testing.T) {
	report := runtime.ExecutionReport{
		Results: []runtime.AgentRunResult{
			{Agent: "diagnoser", Output: "first"},
			{Agent: "diagnoser", Output: ""},
			{Agent: "diagnoser", Output: "final diagnosis"},
		},
	}
	if got := diagnoseLastOutput(report); got != "final diagnosis" {
		t.Fatalf("got=%q", got)
	}
}

func TestBuildDiagnoseFixTaskIncludesIssueAndDiagnosis(t *testing.T) {
	got := buildDiagnoseFixTask("panic: nil pointer", "root cause: main dereferences a nil dependency")
	for _, want := range []string{
		"Use the diagnosis below to implement the minimum safe code fix.",
		"Issue:",
		"panic: nil pointer",
		"Diagnosis:",
		"root cause: main dereferences a nil dependency",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected task to contain %q, got:\n%s", want, got)
		}
	}
}
