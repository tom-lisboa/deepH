package runtime

import (
	"testing"

	"deeph/internal/project"
)

func TestGRPCStructToLLMResponseParsesTopLevel(t *testing.T) {
	cfg := project.ProviderConfig{Name: "grpc_provider", Model: "fallback-model"}
	req := LLMRequest{Model: "request-model"}
	payload := map[string]any{
		"text":          "hello from grpc",
		"provider":      "remote-provider",
		"model":         "remote-model",
		"finish_reason": "done",
		"meta": map[string]any{
			"provider_type": "grpc",
			"source":        "test",
		},
		"tool_calls": []any{
			map[string]any{
				"id":   "call-1",
				"type": "function",
				"function": map[string]any{
					"name":      "file_read_range",
					"arguments": `{"path":"README.md","start_line":1,"end_line":5}`,
				},
			},
		},
	}

	got := grpcStructToLLMResponse(payload, cfg, req)
	if got.Text != "hello from grpc" {
		t.Fatalf("unexpected text: %q", got.Text)
	}
	if got.Provider != "remote-provider" {
		t.Fatalf("unexpected provider: %q", got.Provider)
	}
	if got.Model != "remote-model" {
		t.Fatalf("unexpected model: %q", got.Model)
	}
	if got.FinishReason != "done" {
		t.Fatalf("unexpected finish reason: %q", got.FinishReason)
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got=%d", len(got.ToolCalls))
	}
	if got.ToolCalls[0].Name != "file_read_range" {
		t.Fatalf("unexpected tool call name: %q", got.ToolCalls[0].Name)
	}
	if got.ToolCalls[0].Arguments == "" {
		t.Fatalf("expected tool call arguments to be present")
	}
	if got.Meta["provider_type"] != "grpc" {
		t.Fatalf("expected provider_type=grpc, got=%v", got.Meta["provider_type"])
	}
}

func TestGRPCStructToLLMResponseParsesNestedFallback(t *testing.T) {
	cfg := project.ProviderConfig{Name: "grpc_provider", Model: "fallback-model"}
	req := LLMRequest{Model: "request-model"}
	payload := map[string]any{
		"response": map[string]any{
			"text":              "nested text",
			"reasoning_content": "thinking",
			"finish_reason":     "stop",
			"tool_calls": []any{
				map[string]any{
					"id":        "call-nested",
					"type":      "function",
					"name":      "echo",
					"arguments": `{"foo":"bar"}`,
				},
			},
		},
	}

	got := grpcStructToLLMResponse(payload, cfg, req)
	if got.Text != "nested text" {
		t.Fatalf("unexpected nested text: %q", got.Text)
	}
	if got.ReasoningContent != "thinking" {
		t.Fatalf("unexpected reasoning content: %q", got.ReasoningContent)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "echo" {
		t.Fatalf("expected nested tool call echo, got=%v", got.ToolCalls)
	}
	if got.Provider != "grpc_provider" {
		t.Fatalf("expected provider fallback from config, got=%q", got.Provider)
	}
	if got.Model != "request-model" {
		t.Fatalf("expected model fallback from request, got=%q", got.Model)
	}
	if got.Meta["provider_type"] != "grpc" {
		t.Fatalf("expected default provider_type=grpc, got=%v", got.Meta["provider_type"])
	}
}

func TestGRPCTargetFromConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  project.ProviderConfig
		want string
	}{
		{
			name: "explicit grpc target host and port",
			cfg:  project.ProviderConfig{GRPCTarget: "llm.internal:50051"},
			want: "llm.internal:50051",
		},
		{
			name: "https base url host fallback",
			cfg:  project.ProviderConfig{BaseURL: "https://api.example.com:443/path"},
			want: "api.example.com:443",
		},
		{
			name: "resolver style target preserved",
			cfg:  project.ProviderConfig{GRPCTarget: "dns:///llm.internal:50051"},
			want: "dns:///llm.internal:50051",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := grpcTargetFromConfig(tc.cfg)
			if got != tc.want {
				t.Fatalf("grpcTargetFromConfig()=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestGRPCShouldUseInsecure(t *testing.T) {
	if !grpcShouldUseInsecure(project.ProviderConfig{}, "127.0.0.1:50051") {
		t.Fatalf("expected loopback to default to insecure")
	}
	if !grpcShouldUseInsecure(project.ProviderConfig{}, "localhost:50051") {
		t.Fatalf("expected localhost to default to insecure")
	}
	if grpcShouldUseInsecure(project.ProviderConfig{}, "api.example.com:443") {
		t.Fatalf("expected non-loopback host to default to tls")
	}
	if !grpcShouldUseInsecure(project.ProviderConfig{GRPCInsecure: true}, "api.example.com:443") {
		t.Fatalf("expected grpc_insecure=true to force insecure mode")
	}
}
