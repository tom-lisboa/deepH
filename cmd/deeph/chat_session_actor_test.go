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

func TestChatSessionActorPersistsCompactMemoryAfterTailBoundary(t *testing.T) {
	ws := t.TempDir()
	meta := &chatSessionMeta{
		ID:        "actor-memory",
		AgentSpec: "guide",
	}
	entries := []chatSessionEntry{
		{Turn: 1, Role: "user", Text: "como configuro provider deepseek?"},
		{Turn: 1, Role: "assistant", Agent: "guide", Text: "use `deeph provider add`."},
		{Turn: 2, Role: "user", Text: "como valido o workspace?"},
		{Turn: 2, Role: "assistant", Agent: "guide", Text: "rode `deeph validate`."},
	}
	meta.Turns = 2
	if err := appendChatSessionEntries(ws, meta.ID, entries); err != nil {
		t.Fatalf("append session entries: %v", err)
	}

	actor := newChatSessionActor(chatSessionActorConfig{
		Workspace: ws,
		Plan:      runtime.ExecutionPlan{},
	}, meta, entries)
	defer actor.Close()

	captureStdout(t, func() {
		res := actor.ProcessLine("como configuro o deepseek aqui?")
		if res.Done {
			t.Fatalf("did not expect actor to end session")
		}
	})

	memory, err := loadChatSessionMemory(ws, meta.ID)
	if err != nil {
		t.Fatalf("load session memory: %v", err)
	}
	if memory.CompactedThroughTurn != 1 {
		t.Fatalf("compacted_through_turn=%d", memory.CompactedThroughTurn)
	}
	if len(memory.Episodes) != 1 {
		t.Fatalf("episodes=%d", len(memory.Episodes))
	}
	if len(memory.Episodes[0].Commands) == 0 || memory.Episodes[0].Commands[0] != "deeph provider add" {
		t.Fatalf("unexpected commands: %+v", memory.Episodes[0].Commands)
	}
	if len(memory.WorkingSet.ActiveTopics) == 0 || memory.WorkingSet.ActiveTopics[0] != "como configuro o deepseek aqui?" {
		t.Fatalf("unexpected active topics: %+v", memory.WorkingSet.ActiveTopics)
	}
	if len(memory.WorkingSet.OpenLoops) == 0 || !strings.Contains(memory.WorkingSet.OpenLoops[0], "confirm command:") {
		t.Fatalf("unexpected open loops: %+v", memory.WorkingSet.OpenLoops)
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
