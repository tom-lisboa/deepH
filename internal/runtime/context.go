package runtime

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"deeph/internal/typesys"
)

type ContextDirtyFlags uint64

const (
	ContextDirtyGoal ContextDirtyFlags = 1 << iota
	ContextDirtyConstraints
	ContextDirtyFacts
	ContextDirtyQuestions
	ContextDirtyEvents
	ContextDirtyArtifacts
)

type ContextMoment string

const (
	ContextMomentGeneral   ContextMoment = "general"
	ContextMomentPlan      ContextMoment = "plan"
	ContextMomentDiscovery ContextMoment = "discovery"
	ContextMomentToolLoop  ContextMoment = "tool_loop"
	ContextMomentSynthesis ContextMoment = "synthesis"
	ContextMomentValidate  ContextMoment = "validate"
)

type ContextWeightProfile struct {
	TypeWeights      map[typesys.Kind]float64
	CategoryWeights  map[string]float64
	MomentWeights    map[ContextMoment]map[string]float64 // keys: canonical kind or "cat:<category>"
	MatchMomentBoost float64
}

func DefaultContextWeightProfile() ContextWeightProfile {
	p := ContextWeightProfile{
		TypeWeights:      map[typesys.Kind]float64{},
		CategoryWeights:  map[string]float64{},
		MomentWeights:    map[ContextMoment]map[string]float64{},
		MatchMomentBoost: 1.0,
	}

	p.CategoryWeights = map[string]float64{
		"primitive":  0.2,
		"text":       0.8,
		"code":       0.5,
		"json":       0.9,
		"data":       0.8,
		"artifact":   1.0,
		"tool":       1.8,
		"capability": 0.2,
		"plan":       1.1,
		"diagnostic": 1.9,
		"memory":     2.2,
		"message":    1.1,
		"summary":    2.0,
		"context":    0.4,
	}

	// Populate all known kinds with category defaults first (covers "all types").
	for _, d := range typesys.List() {
		p.TypeWeights[d.Kind] = p.CategoryWeights[d.Category]
	}

	// Type-specific tuning: optimize token quality per semantic value.
	p.TypeWeights[typesys.KindMemoryFact] = 3.0
	p.TypeWeights[typesys.KindMemoryQuestion] = 2.8
	p.TypeWeights[typesys.KindMemorySummary] = 2.6
	p.TypeWeights[typesys.KindSummaryCode] = 2.8
	p.TypeWeights[typesys.KindSummaryText] = 2.4
	p.TypeWeights[typesys.KindDiagnosticBuild] = 3.1
	p.TypeWeights[typesys.KindDiagnosticTest] = 3.0
	p.TypeWeights[typesys.KindDiagnosticLint] = 2.7
	p.TypeWeights[typesys.KindToolError] = 3.6
	p.TypeWeights[typesys.KindToolResult] = 2.0
	p.TypeWeights[typesys.KindArtifactRef] = 1.8
	p.TypeWeights[typesys.KindArtifactSummary] = 2.2
	p.TypeWeights[typesys.KindCodeGo] = 0.9
	p.TypeWeights[typesys.KindCodeTS] = 0.9
	p.TypeWeights[typesys.KindCodeTSX] = 0.8
	p.TypeWeights[typesys.KindCodeJS] = 0.8
	p.TypeWeights[typesys.KindCodePython] = 0.9
	p.TypeWeights[typesys.KindTextMarkdown] = 1.1
	p.TypeWeights[typesys.KindTextPrompt] = 1.6
	p.TypeWeights[typesys.KindMessageSystem] = 2.0
	p.TypeWeights[typesys.KindMessageAgent] = 1.4
	p.TypeWeights[typesys.KindMessageAssistant] = 1.2
	p.TypeWeights[typesys.KindMessageTool] = 1.3

	p.MomentWeights = map[ContextMoment]map[string]float64{
		ContextMomentPlan: {
			"cat:plan":     1.8,
			"cat:memory":   1.0,
			"cat:message":  0.8,
			"cat:tool":     -0.4,
			"cat:artifact": -0.3,
		},
		ContextMomentDiscovery: {
			"cat:memory":   1.4,
			"cat:artifact": 1.2,
			"cat:summary":  1.3,
			"cat:tool":     0.7,
			"cat:code":     0.4,
		},
		ContextMomentToolLoop: {
			"cat:tool":          2.4,
			"tool/error":        1.4,
			"cat:artifact":      1.2,
			"artifact/ref":      0.8,
			"cat:diagnostic":    1.2,
			"cat:summary":       0.9,
			"cat:memory":        0.5,
			"message/assistant": -0.4,
		},
		ContextMomentSynthesis: {
			"cat:summary":    2.0,
			"cat:memory":     1.6,
			"cat:artifact":   0.8,
			"cat:diagnostic": 1.0,
			"cat:tool":       0.4,
			"cat:code":       -0.3,
		},
		ContextMomentValidate: {
			"cat:diagnostic": 2.5,
			"cat:tool":       1.2,
			"cat:summary":    1.0,
			"cat:memory":     0.6,
		},
	}
	return p
}

func (p ContextWeightProfile) normalized() ContextWeightProfile {
	if len(p.TypeWeights) == 0 || len(p.CategoryWeights) == 0 {
		return DefaultContextWeightProfile()
	}
	if p.MomentWeights == nil {
		p.MomentWeights = DefaultContextWeightProfile().MomentWeights
	}
	if p.MatchMomentBoost == 0 {
		p.MatchMomentBoost = 1.0
	}
	return p
}

type ContextBudget struct {
	MaxInputTokens     int
	SafetyMarginTokens int
	MaxConstraints     int
	MaxFacts           int
	MaxOpenQuestions   int
	MaxRecentEvents    int
	MaxArtifacts       int
	MaxArtifactSummary int
	MaxEventSummary    int
}

func DefaultContextBudget() ContextBudget {
	return ContextBudget{
		MaxInputTokens:     1200,
		SafetyMarginTokens: 80,
		MaxConstraints:     8,
		MaxFacts:           16,
		MaxOpenQuestions:   8,
		MaxRecentEvents:    12,
		MaxArtifacts:       8,
		MaxArtifactSummary: 180,
		MaxEventSummary:    160,
	}
}

func (b ContextBudget) normalized() ContextBudget {
	d := DefaultContextBudget()
	if b.MaxInputTokens <= 0 {
		b.MaxInputTokens = d.MaxInputTokens
	}
	if b.SafetyMarginTokens < 0 {
		b.SafetyMarginTokens = d.SafetyMarginTokens
	}
	if b.MaxConstraints <= 0 {
		b.MaxConstraints = d.MaxConstraints
	}
	if b.MaxFacts <= 0 {
		b.MaxFacts = d.MaxFacts
	}
	if b.MaxOpenQuestions <= 0 {
		b.MaxOpenQuestions = d.MaxOpenQuestions
	}
	if b.MaxRecentEvents <= 0 {
		b.MaxRecentEvents = d.MaxRecentEvents
	}
	if b.MaxArtifacts <= 0 {
		b.MaxArtifacts = d.MaxArtifacts
	}
	if b.MaxArtifactSummary <= 0 {
		b.MaxArtifactSummary = d.MaxArtifactSummary
	}
	if b.MaxEventSummary <= 0 {
		b.MaxEventSummary = d.MaxEventSummary
	}
	return b
}

func (b ContextBudget) limitTokens() int {
	b = b.normalized()
	limit := b.MaxInputTokens - b.SafetyMarginTokens
	if limit < 128 {
		limit = b.MaxInputTokens
	}
	if limit < 64 {
		limit = 64
	}
	return limit
}

type ContextFact struct {
	Key         string
	Value       string
	Kind        typesys.Kind
	Moment      ContextMoment
	Confidence  float64
	Source      string
	TargetAgent string
	Channel     string
	UpdatedAt   time.Time
}

type ContextArtifact struct {
	ID          string
	Kind        typesys.Kind
	Moment      ContextMoment
	Source      string
	TargetAgent string
	Channel     string
	Hash        string
	Bytes       int
	Summary     string
	UpdatedAt   time.Time
}

type ContextEvent struct {
	Type        string
	Agent       string
	Skill       string
	Kind        typesys.Kind
	Moment      ContextMoment
	Summary     string
	ArtifactID  string
	TargetAgent string
	Channel     string
	HasError    bool
	UpdatedAt   time.Time
}

type ContextSnapshot struct {
	Goal          string
	Constraints   []string
	Facts         []ContextFact
	OpenQuestions []string
	RecentEvents  []ContextEvent
	Artifacts     []ContextArtifact
	Version       uint64
	DirtyFlags    ContextDirtyFlags
}

type ContextCompileSpec struct {
	AgentName     string
	Channels      []string
	Skills        []string
	Budget        ContextBudget
	Moment        ContextMoment
	WeightProfile ContextWeightProfile
}

type CompiledContext struct {
	Text              string
	EstimatedTokens   int
	BudgetLimitTokens int
	Version           uint64
	DirtyFlags        ContextDirtyFlags
	Moment            ContextMoment
	SelectedFacts     int
	SelectedEvents    int
	SelectedArtifacts int
	SelectedQuestions int
	DroppedItems      int
}

type ContextBus struct {
	mu            sync.RWMutex
	goal          string
	constraints   []string
	facts         map[string]ContextFact
	openQuestions []string
	events        []ContextEvent
	artifacts     map[string]ContextArtifact
	version       uint64
	dirty         ContextDirtyFlags
}

func NewContextBus(goal string) *ContextBus {
	b := &ContextBus{
		goal:      strings.TrimSpace(goal),
		facts:     map[string]ContextFact{},
		artifacts: map[string]ContextArtifact{},
		version:   1,
	}
	if b.goal != "" {
		b.dirty = ContextDirtyGoal
	}
	return b
}

func (b *ContextBus) SetGoal(goal string) {
	goal = strings.TrimSpace(goal)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.goal == goal {
		return
	}
	b.goal = goal
	b.touchLocked(ContextDirtyGoal)
}

func (b *ContextBus) AddConstraint(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range b.constraints {
		if c == text {
			return
		}
	}
	b.constraints = append(b.constraints, text)
	b.touchLocked(ContextDirtyConstraints)
}

func (b *ContextBus) AddOpenQuestion(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, q := range b.openQuestions {
		if q == text {
			return
		}
	}
	b.openQuestions = append(b.openQuestions, text)
	b.touchLocked(ContextDirtyQuestions)
}

func (b *ContextBus) PutFact(key, value string, confidence float64, source string) {
	b.PutTypedFact(key, value, typesys.KindMemoryFact, ContextMomentGeneral, confidence, source)
}

func (b *ContextBus) PutTypedFact(key, value string, kind typesys.Kind, moment ContextMoment, confidence float64, source string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	prev, ok := b.facts[key]
	if ok && prev.Value == value && math.Abs(prev.Confidence-confidence) < 0.0001 && prev.Source == source {
		return
	}
	b.facts[key] = ContextFact{
		Key:        key,
		Value:      value,
		Kind:       defaultKind(kind, typesys.KindMemoryFact),
		Moment:     defaultMoment(moment, ContextMomentGeneral),
		Confidence: confidence,
		Source:     strings.TrimSpace(source),
		UpdatedAt:  time.Now(),
	}
	b.touchLocked(ContextDirtyFacts)
}

func (b *ContextBus) RecordSkillCall(agent string, call SkillCallResult) {
	b.RecordSkillCallAtMoment(agent, call, ContextMomentToolLoop)
}

func (b *ContextBus) RecordSkillCallAtMoment(agent string, call SkillCallResult, moment ContextMoment) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.recordSkillCallLocked(strings.TrimSpace(agent), call, moment)
}

func (b *ContextBus) RecordAgentOutput(agent, output string) {
	output = strings.TrimSpace(output)
	if output == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	summary := trim(output, 180)
	artifactID := ""
	if len(output) > 220 {
		artifactID = b.putArtifactLocked(typesys.KindArtifactRef, ContextMomentSynthesis, agent, output, summary)
		summary = fmt.Sprintf("final output stored as artifact=%s (%d chars)", artifactID, len(output))
	}
	b.events = append(b.events, ContextEvent{
		Type:       "agent_output",
		Agent:      agent,
		Kind:       typesys.KindMessageAgent,
		Moment:     ContextMomentSynthesis,
		Summary:    summary,
		ArtifactID: artifactID,
		UpdatedAt:  now,
	})
	b.trimEventsLocked(64)
	b.touchLocked(ContextDirtyEvents)
}

func (b *ContextBus) PutArtifact(kind typesys.Kind, moment ContextMoment, source, raw, summary string) ContextArtifact {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ContextArtifact{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.putArtifactScopedLocked(kind, moment, source, raw, summary, "", "")
	return b.artifacts[id]
}

func (b *ContextBus) PutScopedArtifact(kind typesys.Kind, moment ContextMoment, source, raw, summary, targetAgent, channel string) ContextArtifact {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ContextArtifact{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.putArtifactScopedLocked(kind, moment, source, raw, summary, targetAgent, channel)
	return b.artifacts[id]
}

func (b *ContextBus) RecordAgentHandoff(link TypedHandoffLink, value typesys.TypedValue) {
	if strings.TrimSpace(link.FromAgent) == "" || strings.TrimSpace(link.ToAgent) == "" {
		return
	}
	now := time.Now()
	kind := defaultKind(value.Kind, link.Kind)
	if kind == "" {
		kind = typesys.KindMessageAgent
	}

	valueSummary := ""
	switch {
	case value.RefID != "":
		valueSummary = "ref=" + value.RefID
	case strings.TrimSpace(value.InlineText) != "":
		valueSummary = "text=" + trim(value.InlineText, 140)
	}
	if valueSummary == "" && kind != "" {
		valueSummary = "kind=" + kind.String()
	}

	parts := []string{
		"ch=" + coalesce(strings.TrimSpace(link.Channel), handoffChannelID(link.FromAgent, link.FromPort, link.ToAgent, link.ToPort, kind)),
		"to=" + link.ToAgent,
		"from_port=" + coalesce(strings.TrimSpace(link.FromPort), "output"),
		"to_port=" + coalesce(strings.TrimSpace(link.ToPort), "input"),
		"kind=" + kind.String(),
	}
	if valueSummary != "" {
		parts = append(parts, valueSummary)
	}
	summary := strings.Join(parts, " ")

	b.mu.Lock()
	defer b.mu.Unlock()

	// Compact fact for the target input port (cheap and high-value in compiled context).
	factKey := fmt.Sprintf("handoff.%s.%s", link.ToAgent, coalesce(strings.TrimSpace(link.ToPort), "input"))
	factValue := fmt.Sprintf("from=%s %s", link.FromAgent, summary)
	prev := b.facts[factKey]
	mergedValue, mergedSource, changed := mergeHandoffFactValue(
		prev,
		trim(factValue, 220),
		kind,
		link.FromAgent,
		link.MergePolicy,
		link.TargetMaxTokens,
	)
	if !changed && prev.Key != "" {
		return
	}
	b.facts[factKey] = ContextFact{
		Key:         factKey,
		Value:       mergedValue,
		Kind:        kind,
		Moment:      ContextMomentSynthesis,
		Confidence:  0.95,
		Source:      mergedSource,
		TargetAgent: strings.TrimSpace(link.ToAgent),
		Channel:     strings.TrimSpace(link.Channel),
		UpdatedAt:   now,
	}
	b.events = append(b.events, ContextEvent{
		Type:        "agent_handoff",
		Agent:       link.FromAgent,
		Kind:        typesys.KindMessageAgent,
		Moment:      ContextMomentSynthesis,
		Summary:     summary,
		ArtifactID:  value.RefID,
		TargetAgent: strings.TrimSpace(link.ToAgent),
		Channel:     strings.TrimSpace(link.Channel),
		UpdatedAt:   now,
	})
	b.trimEventsLocked(96)
	b.touchLocked(ContextDirtyFacts | ContextDirtyEvents)
}

func mergeHandoffFactValue(prev ContextFact, nextValue string, kind typesys.Kind, source, mergePolicy string, targetMaxTokens int) (value, mergedSource string, changed bool) {
	nextValue = strings.TrimSpace(nextValue)
	policy := resolveHandoffMergePolicy(kind, mergePolicy)
	maxChars := handoffFactCharLimit(targetMaxTokens)
	applyCap := func(s string) string {
		switch policy {
		case "append4":
			return mergeFactSegments("", s, 4, coalesceInt(maxChars, 320))
		case "append3":
			return mergeFactSegments("", s, 3, coalesceInt(maxChars, 300))
		case "append2":
			return mergeFactSegments("", s, 2, coalesceInt(maxChars, 260))
		default:
			return trim(s, coalesceInt(maxChars, 220))
		}
	}
	if nextValue == "" {
		if prev.Key == "" {
			return "", "", false
		}
		return prev.Value, prev.Source, false
	}
	if prev.Key == "" || strings.TrimSpace(prev.Value) == "" {
		return applyCap(nextValue), strings.TrimSpace(source), true
	}
	if strings.TrimSpace(prev.Value) == nextValue {
		return prev.Value, prev.Source, false
	}

	merged := nextValue
	switch policy {
	case "append4":
		merged = mergeFactSegments(prev.Value, nextValue, 4, coalesceInt(maxChars, 320))
	case "append3":
		merged = mergeFactSegments(prev.Value, nextValue, 3, coalesceInt(maxChars, 300))
	case "append2":
		merged = mergeFactSegments(prev.Value, nextValue, 2, coalesceInt(maxChars, 260))
	case "latest":
		merged = trim(nextValue, coalesceInt(maxChars, 220))
	default:
		merged = trim(nextValue, coalesceInt(maxChars, 220))
	}
	if merged == strings.TrimSpace(prev.Value) {
		return prev.Value, prev.Source, false
	}
	mergedSource = strings.TrimSpace(source)
	if strings.TrimSpace(prev.Source) != "" && strings.TrimSpace(prev.Source) != strings.TrimSpace(source) {
		mergedSource = "merge"
	}
	if mergedSource == "" {
		mergedSource = prev.Source
	}
	return merged, mergedSource, true
}

func resolveHandoffMergePolicy(kind typesys.Kind, raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return handoffMergePolicyForKind(kind)
	case "latest":
		return "latest"
	case "append", "append3":
		return "append3"
	case "append2":
		return "append2"
	case "append4":
		return "append4"
	default:
		return handoffMergePolicyForKind(kind)
	}
}

func handoffMergePolicyForKind(kind typesys.Kind) string {
	s := kind.String()
	switch {
	case kind == typesys.KindToolError || strings.HasPrefix(s, "diagnostic/"):
		return "append4"
	case strings.HasPrefix(s, "summary/"), strings.HasPrefix(s, "memory/"), strings.HasPrefix(s, "tool/"):
		return "append3"
	case strings.HasPrefix(s, "artifact/"), strings.HasPrefix(s, "code/"), strings.HasPrefix(s, "json/"), strings.HasPrefix(s, "data/"):
		return "append3"
	case strings.HasPrefix(s, "message/"), strings.HasPrefix(s, "text/"):
		return "append2"
	default:
		return "latest"
	}
}

func mergeFactSegments(prevValue, nextValue string, maxSegments, maxChars int) string {
	segments := splitFactSegments(prevValue)
	if maxSegments <= 0 {
		maxSegments = 2
	}
	add := strings.TrimSpace(nextValue)
	if add != "" {
		found := false
		for _, s := range segments {
			if s == add {
				found = true
				break
			}
		}
		if !found {
			segments = append(segments, add)
		}
	}
	if len(segments) > maxSegments {
		segments = segments[len(segments)-maxSegments:]
	}
	joined := strings.Join(segments, " || ")
	if maxChars > 0 && len(joined) > maxChars {
		// Trim oldest first to preserve most recent context.
		for len(segments) > 1 && len(joined) > maxChars {
			segments = segments[1:]
			joined = strings.Join(segments, " || ")
		}
	}
	if maxChars > 0 {
		joined = trim(joined, maxChars)
	}
	return joined
}

func splitFactSegments(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	raw := strings.Split(s, "||")
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func handoffFactCharLimit(targetMaxTokens int) int {
	if targetMaxTokens <= 0 {
		return 0
	}
	chars := targetMaxTokens * 4
	if chars < 96 {
		chars = 96
	}
	if chars > 1200 {
		chars = 1200
	}
	return chars
}

func coalesceInt(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

func (b *ContextBus) Snapshot() ContextSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	facts := make([]ContextFact, 0, len(b.facts))
	for _, f := range b.facts {
		facts = append(facts, f)
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].Key < facts[j].Key })

	artifacts := make([]ContextArtifact, 0, len(b.artifacts))
	for _, a := range b.artifacts {
		artifacts = append(artifacts, a)
	}
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].UpdatedAt.Equal(artifacts[j].UpdatedAt) {
			return artifacts[i].ID < artifacts[j].ID
		}
		return artifacts[i].UpdatedAt.After(artifacts[j].UpdatedAt)
	})

	events := make([]ContextEvent, len(b.events))
	copy(events, b.events)

	constraints := make([]string, len(b.constraints))
	copy(constraints, b.constraints)
	questions := make([]string, len(b.openQuestions))
	copy(questions, b.openQuestions)

	return ContextSnapshot{
		Goal:          b.goal,
		Constraints:   constraints,
		Facts:         facts,
		OpenQuestions: questions,
		RecentEvents:  events,
		Artifacts:     artifacts,
		Version:       b.version,
		DirtyFlags:    b.dirty,
	}
}

func (b *ContextBus) Compile(spec ContextCompileSpec) CompiledContext {
	snap := b.Snapshot()
	return CompileContextSnapshot(snap, spec)
}

func (b *ContextBus) recordSkillCallLocked(agent string, call SkillCallResult, moment ContextMoment) {
	now := time.Now()
	event := ContextEvent{
		Type:      "skill_call",
		Agent:     agent,
		Skill:     call.Skill,
		Kind:      typesys.KindToolResult,
		Moment:    defaultMoment(moment, ContextMomentToolLoop),
		HasError:  call.Error != "",
		UpdatedAt: now,
	}

	if call.Error != "" {
		event.Kind = typesys.KindToolError
		event.Summary = fmt.Sprintf("skill=%s failed: %s", call.Skill, trim(call.Error, 160))
		b.events = append(b.events, event)
		b.trimEventsLocked(64)
		b.touchLocked(ContextDirtyEvents)
		return
	}

	summary, artifactID := b.summarizeSkillResultLocked(call, defaultMoment(moment, ContextMomentToolLoop))
	event.Summary = summary
	event.ArtifactID = artifactID
	b.events = append(b.events, event)
	b.trimEventsLocked(64)
	b.touchLocked(ContextDirtyEvents)
}

func (b *ContextBus) summarizeSkillResultLocked(call SkillCallResult, moment ContextMoment) (summary, artifactID string) {
	if call.Result == nil {
		return fmt.Sprintf("skill=%s ok (empty result)", call.Skill), ""
	}

	valueKind := inferSkillResultKind(call)

	for _, key := range []string{"text", "body"} {
		if s, ok := call.Result[key].(string); ok {
			sTrim := strings.TrimSpace(s)
			if sTrim == "" {
				continue
			}
			if len(sTrim) > 240 {
				smartSummary := summarizeArtifactPayload(valueKind, call.Result, sTrim)
				art := b.putArtifactLocked(valueKind, moment, call.Skill, sTrim, smartSummary)
				return fmt.Sprintf("skill=%s ok (%s stored as artifact=%s type=%s, %d chars)", call.Skill, key, art, valueKind, len(sTrim)), art
			}
		}
	}

	compact, err := json.Marshal(call.Result)
	if err != nil {
		return fmt.Sprintf("skill=%s ok (result json error: %v)", call.Skill, err), ""
	}
	compStr := string(compact)
	if len(compStr) > 320 {
		artKind := valueKind
		if artKind == "" {
			artKind = typesys.KindJSONValue
		}
		smartSummary := summarizeArtifactPayload(artKind, call.Result, compStr)
		art := b.putArtifactLocked(artKind, moment, call.Skill, compStr, smartSummary)
		return fmt.Sprintf("skill=%s ok (json stored as artifact=%s type=%s, %d chars)", call.Skill, art, artKind, len(compStr)), art
	}
	return fmt.Sprintf("skill=%s ok result=%s", call.Skill, trim(compStr, 220)), ""
}

func (b *ContextBus) putArtifactLocked(kind typesys.Kind, moment ContextMoment, source, raw, summary string) string {
	return b.putArtifactScopedLocked(kind, moment, source, raw, summary, "", "")
}

func (b *ContextBus) putArtifactScopedLocked(kind typesys.Kind, moment ContextMoment, source, raw, summary, targetAgent, channel string) string {
	h := sha1.Sum([]byte(raw))
	hash := hex.EncodeToString(h[:])
	k := defaultKind(kind, typesys.KindArtifactRef)
	scopeKey := ""
	targetAgent = strings.TrimSpace(targetAgent)
	channel = strings.TrimSpace(channel)
	if targetAgent != "" {
		scopeKey = ":" + targetAgent
		if channel != "" {
			scopeKey += ":" + shortHashString(channel)
		}
	}
	id := string(k) + ":" + hash[:12] + scopeKey
	art := ContextArtifact{
		ID:          id,
		Kind:        k,
		Moment:      defaultMoment(moment, ContextMomentGeneral),
		Source:      strings.TrimSpace(source),
		TargetAgent: targetAgent,
		Channel:     channel,
		Hash:        hash,
		Bytes:       len(raw),
		Summary:     strings.TrimSpace(summary),
		UpdatedAt:   time.Now(),
	}
	b.artifacts[id] = art
	b.touchLocked(ContextDirtyArtifacts)
	return id
}

func (b *ContextBus) trimEventsLocked(max int) {
	if max <= 0 || len(b.events) <= max {
		return
	}
	b.events = append([]ContextEvent(nil), b.events[len(b.events)-max:]...)
}

func (b *ContextBus) touchLocked(flag ContextDirtyFlags) {
	b.version++
	b.dirty |= flag
}

type contextCandidate struct {
	group  string
	kind   typesys.Kind
	moment ContextMoment
	text   string
	tokens int
	score  float64
	order  int
}

func CompileContextSnapshot(snap ContextSnapshot, spec ContextCompileSpec) CompiledContext {
	snap = filterSnapshotForAgent(snap, strings.TrimSpace(spec.AgentName), spec.Channels)
	budget := spec.Budget.normalized()
	limit := budget.limitTokens()
	currentMoment := defaultMoment(spec.Moment, ContextMomentGeneral)
	weights := spec.WeightProfile.normalized()

	mandatory := make([]string, 0, 6)
	mandatory = append(mandatory, "[context]")
	if strings.TrimSpace(snap.Goal) != "" {
		mandatory = append(mandatory, "goal: "+snap.Goal)
	}
	if strings.TrimSpace(spec.AgentName) != "" {
		mandatory = append(mandatory, "agent: "+spec.AgentName)
	}
	mandatory = append(mandatory, "context_moment: "+string(currentMoment))
	if len(spec.Skills) > 0 {
		skills := append([]string(nil), spec.Skills...)
		sort.Strings(skills)
		mandatory = append(mandatory, "available_skills: "+strings.Join(skills, ", "))
	}
	mandatory = append(mandatory, "instruction: use the shared context below, prefer tools only when needed, avoid repeating large content.")

	used := estimateLinesTokens(mandatory)
	selectedByGroup := map[string][]contextCandidate{}
	dropped := 0
	counts := map[string]int{}

	cands := buildContextCandidates(snap, budget)
	for i := range cands {
		cands[i].score += scoreTypeAndMoment(cands[i].kind, cands[i].moment, currentMoment, weights)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		di := cands[i].score / float64(maxInt(cands[i].tokens, 1))
		dj := cands[j].score / float64(maxInt(cands[j].tokens, 1))
		if math.Abs(di-dj) > 1e-9 {
			return di > dj
		}
		if math.Abs(cands[i].score-cands[j].score) > 1e-9 {
			return cands[i].score > cands[j].score
		}
		return cands[i].order < cands[j].order
	})

	for _, c := range cands {
		if !allowCandidateGroupCount(c.group, counts, budget) {
			dropped++
			continue
		}
		if used+c.tokens > limit {
			dropped++
			continue
		}
		selectedByGroup[c.group] = append(selectedByGroup[c.group], c)
		counts[c.group]++
		used += c.tokens
	}

	lines := append([]string(nil), mandatory...)
	type groupSpec struct {
		key    string
		header string
	}
	groups := []groupSpec{
		{key: "constraints", header: "constraints:"},
		{key: "facts", header: "shared_facts:"},
		{key: "questions", header: "open_questions:"},
		{key: "events", header: "recent_events:"},
		{key: "artifacts", header: "artifacts:"},
	}
	for _, g := range groups {
		items := selectedByGroup[g.key]
		if len(items) == 0 {
			continue
		}
		sort.SliceStable(items, func(i, j int) bool { return items[i].order < items[j].order })
		lines = append(lines, g.header)
		for _, it := range items {
			lines = append(lines, "- "+it.text)
		}
	}

	text := strings.Join(lines, "\n")
	return CompiledContext{
		Text:              text,
		EstimatedTokens:   estimateTokens(text),
		BudgetLimitTokens: limit,
		Version:           snap.Version,
		DirtyFlags:        snap.DirtyFlags,
		Moment:            currentMoment,
		SelectedFacts:     len(selectedByGroup["facts"]),
		SelectedEvents:    len(selectedByGroup["events"]),
		SelectedArtifacts: len(selectedByGroup["artifacts"]),
		SelectedQuestions: len(selectedByGroup["questions"]),
		DroppedItems:      dropped,
	}
}

func filterSnapshotForAgent(snap ContextSnapshot, agentName string, allowedChannels []string) ContextSnapshot {
	agentName = strings.TrimSpace(agentName)
	channelSet := normalizeChannelSet(allowedChannels)
	if agentName == "" && len(channelSet) == 0 {
		return snap
	}

	out := snap
	out.Facts = make([]ContextFact, 0, len(snap.Facts))
	for _, f := range snap.Facts {
		if !contextFactVisibleToAgent(f, agentName, channelSet) {
			continue
		}
		out.Facts = append(out.Facts, f)
	}

	out.RecentEvents = make([]ContextEvent, 0, len(snap.RecentEvents))
	visibleArtifactIDs := map[string]struct{}{}
	for _, ev := range snap.RecentEvents {
		if !contextEventVisibleToAgent(ev, agentName, channelSet) {
			continue
		}
		out.RecentEvents = append(out.RecentEvents, ev)
		if strings.TrimSpace(ev.ArtifactID) != "" {
			visibleArtifactIDs[strings.TrimSpace(ev.ArtifactID)] = struct{}{}
		}
	}
	// Facts may contain ref/artifact tokens too.
	for _, f := range out.Facts {
		for _, id := range referencedArtifactIDsInFact(f.Value) {
			visibleArtifactIDs[id] = struct{}{}
		}
	}

	out.Artifacts = make([]ContextArtifact, 0, len(snap.Artifacts))
	for _, a := range snap.Artifacts {
		if contextArtifactVisibleToAgent(a, agentName, channelSet) {
			out.Artifacts = append(out.Artifacts, a)
			continue
		}
		// Legacy fallback: keep if a visible fact/event explicitly references it.
		if _, ok := visibleArtifactIDs[a.ID]; ok {
			out.Artifacts = append(out.Artifacts, a)
		}
	}
	return out
}

func normalizeChannelSet(channels []string) map[string]struct{} {
	if len(channels) == 0 {
		return nil
	}
	out := map[string]struct{}{}
	for _, raw := range channels {
		ch := strings.TrimSpace(raw)
		if ch == "" {
			continue
		}
		out[ch] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func channelAllowed(channel string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return true // global / unscoped context remains visible
	}
	_, ok := allowed[channel]
	return ok
}

func contextFactVisibleToAgent(f ContextFact, agentName string, allowedChannels map[string]struct{}) bool {
	if !channelAllowed(f.Channel, allowedChannels) {
		return false
	}
	if target := strings.TrimSpace(f.TargetAgent); target != "" {
		return target == agentName
	}
	// Legacy handoff facts were encoded as handoff.<agent>.<port>
	if strings.HasPrefix(f.Key, "handoff.") {
		parts := strings.Split(f.Key, ".")
		if len(parts) >= 3 && strings.TrimSpace(parts[1]) != "" {
			return parts[1] == agentName
		}
	}
	return true
}

func contextEventVisibleToAgent(ev ContextEvent, agentName string, allowedChannels map[string]struct{}) bool {
	if !channelAllowed(ev.Channel, allowedChannels) {
		return false
	}
	if target := strings.TrimSpace(ev.TargetAgent); target != "" {
		return target == agentName
	}
	if ev.Type == "agent_handoff" {
		if to := parseKVField(ev.Summary, "to"); to != "" {
			return to == agentName
		}
	}
	return true
}

func contextArtifactVisibleToAgent(a ContextArtifact, agentName string, allowedChannels map[string]struct{}) bool {
	if !channelAllowed(a.Channel, allowedChannels) {
		return false
	}
	if target := strings.TrimSpace(a.TargetAgent); target != "" {
		return target == agentName
	}
	return true
}

func parseKVField(s, key string) string {
	s = strings.TrimSpace(s)
	if s == "" || key == "" {
		return ""
	}
	prefix := key + "="
	for _, part := range strings.Fields(s) {
		if strings.HasPrefix(part, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(part, prefix))
		}
	}
	return ""
}

func referencedArtifactIDsInFact(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	out := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, tok := range strings.Fields(v) {
		for _, prefix := range []string{"artifact=", "ref="} {
			if !strings.HasPrefix(tok, prefix) {
				continue
			}
			id := strings.TrimSpace(strings.TrimPrefix(tok, prefix))
			id = strings.Trim(id, ",;")
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func shortHashString(s string) string {
	if len(s) <= 12 {
		return s
	}
	h := sha1.Sum([]byte(s))
	sum := hex.EncodeToString(h[:])
	return shortHash(sum)
}

func buildContextCandidates(snap ContextSnapshot, budget ContextBudget) []contextCandidate {
	cands := make([]contextCandidate, 0, 64)
	order := 0

	for i, c := range snap.Constraints {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		text := trim(c, 180)
		cands = append(cands, contextCandidate{
			group:  "constraints",
			kind:   typesys.KindMessageSystem,
			moment: ContextMomentPlan,
			text:   text,
			tokens: estimateTokens("- " + text),
			score:  9.0 - float64(i)*0.1,
			order:  order,
		})
		order++
	}

	for _, f := range snap.Facts {
		line := renderFactCandidateLine(f)
		score := 8.0 + 3.0*clamp01(f.Confidence)
		cands = append(cands, contextCandidate{
			group:  "facts",
			kind:   defaultKind(f.Kind, typesys.KindMemoryFact),
			moment: defaultMoment(f.Moment, ContextMomentGeneral),
			text:   line,
			tokens: estimateTokens("- " + line),
			score:  score,
			order:  order,
		})
		order++
	}

	for i, q := range snap.OpenQuestions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		text := trim(q, 180)
		score := 8.5 - float64(i)*0.1
		cands = append(cands, contextCandidate{
			group:  "questions",
			kind:   typesys.KindMemoryQuestion,
			moment: ContextMomentGeneral,
			text:   text,
			tokens: estimateTokens("- " + text),
			score:  score,
			order:  order,
		})
		order++
	}

	// Recent events: newest first for scoring, but render in stable order among selected.
	events := snap.RecentEvents
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		line := renderEventCandidateLine(ev, budget)
		score := 6.0 + recencyScore(len(events)-1-i)
		if ev.HasError {
			score += 2.0
		}
		cands = append(cands, contextCandidate{
			group:  "events",
			kind:   defaultKind(ev.Kind, inferEventKind(ev)),
			moment: defaultMoment(ev.Moment, ContextMomentGeneral),
			text:   line,
			tokens: estimateTokens("- " + line),
			score:  score,
			order:  order,
		})
		order++
	}

	for i, a := range snap.Artifacts {
		line := renderArtifactCandidateLine(a, budget)
		score := 5.0 + recencyScore(i)
		cands = append(cands, contextCandidate{
			group:  "artifacts",
			kind:   defaultKind(a.Kind, typesys.KindArtifactRef),
			moment: defaultMoment(a.Moment, ContextMomentGeneral),
			text:   line,
			tokens: estimateTokens("- " + line),
			score:  score,
			order:  order,
		})
		order++
	}

	return cands
}

func allowCandidateGroupCount(group string, counts map[string]int, budget ContextBudget) bool {
	switch group {
	case "constraints":
		return counts[group] < budget.MaxConstraints
	case "facts":
		return counts[group] < budget.MaxFacts
	case "questions":
		return counts[group] < budget.MaxOpenQuestions
	case "events":
		return counts[group] < budget.MaxRecentEvents
	case "artifacts":
		return counts[group] < budget.MaxArtifacts
	default:
		return true
	}
}

func estimateLinesTokens(lines []string) int {
	if len(lines) == 0 {
		return 0
	}
	return estimateTokens(strings.Join(lines, "\n"))
}

func estimateTokens(s string) int {
	// Fast heuristic for prompt budgeting. Keeps runtime light and deterministic.
	chars := len(strings.TrimSpace(s))
	if chars == 0 {
		return 0
	}
	return (chars + 3) / 4
}

func recencyScore(indexFromNewest int) float64 {
	if indexFromNewest < 0 {
		return 0
	}
	return 2.0 / float64(indexFromNewest+1)
}

func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func defaultKind(k, fallback typesys.Kind) typesys.Kind {
	if k == "" {
		return fallback
	}
	return k
}

func defaultMoment(m, fallback ContextMoment) ContextMoment {
	if m == "" {
		return fallback
	}
	return m
}

func inferEventKind(ev ContextEvent) typesys.Kind {
	if ev.Kind != "" {
		return ev.Kind
	}
	if ev.HasError {
		return typesys.KindToolError
	}
	switch ev.Type {
	case "skill_call":
		return typesys.KindToolResult
	case "agent_output":
		return typesys.KindMessageAgent
	default:
		return typesys.KindMemorySummary
	}
}

func inferSkillResultKind(call SkillCallResult) typesys.Kind {
	if call.Result == nil {
		return typesys.KindToolResult
	}
	if k, ok := stringMapValue(call.Result, "detected_kind"); ok {
		if kind, ok := typesys.NormalizeKind(k); ok {
			return kind
		}
	}
	if path, ok := stringMapValue(call.Result, "path"); ok && path != "" {
		return typesys.InferKindFromPath(path)
	}
	if _, ok := call.Result["status"]; ok {
		if ct, ok := stringMapValue(call.Result, "content_type"); ok {
			ct = strings.ToLower(ct)
			switch {
			case strings.Contains(ct, "json"):
				return typesys.KindJSONObject
			case strings.Contains(ct, "markdown"):
				return typesys.KindTextMarkdown
			default:
				return typesys.KindTextPlain
			}
		}
		return typesys.KindToolResult
	}
	if _, ok := call.Result["text"]; ok {
		return typesys.KindTextPlain
	}
	if _, ok := call.Result["body"]; ok {
		return typesys.KindTextPlain
	}
	return typesys.KindToolResult
}

func stringMapValue(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func summarizeArtifactPayload(kind typesys.Kind, result map[string]any, raw string) string {
	parts := make([]string, 0, 8)
	if path, ok := stringMapValue(result, "path"); ok && strings.TrimSpace(path) != "" {
		parts = append(parts, "path="+path)
	}
	if r := rangeLabelFromResult(result); r != "" {
		parts = append(parts, "range="+r)
	}
	if v, ok := stringMapValue(result, "content_sha1"); ok && v != "" {
		parts = append(parts, "sha1="+shortHash(v))
	}
	if b, ok := intMapValue(result, "bytes"); ok && b > 0 {
		parts = append(parts, fmt.Sprintf("bytes=%d", b))
	}
	if tr, ok := boolMapValue(result, "truncated"); ok && tr {
		parts = append(parts, "truncated=true")
	}

	switch {
	case isCodeKind(kind):
		if outline := summarizeCodeSnippet(kind, raw); outline != "" {
			parts = append(parts, outline)
		}
	case kind == typesys.KindJSONObject || kind == typesys.KindJSONArray || kind == typesys.KindJSONValue:
		if outline := summarizeJSONPayload(raw); outline != "" {
			parts = append(parts, outline)
		}
	case kind == typesys.KindTextMarkdown:
		if outline := summarizeMarkdown(raw); outline != "" {
			parts = append(parts, outline)
		}
	case kind == typesys.KindTextPlain:
		if outline := summarizePlainText(raw); outline != "" {
			parts = append(parts, outline)
		}
	}

	if len(parts) == 0 {
		return trim(raw, 180)
	}
	return trim(strings.Join(parts, " "), 220)
}

func summarizeRawByKind(kind typesys.Kind, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	switch {
	case isCodeKind(kind):
		return summarizeCodeSnippet(kind, raw)
	case kind == typesys.KindJSONObject || kind == typesys.KindJSONArray || kind == typesys.KindJSONValue:
		return summarizeJSONPayload(raw)
	case kind == typesys.KindTextMarkdown:
		return summarizeMarkdown(raw)
	case kind == typesys.KindSummaryCode:
		return summarizeCodeSnippet(typesys.KindCodeGo, raw)
	case kind == typesys.KindSummaryText, kind == typesys.KindMessageAgent, kind == typesys.KindMessageAssistant, kind == typesys.KindTextPlain:
		return summarizePlainText(raw)
	default:
		if strings.HasPrefix(kind.String(), "summary/") {
			return summarizePlainText(raw)
		}
		return summarizePlainText(raw)
	}
}

func rangeLabelFromResult(result map[string]any) string {
	start, okStart := intMapValue(result, "start_line")
	end, okEnd := intMapValue(result, "actual_end")
	if !okEnd {
		end, okEnd = intMapValue(result, "end_line")
	}
	if okStart && okEnd && start > 0 && end >= start {
		return fmt.Sprintf("%d-%d", start, end)
	}
	return ""
}

func summarizeCodeSnippet(kind typesys.Kind, raw string) string {
	switch kind {
	case typesys.KindCodeGo:
		return summarizeGoCode(raw)
	case typesys.KindCodeTS, typesys.KindCodeTSX, typesys.KindCodeJS, typesys.KindCodeJSX:
		return summarizeTSJSCode(raw)
	case typesys.KindCodePython:
		return summarizePythonCode(raw)
	default:
		return summarizeGenericCode(raw)
	}
}

func summarizeGoCode(raw string) string {
	var (
		pkg      string
		imports  []string
		symbols  []string
		inImport bool
	)
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "package ") && pkg == "" {
			pkg = strings.TrimSpace(strings.TrimPrefix(line, "package "))
			continue
		}
		if strings.HasPrefix(line, "import (") {
			inImport = true
			continue
		}
		if inImport {
			if strings.HasPrefix(line, ")") {
				inImport = false
				continue
			}
			if imp := quotedToken(line); imp != "" {
				imports = appendUniqueLimited(imports, imp, 5)
			}
			continue
		}
		if strings.HasPrefix(line, "import ") {
			if imp := quotedToken(line); imp != "" {
				imports = appendUniqueLimited(imports, imp, 5)
			}
			continue
		}
		if strings.HasPrefix(line, "func ") {
			if name := goFuncName(line); name != "" {
				symbols = appendUniqueLimited(symbols, "func:"+name, 8)
			}
			continue
		}
		if strings.HasPrefix(line, "type ") {
			if name := secondField(line); name != "" {
				symbols = appendUniqueLimited(symbols, "type:"+name, 8)
			}
			continue
		}
		if strings.HasPrefix(line, "var ") {
			if name := secondField(line); name != "" {
				symbols = appendUniqueLimited(symbols, "var:"+name, 8)
			}
			continue
		}
		if strings.HasPrefix(line, "const ") {
			if name := secondField(line); name != "" {
				symbols = appendUniqueLimited(symbols, "const:"+name, 8)
			}
			continue
		}
	}
	return compactCodeSummary(pkg, imports, symbols, raw)
}

func summarizeTSJSCode(raw string) string {
	var imports, symbols []string
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "import ") {
			imports = appendUniqueLimited(imports, trim(line, 70), 5)
			continue
		}
		for _, prefix := range []string{
			"export async function ", "export function ", "function ",
			"export class ", "class ",
			"export interface ", "interface ",
			"export type ", "type ",
			"export const ", "const ",
		} {
			if strings.HasPrefix(line, prefix) {
				if name := identAfterPrefix(line, prefix); name != "" {
					symbols = appendUniqueLimited(symbols, name, 8)
				}
				break
			}
		}
	}
	return compactCodeSummary("", imports, symbols, raw)
}

func summarizePythonCode(raw string) string {
	var imports, symbols []string
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ") {
			imports = appendUniqueLimited(imports, trim(line, 70), 5)
			continue
		}
		if strings.HasPrefix(line, "def ") {
			if name := identAfterPrefix(line, "def "); name != "" {
				symbols = appendUniqueLimited(symbols, "def:"+name, 8)
			}
			continue
		}
		if strings.HasPrefix(line, "class ") {
			if name := identAfterPrefix(line, "class "); name != "" {
				symbols = appendUniqueLimited(symbols, "class:"+name, 8)
			}
			continue
		}
	}
	return compactCodeSummary("", imports, symbols, raw)
}

func summarizeGenericCode(raw string) string {
	var symbols []string
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		symbols = append(symbols, trim(line, 60))
		if len(symbols) >= 3 {
			break
		}
	}
	if len(symbols) == 0 {
		return fmt.Sprintf("lines=%d", countLines(raw))
	}
	return fmt.Sprintf("lines=%d sample=%s", countLines(raw), strings.Join(symbols, " | "))
}

func summarizeJSONPayload(raw string) string {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "json(unparsed)"
	}
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) > 6 {
			keys = keys[:6]
		}
		return fmt.Sprintf("json_object keys=%s", strings.Join(keys, ","))
	case []any:
		return fmt.Sprintf("json_array len=%d", len(x))
	default:
		return "json_value"
	}
}

func summarizeMarkdown(raw string) string {
	headings := make([]string, 0, 4)
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "#") {
			headings = append(headings, trim(line, 50))
			if len(headings) >= 4 {
				break
			}
		}
	}
	if len(headings) == 0 {
		return summarizePlainText(raw)
	}
	return fmt.Sprintf("headings=%s", strings.Join(headings, " | "))
}

func summarizePlainText(raw string) string {
	first := ""
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		first = trim(line, 80)
		break
	}
	if first == "" {
		return fmt.Sprintf("lines=%d", countLines(raw))
	}
	return fmt.Sprintf("lines=%d first=%s", countLines(raw), first)
}

func compactCodeSummary(pkg string, imports, symbols []string, raw string) string {
	parts := make([]string, 0, 5)
	parts = append(parts, fmt.Sprintf("lines=%d", countLines(raw)))
	if pkg != "" {
		parts = append(parts, "pkg="+pkg)
	}
	if len(imports) > 0 {
		parts = append(parts, "imports="+strings.Join(imports, ","))
	}
	if len(symbols) > 0 {
		parts = append(parts, "symbols="+strings.Join(symbols, ","))
	}
	return strings.Join(parts, " ")
}

func countLines(raw string) int {
	if raw == "" {
		return 0
	}
	return strings.Count(raw, "\n") + 1
}

func appendUniqueLimited(dst []string, v string, limit int) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return dst
	}
	for _, cur := range dst {
		if cur == v {
			return dst
		}
	}
	if limit > 0 && len(dst) >= limit {
		return dst
	}
	return append(dst, v)
}

func quotedToken(line string) string {
	start := strings.Index(line, "\"")
	if start < 0 {
		return ""
	}
	end := strings.Index(line[start+1:], "\"")
	if end < 0 {
		return ""
	}
	return line[start+1 : start+1+end]
}

func secondField(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return ""
	}
	return trimIdentifier(fields[1])
}

func goFuncName(line string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "func "))
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, "(") {
		idx := strings.Index(rest, ")")
		if idx < 0 || idx+1 >= len(rest) {
			return ""
		}
		rest = strings.TrimSpace(rest[idx+1:])
	}
	name := identBeforeOneOf(rest, "(", "[", " ")
	return trimIdentifier(name)
}

func identAfterPrefix(line, prefix string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	return trimIdentifier(identBeforeOneOf(rest, "(", "{", "=", ":", "<", " "))
}

func identBeforeOneOf(s string, seps ...string) string {
	if s == "" {
		return ""
	}
	cut := len(s)
	for _, sep := range seps {
		if idx := strings.Index(s, sep); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	return s[:cut]
}

func trimIdentifier(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ",{[(:")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Keep alnum/_/./* for method receivers or aliases if they leak in.
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '*' {
			out = append(out, r)
			continue
		}
		break
	}
	return string(out)
}

func intMapValue(m map[string]any, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func boolMapValue(m map[string]any, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func scoreTypeAndMoment(kind typesys.Kind, itemMoment, currentMoment ContextMoment, profile ContextWeightProfile) float64 {
	profile = profile.normalized()
	score := 0.0
	if def, ok := typesys.Lookup(kind.String()); ok {
		if w, ok := profile.TypeWeights[kind]; ok {
			score += w
		} else if w, ok := profile.CategoryWeights[def.Category]; ok {
			score += w
		}
		score += profile.momentBoost(currentMoment, itemMoment, def.Category, kind)
		return score
	}
	score += profile.momentBoost(currentMoment, itemMoment, "", kind)
	return score
}

func (p ContextWeightProfile) momentBoost(current, item ContextMoment, category string, kind typesys.Kind) float64 {
	if current == "" {
		current = ContextMomentGeneral
	}
	item = defaultMoment(item, ContextMomentGeneral)
	boost := 0.0
	if item == current {
		boost += p.MatchMomentBoost
	}
	if item == ContextMomentGeneral {
		boost += 0.3
	}
	weights := p.MomentWeights[current]
	if len(weights) == 0 {
		return boost
	}
	if w, ok := weights[kind.String()]; ok {
		boost += w
	}
	if category != "" {
		if w, ok := weights["cat:"+category]; ok {
			boost += w
		}
	}
	return boost
}

func renderFactCandidateLine(f ContextFact) string {
	kind := defaultKind(f.Kind, typesys.KindMemoryFact)
	valueLimit := 180
	if isCodeKind(kind) {
		valueLimit = 96
	}
	if isSummaryKind(kind) {
		valueLimit = 220
	}
	value := trim(f.Value, valueLimit)
	base := fmt.Sprintf("%s=%s", f.Key, value)
	switch kind {
	case typesys.KindMemoryFact:
		base = fmt.Sprintf("%s (conf=%.2f)", base, clamp01(f.Confidence))
	case typesys.KindMemoryQuestion:
		base = "question=" + value
	default:
		base = fmt.Sprintf("%s kind=%s conf=%.2f", base, kind, clamp01(f.Confidence))
	}
	if src := strings.TrimSpace(f.Source); src != "" {
		base += " src=" + src
	}
	return base
}

func renderEventCandidateLine(ev ContextEvent, budget ContextBudget) string {
	kind := defaultKind(ev.Kind, inferEventKind(ev))
	summaryLimit := budget.MaxEventSummary
	if summaryLimit <= 0 {
		summaryLimit = 160
	}
	switch {
	case kind == typesys.KindToolError:
		summaryLimit = minInt(summaryLimit, 140)
	case kind == typesys.KindToolResult:
		summaryLimit = minInt(summaryLimit, 140)
	case isSummaryKind(kind):
		summaryLimit = minInt(summaryLimit, 180)
	case isCodeKind(kind):
		summaryLimit = minInt(summaryLimit, 100)
	}
	summary := trim(ev.Summary, summaryLimit)
	parts := make([]string, 0, 6)
	switch kind {
	case typesys.KindToolError:
		parts = append(parts, "tool_error")
	case typesys.KindToolResult:
		parts = append(parts, "tool_result")
	case typesys.KindMessageAgent:
		parts = append(parts, "agent_msg")
	default:
		parts = append(parts, ev.Type)
	}
	if ev.Agent != "" {
		parts = append(parts, "agent="+ev.Agent)
	}
	if ev.Skill != "" {
		parts = append(parts, "skill="+ev.Skill)
	}
	if kind != "" && kind != typesys.KindToolResult && kind != typesys.KindToolError && kind != typesys.KindMessageAgent {
		parts = append(parts, "kind="+kind.String())
	}
	if ev.ArtifactID != "" {
		parts = append(parts, "artifact="+ev.ArtifactID)
	}
	if summary != "" {
		parts = append(parts, "summary="+summary)
	}
	return strings.Join(parts, " ")
}

func renderArtifactCandidateLine(a ContextArtifact, budget ContextBudget) string {
	kind := defaultKind(a.Kind, typesys.KindArtifactRef)
	summaryLimit := budget.MaxArtifactSummary
	if summaryLimit <= 0 {
		summaryLimit = 180
	}
	switch {
	case isCodeKind(kind):
		summaryLimit = minInt(summaryLimit, 120)
	case kind == typesys.KindJSONObject || kind == typesys.KindJSONArray || kind == typesys.KindJSONValue:
		summaryLimit = minInt(summaryLimit, 120)
	case isSummaryKind(kind):
		summaryLimit = minInt(summaryLimit, 200)
	case kind == typesys.KindArtifactRef:
		summaryLimit = minInt(summaryLimit, 140)
	}
	summary := trim(a.Summary, summaryLimit)
	parts := make([]string, 0, 10)
	switch {
	case isCodeKind(kind):
		parts = append(parts, "code_ref")
	case kind == typesys.KindJSONObject || kind == typesys.KindJSONArray || kind == typesys.KindJSONValue:
		parts = append(parts, "json_ref")
	case isSummaryKind(kind):
		parts = append(parts, "summary_ref")
	default:
		parts = append(parts, "artifact_ref")
	}
	parts = append(parts, "id="+a.ID)
	parts = append(parts, "kind="+kind.String())
	if a.Source != "" {
		parts = append(parts, "src="+a.Source)
	}
	parts = append(parts, fmt.Sprintf("bytes=%d", a.Bytes))
	parts = append(parts, "hash="+shortHash(a.Hash))
	if a.Moment != "" && a.Moment != ContextMomentGeneral {
		parts = append(parts, "moment="+string(a.Moment))
	}
	if summary != "" {
		parts = append(parts, "summary="+summary)
	}
	return strings.Join(parts, " ")
}

func isCodeKind(kind typesys.Kind) bool {
	return strings.HasPrefix(kind.String(), "code/")
}

func isSummaryKind(kind typesys.Kind) bool {
	return strings.HasPrefix(kind.String(), "summary/") || kind == typesys.KindArtifactSummary
}

func minInt(a, b int) int {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}
