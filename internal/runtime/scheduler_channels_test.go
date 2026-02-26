package runtime

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"deeph/internal/project"
)

type blockingTestProvider struct {
	mu       sync.Mutex
	bRelease chan struct{}
	cStarted chan struct{}
	closedC  bool
}

func newBlockingTestProvider() *blockingTestProvider {
	return &blockingTestProvider{
		bRelease: make(chan struct{}),
		cStarted: make(chan struct{}),
	}
}

func (p *blockingTestProvider) Name() string { return "blocking-test" }

func (p *blockingTestProvider) Generate(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	switch req.AgentName {
	case "a":
		return LLMResponse{Text: "a-done", Provider: p.Name(), Model: req.Model}, nil
	case "b":
		select {
		case <-ctx.Done():
			return LLMResponse{}, ctx.Err()
		case <-p.bRelease:
			return LLMResponse{Text: "b-done", Provider: p.Name(), Model: req.Model}, nil
		}
	case "c":
		p.mu.Lock()
		if !p.closedC {
			close(p.cStarted)
			p.closedC = true
		}
		p.mu.Unlock()
		return LLMResponse{Text: "c-done", Provider: p.Name(), Model: req.Model}, nil
	default:
		return LLMResponse{Text: fmt.Sprintf("%s-done", req.AgentName), Provider: p.Name(), Model: req.Model}, nil
	}
}

func TestRunSpecSelectiveStageWaitWithChannels(t *testing.T) {
	p := &project.Project{
		Root: project.RootConfig{
			Version:         1,
			DefaultProvider: "mockp",
			Providers: []project.ProviderConfig{
				{Name: "mockp", Type: "mock", Model: "mock-small"},
			},
		},
		Agents: []project.AgentConfig{
			{Name: "a"},
			{Name: "b"},
			{
				Name: "c",
				DependsOnPorts: map[string][]string{
					"input": []string{"a"},
				},
				IO: project.AgentIOConfig{
					Inputs: []project.IOPortConfig{
						{Name: "input", Accepts: []string{"text/plain"}},
					},
				},
			},
		},
		AgentFiles: map[string]string{},
	}
	eng, err := New(t.TempDir(), p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	prov := newBlockingTestProvider()
	eng.providers["mockp"] = prov

	done := make(chan error, 1)
	go func() {
		_, err := eng.RunSpec(context.Background(), "a+b>c", "x")
		done <- err
	}()

	select {
	case <-prov.cStarted:
		// success: c started before b was released, meaning selective wait worked.
	case <-time.After(300 * time.Millisecond):
		close(prov.bRelease)
		t.Fatal("c did not start before b completed; scheduler still waiting for full stage barrier")
	}

	close(prov.bRelease)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunSpec returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("RunSpec did not finish after releasing b")
	}
}
