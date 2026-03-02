package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"deeph/internal/commanddoc"
)

type chatExecRequest struct {
	Path      string
	Args      []string
	Display   string
	Confirmed bool
}

func handleChatExecSlashCommand(line, workspace string) error {
	req, err := parseChatExecLine(line, workspace)
	if err != nil {
		return err
	}
	if chatExecRequiresConfirm(req.Path) && !req.Confirmed {
		fmt.Println("confirmation required for this command. Re-run with:")
		fmt.Printf("  /exec --yes %s\n", req.Display)
		return nil
	}
	_, err = executeChatExecRequest(req)
	return err
}

func executeChatExecRequest(req chatExecRequest) (string, error) {
	fmt.Printf("[exec] %s\n", req.Display)
	if err := run(req.Args); err != nil {
		return "", err
	}
	summary := fmt.Sprintf("Executei `%s`.", req.Display)
	if req.Path == "agent create" {
		summary += " Agora abra o YAML do agent no VS Code, ajuste o arquivo e depois rode `deeph validate --workspace .`."
	}
	if next := chatExecDefaultNext(req.Path); next != "" {
		fmt.Printf("[exec] next: /exec --yes %s\n", next)
		if req.Path != "agent create" {
			summary += " Proximo passo sugerido: `" + next + "`."
		}
	}
	return summary, nil
}

func parseChatExecLine(line, workspace string) (chatExecRequest, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "/exec"))
	if raw == "" {
		return chatExecRequest{}, fmt.Errorf("usage: /exec [--yes] deeph <command>")
	}
	tokens, err := chatSplitArgs(raw)
	if err != nil {
		return chatExecRequest{}, err
	}
	if len(tokens) == 0 {
		return chatExecRequest{}, fmt.Errorf("usage: /exec [--yes] deeph <command>")
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
		return chatExecRequest{}, fmt.Errorf("usage: /exec [--yes] deeph <command>")
	}
	if strings.EqualFold(filtered[0], "deeph") {
		filtered = filtered[1:]
	}
	if len(filtered) == 0 {
		return chatExecRequest{}, fmt.Errorf("usage: /exec [--yes] deeph <command>")
	}

	path, pathLen, err := resolveChatExecPath(filtered)
	if err != nil {
		return chatExecRequest{}, err
	}
	if err := validateChatExecPath(path); err != nil {
		return chatExecRequest{}, err
	}

	args := append([]string{}, filtered...)
	args = augmentChatExecArgs(workspace, path, pathLen, args)
	return chatExecRequest{
		Path:      path,
		Args:      args,
		Display:   "deeph " + renderChatExecArgs(args),
		Confirmed: confirmed,
	}, nil
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
	value = strings.TrimSpace(value)
	if value == "" {
		return workspace
	}
	if filepath.IsAbs(value) {
		return value
	}
	joined := filepath.Join(workspace, value)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return joined
	}
	return abs
}

func chatExecUsesWorkspace(path string) bool {
	switch commanddoc.NormalizePath(path) {
	case "init", "quickstart", "studio", "validate", "trace", "run",
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
	case "help", "validate", "trace",
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

func derivePendingExecFromGuideText(workspace, text string) *chatPendingExec {
	cmd := firstGuideCommand(text)
	if strings.TrimSpace(cmd) == "" {
		return nil
	}
	if strings.Contains(cmd, "<") || strings.Contains(cmd, ">") || strings.Contains(cmd, "...") {
		return nil
	}
	req, err := parseChatExecLine("/exec "+cmd, workspace)
	if err != nil {
		return nil
	}
	return &chatPendingExec{
		Path:    req.Path,
		Args:    append([]string{}, req.Args...),
		Display: req.Display,
	}
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
