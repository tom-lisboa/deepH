package runtime

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"deeph/internal/project"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
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

type GRPCProvider struct {
	cfg  project.ProviderConfig
	mu   sync.Mutex
	conn *grpc.ClientConn
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

const defaultGRPCProviderMethod = "/deeph.runtime.v1.ProviderService/Generate"

func (p *GRPCProvider) Name() string { return p.cfg.Name }

func (p *GRPCProvider) Generate(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	payload := map[string]any{
		"agent_name":       req.AgentName,
		"agent":            req.AgentName,
		"model":            coalesce(req.Model, p.cfg.Model),
		"system_prompt":    req.SystemPrompt,
		"input":            req.Input,
		"available_skills": req.AvailableSkills,
		"startup_results":  req.StartupResults,
		"messages":         req.Messages,
		"tools":            req.Tools,
		"tool_choice":      req.ToolChoice,
	}
	normalizedPayload, err := normalizeGRPCPayload(payload)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("marshal grpc provider payload: %w", err)
	}
	in, err := structpb.NewStruct(normalizedPayload)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("build grpc provider payload: %w", err)
	}

	conn, err := p.connection(ctx)
	if err != nil {
		return LLMResponse{}, err
	}
	method := strings.TrimSpace(p.cfg.GRPCMethod)
	if method == "" {
		method = defaultGRPCProviderMethod
	}

	callCtx := ctx
	if md := p.outgoingMetadata(); len(md) > 0 {
		callCtx = metadata.NewOutgoingContext(callCtx, md)
	}

	out := &structpb.Struct{}
	if err := conn.Invoke(callCtx, method, in, out); err != nil {
		return LLMResponse{}, fmt.Errorf("grpc provider invoke %s: %w", method, err)
	}
	return grpcStructToLLMResponse(out.AsMap(), p.cfg, req), nil
}

func (p *GRPCProvider) connection(ctx context.Context) (*grpc.ClientConn, error) {
	p.mu.Lock()
	if p.conn != nil {
		conn := p.conn
		p.mu.Unlock()
		return conn, nil
	}
	p.mu.Unlock()

	target := grpcTargetFromConfig(p.cfg)
	if target == "" {
		return nil, fmt.Errorf("grpc provider %q requires grpc_target (or base_url fallback)", p.cfg.Name)
	}

	dialOpts := []grpc.DialOption{grpc.WithBlock()}
	if grpcShouldUseInsecure(p.cfg, target) {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}

	conn, err := grpc.DialContext(ctx, target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("grpc provider dial %q: %w", target, err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		_ = conn.Close()
		return p.conn, nil
	}
	p.conn = conn
	return conn, nil
}

func (p *GRPCProvider) outgoingMetadata() metadata.MD {
	md := metadata.MD{}
	for k, v := range p.cfg.Headers {
		key := strings.ToLower(strings.TrimSpace(k))
		val := strings.TrimSpace(v)
		if key == "" || val == "" {
			continue
		}
		md.Set(key, val)
	}
	if p.cfg.APIKeyEnv != "" {
		if key := strings.TrimSpace(os.Getenv(p.cfg.APIKeyEnv)); key != "" {
			md.Set("authorization", "Bearer "+key)
		}
	}
	return md
}

func normalizeGRPCPayload(payload map[string]any) (map[string]any, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func grpcStructToLLMResponse(payload map[string]any, cfg project.ProviderConfig, req LLMRequest) LLMResponse {
	if payload == nil {
		payload = map[string]any{}
	}

	meta := map[string]any{}
	if rawMeta, ok := anyMap(payload["meta"]); ok {
		for k, v := range rawMeta {
			meta[k] = v
		}
	}
	if providerType := firstString(payload, "provider_type"); providerType != "" {
		meta["provider_type"] = providerType
	}
	if len(meta) == 0 {
		meta["provider_type"] = "grpc"
	}

	text := firstString(payload, "text", "output", "response")
	if text == "" {
		if nested, ok := anyMap(payload["response"]); ok {
			text = firstString(nested, "text", "output")
		}
	}
	model := firstString(payload, "model")
	if model == "" {
		model = coalesce(req.Model, cfg.Model)
	}
	providerName := firstString(payload, "provider")
	if providerName == "" {
		providerName = cfg.Name
	}
	finishReason := firstString(payload, "finish_reason")
	if finishReason == "" {
		finishReason = "stop"
	}
	reasoningContent := firstString(payload, "reasoning_content")
	toolCalls := parseLLMToolCallsFromAny(payload["tool_calls"])
	if len(toolCalls) == 0 {
		if nested, ok := anyMap(payload["response"]); ok {
			toolCalls = parseLLMToolCallsFromAny(nested["tool_calls"])
			if reasoningContent == "" {
				reasoningContent = firstString(nested, "reasoning_content")
			}
			if finishReason == "stop" {
				if nestedFinish := firstString(nested, "finish_reason"); nestedFinish != "" {
					finishReason = nestedFinish
				}
			}
		}
	}

	return LLMResponse{
		Text:             text,
		Provider:         providerName,
		Model:            model,
		Meta:             meta,
		FinishReason:     finishReason,
		ReasoningContent: reasoningContent,
		ToolCalls:        toolCalls,
	}
}

func parseLLMToolCallsFromAny(v any) []LLMToolCall {
	rawList, ok := v.([]any)
	if !ok || len(rawList) == 0 {
		return nil
	}
	out := make([]LLMToolCall, 0, len(rawList))
	for _, raw := range rawList {
		item, ok := anyMap(raw)
		if !ok {
			continue
		}
		tc := LLMToolCall{
			ID:        stringFromAny(item["id"]),
			Type:      coalesce(stringFromAny(item["type"]), "function"),
			Name:      stringFromAny(item["name"]),
			Arguments: stringFromAny(item["arguments"]),
		}
		if fn, ok := anyMap(item["function"]); ok {
			if tc.Name == "" {
				tc.Name = stringFromAny(fn["name"])
			}
			if tc.Arguments == "" {
				tc.Arguments = stringFromAny(fn["arguments"])
			}
		}
		if tc.Name == "" {
			continue
		}
		out = append(out, tc)
	}
	return out
}

func firstString(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(stringFromAny(m[key])); value != "" {
			return value
		}
	}
	return ""
}

func anyMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return nil, false
	}
	return m, true
}

func stringFromAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	case json.Number:
		return x.String()
	case float64:
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.6f", x), "0"), ".")
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func grpcTargetFromConfig(cfg project.ProviderConfig) string {
	target := strings.TrimSpace(cfg.GRPCTarget)
	if target == "" {
		target = strings.TrimSpace(cfg.BaseURL)
	}
	if target == "" {
		return ""
	}

	lower := strings.ToLower(target)
	switch {
	case strings.HasPrefix(lower, "dns:///"),
		strings.HasPrefix(lower, "passthrough:///"),
		strings.HasPrefix(lower, "unix://"),
		strings.HasPrefix(lower, "unix:"):
		return target
	}

	if parsed, err := url.Parse(target); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return target
}

func grpcShouldUseInsecure(cfg project.ProviderConfig, target string) bool {
	if cfg.GRPCInsecure {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(target))
	if strings.HasPrefix(lower, "unix://") || strings.HasPrefix(lower, "unix:") {
		return true
	}
	host := grpcTargetHost(target)
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func grpcTargetHost(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	for _, prefix := range []string{"dns:///", "passthrough:///"} {
		if strings.HasPrefix(strings.ToLower(target), prefix) {
			target = target[len(prefix):]
			break
		}
	}
	if idx := strings.Index(target, "/"); idx >= 0 {
		target = target[:idx]
	}
	if host, _, err := net.SplitHostPort(target); err == nil {
		return strings.Trim(host, "[]")
	}
	if strings.HasPrefix(target, "[") && strings.Contains(target, "]") {
		if idx := strings.Index(target, "]"); idx > 1 {
			return target[1:idx]
		}
	}
	if strings.Count(target, ":") == 1 {
		if idx := strings.LastIndex(target, ":"); idx > 0 {
			return target[:idx]
		}
	}
	return target
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
	case "grpc":
		return &GRPCProvider{cfg: pc}
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
