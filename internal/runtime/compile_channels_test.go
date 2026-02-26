package runtime

import (
	"testing"

	"deeph/internal/project"
	"deeph/internal/typesys"
)

func TestSelectTaskCompileChannelsRespectsPriorityAndBudget(t *testing.T) {
	task := Task{
		Agent: project.AgentConfig{
			Name: "reviewer",
			Metadata: map[string]string{
				"context_max_channels": "1",
			},
		},
		Incoming: []TypedHandoffLink{
			{
				Channel:  "planner.summary->reviewer.brief#summary/text",
				ToAgent:  "reviewer",
				ToPort:   "brief",
				Kind:     typesys.KindSummaryText,
				Required: false,
			},
			{
				Channel:         "coder.patch->reviewer.source#text/diff",
				ToAgent:         "reviewer",
				ToPort:          "source",
				Kind:            typesys.KindTextDiff,
				ChannelPriority: 5,
			},
		},
	}

	chs, stats := selectTaskCompileChannels(task, DefaultContextBudget(), ContextMomentValidate)
	if stats.Total != 2 || stats.Selected != 1 || stats.Dropped != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if len(chs) != 1 || chs[0] != "coder.patch->reviewer.source#text/diff" {
		t.Fatalf("expected high-priority channel selected, got %v", chs)
	}
}

func TestSelectTaskCompileChannelsKeepsRequiredPortChannel(t *testing.T) {
	task := Task{
		Agent: project.AgentConfig{
			Name: "reviewer",
			Metadata: map[string]string{
				"context_max_channels": "1",
			},
		},
		Incoming: []TypedHandoffLink{
			{
				Channel:  "planner.summary->reviewer.brief#summary/text",
				ToAgent:  "reviewer",
				ToPort:   "brief",
				Kind:     typesys.KindSummaryText,
				Required: true,
			},
			{
				Channel:         "observer.note->reviewer.brief#summary/text",
				ToAgent:         "reviewer",
				ToPort:          "brief",
				Kind:            typesys.KindSummaryText,
				ChannelPriority: 10,
			},
		},
	}

	chs, stats := selectTaskCompileChannels(task, DefaultContextBudget(), ContextMomentSynthesis)
	if stats.Selected != 1 {
		t.Fatalf("expected one selected channel, got %+v", stats)
	}
	if len(chs) != 1 || chs[0] != "planner.summary->reviewer.brief#summary/text" {
		t.Fatalf("expected required channel to be preserved, got %v", chs)
	}
}
