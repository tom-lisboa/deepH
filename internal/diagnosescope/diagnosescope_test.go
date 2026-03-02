package diagnosescope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildScopeExtractsReferencedGoFileAndSamePackage(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "cmd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mainGo := `package main

func main() {
	panic("boom")
}
`
	helperGo := `package main

func helper() {}
`
	testGo := `package main

func TestMain(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(ws, "cmd", "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "cmd", "helper.go"), []byte(helperGo), 0o644); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "cmd", "main_test.go"), []byte(testGo), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	scope, err := BuildScope(ws, "HEAD", "panic: nil pointer\ncmd/main.go:3", DefaultConfig())
	if err != nil {
		t.Fatalf("build scope: %v", err)
	}
	if len(scope.References) == 0 || scope.References[0].Path != filepath.Clean("cmd/main.go") {
		t.Fatalf("references=%+v", scope.References)
	}
	joined := make([]string, 0, len(scope.WorkingSet))
	for _, wf := range scope.WorkingSet {
		joined = append(joined, wf.Path)
	}
	got := strings.Join(joined, "|")
	for _, want := range []string{"cmd/main.go", "cmd/helper.go", "cmd/main_test.go"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected working set to contain %q, got %q", want, got)
		}
	}
}

func TestBuildInputIncludesIssueAndExcerpt(t *testing.T) {
	scope := Scope{
		Workspace:    "/tmp/ws",
		IssueSummary: "panic in cmd/main.go",
		References:   []Reference{{Path: "cmd/main.go", Line: 10, Source: "issue"}},
		WorkingSet:   []WorkingFile{{Path: "cmd/main.go", Reason: "error reference"}},
	}
	got := BuildInput(scope, "panic: nil pointer\ncmd/main.go:10", DefaultConfig())
	for _, want := range []string{
		"[diagnose_scope]",
		"[issue]",
		"references:",
		"working_set:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected input to contain %q, got:\n%s", want, got)
		}
	}
}
