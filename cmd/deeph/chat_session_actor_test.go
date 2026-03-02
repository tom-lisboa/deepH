package main

import (
	"strings"
	"testing"

	"deeph/internal/runtime"
)

func TestChatSessionActorProcessesLocalGuideTurnAndPersistsState(t *testing.T) {
	ws := t.TempDir()
	actor := newChatSessionActor(chatSessionActorConfig{
		Workspace: ws,
		Plan:      runtime.ExecutionPlan{},
	}, &chatSessionMeta{
		ID:        "actor-guide",
		AgentSpec: "guide",
	}, nil)
	defer actor.Close()

	out := captureStdout(t, func() {
		res := actor.ProcessLine("como configuro o deepseek aqui?")
		if res.Done {
			t.Fatalf("did not expect actor to end session")
		}
	})

	if !strings.Contains(out, "assistant(guide)>") {
		t.Fatalf("expected assistant output, got:\n%s", out)
	}

	snap := actor.Snapshot()
	if snap.Meta.Turns != 1 {
		t.Fatalf("turns=%d", snap.Meta.Turns)
	}
	if snap.Meta.PendingExec == nil {
		t.Fatalf("expected pending exec to be stored in actor state")
	}
	if len(snap.Entries) != 2 {
		t.Fatalf("entries=%d", len(snap.Entries))
	}
	if snap.Entries[0].Role != "user" || snap.Entries[1].Role != "assistant" {
		t.Fatalf("unexpected entries: %+v", snap.Entries)
	}
}

func TestChatSessionActorProcessesExitSlashCommand(t *testing.T) {
	actor := newChatSessionActor(chatSessionActorConfig{}, &chatSessionMeta{
		ID:        "actor-exit",
		AgentSpec: "guide",
	}, nil)
	defer actor.Close()

	out := captureStdout(t, func() {
		res := actor.ProcessLine("/exit")
		if !res.Done {
			t.Fatalf("expected actor to finish session on /exit")
		}
	})

	if !strings.Contains(out, "bye") {
		t.Fatalf("expected exit output, got:\n%s", out)
	}
}

func TestChatSessionActorProcessesPendingExecReply(t *testing.T) {
	actor := newChatSessionActor(chatSessionActorConfig{}, &chatSessionMeta{
		ID:        "actor-exec",
		AgentSpec: "guide",
		PendingExec: &deephCommand{
			Path:    "command list",
			Args:    []string{"command", "list"},
			Display: "deeph command list",
		},
	}, nil)
	defer actor.Close()

	out := captureStdout(t, func() {
		res := actor.ProcessLine("sim")
		if res.Done {
			t.Fatalf("did not expect actor to end session")
		}
	})

	if !strings.Contains(out, "assistant(guide)> Executei `deeph command list`.") {
		t.Fatalf("expected exec confirmation output, got:\n%s", out)
	}
	snap := actor.Snapshot()
	if snap.Meta.PendingExec != nil {
		t.Fatalf("expected pending exec to be cleared")
	}
	if snap.Meta.LastCommandReceipt == nil || !snap.Meta.LastCommandReceipt.Success {
		t.Fatalf("expected successful command receipt, got %+v", snap.Meta.LastCommandReceipt)
	}
}
