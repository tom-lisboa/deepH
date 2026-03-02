package main

import (
	"strings"
	"testing"

	"deeph/internal/runtime"
)

func TestBuildChatTurnInputAddsDeepHCommandPrimer(t *testing.T) {
	meta := &chatSessionMeta{ID: "s1", AgentSpec: "guide"}

	got := buildChatTurnInput(meta, nil, "como configuro provider deepseek e valido o workspace?", 8, 900)

	for _, want := range []string{
		"[deeph_command_primer]",
		"deeph provider add",
		"deeph validate",
		"current_user_message:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected chat input to contain %q, got:\n%s", want, got)
		}
	}
}

func TestBuildChatTurnInputAddsPowerShellCrewNote(t *testing.T) {
	meta := &chatSessionMeta{ID: "s2", AgentSpec: "guide"}

	got := buildChatTurnInput(meta, nil, "no windows powershell como eu rodo uma crew no deeph?", 8, 900)

	if !strings.Contains(got, "prefer 'crew:name' instead of @name") {
		t.Fatalf("expected PowerShell crew note in chat input, got:\n%s", got)
	}
}

func TestBuildChatTurnInputSkipsPrimerForCodingRequest(t *testing.T) {
	meta := &chatSessionMeta{ID: "s3", AgentSpec: "coder"}

	got := buildChatTurnInput(meta, nil, "implemente um CRUD em Go com postgres e testes", 8, 900)

	if strings.Contains(got, "[deeph_command_primer]") {
		t.Fatalf("did not expect command primer for coding-only request, got:\n%s", got)
	}
	if got != "implemente um CRUD em Go com postgres e testes" {
		t.Fatalf("expected plain user message, got %q", got)
	}
}

func TestBuildChatTurnInputAddsOperationalState(t *testing.T) {
	meta := &chatSessionMeta{
		ID:        "s4",
		AgentSpec: "guide",
		Turns:     3,
		PendingExec: &deephCommand{
			Path:    "crud up",
			Args:    []string{"crud", "up", "--workspace", "/tmp/ws"},
			Display: "deeph crud up --workspace /tmp/ws",
		},
		LastCommandReceipt: &deephCommandReceipt{
			Command: deephCommand{
				Path:    "crud run",
				Args:    []string{"crud", "run", "--workspace", "/tmp/ws"},
				Display: "deeph crud run --workspace /tmp/ws",
			},
			Success: true,
			Next:    "deeph crud up --workspace .",
		},
	}

	got := buildChatTurnInput(meta, nil, "segue", 8, 900)

	for _, want := range []string{
		"[chat_operational_state]",
		"turns: 3",
		"pending_exec: deeph crud up --workspace /tmp/ws",
		"last_command: deeph crud run --workspace /tmp/ws",
		"last_command_success: true",
		"last_command_next: deeph crud up --workspace .",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected chat input to contain %q, got:\n%s", want, got)
		}
	}
}

func TestChatPromptLabelIncludesAgentAndSession(t *testing.T) {
	got := chatPromptLabel(&chatSessionMeta{
		ID:        "feat-login-20260302-123456",
		AgentSpec: "guide",
	})
	if !strings.HasPrefix(got, "you[guide|feat-login-2026030") {
		t.Fatalf("prompt=%q", got)
	}
	if !strings.HasSuffix(got, "]> ") {
		t.Fatalf("prompt=%q", got)
	}
}

func TestHandleChatSlashCommandStatusPrintsCompactSummary(t *testing.T) {
	meta := &chatSessionMeta{
		ID:        "sess-1",
		AgentSpec: "guide",
		Turns:     2,
		PendingExec: &deephCommand{
			Path:    "crud up",
			Args:    []string{"crud", "up", "--workspace", "/tmp/ws"},
			Display: "deeph crud up --workspace /tmp/ws",
		},
		LastCommandReceipt: &deephCommandReceipt{
			Command: deephCommand{
				Path:    "crud run",
				Args:    []string{"crud", "run", "--workspace", "/tmp/ws"},
				Display: "deeph crud run --workspace /tmp/ws",
			},
			Success: true,
			Next:    "deeph crud up --workspace .",
		},
	}

	out := captureStdout(t, func() {
		done, err := handleChatSlashCommand("/status", "/tmp/ws", meta, nil, runtime.ExecutionPlan{
			Tasks:    []runtime.TaskPlan{{Agent: "guide"}},
			Stages:   []runtime.PlanStage{{Index: 0, Agents: []string{"guide"}}},
			Parallel: false,
		}, nil, []int{0})
		if err != nil {
			t.Fatalf("slash status: %v", err)
		}
		if done {
			t.Fatalf("did not expect /status to end session")
		}
	})

	for _, want := range []string{
		"[status] session=sess-1 agent=guide turns=2 tasks=1 stages=1 parallel=false sinks=[0]",
		"[status] ui_mode=full",
		"[status] pending_exec=deeph crud up --workspace /tmp/ws",
		"[status] last_command=deeph crud run --workspace /tmp/ws success=true",
		"[status] last_command_next=deeph crud up --workspace .",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected status output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestHandleChatSlashCommandModePersistsToSessionMeta(t *testing.T) {
	ws := t.TempDir()
	meta := &chatSessionMeta{
		ID:        "sess-mode",
		AgentSpec: "guide",
	}
	if err := saveChatSessionMeta(ws, meta); err != nil {
		t.Fatalf("save session meta: %v", err)
	}

	out := captureStdout(t, func() {
		done, err := handleChatSlashCommand("/mode focus", ws, meta, nil, runtime.ExecutionPlan{}, nil, nil)
		if err != nil {
			t.Fatalf("slash mode: %v", err)
		}
		if done {
			t.Fatalf("did not expect /mode to end session")
		}
	})

	if !strings.Contains(out, "[mode] switched to focus") {
		t.Fatalf("expected mode output, got:\n%s", out)
	}
	if meta.UIMode != chatUIModeFocus {
		t.Fatalf("ui mode=%q", meta.UIMode)
	}

	saved, err := loadChatSessionMeta(ws, meta.ID)
	if err != nil {
		t.Fatalf("load session meta: %v", err)
	}
	if saved.UIMode != chatUIModeFocus {
		t.Fatalf("saved ui mode=%q", saved.UIMode)
	}
}
