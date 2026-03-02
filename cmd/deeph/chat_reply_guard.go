package main

import (
	"regexp"
	"strings"
)

var (
	chatAgentRunTaskPattern   = regexp.MustCompile("deeph\\s+agent:run\\s+--task\\s+(\"[^\"]+\"|'[^']+')")
	chatInlineChatTaskPattern = regexp.MustCompile("deeph\\s+chat((?:\\s+--workspace(?:=|\\s+)\\S+)?)\\s+--\\s+(\"[^\"]+\"|'[^']+')")
	chatShellPromptPattern    = regexp.MustCompile(`(?m)^(\s*)\$\s+(deeph\b)`)
)

func sanitizeChatReplies(meta *chatSessionMeta, replies []chatReply) []chatReply {
	if len(replies) == 0 {
		return replies
	}
	out := make([]chatReply, len(replies))
	copy(out, replies)
	defaultAgent := ""
	if meta != nil {
		defaultAgent = strings.TrimSpace(meta.AgentSpec)
	}
	for i := range out {
		agent := strings.TrimSpace(out[i].Agent)
		if agent == "" {
			agent = defaultAgent
		}
		if !strings.EqualFold(agent, "guide") {
			continue
		}
		out[i].Text = sanitizeGuideReplyText(out[i].Text)
	}
	return out
}

func sanitizeGuideReplyText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = chatShellPromptPattern.ReplaceAllString(text, "${1}${2}")
	text = chatAgentRunTaskPattern.ReplaceAllString(text, "deeph run guide $1")
	text = chatInlineChatTaskPattern.ReplaceAllString(text, "deeph run$1 guide $2")
	return text
}

func buildChatRuntimeRules(meta *chatSessionMeta) string {
	if meta == nil || !strings.EqualFold(strings.TrimSpace(meta.AgentSpec), "guide") {
		return ""
	}
	lines := []string{
		"[chat_runtime_rules]",
		"- you are already inside an active `deeph chat` session; do not tell the user to run `deeph chat` again from here.",
		"- never invent deeph commands; only mention documented `deeph ...` commands when you are confident they exist.",
		"- if the user asks for file analysis, code explanation, review, or implementation suggestions in this chat, answer directly instead of redirecting to another deeph command.",
	}
	return strings.Join(lines, "\n")
}
