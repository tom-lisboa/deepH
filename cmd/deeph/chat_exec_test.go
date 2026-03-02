package main

import (
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
	if !chatExecRequiresConfirm("crud up") {
		t.Fatalf("expected crud up to require confirmation")
	}
}
