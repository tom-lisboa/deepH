package main

import (
	"strings"
	"testing"
)

func TestParseMultiverseJudgeDecisionJSON(t *testing.T) {
	raw := "```json\n{\n" +
		`  "winner": "u2",` + "\n" +
		`  "rationale": "Better coverage with clear tradeoffs.",` + "\n" +
		`  "differences": ["u1 faster", "u2 more complete"],` + "\n" +
		`  "risks": ["longer runtime"],` + "\n" +
		`  "follow_up": ["run reviewer", "add tests"]` + "\n" +
		"}\n```"

	d, ok := parseMultiverseJudgeDecision(raw)
	if !ok || d == nil {
		t.Fatalf("expected structured decision, got ok=%v", ok)
	}
	if d.Format != "json" {
		t.Fatalf("expected json format, got %q", d.Format)
	}
	if d.Winner != "u2" {
		t.Fatalf("unexpected winner: %q", d.Winner)
	}
	if !strings.Contains(d.Rationale, "coverage") {
		t.Fatalf("unexpected rationale: %q", d.Rationale)
	}
	if len(d.Differences) != 2 {
		t.Fatalf("expected 2 differences, got %d", len(d.Differences))
	}
	if len(d.Risks) != 1 || d.Risks[0] != "longer runtime" {
		t.Fatalf("unexpected risks: %#v", d.Risks)
	}
	if len(d.FollowUp) != 2 {
		t.Fatalf("unexpected follow_up: %#v", d.FollowUp)
	}
}

func TestParseMultiverseJudgeDecisionSections(t *testing.T) {
	raw := `
## Winner
u3

## Rationale
Best balance of speed and correctness.
Clearer diff than the others.

differences:
- u1 is fastest but shallow
- u3 includes validation details

Risks:
1. Might overfit to current repo shape
2. Requires one extra tool call

follow_up:
- run tests
- verify edge cases
`
	d, ok := parseMultiverseJudgeDecision(raw)
	if !ok || d == nil {
		t.Fatalf("expected structured decision, got ok=%v", ok)
	}
	if d.Format != "sections" {
		t.Fatalf("expected sections format, got %q", d.Format)
	}
	if d.Winner != "u3" {
		t.Fatalf("unexpected winner: %q", d.Winner)
	}
	if !strings.Contains(d.Rationale, "speed and correctness") || !strings.Contains(d.Rationale, "Clearer diff") {
		t.Fatalf("unexpected rationale: %q", d.Rationale)
	}
	if got := len(d.Differences); got != 2 {
		t.Fatalf("expected 2 differences, got %d (%#v)", got, d.Differences)
	}
	if got := len(d.Risks); got != 2 {
		t.Fatalf("expected 2 risks, got %d (%#v)", got, d.Risks)
	}
	if got := len(d.FollowUp); got != 2 {
		t.Fatalf("expected 2 follow_up, got %d (%#v)", got, d.FollowUp)
	}
}

func TestParseMultiverseJudgeDecisionIgnoresNoiseAndFindsInlineJSON(t *testing.T) {
	raw := `
Some explanation before.

Result:
{"winner":"baseline","rationale":"Most stable","risks":["slower"],"follow_up":["benchmark"]}

Thanks.
`
	d, ok := parseMultiverseJudgeDecision(raw)
	if !ok || d == nil {
		t.Fatalf("expected parser to find inline JSON object")
	}
	if d.Winner != "baseline" {
		t.Fatalf("unexpected winner: %q", d.Winner)
	}
	if len(d.Risks) != 1 || d.Risks[0] != "slower" {
		t.Fatalf("unexpected risks: %#v", d.Risks)
	}
}
