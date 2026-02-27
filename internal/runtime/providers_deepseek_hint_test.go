package runtime

import (
	"net/http"
	"strings"
	"testing"
)

func TestLooksLikePlaceholderAPIKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		key  string
		want bool
	}{
		{name: "empty", key: "", want: true},
		{name: "placeholder word", key: "sk-CHAVE_NOVA_REAL", want: true},
		{name: "template token", key: "your_key_here", want: true},
		{name: "real-looking key", key: "sk-abc123xyz", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := looksLikePlaceholderAPIKey(tc.key)
			if got != tc.want {
				t.Fatalf("looksLikePlaceholderAPIKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

func TestDeepSeekHTTPErrorHint(t *testing.T) {
	t.Parallel()

	hint := deepSeekHTTPErrorHint(http.StatusUnauthorized, "DEEPSEEK_API_KEY", "sk-CHAVE_NOVA_REAL", `{"error":{"message":"Authentication Fails"}}`)
	if !strings.Contains(hint, "placeholder") {
		t.Fatalf("expected placeholder hint, got %q", hint)
	}

	hint = deepSeekHTTPErrorHint(http.StatusUnauthorized, "DEEPSEEK_API_KEY", "abc", `{"error":{"message":"Authentication Fails"}}`)
	if !strings.Contains(hint, "expected prefix sk-") {
		t.Fatalf("expected prefix hint, got %q", hint)
	}

	hint = deepSeekHTTPErrorHint(http.StatusTooManyRequests, "DEEPSEEK_API_KEY", "sk-abc123", `{"error":{"message":"rate limit"}}`)
	if hint != "" {
		t.Fatalf("expected no hint for non-auth error, got %q", hint)
	}
}
