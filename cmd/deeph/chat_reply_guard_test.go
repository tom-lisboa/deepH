package main

import (
	"strings"
	"testing"
)

func TestSanitizeGuideReplyTextRewritesInvalidChatCommands(t *testing.T) {
	input := strings.TrimSpace("Use:\n\n```bash\n$ deeph agent:run --task \"Analise cmd/main.go e sugira duas funcoes\"\n$ deeph chat -- \"Analise cmd/main.go e sugira duas funcoes\"\n```")

	got := sanitizeGuideReplyText(input)

	for _, want := range []string{
		"deeph run guide \"Analise cmd/main.go e sugira duas funcoes\"",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected sanitized reply to contain %q, got:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		"$ deeph",
		"deeph agent:run",
		"deeph chat --",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("did not expect sanitized reply to contain %q, got:\n%s", unwanted, got)
		}
	}
}

func TestBuildChatRuntimeRulesForGuide(t *testing.T) {
	got := buildChatRuntimeRules(&chatSessionMeta{ID: "s1", AgentSpec: "guide"})
	for _, want := range []string{
		"[chat_runtime_rules]",
		"do not tell the user to run `deeph chat` again",
		"Never invent deeph commands",
		"answer directly instead of redirecting",
	} {
		if !strings.Contains(strings.ToLower(got), strings.ToLower(want)) {
			t.Fatalf("expected runtime rules to contain %q, got:\n%s", want, got)
		}
	}
	if gotCoder := buildChatRuntimeRules(&chatSessionMeta{ID: "s2", AgentSpec: "coder"}); gotCoder != "" {
		t.Fatalf("expected no runtime rules for non-guide agent, got:\n%s", gotCoder)
	}
}
