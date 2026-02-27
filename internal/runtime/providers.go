package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"deeph/internal/project"
)

type MockProvider struct {
	name     string
	typeName string
	model    string
}

func (p *MockProvider) Name() string { return p.name }

func (p *MockProvider) Generate(_ context.Context, req LLMRequest) (LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	if model == "" {
		model = "mock-small"
	}
	parts := []string{
		fmt.Sprintf("agent=%s", req.AgentName),
		fmt.Sprintf("model=%s", model),
	}
	if req.Input != "" {
		parts = append(parts, fmt.Sprintf("input=%q", req.Input))
	}
	if req.SystemPrompt != "" {
		parts = append(parts, fmt.Sprintf("system_prompt=%q", trim(req.SystemPrompt, 90)))
	}
	if len(req.AvailableSkills) > 0 {
		parts = append(parts, fmt.Sprintf("skills=%s", strings.Join(req.AvailableSkills, ",")))
	}
	if len(req.StartupResults) > 0 {
		parts = append(parts, fmt.Sprintf("startup_calls=%d", len(req.StartupResults)))
	}
	if len(req.Tools) > 0 {
		parts = append(parts, fmt.Sprintf("tools=%d", len(req.Tools)))
	}
	return LLMResponse{
		Text:         "[mock-provider] " + strings.Join(parts, " | "),
		Provider:     p.name,
		Model:        model,
		Meta:         map[string]any{"provider_type": p.typeName},
		FinishReason: "stop",
	}, nil
}

type HTTPProvider struct {
	cfg    project.ProviderConfig
	client *http.Client
}

func (p *HTTPProvider) Name() string { return p.cfg.Name }

func (p *HTTPProvider) Generate(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	payload := map[string]any{
		"agent":            req.AgentName,
		"model":            coalesce(req.Model, p.cfg.Model),
		"system_prompt":    req.SystemPrompt,
		"input":            req.Input,
		"available_skills": req.AvailableSkills,
		"startup_results":  req.StartupResults,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("marshal provider payload: %w", err)
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL, bytes.NewReader(b))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("create provider request: %w", err)
	}
	hreq.Header.Set("Content-Type", "application/json")
	for k, v := range p.cfg.Headers {
		hreq.Header.Set(k, v)
	}
	if p.cfg.APIKeyEnv != "" {
		if key := os.Getenv(p.cfg.APIKeyEnv); key != "" {
			hreq.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := p.client.Do(hreq)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("provider http call: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("read provider response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return LLMResponse{}, fmt.Errorf("provider status %d: %s", resp.StatusCode, trim(string(body), 300))
	}
	var generic map[string]any
	if json.Unmarshal(body, &generic) == nil {
		for _, key := range []string{"text", "output", "response"} {
			if s, ok := generic[key].(string); ok {
				return LLMResponse{Text: s, Provider: p.cfg.Name, Model: coalesce(req.Model, p.cfg.Model), Meta: map[string]any{"provider_type": p.cfg.Type}, FinishReason: "stop"}, nil
			}
		}
	}
	return LLMResponse{Text: string(body), Provider: p.cfg.Name, Model: coalesce(req.Model, p.cfg.Model), Meta: map[string]any{"provider_type": p.cfg.Type}, FinishReason: "stop"}, nil
}

type DeepSeekProvider struct {
	cfg    project.ProviderConfig
	client *http.Client
}

func (p *DeepSeekProvider) Name() string { return p.cfg.Name }

func (p *DeepSeekProvider) Generate(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	apiKeyEnv := coalesce(strings.TrimSpace(p.cfg.APIKeyEnv), "DEEPSEEK_API_KEY")
	apiKey := strings.TrimSpace(os.Getenv(apiKeyEnv))
	if apiKey == "" {
		return LLMResponse{}, fmt.Errorf("deepseek provider requires environment variable %s", apiKeyEnv)
	}

	model := coalesce(req.Model, p.cfg.Model, "deepseek-chat")
	payload, err := p.deepSeekPayload(req, model)
	if err != nil {
		return LLMResponse{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("marshal deepseek payload: %w", err)
	}

	url := deepSeekChatCompletionsURL(p.cfg.BaseURL)
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("create deepseek request: %w", err)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+apiKey)
	for k, v := range p.cfg.Headers {
		hreq.Header.Set(k, v)
	}

	resp, err := p.client.Do(hreq)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("deepseek http call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("read deepseek response: %w", err)
	}
	if resp.StatusCode >= 400 {
		raw := trim(string(respBody), 500)
		if hint := deepSeekHTTPErrorHint(resp.StatusCode, apiKeyEnv, apiKey, raw); hint != "" {
			return LLMResponse{}, fmt.Errorf("deepseek status %d: %s | hint: %s", resp.StatusCode, raw, hint)
		}
		return LLMResponse{}, fmt.Errorf("deepseek status %d: %s", resp.StatusCode, raw)
	}

	var out deepSeekChatCompletionResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return LLMResponse{}, fmt.Errorf("parse deepseek response: %w", err)
	}
	if len(out.Choices) == 0 {
		return LLMResponse{}, fmt.Errorf("deepseek response had no choices")
	}
	msg := out.Choices[0].Message
	msgContent := deepSeekMessageContentString(msg.Content)
	msgReasoning := deepSeekOptionalString(msg.ReasoningContent)

	meta := map[string]any{
		"provider_type": "deepseek",
		"finish_reason": out.Choices[0].FinishReason,
	}
	if msgReasoning != "" {
		meta["reasoning_content"] = msgReasoning
	}
	if out.Usage != nil {
		var usage map[string]any
		if json.Unmarshal(out.Usage, &usage) == nil {
			meta["usage"] = usage
		}
	}

	toolCalls := make([]LLMToolCall, 0, len(msg.ToolCalls))
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, LLMToolCall{
			ID:        tc.ID,
			Type:      tc.Type,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return LLMResponse{
		Text:             msgContent,
		Provider:         p.cfg.Name,
		Model:            coalesce(out.Model, model),
		Meta:             meta,
		FinishReason:     out.Choices[0].FinishReason,
		ReasoningContent: msgReasoning,
		ToolCalls:        toolCalls,
	}, nil
}

func (p *DeepSeekProvider) deepSeekPayload(req LLMRequest, model string) (map[string]any, error) {
	var (
		messages []map[string]any
		err      error
	)
	if len(req.Messages) > 0 {
		messages, err = p.deepSeekPayloadMessagesFromChat(req.Messages)
		if err != nil {
			return nil, err
		}
	} else {
		messages, err = p.deepSeekPayloadMessagesFromSimpleRequest(req)
		if err != nil {
			return nil, err
		}
	}

	payload := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}
	if len(req.Tools) > 0 {
		payload["tools"] = deepSeekTools(req.Tools)
		if strings.TrimSpace(req.ToolChoice) != "" {
			payload["tool_choice"] = req.ToolChoice
		} else {
			payload["tool_choice"] = "auto"
		}
	}
	return payload, nil
}

func deepSeekHTTPErrorHint(statusCode int, apiKeyEnv, apiKey, responseBody string) string {
	body := strings.ToLower(strings.TrimSpace(responseBody))
	isAuth := statusCode == http.StatusUnauthorized ||
		strings.Contains(body, "authentication") ||
		strings.Contains(body, "invalid api key") ||
		strings.Contains(body, "api key") && strings.Contains(body, "invalid")
	if !isAuth {
		return ""
	}

	key := strings.TrimSpace(apiKey)
	if looksLikePlaceholderAPIKey(key) {
		return fmt.Sprintf("%s appears to be placeholder text; set a real DeepSeek key (example: set %s=sk-... on Windows CMD)", apiKeyEnv, apiKeyEnv)
	}
	if !strings.HasPrefix(key, "sk-") {
		return fmt.Sprintf("%s does not look like a DeepSeek key (expected prefix sk-)", apiKeyEnv)
	}
	return fmt.Sprintf("check %s value, key status in DeepSeek dashboard, and provider base_url", apiKeyEnv)
}

func looksLikePlaceholderAPIKey(value string) bool {
	v := strings.ToUpper(strings.TrimSpace(value))
	if v == "" {
		return true
	}
	markers := []string{
		"CHAVE", "SUA_CHAVE", "YOUR_KEY", "REAL", "TOKEN", "API_KEY", "EXAMPLE", "PLACEHOLDER",
		"<", ">", "{", "}", "...",
	}
	for _, m := range markers {
		if strings.Contains(v, m) {
			return true
		}
	}
	return false
}

func (p *DeepSeekProvider) deepSeekPayloadMessagesFromSimpleRequest(req LLMRequest) ([]map[string]any, error) {
	messages := make([]map[string]any, 0, 2)
	if strings.TrimSpace(req.SystemPrompt) != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}

	userContent := strings.TrimSpace(req.Input)
	if len(req.StartupResults) > 0 {
		startupJSON, err := json.Marshal(req.StartupResults)
		if err != nil {
			return nil, fmt.Errorf("marshal startup results for deepseek: %w", err)
		}
		if userContent != "" {
			userContent += "\n\n"
		}
		userContent += "[startup_skill_results]\n" + string(startupJSON)
	}
	if userContent == "" {
		userContent = " "
	}
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": userContent,
	})
	return messages, nil
}

func (p *DeepSeekProvider) deepSeekPayloadMessagesFromChat(chat []ChatMessage) ([]map[string]any, error) {
	messages := make([]map[string]any, 0, len(chat))
	for _, m := range chat {
		role := strings.TrimSpace(m.Role)
		if role == "" {
			continue
		}
		msg := map[string]any{
			"role": role,
		}
		if len(m.ToolCalls) > 0 {
			toolCalls := make([]map[string]any, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				toolCalls = append(toolCalls, map[string]any{
					"id":   tc.ID,
					"type": coalesce(tc.Type, "function"),
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": tc.Arguments,
					},
				})
			}
			msg["tool_calls"] = toolCalls
			if m.Content == "" {
				msg["content"] = nil
			} else {
				msg["content"] = m.Content
			}
		} else {
			msg["content"] = m.Content
		}
		if strings.TrimSpace(m.ToolCallID) != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		if role == "assistant" && strings.TrimSpace(m.ReasoningContent) != "" {
			msg["reasoning_content"] = m.ReasoningContent
		}
		if strings.TrimSpace(m.Name) != "" && role != "tool" {
			msg["name"] = m.Name
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

type deepSeekChatCompletionResponse struct {
	Model   string                     `json:"model"`
	Choices []deepSeekChatChoice       `json:"choices"`
	Usage   json.RawMessage            `json:"usage"`
	Error   *deepSeekErrorResponseBody `json:"error,omitempty"`
}

type deepSeekChatChoice struct {
	FinishReason string              `json:"finish_reason"`
	Message      deepSeekChatMessage `json:"message"`
}

type deepSeekChatMessage struct {
	Role             string             `json:"role"`
	Content          any                `json:"content"`
	ReasoningContent *string            `json:"reasoning_content,omitempty"`
	ToolCalls        []deepSeekToolCall `json:"tool_calls,omitempty"`
}

type deepSeekToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function deepSeekToolFunction `json:"function"`
}

type deepSeekToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type deepSeekErrorResponseBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

type StubProvider struct {
	cfg project.ProviderConfig
}

func (p *StubProvider) Name() string { return p.cfg.Name }

func (p *StubProvider) Generate(_ context.Context, _ LLMRequest) (LLMResponse, error) {
	return LLMResponse{}, fmt.Errorf("provider type %q is declared but not implemented in this MVP yet", p.cfg.Type)
}

func newProvider(pc project.ProviderConfig) Provider {
	timeout := time.Duration(pc.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	switch pc.Type {
	case "mock":
		return &MockProvider{name: pc.Name, typeName: pc.Type, model: pc.Model}
	case "http":
		return &HTTPProvider{cfg: pc, client: &http.Client{Timeout: timeout}}
	case "deepseek":
		return &DeepSeekProvider{cfg: pc, client: &http.Client{Timeout: timeout}}
	case "openai", "anthropic", "ollama":
		if pc.BaseURL != "" {
			return &HTTPProvider{cfg: pc, client: &http.Client{Timeout: timeout}}
		}
		return &StubProvider{cfg: pc}
	default:
		return &StubProvider{cfg: pc}
	}
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func trim(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func deepSeekChatCompletionsURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "https://api.deepseek.com/chat/completions"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}
	return baseURL + "/chat/completions"
}

func deepSeekTools(tools []LLMToolDefinition) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		fn := map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.Parameters,
		}
		if t.Strict {
			fn["strict"] = true
		}
		out = append(out, map[string]any{
			"type":     "function",
			"function": fn,
		})
	}
	return out
}

func deepSeekMessageContentString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func deepSeekOptionalString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
