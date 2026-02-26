package runtime

import (
	"testing"

	"deeph/internal/project"
	"deeph/internal/typesys"
)

func TestPublishTaskOutputsSkipsUnconsumedOutputByDefault(t *testing.T) {
	eng := &Engine{}
	bus := NewContextBus("goal")
	task := Task{
		Agent: project.AgentConfig{
			Name: "solo",
		},
	}
	res := AgentRunResult{Output: "hello world"}

	pub := eng.publishTaskOutputs(bus, task, res)
	if !pub.SkippedUnconsumedOutput {
		t.Fatalf("expected SkippedUnconsumedOutput=true, got %+v", pub)
	}
	if pub.Sent != 0 || pub.Dropped != 0 {
		t.Fatalf("unexpected publish counts: %+v", pub)
	}
	snap := bus.Snapshot()
	for _, ev := range snap.RecentEvents {
		if ev.Type == "agent_output" {
			t.Fatalf("expected no agent_output event when no consumers, got %+v", ev)
		}
	}
}

func TestPublishTaskOutputsRespectsChannelBudgetPriority(t *testing.T) {
	eng := &Engine{}
	bus := NewContextBus("goal")
	task := Task{
		Agent: project.AgentConfig{
			Name: "writer",
			Metadata: map[string]string{
				"publish_max_channels": "1",
			},
		},
		Outgoing: []TypedHandoffLink{
			{
				Channel:   "writer.output->reviewer.brief#summary/text",
				FromAgent: "writer",
				ToAgent:   "reviewer",
				FromPort:  "output",
				ToPort:    "brief",
				Kind:      typesys.KindSummaryText,
				Required:  true,
			},
			{
				Channel:   "writer.output->observer.input#message/agent",
				FromAgent: "writer",
				ToAgent:   "observer",
				FromPort:  "output",
				ToPort:    "input",
				Kind:      typesys.KindMessageAgent,
			},
		},
	}
	res := AgentRunResult{
		Output: "A longer output body that can be summarized and forwarded to multiple consumers.",
	}

	pub := eng.publishTaskOutputs(bus, task, res)
	if pub.Sent != 1 || pub.Dropped != 1 {
		t.Fatalf("expected sent=1 dropped=1, got %+v", pub)
	}
	if pub.Tokens <= 0 {
		t.Fatalf("expected positive publish token accounting, got %+v", pub)
	}

	snap := bus.Snapshot()
	var reviewerFact, observerFact bool
	var observerArtifact bool
	for _, f := range snap.Facts {
		if f.Key == "handoff.reviewer.brief" {
			reviewerFact = true
		}
		if f.Key == "handoff.observer.input" {
			observerFact = true
		}
	}
	if !reviewerFact {
		t.Fatalf("expected required reviewer channel to be published")
	}
	if observerFact {
		t.Fatalf("expected optional observer channel to be dropped by budget")
	}
	for _, a := range snap.Artifacts {
		if a.TargetAgent == "observer" {
			observerArtifact = true
			break
		}
	}
	if observerArtifact {
		t.Fatalf("expected no scoped artifact for dropped observer channel")
	}
}

func TestPublishTaskOutputsRespectsChannelPriorityOverride(t *testing.T) {
	eng := &Engine{}
	bus := NewContextBus("goal")
	task := Task{
		Agent: project.AgentConfig{
			Name: "writer",
			Metadata: map[string]string{
				"publish_max_channels": "1",
			},
		},
		Outgoing: []TypedHandoffLink{
			{
				Channel:   "writer.output->reviewerA.input#message/agent",
				FromAgent: "writer",
				ToAgent:   "reviewerA",
				FromPort:  "output",
				ToPort:    "input",
				Kind:      typesys.KindMessageAgent,
			},
			{
				Channel:         "writer.output->reviewerB.input#message/agent",
				FromAgent:       "writer",
				ToAgent:         "reviewerB",
				FromPort:        "output",
				ToPort:          "input",
				Kind:            typesys.KindMessageAgent,
				ChannelPriority: 5,
			},
		},
	}
	res := AgentRunResult{Output: "Shared output that can go to one consumer under budget."}

	pub := eng.publishTaskOutputs(bus, task, res)
	if pub.Sent != 1 || pub.Dropped != 1 {
		t.Fatalf("expected sent=1 dropped=1, got %+v", pub)
	}

	snap := bus.Snapshot()
	var aFact, bFact bool
	for _, f := range snap.Facts {
		if f.Key == "handoff.reviewerA.input" {
			aFact = true
		}
		if f.Key == "handoff.reviewerB.input" {
			bFact = true
		}
	}
	if aFact {
		t.Fatalf("expected lower-priority reviewerA channel to be dropped")
	}
	if !bFact {
		t.Fatalf("expected higher-priority reviewerB channel to be published")
	}
}
