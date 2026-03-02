package main

import (
	"strings"
	"testing"

	"deeph/internal/runtime"
)

func TestBuildChatTurnInputAddsDeepHCommandPrimer(t *testing.T) {
	meta := &chatSessionMeta{ID: "s1", AgentSpec: "guide"}

	got := buildChatTurnInput(meta, nil, nil, "como configuro provider deepseek e valido o workspace?", 8, 900)

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

	got := buildChatTurnInput(meta, nil, nil, "no windows powershell como eu rodo uma crew no deeph?", 8, 900)

	if !strings.Contains(got, "prefer 'crew:name' instead of @name") {
		t.Fatalf("expected PowerShell crew note in chat input, got:\n%s", got)
	}
}

func TestBuildChatTurnInputSkipsPrimerForCodingRequest(t *testing.T) {
	meta := &chatSessionMeta{ID: "s3", AgentSpec: "coder"}

	got := buildChatTurnInput(meta, nil, nil, "implemente um CRUD em Go com postgres e testes", 8, 900)

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

	got := buildChatTurnInput(meta, nil, nil, "segue", 8, 900)

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

func TestBuildChatTurnInputUsesEpisodeMemoryBeforeOldRawHistory(t *testing.T) {
	meta := &chatSessionMeta{ID: "s5", AgentSpec: "guide", Turns: 4}
	memory := &chatSessionMemory{
		CompactedThroughTurn: 2,
		RawTailTurns:         2,
		WorkingSet: chatWorkingSet{
			ActiveTopics:   []string{"review flow", "studio"},
			OpenLoops:      []string{"next step: deeph review --trace"},
			PinnedCommands: []string{"deeph review --trace", "deeph studio"},
		},
		Episodes: []chatSessionEpisode{{
			StartTurn:         1,
			EndTurn:           2,
			UserGoals:         []string{"configurar provider deepseek", "validar workspace"},
			AssistantOutcomes: []string{"explicou provider add e validate"},
			Commands:          []string{"deeph provider add", "deeph validate"},
		}},
	}
	entries := []chatSessionEntry{
		{Turn: 1, Role: "user", Text: "quero configurar provider deepseek"},
		{Turn: 1, Role: "assistant", Agent: "guide", Text: "use deeph provider add"},
		{Turn: 2, Role: "user", Text: "como valido o workspace?"},
		{Turn: 2, Role: "assistant", Agent: "guide", Text: "rode deeph validate"},
		{Turn: 3, Role: "user", Text: "agora mostra a crew review"},
		{Turn: 3, Role: "assistant", Agent: "guide", Text: "use deeph review --trace"},
		{Turn: 4, Role: "user", Text: "beleza, e o studio?"},
		{Turn: 4, Role: "assistant", Agent: "guide", Text: "rode deeph studio"},
	}

	got := buildChatTurnInput(meta, memory, entries, "continua", 8, 220)

	for _, want := range []string{
		"[chat_working_set]",
		"active_topics: review flow | studio",
		"next step: deeph review --trace",
		"pinned_commands:",
		"[chat_memory]",
		"compacted_through_turn: 2",
		"episode turns=1-2",
		"history_tail:",
		"agora mostra a crew review",
		"beleza, e o studio?",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected chat input to contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "quero configurar provider deepseek") {
		t.Fatalf("did not expect old raw history once it is compacted, got:\n%s", got)
	}
}

func TestComputeChatWorkingSetUsesRecentTurnsAndOperationalState(t *testing.T) {
	meta := &chatSessionMeta{
		ID:        "s6",
		AgentSpec: "guide",
		Turns:     3,
		PendingExec: &deephCommand{
			Path:    "crud up",
			Args:    []string{"crud", "up"},
			Display: "deeph crud up --workspace /tmp/ws",
		},
		LastCommandReceipt: &deephCommandReceipt{
			Command: deephCommand{
				Path:    "crud run",
				Args:    []string{"crud", "run"},
				Display: "deeph crud run --workspace /tmp/ws",
			},
			Success: true,
			Next:    "deeph crud up --workspace /tmp/ws",
		},
	}
	entriesByTurn := indexChatEntriesByTurn([]chatSessionEntry{
		{Turn: 1, Role: "user", Text: "como configuro provider deepseek?"},
		{Turn: 1, Role: "assistant", Agent: "guide", Text: "use `deeph provider add`."},
		{Turn: 2, Role: "user", Text: "quero rodar review flow"},
		{Turn: 2, Role: "assistant", Agent: "guide", Text: "rode `deeph review --trace`."},
		{Turn: 3, Role: "user", Text: "como subo o crud?"},
		{Turn: 3, Role: "assistant", Agent: "guide", Text: "execute `deeph crud run --workspace /tmp/ws`."},
	})

	got := computeChatWorkingSet(meta, entriesByTurn, meta.Turns, defaultChatMemoryConfig)

	for _, want := range []string{
		"como subo o crud?",
		"quero rodar review flow",
	} {
		if !containsString(got.ActiveTopics, want) {
			t.Fatalf("expected active topic %q in %+v", want, got.ActiveTopics)
		}
	}
	for _, want := range []string{
		"confirm command: deeph crud up --workspace /tmp/ws",
		"next step: deeph crud up --workspace /tmp/ws",
	} {
		if !containsString(got.OpenLoops, want) {
			t.Fatalf("expected open loop %q in %+v", want, got.OpenLoops)
		}
	}
	for _, want := range []string{
		"deeph review --trace",
		"deeph crud run --workspace /tmp/ws",
		"deeph crud up --workspace /tmp/ws",
	} {
		if !containsString(got.PinnedCommands, want) {
			t.Fatalf("expected pinned command %q in %+v", want, got.PinnedCommands)
		}
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestBuildChatTurnInputCachedReusesStableBlocks(t *testing.T) {
	meta := &chatSessionMeta{ID: "s7", AgentSpec: "guide", Turns: 4}
	memory := &chatSessionMemory{
		CompactedThroughTurn: 2,
		RawTailTurns:         2,
		WorkingSet: chatWorkingSet{
			ActiveTopics:   []string{"review flow", "studio"},
			OpenLoops:      []string{"next step: deeph review --trace"},
			PinnedCommands: []string{"deeph review --trace", "deeph studio"},
		},
		Episodes: []chatSessionEpisode{{
			StartTurn:         1,
			EndTurn:           2,
			UserGoals:         []string{"configurar provider deepseek"},
			AssistantOutcomes: []string{"explicou provider add"},
			Commands:          []string{"deeph provider add"},
		}},
	}
	entries := []chatSessionEntry{
		{Turn: 3, Role: "user", Text: "agora mostra a crew review"},
		{Turn: 3, Role: "assistant", Agent: "guide", Text: "use deeph review --trace"},
		{Turn: 4, Role: "user", Text: "beleza, e o studio?"},
		{Turn: 4, Role: "assistant", Agent: "guide", Text: "rode deeph studio"},
	}
	cache := &chatPromptCache{}

	first := buildChatTurnInputCached(meta, memory, entries, "continua", 8, 220, cache)
	second := buildChatTurnInputCached(meta, memory, entries, "me da o proximo passo", 8, 220, cache)

	if first == "" || second == "" {
		t.Fatalf("expected non-empty cached inputs")
	}
	if cache.operationalBuilds != 1 {
		t.Fatalf("operational_builds=%d", cache.operationalBuilds)
	}
	if cache.workingSetBuilds != 1 {
		t.Fatalf("working_set_builds=%d", cache.workingSetBuilds)
	}
	if cache.memoryBuilds != 1 {
		t.Fatalf("memory_builds=%d", cache.memoryBuilds)
	}
	if cache.historyBuilds != 1 {
		t.Fatalf("history_builds=%d", cache.historyBuilds)
	}

	entries = append(entries, chatSessionEntry{Turn: 5, Role: "user", Text: "e o validate?"})
	_ = buildChatTurnInputCached(meta, memory, entries, "agora sim", 8, 220, cache)

	if cache.historyBuilds != 2 {
		t.Fatalf("expected history block rebuild after tail change, got %d", cache.historyBuilds)
	}
	if cache.memoryBuilds != 1 {
		t.Fatalf("did not expect memory block rebuild, got %d", cache.memoryBuilds)
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
