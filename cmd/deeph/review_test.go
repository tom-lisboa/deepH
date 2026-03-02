package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deeph/internal/project"
)

func TestParseReviewUnifiedDiffCapturesFilesAndHunks(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/cmd/deeph/review.go b/cmd/deeph/review.go",
		"index 1111111..2222222 100644",
		"--- a/cmd/deeph/review.go",
		"+++ b/cmd/deeph/review.go",
		"@@ -10,0 +11,2 @@",
		"+line one",
		"+line two",
		"@@ -25,2 +27,0 @@",
		"-gone one",
		"-gone two",
		"diff --git a/old_name.go b/new_name.go",
		"similarity index 98%",
		"rename from old_name.go",
		"rename to new_name.go",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")

	files := parseReviewUnifiedDiff(diff)
	if len(files) != 2 {
		t.Fatalf("files=%d", len(files))
	}
	if files[0].Path != filepath.Clean("cmd/deeph/review.go") {
		t.Fatalf("path=%q", files[0].Path)
	}
	if files[0].Added != 2 || files[0].Deleted != 2 {
		t.Fatalf("unexpected line delta: %+v", files[0])
	}
	if len(files[0].Hunks) != 2 {
		t.Fatalf("hunks=%d", len(files[0].Hunks))
	}
	if files[1].Status != "R" || files[1].OldPath != "old_name.go" || files[1].Path != "new_name.go" {
		t.Fatalf("rename=%+v", files[1])
	}
}

func TestExpandReviewWorkingSetIncludesGoNeighbors(t *testing.T) {
	ws := t.TempDir()
	mustWriteReviewFile(t, filepath.Join(ws, "go.mod"), "module example.com/app\n\ngo 1.24.0\n")
	mustWriteReviewFile(t, filepath.Join(ws, "service", "helpers.go"), "package service\n\nfunc helper() {}\n")
	mustWriteReviewFile(t, filepath.Join(ws, "service", "user.go"), "package service\n\nimport \"example.com/app/internal/store\"\n\nfunc Run() { _ = store.Client{} }\n")
	mustWriteReviewFile(t, filepath.Join(ws, "service", "user_test.go"), "package service\n\nfunc TestRun(t *testing.T) {}\n")
	mustWriteReviewFile(t, filepath.Join(ws, "internal", "store", "store.go"), "package store\n\ntype Client struct{}\n")
	mustWriteReviewFile(t, filepath.Join(ws, "cmd", "app", "main.go"), "package main\n\nimport \"example.com/app/service\"\n\nfunc main() { service.Run() }\n")

	scope := reviewScope{
		Workspace:  ws,
		BaseRef:    "HEAD",
		ModulePath: "example.com/app",
		DiffFiles: []reviewChangedFile{{
			Path:   filepath.Join("service", "user.go"),
			Status: "M",
			Hunks:  []reviewDiffHunk{{NewStart: 1, NewCount: 4}},
		}},
	}

	index, err := buildReviewGoWorkspaceIndex(ws, scope.ModulePath)
	if err != nil {
		t.Fatalf("build review index: %v", err)
	}
	if err := expandReviewWorkingSet(&scope, defaultReviewScopeConfig(), index); err != nil {
		t.Fatalf("expand working set: %v", err)
	}

	paths := make(map[string]string, len(scope.WorkingSet))
	for _, file := range scope.WorkingSet {
		paths[file.Path] = file.Reason
	}
	for path, reason := range map[string]string{
		filepath.Join("service", "user.go"):            "diff",
		filepath.Join("service", "helpers.go"):         "same-package context",
		filepath.Join("service", "user_test.go"):       "same-package test",
		filepath.Join("internal", "store", "store.go"): "local import",
		filepath.Join("cmd", "app", "main.go"):         "reverse local import",
	} {
		got, ok := paths[path]
		if !ok {
			t.Fatalf("missing working-set path %q", path)
		}
		if !strings.Contains(got, reason) {
			t.Fatalf("reason for %q = %q", path, got)
		}
	}
	if scope.Imports != 1 {
		t.Fatalf("imports=%d", scope.Imports)
	}
	if scope.ReverseImports != 1 {
		t.Fatalf("reverse_imports=%d", scope.ReverseImports)
	}
}

func TestBuildReviewInputIncludesScopeAndExcerpt(t *testing.T) {
	ws := t.TempDir()
	rel := filepath.Join("service", "user.go")
	mustWriteReviewFile(t, filepath.Join(ws, rel), "package service\n\nfunc Run() error {\n\treturn nil\n}\n")

	scope := reviewScope{
		Workspace:  ws,
		BaseRef:    "HEAD",
		ModulePath: "example.com/app",
		DiffFiles: []reviewChangedFile{{
			Path:   rel,
			Status: "M",
			Added:  3,
			Hunks:  []reviewDiffHunk{{NewStart: 1, NewCount: 4}},
		}},
		WorkingSet: []reviewWorkingFile{{Path: rel, Reason: "diff"}},
		AddedLines: 3,
		GoChanged:  1,
	}

	got := buildReviewInput(scope, "foque em regressao", defaultReviewScopeConfig())
	for _, want := range []string{
		"[review_scope]",
		"strategy: diff_aware_go",
		"review_focus: foque em regressao",
		"- M service/user.go +3 -0 hunks=1",
		"[excerpt service/user.go]",
		"findings first",
		"1 | package service",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected review input to contain %q, got:\n%s", want, got)
		}
	}
}

func TestResolveDefaultReviewSpecPrefersReviewerThenGuide(t *testing.T) {
	p := &project.Project{
		Agents: []project.AgentConfig{{Name: "guide"}, {Name: "reviewer"}},
	}
	if got := resolveDefaultReviewSpec(p, ""); got != "reviewer" {
		t.Fatalf("spec=%q", got)
	}
	if got := resolveDefaultReviewSpec(&project.Project{Agents: []project.AgentConfig{{Name: "guide"}}}, ""); got != "guide" {
		t.Fatalf("fallback spec=%q", got)
	}
	if got := resolveDefaultReviewSpec(p, "planner>reviewer"); got != "planner>reviewer" {
		t.Fatalf("requested spec=%q", got)
	}
}

func TestBuildReviewGoWorkspaceIndexBuildsReverseImports(t *testing.T) {
	ws := t.TempDir()
	mustWriteReviewFile(t, filepath.Join(ws, "go.mod"), "module example.com/app\n\ngo 1.24.0\n")
	mustWriteReviewFile(t, filepath.Join(ws, "internal", "store", "store.go"), "package store\n")
	mustWriteReviewFile(t, filepath.Join(ws, "service", "user.go"), "package service\n\nimport \"example.com/app/internal/store\"\n\nvar _ = store.Client{}\n")
	mustWriteReviewFile(t, filepath.Join(ws, "cmd", "app", "main.go"), "package main\n\nimport \"example.com/app/service\"\n\nfunc main() {}\n")

	index, err := buildReviewGoWorkspaceIndex(ws, "example.com/app")
	if err != nil {
		t.Fatalf("build review index: %v", err)
	}

	if got := reviewPackageFiles(index, filepath.Join("internal", "store")); len(got) != 1 || got[0] != filepath.Join("internal", "store", "store.go") {
		t.Fatalf("package files=%v", got)
	}
	if got := reviewReverseImportDirs(index, filepath.Join("internal", "store")); len(got) != 1 || got[0] != filepath.Join("service") {
		t.Fatalf("reverse import dirs=%v", got)
	}
	if got := reviewReverseImportDirs(index, filepath.Join("service")); len(got) != 1 || got[0] != filepath.Join("cmd", "app") {
		t.Fatalf("service reverse import dirs=%v", got)
	}
}

func mustWriteReviewFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
