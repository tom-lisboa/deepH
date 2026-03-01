package main

import (
	"strings"
	"testing"
)

func TestMaybeAnswerGuideLocallyBackendRecipe(t *testing.T) {
	meta := &chatSessionMeta{ID: "s1", AgentSpec: "guide"}

	got, ok := maybeAnswerGuideLocally(meta, "qual comando eu uso para criar um beck end ? do meu projeto de futebol")
	if !ok {
		t.Fatalf("expected local guide answer")
	}
	for _, want := range []string{
		"Hoje nao existe `deeph create backend`",
		"deeph agent create backend_builder",
		"deeph trace backend_builder",
		"deeph run backend_builder",
		"projeto de futebol",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected local guide answer to contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "deeph create backend") && !strings.Contains(got, "Hoje nao existe `deeph create backend`") {
		t.Fatalf("unexpected hallucinated create backend command in:\n%s", got)
	}
}

func TestMaybeAnswerGuideLocallyProviderRecipe(t *testing.T) {
	meta := &chatSessionMeta{ID: "s2", AgentSpec: "guide"}

	got, ok := maybeAnswerGuideLocally(meta, "como configuro o deepseek aqui?")
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

	if got, ok := maybeAnswerGuideLocally(meta, "como configuro o deepseek?"); ok || got != "" {
		t.Fatalf("did not expect local guide answer for non-guide agent, got %q", got)
	}
}
