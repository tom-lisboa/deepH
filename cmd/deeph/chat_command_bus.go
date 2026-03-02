package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type deephCommandReceipt struct {
	Command   deephCommand `json:"command"`
	StartedAt time.Time    `json:"started_at"`
	EndedAt   time.Time    `json:"ended_at"`
	Success   bool         `json:"success"`
	Summary   string       `json:"summary,omitempty"`
	Next      string       `json:"next,omitempty"`
	Error     string       `json:"error,omitempty"`
}

type deephCommandBus struct {
	workspace string
}

func newDeephCommandBus(workspace string) deephCommandBus {
	return deephCommandBus{workspace: strings.TrimSpace(workspace)}
}

func (b deephCommandBus) ParseExecLine(line string) (deephCommand, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "/exec"))
	if raw == "" {
		return deephCommand{}, fmt.Errorf("usage: /exec [--yes] deeph <command>")
	}
	tokens, err := chatSplitArgs(raw)
	if err != nil {
		return deephCommand{}, err
	}
	if len(tokens) == 0 {
		return deephCommand{}, fmt.Errorf("usage: /exec [--yes] deeph <command>")
	}

	confirmed := false
	filtered := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if tok == "--yes" {
			confirmed = true
			continue
		}
		filtered = append(filtered, tok)
	}
	if len(filtered) == 0 {
		return deephCommand{}, fmt.Errorf("usage: /exec [--yes] deeph <command>")
	}
	if strings.EqualFold(filtered[0], "deeph") {
		filtered = filtered[1:]
	}
	if len(filtered) == 0 {
		return deephCommand{}, fmt.Errorf("usage: /exec [--yes] deeph <command>")
	}

	path, pathLen, err := resolveChatExecPath(filtered)
	if err != nil {
		return deephCommand{}, err
	}
	if err := validateChatExecPath(path); err != nil {
		return deephCommand{}, err
	}

	args := append([]string{}, filtered...)
	args = augmentChatExecArgs(b.workspace, path, pathLen, args)
	return deephCommand{
		Path:      path,
		Args:      args,
		Display:   "deeph " + renderChatExecArgs(args),
		Confirmed: confirmed,
	}, nil
}

func (b deephCommandBus) ParseGuideCommand(text string) *deephCommand {
	cmd := firstGuideCommand(text)
	if strings.TrimSpace(cmd) == "" {
		return nil
	}
	if strings.Contains(cmd, "<") || strings.Contains(cmd, ">") || strings.Contains(cmd, "...") {
		return nil
	}
	req, err := b.ParseExecLine("/exec " + cmd)
	if err != nil {
		return nil
	}
	return &req
}

func (b deephCommandBus) RequiresConfirm(cmd deephCommand) bool {
	return chatExecRequiresConfirm(cmd.Path)
}

func (b deephCommandBus) Execute(cmd deephCommand) (deephCommandReceipt, error) {
	receipt := deephCommandReceipt{
		Command:   cmd,
		StartedAt: time.Now(),
	}

	fmt.Printf("[exec] %s\n", cmd.Display)
	if err := run(cmd.Args); err != nil {
		receipt.EndedAt = time.Now()
		receipt.Error = err.Error()
		return receipt, err
	}

	summary := fmt.Sprintf("Executei `%s`.", cmd.Display)
	if cmd.Path == "agent create" {
		summary += " Agora abra o YAML do agent no VS Code, ajuste o arquivo e depois rode `deeph validate --workspace .`."
	}
	if next := chatExecDefaultNext(cmd.Path); next != "" {
		receipt.Next = next
		fmt.Printf("[exec] next: /exec --yes %s\n", next)
		if cmd.Path != "agent create" {
			summary += " Proximo passo sugerido: `" + next + "`."
		}
	}

	receipt.EndedAt = time.Now()
	receipt.Success = true
	receipt.Summary = summary
	return receipt, nil
}

func (b deephCommandBus) normalizeWorkspaceValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return b.workspace
	}
	if filepath.IsAbs(value) {
		return value
	}
	joined := filepath.Join(b.workspace, value)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return joined
	}
	return abs
}
