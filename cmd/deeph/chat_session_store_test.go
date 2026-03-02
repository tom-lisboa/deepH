package main

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOpenOrCreateChatSessionRejectsDifferentSpecOnResume(t *testing.T) {
	ws := t.TempDir()

	meta, entries, created, err := openOrCreateChatSession(ws, "feat-login", "guide")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if !created {
		t.Fatalf("expected new session to be created")
	}
	if len(entries) != 0 {
		t.Fatalf("entries=%d", len(entries))
	}
	if meta.AgentSpec != "guide" {
		t.Fatalf("agent spec=%q", meta.AgentSpec)
	}

	if _, _, _, err := openOrCreateChatSession(ws, "feat-login", "coder"); err == nil {
		t.Fatalf("expected spec mismatch error on resume")
	}
}

func TestOpenOrCreateChatSessionAdoptsRequestedSpecWhenMetaIsEmpty(t *testing.T) {
	ws := t.TempDir()
	meta := &chatSessionMeta{ID: "empty-spec"}
	if err := saveChatSessionMeta(ws, meta); err != nil {
		t.Fatalf("save session meta: %v", err)
	}

	resumed, entries, created, err := openOrCreateChatSession(ws, "empty-spec", "guide")
	if err != nil {
		t.Fatalf("resume session: %v", err)
	}
	if created {
		t.Fatalf("did not expect a new session to be created")
	}
	if len(entries) != 0 {
		t.Fatalf("entries=%d", len(entries))
	}
	if resumed.AgentSpec != "guide" {
		t.Fatalf("agent spec=%q", resumed.AgentSpec)
	}
}

func TestCmdSessionShowPrintsLastCommandReceipt(t *testing.T) {
	ws := t.TempDir()
	meta := &chatSessionMeta{
		ID:        "show-receipt",
		AgentSpec: "guide",
		UIMode:    chatUIModeCompact,
		CreatedAt: time.Now().Add(-time.Minute),
		UpdatedAt: time.Now(),
		Turns:     1,
		LastCommandReceipt: &deephCommandReceipt{
			Command: deephCommand{
				Path:    "command list",
				Args:    []string{"command", "list"},
				Display: "deeph command list",
			},
			StartedAt: time.Now().Add(-10 * time.Second),
			EndedAt:   time.Now().Add(-9 * time.Second),
			Success:   true,
			Summary:   "Executei `deeph command list`.",
		},
	}
	if err := saveChatSessionMeta(ws, meta); err != nil {
		t.Fatalf("save session meta: %v", err)
	}
	if err := saveChatSessionMemory(ws, meta.ID, &chatSessionMemory{
		CompactedThroughTurn: 2,
		RawTailTurns:         2,
		WorkingSet: chatWorkingSet{
			ActiveTopics:   []string{"configurar provider"},
			OpenLoops:      []string{"next step: deeph validate --workspace ."},
			PinnedCommands: []string{"deeph provider add", "deeph validate --workspace ."},
		},
		Episodes: []chatSessionEpisode{{
			StartTurn:         1,
			EndTurn:           2,
			UserGoals:         []string{"configurar provider"},
			AssistantOutcomes: []string{"indicou deeph provider add"},
		}},
	}); err != nil {
		t.Fatalf("save session memory: %v", err)
	}
	if err := appendChatSessionEntries(ws, meta.ID, []chatSessionEntry{{
		Turn:      1,
		Role:      "assistant",
		Agent:     "guide",
		Text:      "ok",
		CreatedAt: time.Now(),
	}}); err != nil {
		t.Fatalf("append session entries: %v", err)
	}

	out := captureStdout(t, func() {
		if err := cmdSessionShow([]string{"--workspace", ws, meta.ID}); err != nil {
			t.Fatalf("session show: %v", err)
		}
	})

	for _, want := range []string{
		"session: show-receipt",
		"ui_mode: compact",
		"memory_episodes: 1 compacted_through_turn=2 raw_tail_turns=2",
		"active_topics: configurar provider",
		"open_loops: next step: deeph validate --workspace .",
		"pinned_commands: deeph provider add | deeph validate --workspace .",
		"last_command: deeph command list success=true",
		"entries:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	return string(out)
}
