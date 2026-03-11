package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"

	"deeph/internal/project"
)

func TestShouldUseDeepSeekToolLoop(t *testing.T) {
	provider := project.ProviderConfig{Type: "deepseek"}

	if !shouldUseDeepSeekToolLoop(project.AgentConfig{Skills: []string{"echo"}}, provider) {
		t.Fatalf("expected tool loop enabled by default for deepseek + skills")
	}

	if shouldUseDeepSeekToolLoop(project.AgentConfig{
		Skills:   []string{"echo"},
		Metadata: map[string]string{"tool_loop": "off"},
	}, provider) {
		t.Fatalf("expected tool loop disabled with metadata tool_loop=off")
	}

	if shouldUseDeepSeekToolLoop(project.AgentConfig{
		Skills:   []string{"echo"},
		Metadata: map[string]string{"disable_tool_loop": "true"},
	}, provider) {
		t.Fatalf("expected tool loop disabled with metadata disable_tool_loop=true")
	}

	if shouldUseDeepSeekToolLoop(project.AgentConfig{Skills: []string{"echo"}}, project.ProviderConfig{Type: "mock"}) {
		t.Fatalf("expected tool loop disabled for non-deepseek provider")
	}
}

func TestContextMomentForTaskHonorsToolLoopMode(t *testing.T) {
	eng := &Engine{}
	provider := project.ProviderConfig{Type: "deepseek"}

	moment := eng.contextMomentForTask(project.AgentConfig{
		Name:   "coder",
		Skills: []string{"file_read_range"},
	}, provider)
	if moment != ContextMomentToolLoop {
		t.Fatalf("moment=%s want=%s", moment, ContextMomentToolLoop)
	}

	moment = eng.contextMomentForTask(project.AgentConfig{
		Name:     "coder",
		Skills:   []string{"file_read_range"},
		Metadata: map[string]string{"tool_loop": "off"},
	}, provider)
	if moment != ContextMomentDiscovery {
		t.Fatalf("moment=%s want=%s when tool loop is disabled", moment, ContextMomentDiscovery)
	}
}

func TestIsToolCallUnsupportedError(t *testing.T) {
	if !isToolCallUnsupportedError(errors.New(`deepseek status 400: {"error":{"message":"This model does not support function calling"}}`)) {
		t.Fatalf("expected function-calling unsupported message to match")
	}
	if !isToolCallUnsupportedError(errors.New(`provider status 400: unknown parameter: tools`)) {
		t.Fatalf("expected unknown tools parameter message to match")
	}
	if isToolCallUnsupportedError(errors.New("deepseek http call: context deadline exceeded")) {
		t.Fatalf("did not expect network timeout to be considered unsupported tool call")
	}
}

type toolLoopRecordProvider struct {
	mu      sync.Mutex
	records []LLMRequest
}

func (p *toolLoopRecordProvider) Name() string { return "deepseek" }

func (p *toolLoopRecordProvider) Generate(_ context.Context, req LLMRequest) (LLMResponse, error) {
	p.mu.Lock()
	p.records = append(p.records, req)
	p.mu.Unlock()
	return LLMResponse{
		Text:         "ok",
		Provider:     "deepseek",
		Model:        req.Model,
		Meta:         map[string]any{"provider_type": "deepseek"},
		FinishReason: "stop",
	}, nil
}

func (p *toolLoopRecordProvider) snapshot() []LLMRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]LLMRequest, len(p.records))
	copy(out, p.records)
	return out
}

func TestRunSkipsToolLoopWhenAgentDisablesIt(t *testing.T) {
	proj := &project.Project{
		Root: project.RootConfig{
			Version:         1,
			DefaultProvider: "deepseek",
			Providers: []project.ProviderConfig{
				{Name: "deepseek", Type: "deepseek", Model: "deepseek-chat"},
			},
		},
		Agents: []project.AgentConfig{
			{
				Name:     "writer",
				Provider: "deepseek",
				Model:    "deepseek-chat",
				Skills:   []string{"echo"},
				Metadata: map[string]string{"tool_loop": "off"},
			},
		},
		Skills: []project.SkillConfig{
			{Name: "echo", Type: "echo"},
		},
	}
	eng, err := New(t.TempDir(), proj)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	spy := &toolLoopRecordProvider{}
	eng.providers["deepseek"] = spy

	report, err := eng.Run(context.Background(), []string{"writer"}, "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Results) != 1 {
		t.Fatalf("results=%d want=1", len(report.Results))
	}
	if report.Results[0].Error != "" {
		t.Fatalf("unexpected run error: %s", report.Results[0].Error)
	}

	calls := spy.snapshot()
	if len(calls) != 1 {
		t.Fatalf("provider calls=%d want=1 when tool loop is disabled", len(calls))
	}
	if len(calls[0].Tools) != 0 {
		t.Fatalf("expected plain completion call without tools, got tools=%d", len(calls[0].Tools))
	}
}

type toolUnsupportedProvider struct {
	mu      sync.Mutex
	records []LLMRequest
}

func (p *toolUnsupportedProvider) Name() string { return "deepseek" }

func (p *toolUnsupportedProvider) Generate(_ context.Context, req LLMRequest) (LLMResponse, error) {
	p.mu.Lock()
	p.records = append(p.records, req)
	p.mu.Unlock()
	if len(req.Tools) > 0 {
		return LLMResponse{}, errors.New(`deepseek status 400: {"error":{"message":"This model does not support function calling"}}`)
	}
	return LLMResponse{
		Text:         "fallback-ok",
		Provider:     "deepseek",
		Model:        req.Model,
		Meta:         map[string]any{"provider_type": "deepseek"},
		FinishReason: "stop",
	}, nil
}

func (p *toolUnsupportedProvider) snapshot() []LLMRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]LLMRequest, len(p.records))
	copy(out, p.records)
	return out
}

func TestRunFallsBackWhenToolCallsUnsupported(t *testing.T) {
	proj := &project.Project{
		Root: project.RootConfig{
			Version:         1,
			DefaultProvider: "deepseek",
			Providers: []project.ProviderConfig{
				{Name: "deepseek", Type: "deepseek", Model: "deepseek-chat"},
			},
		},
		Agents: []project.AgentConfig{
			{
				Name:     "writer",
				Provider: "deepseek",
				Model:    "deepseek-chat",
				Skills:   []string{"echo"},
			},
		},
		Skills: []project.SkillConfig{
			{Name: "echo", Type: "echo"},
		},
	}
	eng, err := New(t.TempDir(), proj)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stub := &toolUnsupportedProvider{}
	eng.providers["deepseek"] = stub

	report, err := eng.Run(context.Background(), []string{"writer"}, "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Results) != 1 {
		t.Fatalf("results=%d want=1", len(report.Results))
	}
	got := report.Results[0]
	if got.Error != "" {
		t.Fatalf("unexpected run error: %s", got.Error)
	}
	if got.Output != "fallback-ok" {
		t.Fatalf("output=%q want fallback-ok", got.Output)
	}
	if len(got.ToolCalls) != 0 {
		t.Fatalf("expected no executed tool calls on fallback path, got=%d", len(got.ToolCalls))
	}

	calls := stub.snapshot()
	if len(calls) != 2 {
		t.Fatalf("provider calls=%d want=2 (first with tools, then fallback without tools)", len(calls))
	}
	if len(calls[0].Tools) == 0 {
		t.Fatalf("first call should include tools")
	}
	if len(calls[1].Tools) != 0 {
		t.Fatalf("fallback call should not include tools, got=%d", len(calls[1].Tools))
	}
}
