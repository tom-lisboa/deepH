package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"deeph/internal/project"
	"deeph/internal/runtime"
)

type reviewScopeConfig struct {
	MaxInputChars            int
	MaxExcerptChars          int
	MaxChangedExcerptFiles   int
	MaxWorkingSetFiles       int
	MaxSamePackageFiles      int
	MaxSamePackageTests      int
	MaxImportedPackages      int
	MaxImportedPackageFiles  int
	MaxReverseImportPackages int
	MaxReverseImportFiles    int
}

type reviewScope struct {
	Workspace      string              `json:"workspace"`
	BaseRef        string              `json:"base_ref"`
	ModulePath     string              `json:"module_path,omitempty"`
	DiffFiles      []reviewChangedFile `json:"diff_files"`
	WorkingSet     []reviewWorkingFile `json:"working_set"`
	AddedLines     int                 `json:"added_lines"`
	DeletedLines   int                 `json:"deleted_lines"`
	GoChanged      int                 `json:"go_changed"`
	SamePackage    int                 `json:"same_package"`
	TestFiles      int                 `json:"test_files"`
	Imports        int                 `json:"imports"`
	ReverseImports int                 `json:"reverse_imports"`
}

type reviewChangedFile struct {
	Path    string           `json:"path"`
	OldPath string           `json:"old_path,omitempty"`
	Status  string           `json:"status"`
	Added   int              `json:"added"`
	Deleted int              `json:"deleted"`
	Hunks   []reviewDiffHunk `json:"hunks,omitempty"`
}

type reviewDiffHunk struct {
	OldStart int `json:"old_start"`
	OldCount int `json:"old_count"`
	NewStart int `json:"new_start"`
	NewCount int `json:"new_count"`
}

type reviewWorkingFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type reviewJSONPayload struct {
	Spec         string      `json:"spec"`
	PromptTokens int         `json:"prompt_tokens_estimate"`
	Scope        reviewScope `json:"scope"`
	Input        string      `json:"input"`
}

type reviewPromptBuilder struct {
	max int
	b   strings.Builder
}

type reviewGoWorkspaceIndex struct {
	PackageFiles   map[string][]string
	ReverseImports map[string][]string
}

var reviewHunkPattern = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func defaultReviewScopeConfig() reviewScopeConfig {
	return reviewScopeConfig{
		MaxInputChars:            3600,
		MaxExcerptChars:          420,
		MaxChangedExcerptFiles:   5,
		MaxWorkingSetFiles:       14,
		MaxSamePackageFiles:      2,
		MaxSamePackageTests:      2,
		MaxImportedPackages:      3,
		MaxImportedPackageFiles:  1,
		MaxReverseImportPackages: 3,
		MaxReverseImportFiles:    1,
	}
}

func cmdReview(args []string) error {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	spec := fs.String("spec", "", "agent spec used for the review (defaults to reviewer, then guide)")
	baseRef := fs.String("base", "HEAD", "git base ref used for diff-aware review")
	showTrace := fs.Bool("trace", false, "print review scope summary before running")
	showCoach := fs.Bool("coach", true, "show occasional semantic tips while waiting")
	jsonOut := fs.Bool("json", false, "print diff-aware review payload as JSON instead of running")
	if err := fs.Parse(args); err != nil {
		return err
	}
	focus := strings.TrimSpace(strings.Join(fs.Args(), " "))

	p, abs, verr, err := loadAndValidate(*workspace)
	if err != nil {
		return err
	}
	printValidation(verr)
	if verr != nil && verr.HasErrors() {
		return verr
	}

	cfg := defaultReviewScopeConfig()
	scope, err := buildReviewScope(abs, strings.TrimSpace(*baseRef), cfg)
	if err != nil {
		return err
	}
	input := buildReviewInput(scope, focus, cfg)
	selectedSpec := resolveDefaultReviewSpec(p, strings.TrimSpace(*spec))
	promptTokens := estimateReviewTokens(input)

	if *jsonOut {
		payload := reviewJSONPayload{
			Spec:         selectedSpec,
			PromptTokens: promptTokens,
			Scope:        scope,
			Input:        input,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	eng, err := runtime.New(abs, p)
	if err != nil {
		return err
	}
	resolvedSpec, _, err := resolveAgentSpecOrCrew(abs, selectedSpec)
	if err != nil {
		return err
	}
	ctx := context.Background()
	plan, tasks, err := eng.PlanSpec(ctx, resolvedSpec, input)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "review", resolvedSpec)
	if *showTrace {
		printReviewScope(scope, resolvedSpec, promptTokens)
		printCompactChatPlan(plan, chatSinkTaskIndexes(tasks))
	}
	stopCoach := func() {}
	if *showCoach {
		stopCoach = startCoachHint(ctx, coachHintRequest{
			Workspace:   abs,
			CommandPath: "review",
			AgentSpec:   resolvedSpec,
			Input:       input,
			Plan:        &plan,
			Tasks:       tasks,
			ShowTrace:   *showTrace,
		})
	}
	report, err := eng.RunSpec(ctx, resolvedSpec, input)
	stopCoach()
	if err != nil {
		return err
	}
	recordCoachRunSignals(abs, &plan, report)
	fmt.Printf("Review started=%s base=%q changed=%d working_set=%d prompt=%dt spec=%q\n", report.StartedAt.Format(time.RFC3339), scope.BaseRef, len(scope.DiffFiles), len(scope.WorkingSet), promptTokens, resolvedSpec)
	printExecutionReport(report)
	fmt.Printf("\nFinished in %s\n", report.EndedAt.Sub(report.StartedAt).Round(time.Millisecond))
	if *showCoach {
		maybePrintCoachPostRunHint(abs, "review", &plan, report)
	}
	saveStudioRecent(abs, resolvedSpec, "")
	return nil
}

func resolveDefaultReviewSpec(p *project.Project, requested string) string {
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested)
	}
	for _, candidate := range []string{"reviewer", "guide"} {
		for _, agent := range p.Agents {
			if strings.EqualFold(strings.TrimSpace(agent.Name), candidate) {
				return agent.Name
			}
		}
	}
	return "reviewer"
}

func buildReviewScope(workspace, baseRef string, cfg reviewScopeConfig) (reviewScope, error) {
	modulePath := readGoModulePath(workspace)
	goIndex, err := buildReviewGoWorkspaceIndex(workspace, modulePath)
	if err != nil {
		return reviewScope{}, err
	}
	diffText, effectiveBase, err := gitRelativeDiff(workspace, baseRef)
	if err != nil {
		return reviewScope{}, err
	}
	changed := parseReviewUnifiedDiff(diffText)
	untracked, err := gitUntrackedFiles(workspace)
	if err != nil {
		return reviewScope{}, err
	}
	seen := make(map[string]struct{}, len(changed))
	scope := reviewScope{
		Workspace:  workspace,
		BaseRef:    effectiveBase,
		ModulePath: modulePath,
		DiffFiles:  changed,
	}
	for _, file := range scope.DiffFiles {
		seen[file.Path] = struct{}{}
		scope.AddedLines += file.Added
		scope.DeletedLines += file.Deleted
		if isGoSourcePath(file.Path) {
			scope.GoChanged++
		}
	}
	for _, path := range untracked {
		if _, ok := seen[path]; ok {
			continue
		}
		scope.DiffFiles = append(scope.DiffFiles, reviewChangedFile{Path: path, Status: "?"})
		seen[path] = struct{}{}
		if isGoSourcePath(path) {
			scope.GoChanged++
		}
	}
	if len(scope.DiffFiles) == 0 {
		return reviewScope{}, errors.New("review requires local git changes or untracked files")
	}
	if err := expandReviewWorkingSet(&scope, cfg, goIndex); err != nil {
		return reviewScope{}, err
	}
	return scope, nil
}

func gitRelativeDiff(workspace, baseRef string) (diffText string, effectiveBase string, err error) {
	baseRef = strings.TrimSpace(baseRef)
	if baseRef == "" {
		baseRef = "HEAD"
	}
	args := []string{"diff", "--no-ext-diff", "--unified=0", "--relative"}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	args = append(args, "--")
	out, runErr := runGitCapture(workspace, args...)
	if runErr == nil {
		return out, baseRef, nil
	}
	if baseRef != "HEAD" {
		return "", "", runErr
	}
	fallback, fallbackErr := runGitCapture(workspace, "diff", "--no-ext-diff", "--unified=0", "--relative", "--")
	if fallbackErr != nil {
		return "", "", runErr
	}
	return fallback, "working-tree", nil
}

func gitUntrackedFiles(workspace string) ([]string, error) {
	out, err := runGitCapture(workspace, "ls-files", "--others", "--exclude-standard", "--")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, filepath.Clean(line))
	}
	return files, nil
}

func runGitCapture(workspace string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", workspace}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func parseReviewUnifiedDiff(diffText string) []reviewChangedFile {
	lines := strings.Split(strings.ReplaceAll(diffText, "\r\n", "\n"), "\n")
	out := make([]reviewChangedFile, 0)
	var cur *reviewChangedFile
	flush := func() {
		if cur == nil {
			return
		}
		cur.Path = filepath.Clean(strings.TrimSpace(cur.Path))
		cur.OldPath = filepath.Clean(strings.TrimSpace(cur.OldPath))
		if cur.Path == "." || cur.Path == "" {
			if cur.OldPath != "" && cur.OldPath != "." {
				cur.Path = cur.OldPath
			}
		}
		if cur.Path != "" && cur.Path != "." {
			out = append(out, *cur)
		}
		cur = nil
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			oldPath, newPath := parseReviewDiffPaths(line)
			cur = &reviewChangedFile{Path: newPath, OldPath: oldPath, Status: "M"}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "new file mode "):
			cur.Status = "A"
		case strings.HasPrefix(line, "deleted file mode "):
			cur.Status = "D"
		case strings.HasPrefix(line, "rename from "):
			cur.Status = "R"
			cur.OldPath = filepath.Clean(strings.TrimSpace(strings.TrimPrefix(line, "rename from ")))
		case strings.HasPrefix(line, "rename to "):
			cur.Path = filepath.Clean(strings.TrimSpace(strings.TrimPrefix(line, "rename to ")))
		case strings.HasPrefix(line, "--- "):
			path := trimReviewDiffPath(strings.TrimSpace(strings.TrimPrefix(line, "--- ")))
			if path != "" {
				cur.OldPath = path
			}
		case strings.HasPrefix(line, "+++ "):
			path := trimReviewDiffPath(strings.TrimSpace(strings.TrimPrefix(line, "+++ ")))
			if path != "" {
				cur.Path = path
			}
		case strings.HasPrefix(line, "@@ "):
			if hunk, ok := parseReviewHunk(line); ok {
				cur.Hunks = append(cur.Hunks, hunk)
			}
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			cur.Added++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			cur.Deleted++
		}
	}
	flush()
	return out
}

func parseReviewDiffPaths(line string) (oldPath, newPath string) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return "", ""
	}
	return trimReviewDiffPath(fields[2]), trimReviewDiffPath(fields[3])
}

func trimReviewDiffPath(raw string) string {
	raw = strings.TrimSpace(raw)
	switch raw {
	case "", "/dev/null":
		return ""
	}
	raw = strings.TrimPrefix(raw, "a/")
	raw = strings.TrimPrefix(raw, "b/")
	return filepath.Clean(raw)
}

func parseReviewHunk(line string) (reviewDiffHunk, bool) {
	m := reviewHunkPattern.FindStringSubmatch(line)
	if len(m) == 0 {
		return reviewDiffHunk{}, false
	}
	parseCount := func(s string) int {
		if strings.TrimSpace(s) == "" {
			return 1
		}
		v, err := strconv.Atoi(s)
		if err != nil {
			return 1
		}
		return v
	}
	oldStart, _ := strconv.Atoi(m[1])
	newStart, _ := strconv.Atoi(m[3])
	return reviewDiffHunk{
		OldStart: oldStart,
		OldCount: parseCount(m[2]),
		NewStart: newStart,
		NewCount: parseCount(m[4]),
	}, true
}

func expandReviewWorkingSet(scope *reviewScope, cfg reviewScopeConfig, goIndex *reviewGoWorkspaceIndex) error {
	if scope == nil {
		return nil
	}
	indexByPath := map[string]int{}
	hasWorking := func(path string) bool {
		path = filepath.Clean(strings.TrimSpace(path))
		_, ok := indexByPath[path]
		return ok
	}
	addWorking := func(path, reason string, force bool) bool {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" || path == "." {
			return false
		}
		if idx, ok := indexByPath[path]; ok {
			scope.WorkingSet[idx].Reason = mergeReviewReasons(scope.WorkingSet[idx].Reason, reason)
			return false
		}
		if !force && cfg.MaxWorkingSetFiles > 0 && len(scope.WorkingSet) >= cfg.MaxWorkingSetFiles {
			return false
		}
		indexByPath[path] = len(scope.WorkingSet)
		scope.WorkingSet = append(scope.WorkingSet, reviewWorkingFile{Path: path, Reason: reason})
		return true
	}

	for _, file := range scope.DiffFiles {
		addWorking(file.Path, "diff", true)
	}
	for _, file := range scope.DiffFiles {
		if !isGoSourcePath(file.Path) || file.Status == "D" {
			continue
		}
		if err := expandGoReviewContext(scope, file, cfg, addWorking, hasWorking, goIndex); err != nil {
			return err
		}
	}
	return nil
}

func expandGoReviewContext(scope *reviewScope, changed reviewChangedFile, cfg reviewScopeConfig, addWorking func(path, reason string, force bool) bool, hasWorking func(path string) bool, goIndex *reviewGoWorkspaceIndex) error {
	if scope == nil {
		return nil
	}
	abs := filepath.Join(scope.Workspace, changed.Path)
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return nil
	}

	relDir := filepath.Clean(filepath.Dir(changed.Path))
	if relDir == "" {
		relDir = "."
	}
	absPackageDir := filepath.Join(scope.Workspace, relDir)
	siblings := reviewPackageFiles(goIndex, relDir)
	contextAdded := 0
	testsAdded := 0
	baseName := strings.TrimSuffix(filepath.Base(changed.Path), "_test.go")
	for _, rel := range siblings {
		if rel == changed.Path {
			continue
		}
		name := filepath.Base(rel)
		switch {
		case strings.HasSuffix(name, "_test.go"):
			if strings.HasSuffix(changed.Path, "_test.go") {
				continue
			}
			if testsAdded >= cfg.MaxSamePackageTests {
				continue
			}
			if addWorking(rel, "same-package test", false) {
				testsAdded++
				scope.TestFiles++
			}
		case strings.HasSuffix(changed.Path, "_test.go") && strings.TrimSuffix(name, ".go") == baseName:
			if addWorking(rel, "paired source", false) {
				scope.SamePackage++
			}
		default:
			if contextAdded >= cfg.MaxSamePackageFiles {
				continue
			}
			if addWorking(rel, "same-package context", false) {
				contextAdded++
				scope.SamePackage++
			}
		}
	}

	imports, err := readGoImports(abs)
	if err != nil {
		return nil
	}
	importedPackages := 0
	for _, imp := range imports {
		if importedPackages >= cfg.MaxImportedPackages {
			break
		}
		relDir, ok := localImportDir(scope.ModulePath, imp)
		if !ok {
			continue
		}
		absDir := filepath.Join(scope.Workspace, relDir)
		if filepath.Clean(absDir) == filepath.Clean(absPackageDir) {
			continue
		}
		files, err := os.ReadDir(absDir)
		if err != nil {
			continue
		}
		addedForPackage := 0
		candidates := make([]string, 0, len(files))
		for _, entry := range files {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			candidates = append(candidates, entry.Name())
		}
		sort.Strings(candidates)
		for _, name := range candidates {
			if addedForPackage >= cfg.MaxImportedPackageFiles {
				break
			}
			if addWorking(filepath.Join(relDir, name), "local import", false) {
				addedForPackage++
				scope.Imports++
			}
		}
		if addedForPackage > 0 {
			importedPackages++
		}
	}
	importerDirs := reviewReverseImportDirs(goIndex, relDir)
	reversePackages := 0
	for _, importerDir := range importerDirs {
		if reversePackages >= cfg.MaxReverseImportPackages {
			break
		}
		if importerDir == relDir {
			continue
		}
		files := reviewPackageFiles(goIndex, importerDir)
		addedForPackage := 0
		for _, rel := range files {
			if strings.HasSuffix(rel, "_test.go") {
				continue
			}
			if hasWorking != nil && hasWorking(rel) {
				continue
			}
			if addWorking(rel, "reverse local import", false) {
				addedForPackage++
				scope.ReverseImports++
			}
			if addedForPackage >= cfg.MaxReverseImportFiles {
				break
			}
		}
		if addedForPackage > 0 {
			reversePackages++
		}
	}
	return nil
}

func buildReviewGoWorkspaceIndex(workspace, modulePath string) (*reviewGoWorkspaceIndex, error) {
	index := &reviewGoWorkspaceIndex{
		PackageFiles:   map[string][]string{},
		ReverseImports: map[string][]string{},
	}
	modulePath = strings.TrimSpace(modulePath)
	err := filepath.WalkDir(workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "dist", "sessions":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		rel = filepath.Clean(rel)
		dir := filepath.Clean(filepath.Dir(rel))
		if dir == "" {
			dir = "."
		}
		index.PackageFiles[dir] = append(index.PackageFiles[dir], rel)
		if modulePath == "" {
			return nil
		}
		imports, err := readGoImports(path)
		if err != nil {
			return nil
		}
		seenImports := make(map[string]struct{}, len(imports))
		for _, imp := range imports {
			targetDir, ok := localImportDir(modulePath, imp)
			if !ok {
				continue
			}
			targetDir = filepath.Clean(targetDir)
			if targetDir == "" {
				targetDir = "."
			}
			if targetDir == dir {
				continue
			}
			if _, ok := seenImports[targetDir]; ok {
				continue
			}
			seenImports[targetDir] = struct{}{}
			index.ReverseImports[targetDir] = append(index.ReverseImports[targetDir], dir)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for dir := range index.PackageFiles {
		sort.Strings(index.PackageFiles[dir])
	}
	for dir, importers := range index.ReverseImports {
		sort.Strings(importers)
		index.ReverseImports[dir] = dedupeSortedStrings(importers)
	}
	return index, nil
}

func reviewPackageFiles(index *reviewGoWorkspaceIndex, relDir string) []string {
	relDir = filepath.Clean(strings.TrimSpace(relDir))
	if relDir == "" {
		relDir = "."
	}
	if index != nil {
		if files := index.PackageFiles[relDir]; len(files) > 0 {
			return append([]string(nil), files...)
		}
	}
	return nil
}

func reviewReverseImportDirs(index *reviewGoWorkspaceIndex, relDir string) []string {
	relDir = filepath.Clean(strings.TrimSpace(relDir))
	if relDir == "" {
		relDir = "."
	}
	if index != nil {
		if dirs := index.ReverseImports[relDir]; len(dirs) > 0 {
			return append([]string(nil), dirs...)
		}
	}
	return nil
}

func dedupeSortedStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := items[:1]
	last := items[0]
	for _, item := range items[1:] {
		if item == last {
			continue
		}
		out = append(out, item)
		last = item
	}
	return out
}

func readGoImports(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(file.Imports))
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(strings.TrimSpace(imp.Path.Value))
		if err != nil || value == "" {
			continue
		}
		out = append(out, value)
	}
	return out, nil
}

func localImportDir(modulePath, importPath string) (string, bool) {
	modulePath = strings.TrimSpace(modulePath)
	importPath = strings.TrimSpace(importPath)
	if modulePath == "" || importPath == "" {
		return "", false
	}
	if importPath == modulePath {
		return ".", true
	}
	if !strings.HasPrefix(importPath, modulePath+"/") {
		return "", false
	}
	rel := strings.TrimPrefix(importPath, modulePath+"/")
	rel = filepath.FromSlash(rel)
	if strings.TrimSpace(rel) == "" {
		return ".", true
	}
	return filepath.Clean(rel), true
}

func readGoModulePath(workspace string) string {
	b, err := os.ReadFile(filepath.Join(workspace, "go.mod"))
	if err != nil {
		return ""
	}
	for _, raw := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func isGoSourcePath(path string) bool {
	return strings.HasSuffix(strings.TrimSpace(path), ".go")
}

func mergeReviewReasons(existing, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if existing == "" {
		return incoming
	}
	if incoming == "" || existing == incoming {
		return existing
	}
	parts := strings.Split(existing, ", ")
	for _, part := range parts {
		if part == incoming {
			return existing
		}
	}
	return existing + ", " + incoming
}

func buildReviewInput(scope reviewScope, focus string, cfg reviewScopeConfig) string {
	p := &reviewPromptBuilder{max: cfg.MaxInputChars}
	p.addLine("[review_scope]")
	p.addLine("strategy: diff_aware_go")
	p.addLine("workspace: " + scope.Workspace)
	p.addLine("base_ref: " + scope.BaseRef)
	if scope.ModulePath != "" {
		p.addLine("module: " + scope.ModulePath)
	}
	p.addLine(fmt.Sprintf("changed_files: %d", len(scope.DiffFiles)))
	p.addLine(fmt.Sprintf("working_set_files: %d", len(scope.WorkingSet)))
	p.addLine(fmt.Sprintf("line_delta: +%d -%d", scope.AddedLines, scope.DeletedLines))
	if strings.TrimSpace(focus) != "" {
		p.addLine("review_focus: " + clipLine(focus, 220))
	}
	p.addLine("instruction: findings first. prioritize bugs, regressions, missing tests, concurrency, context cancellation, nil/pointer mistakes, API drift, resource leaks, and risky assumptions. cite file paths and explain impact. if no issues, say that explicitly and mention residual risks.")
	p.addLine("changed:")
	for _, file := range scope.DiffFiles {
		line := fmt.Sprintf("- %s %s +%d -%d hunks=%d", file.Status, file.Path, file.Added, file.Deleted, len(file.Hunks))
		if !p.addLine(line) {
			break
		}
	}
	p.addLine("working_set:")
	for _, file := range scope.WorkingSet {
		line := fmt.Sprintf("- %s reason=%s", file.Path, file.Reason)
		if !p.addLine(line) {
			break
		}
	}

	changedByPath := make(map[string]reviewChangedFile, len(scope.DiffFiles))
	for _, file := range scope.DiffFiles {
		changedByPath[file.Path] = file
	}
	excerptsAdded := 0
	for _, file := range scope.WorkingSet {
		if excerptsAdded >= cfg.MaxChangedExcerptFiles {
			break
		}
		changed, ok := changedByPath[file.Path]
		if !ok {
			continue
		}
		excerpt := buildReviewExcerpt(scope.Workspace, file.Path, changed.Hunks, cfg.MaxExcerptChars)
		if strings.TrimSpace(excerpt) == "" {
			continue
		}
		if !p.addBlock(fmt.Sprintf("[excerpt %s]", file.Path), excerpt) {
			break
		}
		excerptsAdded++
	}
	return strings.TrimSpace(p.String())
}

func buildReviewExcerpt(workspace, relPath string, hunks []reviewDiffHunk, maxChars int) string {
	abs := filepath.Join(workspace, relPath)
	b, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "(file no longer exists in working tree)"
		}
		return ""
	}
	if strings.IndexByte(string(b), 0) >= 0 {
		return "(binary or non-text file)"
	}
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(string(b), "\r\n", "\n"), "\r", "\n"), "\n")
	windows := make([][2]int, 0, len(hunks))
	for _, hunk := range hunks {
		startLine := max(1, hunk.NewStart-3)
		endLine := hunk.NewStart + max(hunk.NewCount, 1) + 2
		windows = append(windows, [2]int{startLine - 1, min(endLine-1, max(len(lines)-1, 0))})
	}
	if len(windows) == 0 {
		limit := min(len(lines)-1, 24)
		windows = append(windows, [2]int{0, max(limit, 0)})
	}
	windows = mergeReviewWindows(windows)
	width := len(strconv.Itoa(len(lines)))
	if width < 2 {
		width = 2
	}

	var out strings.Builder
	lastEnd := -1
	for _, win := range windows {
		if lastEnd >= 0 && win[0] > lastEnd+1 {
			out.WriteString("...\n")
		}
		for i := win[0]; i <= win[1] && i < len(lines); i++ {
			fmt.Fprintf(&out, "%*d | %s\n", width, i+1, lines[i])
		}
		lastEnd = win[1]
		if out.Len() >= maxChars {
			break
		}
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return ""
	}
	if maxChars > 0 && len(text) > maxChars {
		return strings.TrimSpace(text[:maxChars-3]) + "..."
	}
	return text
}

func mergeReviewWindows(windows [][2]int) [][2]int {
	if len(windows) == 0 {
		return nil
	}
	sort.Slice(windows, func(i, j int) bool {
		if windows[i][0] == windows[j][0] {
			return windows[i][1] < windows[j][1]
		}
		return windows[i][0] < windows[j][0]
	})
	out := make([][2]int, 0, len(windows))
	cur := windows[0]
	for _, win := range windows[1:] {
		if win[0] <= cur[1]+1 {
			if win[1] > cur[1] {
				cur[1] = win[1]
			}
			continue
		}
		out = append(out, cur)
		cur = win
	}
	out = append(out, cur)
	return out
}

func estimateReviewTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return max(1, (len(text)+3)/4)
}

func printReviewScope(scope reviewScope, spec string, promptTokens int) {
	fmt.Printf("Review scope (%s)\n", scope.Workspace)
	fmt.Printf("  spec: %q\n", spec)
	fmt.Printf("  base_ref: %s\n", scope.BaseRef)
	fmt.Printf("  changed_files: %d (+%d -%d) go=%d\n", len(scope.DiffFiles), scope.AddedLines, scope.DeletedLines, scope.GoChanged)
	fmt.Printf("  working_set: %d (same_package=%d tests=%d imports=%d reverse_imports=%d)\n", len(scope.WorkingSet), scope.SamePackage, scope.TestFiles, scope.Imports, scope.ReverseImports)
	fmt.Printf("  prompt_estimate: %dt\n", promptTokens)
	for _, file := range scope.DiffFiles {
		fmt.Printf("  diff: %s %s +%d -%d hunks=%d\n", file.Status, file.Path, file.Added, file.Deleted, len(file.Hunks))
	}
}

func printExecutionReport(report runtime.ExecutionReport) {
	for _, r := range report.Results {
		fmt.Printf("\n[%s] stage=%d provider=%s(%s) model=%s duration=%s context=%d/%dt dropped=%d version=%d moment=%s\n", r.Agent, r.StageIndex, r.Provider, r.ProviderType, r.Model, r.Duration.Round(time.Millisecond), r.ContextTokens, r.ContextBudget, r.ContextDropped, r.ContextVersion, r.ContextMoment)
		if len(r.DependsOn) > 0 {
			fmt.Printf("  depends_on=%v\n", r.DependsOn)
		}
		if r.ContextChannelsTotal > 0 {
			fmt.Printf("  context_channels=%d/%d dropped=%d\n", r.ContextChannelsUsed, r.ContextChannelsTotal, r.ContextChannelsDropped)
		}
		if r.ToolCacheHits > 0 || r.ToolCacheMisses > 0 {
			fmt.Printf("  tool_cache hits=%d misses=%d\n", r.ToolCacheHits, r.ToolCacheMisses)
		}
		if r.ToolBudgetCallsLimit > 0 || r.ToolBudgetExecMSLimit > 0 {
			callLimit := "unlimited"
			if r.ToolBudgetCallsLimit > 0 {
				callLimit = fmt.Sprintf("%d", r.ToolBudgetCallsLimit)
			}
			execLimit := "unlimited"
			if r.ToolBudgetExecMSLimit > 0 {
				execLimit = fmt.Sprintf("%dms", r.ToolBudgetExecMSLimit)
			}
			fmt.Printf("  tool_budget calls=%d/%s exec_ms=%d/%s\n", r.ToolBudgetCallsUsed, callLimit, r.ToolBudgetExecMSUsed, execLimit)
		}
		if r.StageToolBudgetCallsLimit > 0 || r.StageToolBudgetExecMSLimit > 0 {
			callLimit := "unlimited"
			if r.StageToolBudgetCallsLimit > 0 {
				callLimit = fmt.Sprintf("%d", r.StageToolBudgetCallsLimit)
			}
			execLimit := "unlimited"
			if r.StageToolBudgetExecMSLimit > 0 {
				execLimit = fmt.Sprintf("%dms", r.StageToolBudgetExecMSLimit)
			}
			fmt.Printf("  stage_tool_budget calls=%d/%s exec_ms=%d/%s\n", r.StageToolBudgetCallsUsed, callLimit, r.StageToolBudgetExecMSUsed, execLimit)
		}
		if r.SentHandoffs > 0 {
			fmt.Printf("  handoffs_sent=%d\n", r.SentHandoffs)
		}
		if r.HandoffTokens > 0 || r.DroppedHandoffs > 0 {
			fmt.Printf("  handoff_publish tokens=%d dropped=%d\n", r.HandoffTokens, r.DroppedHandoffs)
		}
		if r.SkippedOutputPublish {
			fmt.Println("  handoff_publish skipped_unconsumed_output=true")
		}
		if len(r.StartupCalls) > 0 {
			for _, c := range r.StartupCalls {
				if c.Error != "" {
					fmt.Printf("  startup_call %s failed (%s): %s\n", c.Skill, c.Duration.Round(time.Millisecond), c.Error)
				} else {
					fmt.Printf("  startup_call %s ok (%s)\n", c.Skill, c.Duration.Round(time.Millisecond))
				}
			}
		}
		if len(r.ToolCalls) > 0 {
			for _, c := range r.ToolCalls {
				if c.Error != "" {
					fmt.Printf("  tool_call %s", c.Skill)
					if c.CallID != "" {
						fmt.Printf(" id=%s", c.CallID)
					}
					fmt.Printf(" failed (%s): %s\n", c.Duration.Round(time.Millisecond), c.Error)
					continue
				}
				fmt.Printf("  tool_call %s", c.Skill)
				if c.CallID != "" {
					fmt.Printf(" id=%s", c.CallID)
				}
				if len(c.Args) > 0 {
					fmt.Printf(" args=%v", c.Args)
				}
				if c.Cached {
					fmt.Printf(" cached=true")
				}
				if c.Cacheable && !c.Cached {
					fmt.Printf(" cacheable=true")
				}
				fmt.Printf(" ok (%s)\n", c.Duration.Round(time.Millisecond))
			}
		}
		if r.Error != "" {
			fmt.Printf("  error: %s\n", r.Error)
			continue
		}
		fmt.Println(r.Output)
	}
}

func (p *reviewPromptBuilder) addLine(line string) bool {
	if p == nil {
		return false
	}
	line = strings.TrimRight(line, "\n")
	if line == "" {
		line = ""
	}
	if p.max > 0 && p.b.Len()+len(line)+1 > p.max {
		return false
	}
	p.b.WriteString(line)
	p.b.WriteByte('\n')
	return true
}

func (p *reviewPromptBuilder) addBlock(header, body string) bool {
	if p == nil {
		return false
	}
	header = strings.TrimSpace(header)
	body = strings.TrimSpace(body)
	if header == "" || body == "" {
		return true
	}
	remaining := p.max - p.b.Len()
	overhead := len(header) + len("\n```text\n\n```\n")
	if p.max > 0 && remaining <= overhead+16 {
		return false
	}
	if p.max > 0 && len(body) > remaining-overhead {
		body = strings.TrimSpace(body[:remaining-overhead-3]) + "..."
	}
	p.b.WriteString(header)
	p.b.WriteString("\n```text\n")
	p.b.WriteString(body)
	p.b.WriteString("\n```\n")
	return true
}

func (p *reviewPromptBuilder) String() string {
	if p == nil {
		return ""
	}
	return p.b.String()
}
