package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseChatExecLineInjectsWorkspaceAndNoPrompt(t *testing.T) {
	req, err := parseChatExecLine(`/exec deeph crud init --mode backend --entity players --fields nome:text`, "/tmp/workspace")
	if err != nil {
		t.Fatalf("parse exec line: %v", err)
	}
	if req.Path != "crud init" {
		t.Fatalf("path=%q", req.Path)
	}
	if req.Confirmed {
		t.Fatalf("expected command to require explicit confirmation")
	}
	got := strings.Join(req.Args, " ")
	for _, want := range []string{
		"crud init",
		"--workspace /tmp/workspace",
		"--no-prompt",
		"--mode backend",
		"--entity players",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected args to contain %q, got %q", want, got)
		}
	}
}

func TestParseChatExecLineNormalizesRelativeWorkspace(t *testing.T) {
	req, err := parseChatExecLine(`/exec deeph crud run --workspace .`, "/tmp/workspace")
	if err != nil {
		t.Fatalf("parse exec line: %v", err)
	}
	got := strings.Join(req.Args, " ")
	want := "--workspace " + filepath.Clean("/tmp/workspace")
	if !strings.Contains(got, want) {
		t.Fatalf("expected args to contain %q, got %q", want, got)
	}
}

func TestParseChatExecLineHandlesQuotes(t *testing.T) {
	req, err := parseChatExecLine(`/exec deeph command explain "crud run"`, "/tmp/workspace")
	if err != nil {
		t.Fatalf("parse exec line: %v", err)
	}
	if req.Path != "command explain" {
		t.Fatalf("path=%q", req.Path)
	}
	if got := strings.Join(req.Args, "|"); got != "command|explain|crud run" {
		t.Fatalf("args=%q", got)
	}
}

func TestParseChatExecLineSupportsEditShortcut(t *testing.T) {
	req, err := parseChatExecLine(`/exec deeph edit "add two helper functions"`, "/tmp/workspace")
	if err != nil {
		t.Fatalf("parse exec line: %v", err)
	}
	if req.Path != "edit" {
		t.Fatalf("path=%q", req.Path)
	}
	got := strings.Join(req.Args, "|")
	want := "edit|--workspace|/tmp/workspace|add two helper functions"
	if got != want {
		t.Fatalf("args=%q want=%q", got, want)
	}
}

func TestParseChatExecLineSupportsDiagnoseShortcut(t *testing.T) {
	req, err := parseChatExecLine(`/exec deeph diagnose "panic in cmd/main.go:12"`, "/tmp/workspace")
	if err != nil {
		t.Fatalf("parse exec line: %v", err)
	}
	if req.Path != "diagnose" {
		t.Fatalf("path=%q", req.Path)
	}
	got := strings.Join(req.Args, "|")
	want := "diagnose|--workspace|/tmp/workspace|panic in cmd/main.go:12"
	if got != want {
		t.Fatalf("args=%q want=%q", got, want)
	}
}

func TestParseChatExecLineRequiresKnownCommand(t *testing.T) {
	if _, err := parseChatExecLine(`/exec deeph made up command`, "/tmp/workspace"); err == nil {
		t.Fatalf("expected unknown command error")
	}
}

func TestParseChatExecLineBlocksNestedChat(t *testing.T) {
	if _, err := parseChatExecLine(`/exec deeph chat guide`, "/tmp/workspace"); err == nil {
		t.Fatalf("expected nested chat to be blocked")
	}
}

func TestChatExecRequiresConfirm(t *testing.T) {
	if chatExecRequiresConfirm("crud trace") {
		t.Fatalf("expected crud trace to be read-only")
	}
	if chatExecRequiresConfirm("diagnose") {
		t.Fatalf("expected diagnose to be read-only")
	}
	if !chatExecRequiresConfirm("crud up") {
		t.Fatalf("expected crud up to require confirmation")
	}
	if next := chatExecDefaultNext("agent create"); next != "deeph validate --workspace ." {
		t.Fatalf("next=%q", next)
	}
}

func TestDerivePendingExecFromGuideText(t *testing.T) {
	text := "Comando agora:\n```bash\ndeeph crud up --workspace .\n```\n"
	pending := derivePendingExecFromGuideText("/tmp/workspace", text)
	if pending == nil {
		t.Fatalf("expected pending exec")
	}
	if pending.Path != "crud up" {
		t.Fatalf("path=%q", pending.Path)
	}
	if !strings.Contains(strings.Join(pending.Args, " "), "--workspace /tmp/workspace") {
		t.Fatalf("args=%q", strings.Join(pending.Args, " "))
	}
}

func TestMaybeHandlePendingExecReplyNegative(t *testing.T) {
	meta := &chatSessionMeta{
		AgentSpec: "guide",
		PendingExec: &deephCommand{
			Path:    "crud up",
			Args:    []string{"crud", "up", "--workspace", "/tmp/workspace"},
			Display: "deeph crud up --workspace /tmp/workspace",
		},
	}
	handled, replies, err := maybeHandlePendingExecReply(meta, "nao")
	if err != nil {
		t.Fatalf("pending reply: %v", err)
	}
	if !handled || len(replies) != 1 {
		t.Fatalf("handled=%v replies=%d", handled, len(replies))
	}
	if meta.PendingExec != nil {
		t.Fatalf("expected pending exec to be cleared")
	}
	if !strings.Contains(replies[0].Text, "Nao executei") {
		t.Fatalf("reply=%q", replies[0].Text)
	}
}

func TestMaybeHandlePendingExecReplyAffirmativeRecordsReceipt(t *testing.T) {
	meta := &chatSessionMeta{
		AgentSpec: "guide",
		PendingExec: &deephCommand{
			Path:    "command list",
			Args:    []string{"command", "list"},
			Display: "deeph command list",
		},
	}

	handled, replies, err := maybeHandlePendingExecReply(meta, "sim")
	if err != nil {
		t.Fatalf("pending reply: %v", err)
	}
	if !handled || len(replies) != 1 {
		t.Fatalf("handled=%v replies=%d", handled, len(replies))
	}
	if meta.LastCommandReceipt == nil {
		t.Fatalf("expected last command receipt to be recorded")
	}
	if meta.LastCommandReceipt.Command.Path != "command list" {
		t.Fatalf("path=%q", meta.LastCommandReceipt.Command.Path)
	}
	if !meta.LastCommandReceipt.Success {
		t.Fatalf("expected successful receipt, got %+v", *meta.LastCommandReceipt)
	}
	if !strings.Contains(replies[0].Text, "Executei `deeph command list`.") {
		t.Fatalf("reply=%q", replies[0].Text)
	}
}

func TestMaybeHandlePendingPlanReplyNegative(t *testing.T) {
	meta := &chatSessionMeta{
		AgentSpec: "guide",
		PendingPlan: &chatPendingPlan{
			Kind:    "bootstrap_code_capabilities",
			Summary: "prepare workspace for code",
			Commands: []deephCommand{{
				Path:    "command list",
				Args:    []string{"command", "list"},
				Display: "deeph command list",
			}},
		},
	}

	handled, replies, err := maybeHandlePendingPlanReply(meta, "nao")
	if err != nil {
		t.Fatalf("pending plan reply: %v", err)
	}
	if !handled || len(replies) != 1 {
		t.Fatalf("handled=%v replies=%d", handled, len(replies))
	}
	if meta.PendingPlan != nil {
		t.Fatalf("expected pending plan to be cleared")
	}
	if !strings.Contains(replies[0].Text, "Nao executei o plano") {
		t.Fatalf("reply=%q", replies[0].Text)
	}
}

func TestMaybeHandlePendingPlanReplyAffirmativeSetsFollowup(t *testing.T) {
	meta := &chatSessionMeta{
		AgentSpec: "guide",
		PendingPlan: &chatPendingPlan{
			Kind:    "bootstrap_code_capabilities",
			Summary: "prepare workspace for code",
			Commands: []deephCommand{{
				Path:    "command list",
				Args:    []string{"command", "list"},
				Display: "deeph command list",
			}},
			Followup: &deephCommand{
				Path:    "command list",
				Args:    []string{"command", "list"},
				Display: "deeph command list",
			},
			FollowupSummary: "rodar o proximo passo",
		},
	}

	handled, replies, err := maybeHandlePendingPlanReply(meta, "sim")
	if err != nil {
		t.Fatalf("pending plan reply: %v", err)
	}
	if !handled || len(replies) != 1 {
		t.Fatalf("handled=%v replies=%d", handled, len(replies))
	}
	if meta.PendingPlan != nil {
		t.Fatalf("expected pending plan to be cleared")
	}
	if meta.PendingExec == nil || meta.PendingExec.Path != "command list" {
		t.Fatalf("expected followup pending exec, got %+v", meta.PendingExec)
	}
	if meta.LastCommandReceipt == nil || !meta.LastCommandReceipt.Success {
		t.Fatalf("expected successful receipt, got %+v", meta.LastCommandReceipt)
	}
	if !strings.Contains(replies[0].Text, "Agora posso executar este proximo passo") {
		t.Fatalf("reply=%q", replies[0].Text)
	}
}

func TestHandleChatExecSlashCommandSavesReceiptInSessionMeta(t *testing.T) {
	ws := t.TempDir()
	meta := &chatSessionMeta{
		ID:        "exec-meta",
		AgentSpec: "guide",
	}
	if err := saveChatSessionMeta(ws, meta); err != nil {
		t.Fatalf("save session meta: %v", err)
	}

	if err := handleChatExecSlashCommand("/exec --yes deeph command list", ws, meta); err != nil {
		t.Fatalf("handle /exec: %v", err)
	}
	if meta.LastCommandReceipt == nil {
		t.Fatalf("expected last command receipt in memory")
	}

	loaded, err := loadChatSessionMeta(ws, meta.ID)
	if err != nil {
		t.Fatalf("load session meta: %v", err)
	}
	if loaded.LastCommandReceipt == nil {
		t.Fatalf("expected last command receipt to be persisted")
	}
	if loaded.LastCommandReceipt.Command.Path != "command list" {
		t.Fatalf("path=%q", loaded.LastCommandReceipt.Command.Path)
	}
}
