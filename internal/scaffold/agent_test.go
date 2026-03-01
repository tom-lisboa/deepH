package scaffold

import (
	"strings"
	"testing"
)

func TestRenderGuideStarterTemplatePrefersDeepHCommands(t *testing.T) {
	got := renderGuideStarterTemplate(AgentTemplateOptions{
		Name:     "guide",
		Provider: "deepseek",
		Model:    "deepseek-chat",
	})

	for _, want := range []string{
		"Prefer deeph-native commands over generic shell commands",
		"resolve it in favor of the deeph CLI first",
		"For crew examples on PowerShell, prefer crew:name instead of @name.",
		"  - command_doc",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected guide template to contain %q, got:\n%s", want, got)
		}
	}
}
