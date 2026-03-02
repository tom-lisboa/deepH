package main

import (
	"strings"
	"testing"
)

func TestMaybeAnswerGuideLocallyBackendRecipe(t *testing.T) {
	meta := &chatSessionMeta{ID: "s1", AgentSpec: "guide"}
	ws := t.TempDir()

	got, ok := maybeAnswerGuideLocally(ws, meta, "qual comando eu uso para criar um beck end ? do meu projeto de futebol")
	if !ok {
		t.Fatalf("expected local guide answer")
	}
	for _, want := range []string{
		"Comando agora:",
		"deeph crud init --workspace . --mode backend",
		"O que vai acontecer:",
		"Proximo passo:",
		"deeph crud run --workspace .",
		"deeph crud up",
		"deeph crud smoke",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected local guide answer to contain %q, got:\n%s", want, got)
		}
	}
}

func TestMaybeAnswerGuideLocallyDockerRecipe(t *testing.T) {
	meta := &chatSessionMeta{ID: "s4", AgentSpec: "guide"}
	ws := t.TempDir()

	got, ok := maybeAnswerGuideLocally(ws, meta, "como eu subo os containers docker do meu crud?")
	if !ok {
		t.Fatalf("expected local guide docker answer")
	}
	for _, want := range []string{
		"Comando agora:",
		"deeph crud init --workspace .",
		"deeph crud run --workspace .",
		"deeph crud up",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected docker answer to contain %q, got:\n%s", want, got)
		}
	}
}

func TestMaybeAnswerGuideLocallyProviderRecipe(t *testing.T) {
	meta := &chatSessionMeta{ID: "s2", AgentSpec: "guide"}
	ws := t.TempDir()

	got, ok := maybeAnswerGuideLocally(ws, meta, "como configuro o deepseek aqui?")
	if !ok {
		t.Fatalf("expected local guide provider answer")
	}
	for _, want := range []string{
		"deeph provider add --name deepseek --model deepseek-chat --set-default --force deepseek",
		"deeph validate",
		"DEEPSEEK_API_KEY",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected provider answer to contain %q, got:\n%s", want, got)
		}
	}
}

func TestMaybeAnswerGuideLocallySkipsNonGuide(t *testing.T) {
	meta := &chatSessionMeta{ID: "s3", AgentSpec: "coder"}
	ws := t.TempDir()

	if got, ok := maybeAnswerGuideLocally(ws, meta, "como configuro o deepseek?"); ok || got != "" {
		t.Fatalf("did not expect local guide answer for non-guide agent, got %q", got)
	}
}
