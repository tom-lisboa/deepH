package runtime

import (
	"strings"
	"testing"

	"deeph/internal/typesys"
)

func TestRecordAgentHandoffMergesByType(t *testing.T) {
	bus := NewContextBus("goal")
	link := TypedHandoffLink{
		FromAgent: "a",
		ToAgent:   "reviewer",
		FromPort:  "summary",
		ToPort:    "brief",
		Kind:      typesys.KindSummaryCode,
	}
	bus.RecordAgentHandoff(link, typesys.TypedValue{
		Kind:       typesys.KindSummaryCode,
		InlineText: "first summary",
	})
	link.FromAgent = "b"
	bus.RecordAgentHandoff(link, typesys.TypedValue{
		Kind:       typesys.KindSummaryCode,
		InlineText: "second summary",
	})

	snap := bus.Snapshot()
	var factValue string
	for _, f := range snap.Facts {
		if f.Key == "handoff.reviewer.brief" {
			factValue = f.Value
			break
		}
	}
	if factValue == "" {
		t.Fatalf("handoff fact not found: %#v", snap.Facts)
	}
	if !strings.Contains(factValue, "from=a") || !strings.Contains(factValue, "from=b") {
		t.Fatalf("expected merged fact to include both upstream agents, got %q", factValue)
	}
	if !strings.Contains(factValue, "||") {
		t.Fatalf("expected merged fact delimiter for summary merge policy, got %q", factValue)
	}
}

func TestRecordAgentHandoffMergePolicyLatestOverridesTypeDefault(t *testing.T) {
	bus := NewContextBus("goal")
	link := TypedHandoffLink{
		FromAgent:       "a",
		ToAgent:         "reviewer",
		FromPort:        "summary",
		ToPort:          "brief",
		Kind:            typesys.KindSummaryCode,
		MergePolicy:     "latest",
		TargetMaxTokens: 0,
	}
	bus.RecordAgentHandoff(link, typesys.TypedValue{
		Kind:       typesys.KindSummaryCode,
		InlineText: "first summary",
	})
	link.FromAgent = "b"
	bus.RecordAgentHandoff(link, typesys.TypedValue{
		Kind:       typesys.KindSummaryCode,
		InlineText: "second summary",
	})

	snap := bus.Snapshot()
	for _, f := range snap.Facts {
		if f.Key != "handoff.reviewer.brief" {
			continue
		}
		if strings.Contains(f.Value, "||") {
			t.Fatalf("latest policy should not append segments, got %q", f.Value)
		}
		if !strings.Contains(f.Value, "from=b") {
			t.Fatalf("latest policy should keep latest upstream, got %q", f.Value)
		}
		return
	}
	t.Fatalf("handoff fact not found")
}

func TestRecordAgentHandoffTargetMaxTokensCapsFact(t *testing.T) {
	bus := NewContextBus("goal")
	longText := strings.Repeat("x", 500)
	link := TypedHandoffLink{
		FromAgent:       "a",
		ToAgent:         "reviewer",
		FromPort:        "notes",
		ToPort:          "brief",
		Kind:            typesys.KindTextPlain,
		MergePolicy:     "latest",
		TargetMaxTokens: 12,
	}
	bus.RecordAgentHandoff(link, typesys.TypedValue{
		Kind:       typesys.KindTextPlain,
		InlineText: longText,
	})
	snap := bus.Snapshot()
	for _, f := range snap.Facts {
		if f.Key != "handoff.reviewer.brief" {
			continue
		}
		// 12 tokens ~= 48 chars, but runtime enforces a floor for usefulness.
		if len(f.Value) > 120 {
			t.Fatalf("expected capped handoff fact, got len=%d value=%q", len(f.Value), f.Value)
		}
		return
	}
	t.Fatalf("handoff fact not found")
}

func TestCompileFiltersHandoffsByTargetAgentAndChannel(t *testing.T) {
	bus := NewContextBus("goal")

	artB := bus.PutScopedArtifact(typesys.KindMessageAgent, ContextMomentSynthesis, "a.output", strings.Repeat("B", 400), "to b", "b", "a.output->b.input#message/agent")
	artC := bus.PutScopedArtifact(typesys.KindMessageAgent, ContextMomentSynthesis, "a.output", strings.Repeat("C", 400), "to c", "c", "a.output->c.input#message/agent")

	bus.RecordAgentHandoff(TypedHandoffLink{
		Channel:   "a.output->b.input#message/agent",
		FromAgent: "a",
		ToAgent:   "b",
		FromPort:  "output",
		ToPort:    "input",
		Kind:      typesys.KindMessageAgent,
	}, typesys.TypedValue{
		Kind:  typesys.KindMessageAgent,
		RefID: artB.ID,
	})

	bus.RecordAgentHandoff(TypedHandoffLink{
		Channel:   "a.output->c.input#message/agent",
		FromAgent: "a",
		ToAgent:   "c",
		FromPort:  "output",
		ToPort:    "input",
		Kind:      typesys.KindMessageAgent,
	}, typesys.TypedValue{
		Kind:  typesys.KindMessageAgent,
		RefID: artC.ID,
	})

	compiledB := bus.Compile(ContextCompileSpec{
		AgentName: "b",
		Budget:    DefaultContextBudget(),
		Moment:    ContextMomentSynthesis,
	})
	if strings.Contains(compiledB.Text, "handoff.c.input") || strings.Contains(compiledB.Text, artC.ID) {
		t.Fatalf("compiled context for b leaked c handoff/artifact: %s", compiledB.Text)
	}
	if !strings.Contains(compiledB.Text, "handoff.b.input") || !strings.Contains(compiledB.Text, artB.ID) {
		t.Fatalf("compiled context for b missing b handoff/artifact: %s", compiledB.Text)
	}

	compiledC := bus.Compile(ContextCompileSpec{
		AgentName: "c",
		Budget:    DefaultContextBudget(),
		Moment:    ContextMomentSynthesis,
	})
	if strings.Contains(compiledC.Text, "handoff.b.input") || strings.Contains(compiledC.Text, artB.ID) {
		t.Fatalf("compiled context for c leaked b handoff/artifact: %s", compiledC.Text)
	}
}

func TestCompileFiltersBySpecificChannelWithinSameTargetAgent(t *testing.T) {
	bus := NewContextBus("goal")

	chBrief := "planner.summary->reviewer.brief#summary/text"
	chCode := "coder.patch->reviewer.source#text/diff"

	bus.RecordAgentHandoff(TypedHandoffLink{
		Channel:   chBrief,
		FromAgent: "planner",
		ToAgent:   "reviewer",
		FromPort:  "summary",
		ToPort:    "brief",
		Kind:      typesys.KindSummaryText,
	}, typesys.TypedValue{
		Kind:       typesys.KindSummaryText,
		InlineText: "Planner brief summary",
	})
	bus.RecordAgentHandoff(TypedHandoffLink{
		Channel:   chCode,
		FromAgent: "coder",
		ToAgent:   "reviewer",
		FromPort:  "patch",
		ToPort:    "source",
		Kind:      typesys.KindTextDiff,
	}, typesys.TypedValue{
		Kind:       typesys.KindTextDiff,
		InlineText: "diff --git a/x b/x",
	})

	compiled := bus.Compile(ContextCompileSpec{
		AgentName: "reviewer",
		Channels:  []string{chBrief},
		Budget:    DefaultContextBudget(),
		Moment:    ContextMomentSynthesis,
	})
	if !strings.Contains(compiled.Text, "handoff.reviewer.brief") {
		t.Fatalf("expected brief handoff in compiled context: %s", compiled.Text)
	}
	if strings.Contains(compiled.Text, "handoff.reviewer.source") {
		t.Fatalf("expected source handoff filtered by channel set: %s", compiled.Text)
	}
	if strings.Contains(compiled.Text, chCode) {
		t.Fatalf("expected disallowed channel id filtered out: %s", compiled.Text)
	}
}
