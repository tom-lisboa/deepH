package main

import (
	"strings"
	"testing"

	"deeph/internal/runtime"
)

func TestRouteChatTurnUsesLocalGuideReplyAndSetsPendingExec(t *testing.T) {
	ws := t.TempDir()
	meta := &chatSessionMeta{ID: "s1", AgentSpec: "guide"}

	route, err := routeChatTurn(ws, meta, nil, "como configuro o deepseek aqui?", runtime.ExecutionPlan{}, nil, nil)
	if err != nil {
		t.Fatalf("route chat turn: %v", err)
	}
	if route.Kind != chatRouteHandled {
		t.Fatalf("kind=%q", route.Kind)
	}
	if len(route.Replies) != 1 {
		t.Fatalf("replies=%d", len(route.Replies))
	}
	if meta.PendingExec == nil {
		t.Fatalf("expected pending exec to be set")
	}
	if meta.PendingExec.Path != "provider add" {
		t.Fatalf("path=%q", meta.PendingExec.Path)
	}
	if !strings.Contains(route.Replies[0].Text, "Se quiser, responda `sim`") {
		t.Fatalf("reply missing exec call to action:\n%s", route.Replies[0].Text)
	}
}

func TestRouteChatTurnPendingExecFallsBackToLLMWhenReplyIsNeutral(t *testing.T) {
	meta := &chatSessionMeta{
		ID:        "s2",
		AgentSpec: "guide",
		PendingExec: &deephCommand{
			Path:    "provider add",
			Args:    []string{"provider", "add", "--workspace", "/tmp/ws", "--name", "deepseek", "deepseek"},
			Display: "deeph provider add --workspace /tmp/ws --name deepseek deepseek",
		},
	}

	route, err := routeChatTurn("/tmp/ws", meta, nil, "talvez depois", runtime.ExecutionPlan{}, nil, nil)
	if err != nil {
		t.Fatalf("route chat turn: %v", err)
	}
	if route.Kind != chatRouteLLM {
		t.Fatalf("kind=%q", route.Kind)
	}
	if meta.PendingExec != nil {
		t.Fatalf("expected pending exec to be cleared")
	}
}

func TestRouteChatTurnHandlesPendingExecAffirmative(t *testing.T) {
	meta := &chatSessionMeta{
		ID:        "s3",
		AgentSpec: "guide",
		PendingExec: &deephCommand{
			Path:    "command list",
			Args:    []string{"command", "list"},
			Display: "deeph command list",
		},
	}

	route, err := routeChatTurn("/tmp/ws", meta, nil, "sim", runtime.ExecutionPlan{}, nil, nil)
	if err != nil {
		t.Fatalf("route chat turn: %v", err)
	}
	if route.Kind != chatRouteHandled {
		t.Fatalf("kind=%q", route.Kind)
	}
	if len(route.Replies) != 1 {
		t.Fatalf("replies=%d", len(route.Replies))
	}
	if meta.PendingExec != nil {
		t.Fatalf("expected pending exec to be cleared")
	}
	if !strings.Contains(route.Replies[0].Text, "Executei `deeph command list`.") {
		t.Fatalf("reply=%q", route.Replies[0].Text)
	}
}
