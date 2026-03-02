package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"deeph/internal/commanddoc"
)

type deephCommand struct {
	Path      string   `json:"path"`
	Args      []string `json:"args"`
	Display   string   `json:"display"`
	Confirmed bool     `json:"confirmed,omitempty"`
}

func handleChatExecSlashCommand(line, workspace string, meta *chatSessionMeta) error {
	bus := newDeephCommandBus(workspace)
	req, err := bus.ParseExecLine(line)
	if err != nil {
		return err
	}
	if bus.RequiresConfirm(req) && !req.Confirmed {
		fmt.Println("confirmation required for this command. Re-run with:")
		fmt.Printf("  /exec --yes %s\n", req.Display)
		return nil
	}
	receipt, err := executeChatExecRequest(req)
	recordLastCommandReceipt(meta, receipt)
	if meta != nil {
		meta.UpdatedAt = time.Now()
		if saveErr := saveChatSessionMeta(workspace, meta); saveErr != nil {
			fmt.Printf("warning: failed to save session metadata: %v\n", saveErr)
		}
	}
	return err
}

func executeChatExecRequest(req deephCommand) (deephCommandReceipt, error) {
	return newDeephCommandBus("").Execute(req)
}

func recordLastCommandReceipt(meta *chatSessionMeta, receipt deephCommandReceipt) {
	if meta == nil {
		return
	}
	if receipt.StartedAt.IsZero() && receipt.EndedAt.IsZero() && receipt.Command.Path == "" && receipt.Command.Display == "" {
		return
	}
	copyReceipt := receipt
	meta.LastCommandReceipt = &copyReceipt
}

func parseChatExecLine(line, workspace string) (deephCommand, error) {
	return newDeephCommandBus(workspace).ParseExecLine(line)
}

func resolveChatExecPath(args []string) (string, int, error) {
	maxParts := min(2, len(args))
	for n := maxParts; n >= 1; n-- {
		path := commanddoc.NormalizePath(strings.Join(args[:n], " "))
		if _, ok := commanddoc.Lookup(path); ok {
			return path, n, nil
		}
	}
	return "", 0, fmt.Errorf("unknown deeph command %q (tip: use `/exec deeph command list`)", strings.Join(args, " "))
}

func validateChatExecPath(path string) error {
	switch commanddoc.NormalizePath(path) {
	case "chat":
		return fmt.Errorf("`deeph chat` cannot be executed from inside chat")
	case "studio":
		return fmt.Errorf("`deeph studio` cannot be executed from inside chat")
	case "update":
		return fmt.Errorf("`deeph update` is blocked inside chat; run it from your terminal")
	default:
		return nil
	}
}

func augmentChatExecArgs(workspace, path string, pathLen int, args []string) []string {
	out := append([]string{}, args[:pathLen]...)
	rest := append([]string{}, args[pathLen:]...)

	if chatExecUsesWorkspace(path) {
		rest = normalizeChatExecWorkspaceArgs(workspace, rest)
	}
	if chatExecUsesWorkspace(path) && !chatExecHasFlag(rest, "workspace") {
		out = append(out, "--workspace", workspace)
	}
	if path == "crud init" && !chatExecHasFlag(rest, "no-prompt") {
		out = append(out, "--no-prompt")
	}
	out = append(out, rest...)
	return out
}

func normalizeChatExecWorkspaceArgs(workspace string, args []string) []string {
	if strings.TrimSpace(workspace) == "" {
		return args
	}
	out := append([]string{}, args...)
	for i := 0; i < len(out); i++ {
		arg := out[i]
		if arg == "--workspace" && i+1 < len(out) {
			out[i+1] = normalizeChatExecWorkspaceValue(workspace, out[i+1])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--workspace=") {
			value := strings.TrimPrefix(arg, "--workspace=")
			out[i] = "--workspace=" + normalizeChatExecWorkspaceValue(workspace, value)
		}
	}
	return out
}

func normalizeChatExecWorkspaceValue(workspace, value string) string {
	return newDeephCommandBus(workspace).normalizeWorkspaceValue(value)
}

func chatExecUsesWorkspace(path string) bool {
	switch commanddoc.NormalizePath(path) {
	case "init", "quickstart", "studio", "validate", "trace", "run",
		"edit",
		"review",
		"session list", "session show",
		"crew list", "crew show",
		"agent create",
		"provider list", "provider add",
		"kit list", "kit add",
		"coach stats", "coach reset",
		"skill add",
		"crud init", "crud prompt", "crud trace", "crud run", "crud up", "crud smoke", "crud down":
		return true
	default:
		return false
	}
}

func chatExecRequiresConfirm(path string) bool {
	switch commanddoc.NormalizePath(path) {
	case "help", "validate", "review", "trace",
		"session list", "session show",
		"crew list", "crew show",
		"provider list",
		"kit list",
		"command list", "command explain",
		"skill list",
		"type list", "type explain",
		"coach stats",
		"crud prompt", "crud trace":
		return false
	default:
		return true
	}
}

func chatExecDefaultNext(path string) string {
	switch commanddoc.NormalizePath(path) {
	case "agent create":
		return "deeph validate --workspace ."
	case "crud init":
		return "deeph crud run --workspace ."
	case "crud run":
		return "deeph crud up --workspace ."
	case "crud up":
		return "deeph crud smoke --workspace ."
	case "crud smoke":
		return "deeph crud down --workspace ."
	default:
		return ""
	}
}

func chatExecHasFlag(args []string, name string) bool {
	prefix := "--" + strings.TrimLeft(strings.TrimSpace(name), "-")
	for _, arg := range args {
		if arg == prefix || strings.HasPrefix(arg, prefix+"=") {
			return true
		}
	}
	return false
}

func renderChatExecArgs(args []string) string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			out = append(out, `""`)
			continue
		}
		if strings.ContainsAny(arg, " \t\n\"'") {
			out = append(out, strconv.Quote(arg))
			continue
		}
		out = append(out, arg)
	}
	return strings.Join(out, " ")
}

func chatSplitArgs(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		if cur.Len() == 0 {
			return
		}
		out = append(out, cur.String())
		cur.Reset()
	}

	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			cur.WriteRune(r)
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	if escaped {
		return nil, fmt.Errorf("unfinished escape in /exec command")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in /exec command")
	}
	flush()
	return out, nil
}

func derivePendingExecFromGuideText(workspace, text string) *deephCommand {
	return newDeephCommandBus(workspace).ParseGuideCommand(text)
}

func firstGuideCommand(text string) string {
	lines := strings.Split(text, "\n")
	inCode := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			inCode = !inCode
		case inCode && trimmed != "":
			return trimmed
		}
	}
	return ""
}

func appendGuideExecCallToAction(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Se quiser, responda `sim` e eu executo esse comando aqui no chat."
	}
	return text + "\n\nSe quiser, responda `sim` e eu executo esse comando aqui no chat."
}

func chatLooksAffirmative(s string) bool {
	norm := normalizeChatLookupText(s)
	switch norm {
	case "sim", "s", "yes", "y", "ok", "okay", "pode", "pode sim", "manda", "manda bala", "vai", "pode executar", "executa", "execute":
		return true
	default:
		return false
	}
}

func chatLooksNegative(s string) bool {
	norm := normalizeChatLookupText(s)
	switch norm {
	case "nao", "não", "n", "no", "cancelar", "cancela", "deixa", "deixa quieto", "deixa pra la", "deixa pra lá":
		return true
	default:
		return false
	}
}
