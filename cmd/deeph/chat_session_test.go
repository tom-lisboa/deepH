package main

import (
	"strings"
	"testing"
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
