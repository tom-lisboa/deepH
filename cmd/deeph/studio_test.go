package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildStudioQuickResumePlanPrefersLatestWorkspaceSession(t *testing.T) {
	ws := t.TempDir()
	writeStudioRootConfig(t, ws)
	if err := saveChatSessionMeta(ws, &chatSessionMeta{
		ID:        "sess-latest",
		AgentSpec: "guide",
		CreatedAt: time.Now().Add(-time.Minute),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save session meta: %v", err)
	}

	status := collectStudioStatus(ws)
	plan := buildStudioQuickResumePlan(status, &studioState{
		LastWorkspace: ws,
		LastAgentSpec: "planner",
		LastSessionID: "older",
	})

	if !plan.Available {
		t.Fatalf("expected available quick resume plan: %+v", plan)
	}
	if plan.SessionID != "sess-latest" {
		t.Fatalf("session_id=%q", plan.SessionID)
	}
	if plan.Spec != "guide" {
		t.Fatalf("spec=%q", plan.Spec)
	}
}

func TestBuildStudioQuickResumePlanFallsBackToGuideWithoutSession(t *testing.T) {
	ws := t.TempDir()
	writeStudioRootConfig(t, ws)

	status := collectStudioStatus(ws)
	plan := buildStudioQuickResumePlan(status, &studioState{
		LastWorkspace: filepath.Join(ws, "other"),
		LastAgentSpec: "planner",
		LastSessionID: "sess-other",
	})

	if !plan.Available {
		t.Fatalf("expected available quick resume plan: %+v", plan)
	}
	if plan.SessionID != "" {
		t.Fatalf("session_id=%q", plan.SessionID)
	}
	if plan.Spec != "guide" {
		t.Fatalf("spec=%q", plan.Spec)
	}
}

func TestStudioRecommendedActionPrioritizesValidationAndProviderSetup(t *testing.T) {
	status := studioStatus{
		Initialized:      true,
		DefaultProvider:  "",
		ValidationErrors: 2,
	}
	if got := studioRecommendedAction(status, studioQuickResumePlan{Available: true, Detail: "Resume chat"}); got != "Add a provider." {
		t.Fatalf("recommended=%q", got)
	}

	status.DefaultProvider = "deepseek"
	status.APIKeyEnv = "DEEPSEEK_API_KEY"
	if got := studioRecommendedAction(status, studioQuickResumePlan{Available: true, Detail: "Resume chat"}); got != "Set DEEPSEEK_API_KEY in this shell." {
		t.Fatalf("recommended=%q", got)
	}

	status.APIKeySet = true
	if got := studioRecommendedAction(status, studioQuickResumePlan{Available: true, Detail: "Resume chat"}); got != "Validate and fix workspace errors." {
		t.Fatalf("recommended=%q", got)
	}
}

func writeStudioRootConfig(t *testing.T, ws string) {
	t.Helper()
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	body := "version: 1\n" +
		"default_provider: local_mock\n" +
		"providers:\n" +
		"  - name: local_mock\n" +
		"    type: mock\n" +
		"    model: mock-small\n"
	if err := os.WriteFile(filepath.Join(ws, "deeph.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write deeph.yaml: %v", err)
	}
}
