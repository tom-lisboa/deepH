package reviewfindings

import "testing"

func TestParseJSONReport(t *testing.T) {
	raw := "```json\n{\n" +
		`  "findings": [` + "\n" +
		`    {"severity":"high","file":"cmd/deeph/review.go:77","title":"Explicit --spec still triggers builtin flow","impact":"Prevents a single-agent review path."}` + "\n" +
		"  ],\n" +
		`  "residual_risks": ["No end-to-end reviewflow test"]` + "\n" +
		"}\n```"
	r, ok := Parse(raw)
	if !ok || r == nil {
		t.Fatalf("expected parsed report")
	}
	if r.Format != "json" {
		t.Fatalf("format=%q", r.Format)
	}
	if len(r.Findings) != 1 || r.Findings[0].Severity != "high" {
		t.Fatalf("findings=%#v", r.Findings)
	}
	if len(r.ResidualRisks) != 1 {
		t.Fatalf("residual_risks=%#v", r.ResidualRisks)
	}
}

func TestParseSectionReport(t *testing.T) {
	raw := `
## Findings
- [high] cmd/deeph/review.go: Explicit --spec reviewer still triggers builtin multiverse fallback
  impact: surprises callers and prevents a single-reviewer run
  evidence: happens when no crew exists

## Residual Risks
- missing e2e with real provider
`
	r, ok := Parse(raw)
	if !ok || r == nil {
		t.Fatalf("expected structured report")
	}
	if r.Format != "sections" {
		t.Fatalf("format=%q", r.Format)
	}
	if len(r.Findings) != 1 {
		t.Fatalf("findings=%#v", r.Findings)
	}
	if r.Findings[0].File != "cmd/deeph/review.go:Explicit" && r.Findings[0].File != "cmd/deeph/review.go" {
		t.Fatalf("file=%q", r.Findings[0].File)
	}
	if r.Findings[0].Severity != "high" {
		t.Fatalf("severity=%q", r.Findings[0].Severity)
	}
	if len(r.ResidualRisks) != 1 {
		t.Fatalf("residual_risks=%#v", r.ResidualRisks)
	}
}

func TestParseNoIssuesText(t *testing.T) {
	raw := "No convincing issues found. Residual risk: reviewflow still lacks real-provider coverage."
	r, ok := Parse(raw)
	if !ok || r == nil {
		t.Fatalf("expected no-issues report")
	}
	if !r.NoIssues {
		t.Fatalf("expected no_issues=true")
	}
}

func TestParsePlainFindingsSection(t *testing.T) {
	raw := `
Findings:
- severity: high
  file: cmd/deeph/review.go
  title: Explicit --spec reviewer still triggers builtin fallback
  impact: Prevents a direct single-agent review when requested.

Residual Risks:
- No live-provider end-to-end test yet.
`
	r, ok := Parse(raw)
	if !ok || r == nil {
		t.Fatalf("expected parsed report")
	}
	if len(r.Findings) != 1 {
		t.Fatalf("findings=%#v", r.Findings)
	}
	if r.Findings[0].Severity != "high" {
		t.Fatalf("severity=%q", r.Findings[0].Severity)
	}
	if r.Findings[0].File != "cmd/deeph/review.go" {
		t.Fatalf("file=%q", r.Findings[0].File)
	}
}
