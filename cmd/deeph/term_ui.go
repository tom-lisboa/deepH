package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"deeph/internal/runtime"
)

const (
	chatUIModeFull    = "full"
	chatUIModeCompact = "compact"
	chatUIModeFocus   = "focus"
)

func stdoutThemeEnabled() bool {
	return supportsANSIColor() && isCharDevice(os.Stdout)
}

func uiColor(code, text string) string {
	if !stdoutThemeEnabled() || strings.TrimSpace(text) == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func uiStrong(text string) string { return uiColor("1", text) }
func uiMuted(text string) string  { return uiColor("90", text) }
func uiAccent(text string) string { return uiColor("36", text) }
func uiSuccess(text string) string {
	return uiColor("32", text)
}
func uiWarn(text string) string { return uiColor("33", text) }
func uiErrorText(text string) string {
	return uiColor("31", text)
}

func uiBadge(label, tone string) string {
	raw := "[" + strings.TrimSpace(label) + "]"
	switch tone {
	case "accent":
		return uiAccent(raw)
	case "success":
		return uiSuccess(raw)
	case "warn":
		return uiWarn(raw)
	case "error":
		return uiErrorText(raw)
	case "muted":
		return uiMuted(raw)
	default:
		return raw
	}
}

func uiSectionTitle(title string) string {
	if !stdoutThemeEnabled() {
		return title
	}
	return uiStrong(title)
}

func normalizeChatUIMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", chatUIModeFull:
		return chatUIModeFull
	case chatUIModeCompact:
		return chatUIModeCompact
	case chatUIModeFocus:
		return chatUIModeFocus
	default:
		return chatUIModeFull
	}
}

func chatBannerTone(created bool) string {
	if created {
		return "success"
	}
	return "accent"
}

func renderChatPrompt(meta *chatSessionMeta) string {
	raw := chatPromptLabel(meta)
	if !stdoutThemeEnabled() || meta == nil {
		return raw
	}
	agent := strings.TrimSpace(meta.AgentSpec)
	if agent == "" {
		agent = "chat"
	}
	sessionID := strings.TrimSpace(meta.ID)
	if sessionID != "" && len(sessionID) > 18 {
		sessionID = sessionID[:18]
	}
	if sessionID == "" {
		return uiMuted("you") + uiStrong("[") + uiAccent(agent) + uiStrong("]") + "> "
	}
	return uiMuted("you") + uiStrong("[") + uiAccent(agent) + uiMuted("|"+sessionID) + uiStrong("]") + "> "
}

func renderChatReplyPrefix(agent string, isError bool) string {
	rawAgent := strings.TrimSpace(agent)
	if rawAgent == "" {
		rawAgent = "assistant"
	}
	base := "assistant(" + rawAgent + ")>"
	if !stdoutThemeEnabled() {
		return base
	}
	if isError {
		return uiErrorText(base)
	}
	return uiAccent(base)
}

func renderChatReplyBody(body string) string {
	body = renderChatRichText(body)
	if !strings.Contains(body, "\n") {
		return body
	}
	return "\n" + indentForSessionShow(body)
}

func printChatSessionIntro(created bool, meta *chatSessionMeta, workspace string, historyEntries int) {
	label := "resumed"
	if created {
		label = "created"
	}
	title := "deepH chat"
	if stdoutThemeEnabled() {
		title = uiStrong("deepH") + " " + uiAccent("chat")
	}
	fmt.Printf("%s %s\n", title, uiBadge(label, chatBannerTone(created)))
	fmt.Printf("%s %s   %s %s   %s %d\n",
		uiMuted("workspace:"), workspace,
		uiMuted("agent:"), strings.TrimSpace(meta.AgentSpec),
		uiMuted("history:"), historyEntries,
	)
	fmt.Printf("%s %s\n", uiMuted("mode:"), normalizeChatUIMode(meta.UIMode))
	fmt.Printf("%s %s\n", uiMuted("commands:"), "/help /mode /status /history /trace /exec /exit")
}

func printStudioTitle() {
	fmt.Println(uiStrong("deepH") + " " + uiAccent("STUDIO"))
	fmt.Println(uiMuted("=============="))
}

func formatStudioOption(index, label string) string {
	if !stdoutThemeEnabled() {
		return fmt.Sprintf("%s) %s", index, label)
	}
	return fmt.Sprintf("%s %s", uiBadge(index, "accent"), label)
}

func uiClip(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func buildChatStatusBar(meta *chatSessionMeta, plan runtime.ExecutionPlan) string {
	if meta == nil {
		return ""
	}
	mode := normalizeChatUIMode(meta.UIMode)
	if mode == chatUIModeFocus {
		return ""
	}
	parts := []string{
		uiBadge("chat", "accent"),
		uiStrong(strings.TrimSpace(meta.AgentSpec)),
	}
	parts = append(parts, uiMuted("mode="+mode))
	if id := strings.TrimSpace(meta.ID); id != "" {
		if len(id) > 18 {
			id = id[:18]
		}
		parts = append(parts, uiMuted("session="+id))
	}
	parts = append(parts, uiMuted(fmt.Sprintf("turns=%d", meta.Turns)))
	if mode == chatUIModeFull {
		if len(plan.Tasks) > 0 {
			parts = append(parts, uiMuted(fmt.Sprintf("tasks=%d", len(plan.Tasks))))
		}
		if len(plan.Stages) > 0 {
			parts = append(parts, uiMuted(fmt.Sprintf("stages=%d", len(plan.Stages))))
		}
		if len(plan.Tasks) > 0 {
			model := strings.TrimSpace(plan.Tasks[0].Model)
			if model != "" {
				parts = append(parts, uiMuted("model="+model))
			}
		}
	}
	if meta.PendingExec != nil && strings.TrimSpace(meta.PendingExec.Path) != "" {
		parts = append(parts, uiWarn("pending="+meta.PendingExec.Path))
	}
	if meta.LastCommandReceipt != nil && strings.TrimSpace(meta.LastCommandReceipt.Command.Path) != "" {
		tone := "success"
		if !meta.LastCommandReceipt.Success {
			tone = "error"
		}
		parts = append(parts, uiBadge("last:"+meta.LastCommandReceipt.Command.Path, tone))
	}
	line := strings.Join(parts, "  ")
	width := 140
	if mode == chatUIModeCompact {
		width = 96
	}
	return uiClip(line, width)
}

func renderChatRichText(body string) string {
	body = formatChatReplyForTerminal(body)
	if !stdoutThemeEnabled() {
		return body
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	inCode := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			inCode = !inCode
			if inCode {
				out = append(out, uiBadge("code", "muted"))
			}
			continue
		case inCode:
			if trimmed == "" {
				out = append(out, "")
				continue
			}
			prefix := uiMuted("  |")
			out = append(out, prefix+" "+trimmed)
		case trimmed == "":
			out = append(out, "")
		case strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "- "):
			out = append(out, uiStrong(trimmed))
		case strings.HasPrefix(trimmed, "- "):
			out = append(out, uiMuted("-")+" "+strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func shouldShowChatProgress(showCoach bool, meta *chatSessionMeta, line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || !isCharDevice(os.Stderr) {
		return false
	}
	if strings.HasPrefix(line, "/exec") {
		return true
	}
	if meta != nil && meta.PendingExec != nil && chatLooksAffirmative(line) {
		return true
	}
	return !showCoach
}

func chatProgressLabel(meta *chatSessionMeta, line string) string {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "/exec"):
		return "running deeph"
	case meta != nil && meta.PendingExec != nil && chatLooksAffirmative(line):
		return "running deeph"
	case strings.HasPrefix(line, "/"):
		return "processing"
	default:
		return "thinking"
	}
}

func startChatProgress(ctx context.Context, label string) func() {
	if !isCharDevice(os.Stderr) {
		return func() {}
	}
	if strings.TrimSpace(label) == "" {
		label = "working"
	}
	done := make(chan struct{})
	go func() {
		timer := time.NewTimer(180 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-timer.C:
		}

		frames := []string{"-", "\\", "|", "/"}
		prefix := "[deepH]"
		if supportsANSIColor() {
			prefix = "\x1b[36m[deepH]\x1b[0m"
		}
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		frameIdx := 0
		width := 80
		printChatProgressLine(prefix, frames[frameIdx], label, width)
		for {
			select {
			case <-ctx.Done():
				clearChatProgressLine(width)
				return
			case <-done:
				clearChatProgressLine(width)
				return
			case <-ticker.C:
				frameIdx = (frameIdx + 1) % len(frames)
				printChatProgressLine(prefix, frames[frameIdx], label, width)
			}
		}
	}()
	return func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}
}

func printChatProgressLine(prefix, frame, label string, width int) {
	line := uiClip(fmt.Sprintf("%s %s %s", prefix, frame, label), width)
	_, _ = fmt.Fprintf(os.Stderr, "\r%-*s", width, line)
}

func clearChatProgressLine(width int) {
	_, _ = fmt.Fprintf(os.Stderr, "\r%-*s\r", width, "")
}
