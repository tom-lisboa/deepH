package main

import (
	"os"
	"path/filepath"
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

func TestMaybeAnswerGuideLocallyCodeWorkflowUsesCoderWhenAvailable(t *testing.T) {
	meta := &chatSessionMeta{ID: "s5", AgentSpec: "guide"}
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "deeph.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write deeph.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "agents"), 0o755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	coder := "name: coder\ndescription: test\nprovider: local_mock\nmodel: mock-small\nsystem_prompt: |\n  test\nskills:\n  - file_read_range\n  - file_write_safe\n"
	if err := os.WriteFile(filepath.Join(ws, "agents", "coder.yaml"), []byte(coder), 0o644); err != nil {
		t.Fatalf("write coder agent: %v", err)
	}

	got, ok := maybeAnswerGuideLocally(ws, meta, "Analise cmd/main.go e sugira duas funções adicionais com implementação")
	if !ok {
		t.Fatalf("expected local guide code workflow answer")
	}
	if !strings.Contains(got, "deeph edit --workspace .") {
		t.Fatalf("expected edit shortcut command, got:\n%s", got)
	}
	if !strings.Contains(got, "Se voce responder `sim` aqui no chat") {
		t.Fatalf("expected confirmation hint, got:\n%s", got)
	}
}

func TestMaybeAnswerGuideLocallyCodeWorkflowBootstrapsOldWorkspace(t *testing.T) {
	meta := &chatSessionMeta{ID: "s6", AgentSpec: "guide"}
	ws := t.TempDir()

	got, ok := maybeAnswerGuideLocally(ws, meta, "pode analisar meu main.py e editar para mim?")
	if !ok {
		t.Fatalf("expected local guide code bootstrap answer")
	}
	for _, want := range []string{
		"deeph quickstart --workspace",
		"deeph edit --workspace",
		"Se quiser, responda `sim` e eu executo esse plano aqui no chat.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected bootstrap answer to contain %q, got:\n%s", want, got)
		}
	}
}

func TestMaybeAnswerGuideLocallyCodeWorkflowUsesReviewerForReviewIntent(t *testing.T) {
	meta := &chatSessionMeta{ID: "s7", AgentSpec: "guide"}
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "deeph.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write deeph.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "agents"), 0o755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	reviewer := "name: reviewer\ndescription: test\nprovider: local_mock\nmodel: mock-small\nsystem_prompt: |\n  test\nskills:\n  - file_read_range\n"
	if err := os.WriteFile(filepath.Join(ws, "agents", "reviewer.yaml"), []byte(reviewer), 0o644); err != nil {
		t.Fatalf("write reviewer agent: %v", err)
	}

	got, ok := maybeAnswerGuideLocally(ws, meta, "revise as mudancas no cmd/main.go e procure regressao e testes faltando")
	if !ok {
		t.Fatalf("expected local guide review workflow answer")
	}
	if !strings.Contains(got, "deeph run --workspace . reviewer") {
		t.Fatalf("expected reviewer run command, got:\n%s", got)
	}
}

func TestMaybeAnswerGuideLocallyCodeWorkflowUsesDiagnoseForErrorIntent(t *testing.T) {
	meta := &chatSessionMeta{ID: "s8", AgentSpec: "guide"}
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "deeph.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write deeph.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "agents"), 0o755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	diagnoser := "name: diagnoser\ndescription: test\nprovider: local_mock\nmodel: mock-small\nsystem_prompt: |\n  test\nskills:\n  - file_read_range\n"
	if err := os.WriteFile(filepath.Join(ws, "agents", "diagnoser.yaml"), []byte(diagnoser), 0o644); err != nil {
		t.Fatalf("write diagnoser agent: %v", err)
	}

	got, ok := maybeAnswerGuideLocally(ws, meta, "analise este panic: nil pointer em cmd/main.go:12")
	if !ok {
		t.Fatalf("expected local guide diagnose workflow answer")
	}
	if !strings.Contains(got, "deeph diagnose --workspace .") {
		t.Fatalf("expected diagnose command, got:\n%s", got)
	}
}
