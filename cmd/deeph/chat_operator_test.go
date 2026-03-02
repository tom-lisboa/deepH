package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaybeAnswerGuideOperationalPrefersRunWhenCRUDProfileExists(t *testing.T) {
	ws := t.TempDir()
	meta := &chatSessionMeta{ID: "s1", AgentSpec: "guide"}

	if err := os.WriteFile(filepath.Join(ws, "deeph.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write deeph.yaml: %v", err)
	}
	if err := saveCRUDWorkspaceConfig(ws, crudWorkspaceConfig{
		Version:     1,
		Entity:      "players",
		Fields:      []crudField{{Name: "nome", Type: "text"}},
		DBKind:      "relational",
		DB:          "postgres",
		Backend:     "go",
		BackendOnly: true,
		Containers:  true,
	}); err != nil {
		t.Fatalf("save crud config: %v", err)
	}

	got, ok := maybeAnswerGuideLocally(ws, meta, "qual o proximo passo do meu crud?")
	if !ok {
		t.Fatalf("expected local guide operator answer")
	}
	for _, want := range []string{
		"deeph crud run --workspace .",
		"deeph crud up --workspace .",
		"perfil CRUD detectado para a entidade `players`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected reply to contain %q, got:\n%s", want, got)
		}
	}
}

func TestMaybeAnswerGuideOperationalPrefersSmokeAfterUp(t *testing.T) {
	ws := t.TempDir()
	meta := &chatSessionMeta{ID: "s2", AgentSpec: "guide"}

	if err := os.WriteFile(filepath.Join(ws, "deeph.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write deeph.yaml: %v", err)
	}
	if err := saveCRUDWorkspaceConfig(ws, crudWorkspaceConfig{
		Version:     1,
		Entity:      "people",
		Fields:      []crudField{{Name: "nome", Type: "text"}, {Name: "cidade", Type: "text"}},
		DBKind:      "relational",
		DB:          "postgres",
		Backend:     "go",
		BackendOnly: true,
		Containers:  true,
	}); err != nil {
		t.Fatalf("save crud config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "docker-compose.yml"), []byte("services:\n  api:\n    ports:\n      - \"8080:8080\"\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "scripts", "smoke.sh"), []byte("echo ok\n"), 0o644); err != nil {
		t.Fatalf("write smoke: %v", err)
	}
	if err := saveCoachState(ws, &coachState{
		Version:           1,
		LastCommand:       "crud up",
		LastCommandAt:     time.Now(),
		Transitions:       map[string]int{"crud up->crud smoke": 3},
		HintSeen:          map[string]int{},
		CommandSeen:       map[string]int{},
		PortSignals:       map[string]int{},
		ScopedTransitions: map[string]map[string]int{},
		ScopedPortSignals: map[string]map[string]int{},
	}); err != nil {
		t.Fatalf("save coach state: %v", err)
	}

	got, ok := maybeAnswerGuideLocally(ws, meta, "e agora, como eu valido esse crud?")
	if !ok {
		t.Fatalf("expected local guide operator answer")
	}
	for _, want := range []string{
		"deeph crud smoke --workspace .",
		"deeph crud down --workspace .",
		"compose detectado em `docker-compose.yml`",
		"script de smoke detectado em `scripts/smoke.sh`",
		"ultimo comando observado pelo coach: `crud up`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected reply to contain %q, got:\n%s", want, got)
		}
	}
}

func TestMaybeAnswerGuideOperationalAgentCreate(t *testing.T) {
	ws := t.TempDir()
	meta := &chatSessionMeta{ID: "s3", AgentSpec: "guide"}

	if err := os.WriteFile(filepath.Join(ws, "deeph.yaml"), []byte("version: 1\ndefault_provider: deepseek\n"), 0o644); err != nil {
		t.Fatalf("write deeph.yaml: %v", err)
	}

	got, ok := maybeAnswerGuideLocally(ws, meta, "cria um agent reviewer")
	if !ok {
		t.Fatalf("expected local guide operator answer")
	}
	for _, want := range []string{
		"deeph agent create --workspace . reviewer",
		"deeph validate --workspace .",
		"agents/reviewer.yaml",
		"VS Code",
		"default_provider detectado: `deepseek`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected reply to contain %q, got:\n%s", want, got)
		}
	}
}

func TestMaybeAnswerGuideOperationalWorkflowFromAgents(t *testing.T) {
	ws := t.TempDir()
	meta := &chatSessionMeta{ID: "s4", AgentSpec: "guide"}

	if err := os.WriteFile(filepath.Join(ws, "deeph.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write deeph.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "agents"), 0o755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	for _, name := range []string{"planner", "coder", "reviewer"} {
		if err := os.WriteFile(filepath.Join(ws, "agents", name+".yaml"), []byte("name: "+name+"\nmodel: mock-small\nsystem_prompt: |\n  test\n"), 0o644); err != nil {
			t.Fatalf("write agent %s: %v", name, err)
		}
	}

	got, ok := maybeAnswerGuideLocally(ws, meta, "quero criar um workflow agora")
	if !ok {
		t.Fatalf("expected local guide operator answer")
	}
	for _, want := range []string{
		`deeph trace --workspace . "planner>coder>reviewer" "sua tarefa"`,
		`deeph run --workspace . "planner>coder>reviewer" "sua tarefa"`,
		"3 agent(s) detectado(s): `coder`, `planner`, `reviewer`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected reply to contain %q, got:\n%s", want, got)
		}
	}
}
