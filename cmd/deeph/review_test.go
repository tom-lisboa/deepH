package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deeph/internal/project"
)

func TestDefaultReviewAgentSpecPrefersReviewerThenGuide(t *testing.T) {
	p := &project.Project{
		Agents: []project.AgentConfig{{Name: "guide"}, {Name: "reviewer"}},
	}
	if got := defaultReviewAgentSpec(p); got != "reviewer" {
		t.Fatalf("spec=%q", got)
	}
	if got := defaultReviewAgentSpec(&project.Project{Agents: []project.AgentConfig{{Name: "guide"}}}); got != "guide" {
		t.Fatalf("fallback spec=%q", got)
	}
	if got := defaultReviewAgentSpec(&project.Project{}); got != "reviewer" {
		t.Fatalf("empty project fallback=%q", got)
	}
}

func TestDefaultReviewSynthSpecPrefersSynthThenReviewerThenGuide(t *testing.T) {
	p := &project.Project{
		Agents: []project.AgentConfig{{Name: "guide"}, {Name: "reviewer"}, {Name: "review_synth"}},
	}
	if got := defaultReviewSynthSpec(p); got != "review_synth" {
		t.Fatalf("spec=%q", got)
	}
	if got := defaultReviewSynthSpec(&project.Project{Agents: []project.AgentConfig{{Name: "reviewer"}}}); got != "reviewer" {
		t.Fatalf("fallback spec=%q", got)
	}
	if got := defaultReviewSynthSpec(&project.Project{Agents: []project.AgentConfig{{Name: "guide"}}}); got != "guide" {
		t.Fatalf("guide fallback=%q", got)
	}
}

func TestResolveDefaultReviewTargetPrefersCrewWhenPresent(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "crews"), 0o755); err != nil {
		t.Fatalf("mkdir crews: %v", err)
	}
	body := strings.TrimSpace(`
name: reviewflow
spec: reviewer
universes:
  - name: baseline
    spec: reviewer
  - name: synth
    spec: review_synth
    depends_on: [baseline]
`) + "\n"
	if err := os.WriteFile(filepath.Join(ws, "crews", "reviewflow.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write crew: %v", err)
	}

	selected, builtin := resolveDefaultReviewTarget(ws, &project.Project{Agents: []project.AgentConfig{{Name: "guide"}, {Name: "reviewer"}}}, "")
	if selected != "@reviewflow" || builtin {
		t.Fatalf("selected=%q builtin=%v", selected, builtin)
	}
}

func TestResolveDefaultReviewTargetUsesBuiltinOnlyForImplicitFallback(t *testing.T) {
	p := &project.Project{Agents: []project.AgentConfig{{Name: "guide"}, {Name: "reviewer"}}}

	selected, builtin := resolveDefaultReviewTarget(t.TempDir(), p, "")
	if selected != "reviewer" || !builtin {
		t.Fatalf("selected=%q builtin=%v", selected, builtin)
	}

	selected, builtin = resolveDefaultReviewTarget(t.TempDir(), p, "reviewer")
	if selected != "reviewer" || builtin {
		t.Fatalf("explicit selected=%q builtin=%v", selected, builtin)
	}
}

func TestBuildBuiltinReviewUniversesCreatesSynthDAG(t *testing.T) {
	universes := buildBuiltinReviewUniverses("reviewer", "review_synth", "review input")
	if len(universes) != 5 {
		t.Fatalf("universes=%d", len(universes))
	}
	if universes[0].Spec != "reviewer" || universes[0].Label != "baseline" {
		t.Fatalf("baseline=%+v", universes[0])
	}
	if universes[4].Spec != "review_synth" || universes[4].Label != "synth" {
		t.Fatalf("synth=%+v", universes[4])
	}
	if got := universes[4].DependsOn; len(got) != 4 || got[0] != "u1" || got[3] != "u4" {
		t.Fatalf("depends_on=%v", got)
	}
}

func TestReviewDisplaySpecUsesBuiltinMarkerOnlyWhenNeeded(t *testing.T) {
	if got := reviewDisplaySpec("@reviewflow", "reviewer", "review_synth", false); got != "@reviewflow" {
		t.Fatalf("display spec=%q", got)
	}
	if got := reviewDisplaySpec("reviewer", "reviewer", "review_synth", true); got != "builtin:reviewflow(reviewer>review_synth)" {
		t.Fatalf("builtin display spec=%q", got)
	}
}
