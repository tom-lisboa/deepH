package diagnosescope

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"deeph/internal/reviewscope"
)

type Config struct {
	MaxInputChars      int
	MaxExcerptChars    int
	MaxIssueChars      int
	MaxReferencedFiles int
	MaxWorkingSetFiles int
	MaxSamePackage     int
	MaxTestFiles       int
	MaxDiffFiles       int
}

type Scope struct {
	Workspace       string        `json:"workspace"`
	BaseRef         string        `json:"base_ref,omitempty"`
	IssueSummary    string        `json:"issue_summary"`
	References      []Reference   `json:"references,omitempty"`
	DiffFiles       []string      `json:"diff_files,omitempty"`
	WorkingSet      []WorkingFile `json:"working_set,omitempty"`
	SamePackage     int           `json:"same_package"`
	TestFiles       int           `json:"test_files"`
	ReferencedFiles int           `json:"referenced_files"`
}

type Reference struct {
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
	Source string `json:"source,omitempty"`
}

type WorkingFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

var fileRefPattern = regexp.MustCompile(`(?m)([A-Za-z0-9_./-]+\.(?:go|py|ts|tsx|js|jsx|java|rb|rs|c|cc|cpp|h|hpp|cs|php|kt|swift|sh|yaml|yml|json|toml))(?:[:(](\d+)(?::\d+)?\)?)?`)

func DefaultConfig() Config {
	return Config{
		MaxInputChars:      3400,
		MaxExcerptChars:    420,
		MaxIssueChars:      1000,
		MaxReferencedFiles: 4,
		MaxWorkingSetFiles: 10,
		MaxSamePackage:     2,
		MaxTestFiles:       2,
		MaxDiffFiles:       3,
	}
}

func BuildScope(workspace, baseRef, issue string, cfg Config) (Scope, error) {
	issue = strings.TrimSpace(issue)
	if issue == "" {
		return Scope{}, fmt.Errorf("diagnose requires an error, stack trace, failing output, or issue description")
	}
	scope := Scope{
		Workspace:    workspace,
		BaseRef:      strings.TrimSpace(baseRef),
		IssueSummary: trimInline(issue, 240),
	}

	refs := extractReferences(workspace, issue, cfg.MaxReferencedFiles)
	scope.References = refs
	scope.ReferencedFiles = len(refs)

	diffFiles, effectiveBase := gitChangedFiles(workspace, baseRef, cfg.MaxDiffFiles)
	scope.DiffFiles = diffFiles
	if effectiveBase != "" {
		scope.BaseRef = effectiveBase
	}

	seen := map[string]int{}
	addWorking := func(path, reason string) {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" {
			return
		}
		if idx, ok := seen[path]; ok {
			scope.WorkingSet[idx].Reason = mergeReasons(scope.WorkingSet[idx].Reason, reason)
			return
		}
		if len(scope.WorkingSet) >= cfg.MaxWorkingSetFiles {
			return
		}
		seen[path] = len(scope.WorkingSet)
		scope.WorkingSet = append(scope.WorkingSet, WorkingFile{Path: path, Reason: reason})
	}

	for _, ref := range refs {
		addWorking(ref.Path, "error reference")
		expandSamePackage(workspace, ref.Path, cfg, &scope, addWorking)
	}
	for _, path := range diffFiles {
		addWorking(path, "current diff")
		if isGoFile(path) {
			expandSamePackage(workspace, path, cfg, &scope, addWorking)
		}
	}
	return scope, nil
}

func BuildInput(scope Scope, issue string, cfg Config) string {
	p := &promptBuilder{max: cfg.MaxInputChars}
	p.addLine("[diagnose_scope]")
	p.addLine("strategy: error_aware_workspace")
	p.addLine("workspace: " + scope.Workspace)
	if scope.BaseRef != "" {
		p.addLine("base_ref: " + scope.BaseRef)
	}
	p.addLine(fmt.Sprintf("referenced_files: %d", scope.ReferencedFiles))
	p.addLine(fmt.Sprintf("working_set_files: %d", len(scope.WorkingSet)))
	if len(scope.DiffFiles) > 0 {
		p.addLine(fmt.Sprintf("diff_files: %d", len(scope.DiffFiles)))
	}
	p.addLine("issue_summary: " + trimInline(scope.IssueSummary, 220))
	p.addLine("instruction: identify the most likely root cause, affected files, minimum safe fix, and tests or validation steps. if evidence is incomplete, say what is hypothesis vs what is directly supported by the error output.")
	p.addLine("preferred_output: root_cause, likely_files, fix_plan, validation, residual_risks.")
	if trimmedIssue := trimBlock(issue, cfg.MaxIssueChars); trimmedIssue != "" {
		_ = p.addBlock("[issue]", trimmedIssue)
	}
	if len(scope.References) > 0 {
		p.addLine("references:")
		for _, ref := range scope.References {
			line := "- " + ref.Path
			if ref.Line > 0 {
				line += ":" + strconv.Itoa(ref.Line)
			}
			if strings.TrimSpace(ref.Source) != "" {
				line += " source=" + ref.Source
			}
			if !p.addLine(line) {
				break
			}
		}
	}
	if len(scope.WorkingSet) > 0 {
		p.addLine("working_set:")
		for _, file := range scope.WorkingSet {
			if !p.addLine(fmt.Sprintf("- %s reason=%s", file.Path, file.Reason)) {
				break
			}
		}
	}
	for _, ref := range scope.References {
		excerpt := buildExcerpt(scope.Workspace, ref.Path, ref.Line, cfg.MaxExcerptChars)
		if strings.TrimSpace(excerpt) == "" {
			continue
		}
		if !p.addBlock(fmt.Sprintf("[excerpt %s]", ref.Path), excerpt) {
			break
		}
	}
	for _, file := range scope.WorkingSet {
		if hasReference(scope.References, file.Path) {
			continue
		}
		excerpt := buildExcerpt(scope.Workspace, file.Path, 0, cfg.MaxExcerptChars)
		if strings.TrimSpace(excerpt) == "" {
			continue
		}
		if !p.addBlock(fmt.Sprintf("[excerpt %s]", file.Path), excerpt) {
			break
		}
	}
	return strings.TrimSpace(p.String())
}

func EstimateTokens(text string) int {
	return reviewscope.EstimateTokens(text)
}

func extractReferences(workspace, issue string, limit int) []Reference {
	matches := fileRefPattern.FindAllStringSubmatch(issue, -1)
	if len(matches) == 0 {
		return nil
	}
	refs := make([]Reference, 0, min(limit, len(matches)))
	seen := map[string]struct{}{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		path := normalizeReferencedPath(workspace, match[1])
		if path == "" {
			continue
		}
		line := 0
		if len(match) > 2 && strings.TrimSpace(match[2]) != "" {
			line, _ = strconv.Atoi(strings.TrimSpace(match[2]))
		}
		key := path + ":" + strconv.Itoa(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, Reference{Path: path, Line: line, Source: "issue"})
		if len(refs) >= limit {
			break
		}
	}
	return refs
}

func normalizeReferencedPath(workspace, raw string) string {
	raw = filepath.Clean(strings.TrimSpace(raw))
	if raw == "" || strings.HasPrefix(raw, "..") {
		return ""
	}
	abs := raw
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(workspace, raw)
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return ""
	}
	rel, err := filepath.Rel(workspace, abs)
	if err != nil {
		return ""
	}
	rel = filepath.Clean(rel)
	if strings.HasPrefix(rel, "..") {
		return ""
	}
	return rel
}

func gitChangedFiles(workspace, baseRef string, limit int) ([]string, string) {
	baseRef = strings.TrimSpace(baseRef)
	if baseRef == "" {
		baseRef = "HEAD"
	}
	diffCmd := exec.Command("git", "diff", "--name-only", baseRef, "--")
	diffCmd.Dir = workspace
	out, err := diffCmd.Output()
	if err != nil {
		return nil, ""
	}
	paths := splitNonEmptyLines(string(out))
	untrackedCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	untrackedCmd.Dir = workspace
	if out, err := untrackedCmd.Output(); err == nil {
		paths = append(paths, splitNonEmptyLines(string(out))...)
	}
	seen := map[string]struct{}{}
	deduped := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		deduped = append(deduped, path)
		if limit > 0 && len(deduped) >= limit {
			break
		}
	}
	return deduped, baseRef
}

func expandSamePackage(workspace, relPath string, cfg Config, scope *Scope, add func(string, string)) {
	if !isGoFile(relPath) || scope == nil {
		return
	}
	dir := filepath.Join(workspace, filepath.Dir(relPath))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	contextAdded := 0
	testsAdded := 0
	baseName := strings.TrimSuffix(filepath.Base(relPath), ".go")
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") {
			continue
		}
		sibling := filepath.Clean(filepath.Join(filepath.Dir(relPath), ent.Name()))
		if sibling == relPath {
			continue
		}
		switch {
		case strings.HasSuffix(ent.Name(), "_test.go"):
			if testsAdded >= cfg.MaxTestFiles {
				continue
			}
			add(sibling, "same-package test")
			testsAdded++
			scope.TestFiles++
		case contextAdded < cfg.MaxSamePackage:
			if strings.HasSuffix(relPath, "_test.go") && strings.TrimSuffix(ent.Name(), ".go") != strings.TrimSuffix(baseName, "_test") {
				continue
			}
			add(sibling, "same-package context")
			contextAdded++
			scope.SamePackage++
		}
	}
}

func buildExcerpt(workspace, relPath string, line, maxChars int) string {
	abs := filepath.Join(workspace, relPath)
	b, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	text := string(b)
	if strings.IndexByte(text, 0) >= 0 {
		return "(binary or non-text file)"
	}
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	start := 0
	end := min(len(lines), 24)
	if line > 0 {
		start = max(0, line-4)
		end = min(len(lines), line+3)
	}
	width := len(strconv.Itoa(len(lines)))
	if width < 2 {
		width = 2
	}
	var out strings.Builder
	for i := start; i < end && i < len(lines); i++ {
		fmt.Fprintf(&out, "%*d | %s\n", width, i+1, lines[i])
	}
	result := strings.TrimSpace(out.String())
	if maxChars > 0 && len(result) > maxChars {
		return strings.TrimSpace(result[:maxChars-3]) + "..."
	}
	return result
}

func hasReference(refs []Reference, path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	for _, ref := range refs {
		if filepath.Clean(ref.Path) == path {
			return true
		}
	}
	return false
}

func splitNonEmptyLines(s string) []string {
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func isGoFile(path string) bool {
	return strings.HasSuffix(strings.TrimSpace(path), ".go")
}

func trimInline(s string, maxChars int) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	if maxChars <= 3 {
		return s[:maxChars]
	}
	return s[:maxChars-3] + "..."
}

func trimBlock(s string, maxChars int) string {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n"))
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	if maxChars <= 3 {
		return s[:maxChars]
	}
	return s[:maxChars-3] + "..."
}

func mergeReasons(existing, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if existing == "" {
		return incoming
	}
	if incoming == "" || existing == incoming {
		return existing
	}
	for _, part := range strings.Split(existing, ", ") {
		if part == incoming {
			return existing
		}
	}
	return existing + ", " + incoming
}

type promptBuilder struct {
	max int
	b   strings.Builder
}

func (p *promptBuilder) addLine(line string) bool {
	line = strings.TrimRight(line, "\n")
	if p.max > 0 && p.b.Len()+len(line)+1 > p.max {
		return false
	}
	p.b.WriteString(line)
	p.b.WriteByte('\n')
	return true
}

func (p *promptBuilder) addBlock(header, body string) bool {
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

func (p *promptBuilder) String() string {
	return p.b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
