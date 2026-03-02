package reviewfindings

import (
	"encoding/json"
	"regexp"
	"strings"
)

type Finding struct {
	Severity string `json:"severity,omitempty"`
	File     string `json:"file,omitempty"`
	Title    string `json:"title,omitempty"`
	Impact   string `json:"impact,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

type Report struct {
	Findings      []Finding `json:"findings,omitempty"`
	ResidualRisks []string  `json:"residual_risks,omitempty"`
	NoIssues      bool      `json:"no_issues,omitempty"`
	Summary       string    `json:"summary,omitempty"`
	Format        string    `json:"format,omitempty"`
}

var (
	jsonFencePattern     = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")
	noIssuesPattern      = regexp.MustCompile(`(?i)\b(no convincing issue(?:s)? found|no issues found|no material issues found|no significant issues found|sem issues convincentes|nenhum issue convincente)\b`)
	findingBulletPattern = regexp.MustCompile(`^\s*(?:[-*]|\d+[.)])\s+`)
	severityTagPattern   = regexp.MustCompile(`(?i)^\[?\s*(p[0-3]|critical|high|medium|low|info)\s*\]?\s*[:\-]?\s*`)
	filePattern          = regexp.MustCompile("`?([A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+\\.[A-Za-z0-9]+(?::\\d+)?)`?")
)

func Parse(raw string) (*Report, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	if r, ok := parseJSON(raw); ok && r.meaningful() {
		r.Format = "json"
		return r, true
	}
	if r, ok := parseSections(raw); ok && r.meaningful() {
		r.Format = "sections"
		return r, true
	}
	if r, ok := parseBullets(raw); ok && r.meaningful() {
		r.Format = "bullets"
		return r, true
	}
	if noIssuesPattern.MatchString(raw) {
		return &Report{NoIssues: true, Summary: firstSentence(raw), Format: "text"}, true
	}
	return nil, false
}

func (r *Report) meaningful() bool {
	if r == nil {
		return false
	}
	return len(r.Findings) > 0 || len(r.ResidualRisks) > 0 || r.NoIssues || strings.TrimSpace(r.Summary) != ""
}

func parseJSON(raw string) (*Report, bool) {
	cands := extractJSONCandidates(raw)
	for _, cand := range cands {
		if r, ok := parseJSONCandidate(cand); ok {
			return r, true
		}
	}
	return nil, false
}

func extractJSONCandidates(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make([]string, 0, 4)
	for _, m := range jsonFencePattern.FindAllStringSubmatch(raw, -1) {
		if len(m) > 1 {
			out = append(out, strings.TrimSpace(m[1]))
		}
	}
	if strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "}") {
		out = append(out, raw)
	}
	if start := strings.IndexByte(raw, '{'); start >= 0 {
		if end := strings.LastIndexByte(raw, '}'); end > start {
			out = append(out, strings.TrimSpace(raw[start:end+1]))
		}
	}
	return dedupeStrings(out)
}

func parseJSONCandidate(s string) (*Report, bool) {
	var anyMap map[string]any
	if err := json.Unmarshal([]byte(s), &anyMap); err != nil {
		return nil, false
	}
	r := &Report{}
	r.NoIssues = boolValue(anyMap, "no_issues", "clean", "ok")
	r.Summary = firstString(anyMap, "summary", "status", "conclusion", "rationale")
	r.ResidualRisks = listValue(anyMap, "residual_risks", "risks", "testing_gaps")
	for _, key := range []string{"findings", "issues", "bugs"} {
		rawFindings, ok := anyMap[key]
		if !ok {
			continue
		}
		items, ok := rawFindings.([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			f := findingFromAny(item)
			if findingMeaningful(f) {
				r.Findings = append(r.Findings, f)
			}
		}
	}
	if len(r.Findings) == 0 && !r.NoIssues && noIssuesPattern.MatchString(r.Summary) {
		r.NoIssues = true
	}
	if !r.meaningful() {
		return nil, false
	}
	r.Findings = dedupeFindings(r.Findings)
	r.ResidualRisks = dedupeStrings(r.ResidualRisks)
	return r, true
}

func parseSections(raw string) (*Report, bool) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	sections := map[string][]string{}
	current := ""
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if sec, ok := parseSectionHeading(line); ok {
			current = sec
			if _, exists := sections[current]; !exists {
				sections[current] = nil
			}
			continue
		}
		if key, val, ok := parseSectionKV(line); ok {
			current = key
			if val != "" {
				sections[current] = append(sections[current], val)
			}
			continue
		}
		if current == "" {
			current = "summary"
		}
		sections[current] = append(sections[current], line)
	}
	if len(sections) == 0 {
		return nil, false
	}
	r := &Report{}
	if parts := sections["summary"]; len(parts) > 0 {
		r.Summary = cleanText(strings.Join(parts, " "))
	}
	for _, sec := range []string{"findings", "issues"} {
		for _, item := range splitListBlocks(sections[sec]) {
			f := parseFindingBlock(item)
			if findingMeaningful(f) {
				r.Findings = append(r.Findings, f)
			}
		}
	}
	for _, sec := range []string{"residual_risks", "risks", "testing_gaps"} {
		r.ResidualRisks = append(r.ResidualRisks, splitListBlocks(sections[sec])...)
	}
	for _, sec := range []string{"status", "conclusion"} {
		for _, line := range sections[sec] {
			if noIssuesPattern.MatchString(line) {
				r.NoIssues = true
			}
		}
	}
	if !r.NoIssues && noIssuesPattern.MatchString(r.Summary) {
		r.NoIssues = true
	}
	if !r.meaningful() {
		return nil, false
	}
	r.Findings = dedupeFindings(r.Findings)
	r.ResidualRisks = dedupeStrings(r.ResidualRisks)
	return r, true
}

func parseBullets(raw string) (*Report, bool) {
	blocks := splitListBlocks(strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n"))
	if len(blocks) == 0 {
		return nil, false
	}
	r := &Report{}
	for _, block := range blocks {
		f := parseFindingBlock(block)
		if findingMeaningful(f) {
			r.Findings = append(r.Findings, f)
		}
	}
	if len(r.Findings) == 0 {
		return nil, false
	}
	r.Findings = dedupeFindings(r.Findings)
	return r, true
}

func parseSectionHeading(line string) (string, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
	line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
	switch normalizeKey(line) {
	case "findings", "issues":
		return "findings", true
	case "residualrisks", "residualrisk", "risks":
		return "residual_risks", true
	case "testinggaps", "testinggap":
		return "testing_gaps", true
	case "summary", "conclusion":
		return "summary", true
	case "status":
		return "status", true
	default:
		return "", false
	}
}

func parseSectionKV(line string) (string, string, bool) {
	i := strings.IndexByte(line, ':')
	if i <= 0 {
		return "", "", false
	}
	key := normalizeKey(line[:i])
	val := cleanText(line[i+1:])
	switch key {
	case "findings", "issues":
		return "findings", val, true
	case "residualrisks", "residualrisk", "risks":
		return "residual_risks", val, true
	case "testinggaps", "testinggap":
		return "testing_gaps", val, true
	case "summary", "conclusion":
		return "summary", val, true
	case "status":
		return "status", val, true
	default:
		return "", "", false
	}
}

func splitListBlocks(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, 0, len(lines))
	var cur []string
	flush := func() {
		if len(cur) == 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(cur, "\n"))
		if text != "" {
			out = append(out, text)
		}
		cur = nil
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}
		if findingBulletPattern.MatchString(line) {
			flush()
			cur = append(cur, findingBulletPattern.ReplaceAllString(line, ""))
			continue
		}
		if cur == nil {
			cur = append(cur, line)
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return out
}

func parseFindingBlock(block string) Finding {
	block = strings.TrimSpace(block)
	if block == "" {
		return Finding{}
	}
	lines := strings.Split(block, "\n")
	f := Finding{}
	for i, line := range lines {
		line = cleanText(line)
		if line == "" {
			continue
		}
		if key, val, ok := parseFindingKV(line); ok {
			switch key {
			case "severity":
				f.Severity = normalizeSeverity(val)
			case "file":
				f.File = clipFileRef(val)
			case "title":
				f.Title = val
			case "impact":
				f.Impact = val
			case "evidence":
				f.Evidence = val
			}
			continue
		}
		if i == 0 {
			line = extractSeverity(line, &f)
			line = extractFile(line, &f)
			if f.Title == "" {
				f.Title = strings.TrimSpace(strings.TrimPrefix(line, ":"))
			}
			continue
		}
		if f.Impact == "" {
			f.Impact = line
		} else if f.Evidence == "" {
			f.Evidence = line
		} else {
			f.Evidence = cleanText(f.Evidence + " " + line)
		}
	}
	f.Title = cleanText(f.Title)
	f.Impact = cleanText(f.Impact)
	f.Evidence = cleanText(f.Evidence)
	return f
}

func parseFindingKV(line string) (string, string, bool) {
	i := strings.IndexByte(line, ':')
	if i <= 0 {
		return "", "", false
	}
	key := normalizeKey(line[:i])
	val := cleanText(line[i+1:])
	switch key {
	case "severity", "priority":
		return "severity", val, true
	case "file", "path", "location":
		return "file", val, true
	case "title", "finding", "issue":
		return "title", val, true
	case "impact", "why":
		return "impact", val, true
	case "evidence", "details":
		return "evidence", val, true
	default:
		return "", "", false
	}
}

func extractSeverity(line string, f *Finding) string {
	if f == nil {
		return line
	}
	if m := severityTagPattern.FindString(line); m != "" {
		f.Severity = normalizeSeverity(m)
		line = strings.TrimSpace(line[len(m):])
	}
	return line
}

func extractFile(line string, f *Finding) string {
	if f == nil {
		return line
	}
	m := filePattern.FindStringSubmatch(line)
	if len(m) > 1 {
		f.File = clipFileRef(m[1])
		line = strings.Replace(line, m[0], "", 1)
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
	}
	return line
}

func findingFromAny(v any) Finding {
	switch x := v.(type) {
	case map[string]any:
		return Finding{
			Severity: normalizeSeverity(firstString(x, "severity", "priority")),
			File:     clipFileRef(firstString(x, "file", "path", "location")),
			Title:    cleanText(firstString(x, "title", "finding", "issue", "summary")),
			Impact:   cleanText(firstString(x, "impact", "why")),
			Evidence: cleanText(firstString(x, "evidence", "details")),
		}
	case string:
		return parseFindingBlock(x)
	default:
		return Finding{}
	}
}

func boolValue(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		v, ok := m[key]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case bool:
			return x
		case string:
			t := strings.TrimSpace(strings.ToLower(x))
			return t == "true" || t == "yes" || t == "ok"
		}
	}
	return false
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := m[key]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case string:
			if t := cleanText(x); t != "" {
				return t
			}
		}
	}
	return ""
}

func listValue(m map[string]any, keys ...string) []string {
	for _, key := range keys {
		v, ok := m[key]
		if !ok {
			continue
		}
		items, ok := v.([]any)
		if !ok {
			continue
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok {
				if t := cleanText(s); t != "" {
					out = append(out, t)
				}
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func dedupeFindings(in []Finding) []Finding {
	if len(in) == 0 {
		return nil
	}
	out := make([]Finding, 0, len(in))
	seen := map[string]struct{}{}
	for _, f := range in {
		if !findingMeaningful(f) {
			continue
		}
		key := strings.ToLower(f.Severity + "|" + f.File + "|" + f.Title + "|" + f.Impact)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, f)
	}
	return out
}

func findingMeaningful(f Finding) bool {
	return strings.TrimSpace(f.File) != "" || strings.TrimSpace(f.Title) != "" || strings.TrimSpace(f.Impact) != ""
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, item := range in {
		item = cleanText(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func normalizeSeverity(s string) string {
	s = normalizeKey(s)
	switch s {
	case "p0", "critical":
		return "critical"
	case "p1", "high":
		return "high"
	case "p2", "medium", "med":
		return "medium"
	case "p3", "low", "info":
		return "low"
	default:
		return ""
	}
}

func normalizeKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	repl := strings.NewReplacer(" ", "", "_", "", "-", "", "`", "", "#", "", ".", "", ":", "", "[", "", "]", "", "(", "", ")", "")
	return repl.Replace(s)
}

func cleanText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func clipFileRef(s string) string {
	s = cleanText(s)
	s = strings.Trim(s, "`")
	if i := strings.IndexByte(s, ' '); i >= 0 {
		s = s[:i]
	}
	return s
}

func firstSentence(s string) string {
	s = cleanText(s)
	if s == "" {
		return ""
	}
	for _, sep := range []string{". ", "! ", "? "} {
		if i := strings.Index(s, sep); i >= 0 {
			return strings.TrimSpace(s[:i+1])
		}
	}
	return s
}
