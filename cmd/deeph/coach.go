package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"deeph/internal/runtime"
)

type coachHint struct {
	ID         string
	Kind       string
	Text       string
	Weight     float64
	Confidence int
}

type coachHintRequest struct {
	Workspace   string
	CommandPath string
	AgentSpec   string
	Input       string
	Plan        *runtime.ExecutionPlan
	Tasks       []runtime.Task
	InChat      bool
	ShowTrace   bool
	SessionID   string
	Turn        int
}

type coachState struct {
	Version           int                       `json:"version"`
	LastShownAt       time.Time                 `json:"last_shown_at,omitempty"`
	HintSeen          map[string]int            `json:"hint_seen,omitempty"`
	CommandSeen       map[string]int            `json:"command_seen,omitempty"`
	LastCommand       string                    `json:"last_command,omitempty"`
	LastCommandSpec   string                    `json:"last_command_spec,omitempty"`
	LastCommandAt     time.Time                 `json:"last_command_at,omitempty"`
	Transitions       map[string]int            `json:"transitions,omitempty"`
	PortSignals       map[string]int            `json:"port_signals,omitempty"`
	ScopedTransitions map[string]map[string]int `json:"scoped_transitions,omitempty"`
	ScopedPortSignals map[string]map[string]int `json:"scoped_port_signals,omitempty"`
}

type coachWaitHandle struct {
	once  sync.Once
	done  chan struct{}
	shown int32
}

func startCoachHint(ctx context.Context, req coachHintRequest) func() {
	if !coachEnabled() {
		return func() {}
	}
	if !isCharDevice(os.Stderr) {
		return func() {}
	}
	if strings.TrimSpace(req.Workspace) == "" {
		return func() {}
	}
	state, _ := loadCoachState(req.Workspace)
	hint, ok := chooseCoachHint(req, state)
	if !ok {
		return func() {}
	}

	h := &coachWaitHandle{done: make(chan struct{})}
	go h.loop(ctx, req, hint)
	return func() {
		h.once.Do(func() {
			close(h.done)
		})
		if atomic.LoadInt32(&h.shown) == 1 {
			recordCoachHintShown(req.Workspace, req.CommandPath, hint.ID)
		} else {
			recordCoachCommandUse(req.Workspace, req.CommandPath)
		}
	}
}

func (h *coachWaitHandle) loop(ctx context.Context, req coachHintRequest, hint coachHint) {
	delay := 900 * time.Millisecond
	if req.InChat {
		delay = 700 * time.Millisecond
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-h.done:
		return
	case <-timer.C:
	}

	atomic.StoreInt32(&h.shown, 1)
	lineWidth := 120
	prefix := "[deepH coach]"
	if supportsANSIColor() {
		prefix = "\x1b[36m[deepH coach]\x1b[0m"
	}
	msg := renderCoachHintLine(req, hint)
	frames := []string{"-", "\\", "|", "/"}
	tk := time.NewTicker(140 * time.Millisecond)
	defer tk.Stop()
	frameIdx := 0

	printCoachLine(prefix, frames[frameIdx], msg, lineWidth)
	for {
		select {
		case <-ctx.Done():
			clearCoachLine(lineWidth)
			return
		case <-h.done:
			clearCoachLine(lineWidth)
			return
		case <-tk.C:
			frameIdx = (frameIdx + 1) % len(frames)
			printCoachLine(prefix, frames[frameIdx], msg, lineWidth)
		}
	}
}

func printCoachLine(prefix, frame, msg string, lineWidth int) {
	line := fmt.Sprintf("%s %s %s", prefix, frame, msg)
	line = coachClip(line, lineWidth)
	_, _ = fmt.Fprintf(os.Stderr, "\r%-*s", lineWidth, line)
}

func clearCoachLine(lineWidth int) {
	_, _ = fmt.Fprintf(os.Stderr, "\r%-*s\r", lineWidth, "")
}

func renderCoachHintLine(req coachHintRequest, hint coachHint) string {
	label := strings.ToUpper(hint.Kind)
	if hint.Confidence > 0 {
		return fmt.Sprintf("%s %d%% | %s", label, hint.Confidence, hint.Text)
	}
	return fmt.Sprintf("%s | %s", label, hint.Text)
}

func chooseCoachHint(req coachHintRequest, state *coachState) (coachHint, bool) {
	cands := coachHintCandidates(req)
	if len(cands) == 0 {
		return coachHint{}, false
	}
	if state == nil {
		state = &coachState{Version: 1}
	}
	if state.HintSeen == nil {
		state.HintSeen = map[string]int{}
	}
	if state.CommandSeen == nil {
		state.CommandSeen = map[string]int{}
	}
	if state.Transitions == nil {
		state.Transitions = map[string]int{}
	}
	if state.PortSignals == nil {
		state.PortSignals = map[string]int{}
	}
	if state.ScopedTransitions == nil {
		state.ScopedTransitions = map[string]map[string]int{}
	}
	if state.ScopedPortSignals == nil {
		state.ScopedPortSignals = map[string]map[string]int{}
	}
	if !coachPassesGlobalCooldown(state) {
		return coachHint{}, false
	}
	if th, ok := coachTransitionHint(req, state); ok {
		cands = append(cands, th)
	}

	bestIdx := -1
	bestScore := -1e9
	cmdSeen := state.CommandSeen[strings.ToLower(strings.TrimSpace(req.CommandPath))]
	for i, c := range cands {
		seen := state.HintSeen[c.ID]
		if !coachShouldShowHint(seen, cmdSeen) {
			continue
		}
		score := c.Weight
		if seen > 0 {
			score -= float64(seen) * 0.9
		}
		if bestIdx == -1 || score > bestScore {
			bestIdx = i
			bestScore = score
		}
	}
	if bestIdx < 0 {
		return coachHint{}, false
	}
	return cands[bestIdx], true
}

func coachHintCandidates(req coachHintRequest) []coachHint {
	var out []coachHint

	hasMulti := req.Plan != nil && len(req.Plan.Tasks) > 1
	hasStages := req.Plan != nil && len(req.Plan.Stages) > 1
	hasDeepSeek := false
	hasTools := false
	hasFileRead := false
	hasFileReadRange := false
	hasTypedIO := false
	hasHandoffs := req.Plan != nil && len(req.Plan.Handoffs) > 0

	if req.Plan != nil {
		for _, t := range req.Plan.Tasks {
			if t.ProviderType == "deepseek" {
				hasDeepSeek = true
			}
			if len(t.Skills) > 0 {
				hasTools = true
			}
			if len(t.IO.Inputs) > 0 || len(t.IO.Outputs) > 0 {
				hasTypedIO = true
			}
			for _, s := range t.Skills {
				switch s {
				case "file_read":
					hasFileRead = true
				case "file_read_range":
					hasFileReadRange = true
				}
			}
		}
	}

	switch req.CommandPath {
	case "chat":
		if req.InChat && req.Turn <= 1 {
			out = append(out, coachHint{
				ID:         "chat.slash.help",
				Kind:       "ux",
				Confidence: 85,
				Weight:     8.2,
				Text:       "Use /history and /trace during chat to inspect context and orchestration without leaving the session.",
			})
		}
		if hasMulti || hasStages {
			out = append(out, coachHint{
				ID:         "chat.trace.json.multi",
				Kind:       "next",
				Confidence: 80,
				Weight:     8.0,
				Text:       fmt.Sprintf("After this reply, run `deeph trace --json %q` to inspect channels/handoffs and tune ports.", req.AgentSpec),
			})
		}
		if hasTypedIO || hasHandoffs {
			out = append(out, coachHint{
				ID:         "chat.ports.merge",
				Kind:       "best-practice",
				Confidence: 72,
				Weight:     7.0,
				Text:       "For cleaner multi-agent chat, set `merge_policy`, `max_tokens` and `channel_priority` on input ports that receive handoffs.",
			})
		}
		if hasFileRead && !hasFileReadRange {
			out = append(out, coachHint{
				ID:         "chat.file_read_range",
				Kind:       "efficiency",
				Confidence: 78,
				Weight:     8.4,
				Text:       "Prefer `file_read_range` over `file_read` for chat flows; it reduces token replay and tool-loop churn.",
			})
		}
		if hasDeepSeek {
			out = append(out, coachHint{
				ID:         "chat.deepseek.cache",
				Kind:       "efficiency",
				Confidence: 76,
				Weight:     6.8,
				Text:       "Keep prompts and schemas stable across turns to improve DeepSeek prefix cache hits (lower latency/cost).",
			})
		}
		out = append(out, coachHint{
			ID:         "chat.session.show",
			Kind:       "next",
			Confidence: 67,
			Weight:     5.4,
			Text:       "Use `deeph session show --tail 50 <id>` after longer chats to audit decisions, tool calls and outputs.",
		})

	case "run":
		if hasMulti && !req.ShowTrace {
			out = append(out, coachHint{
				ID:         "run.trace.before",
				Kind:       "next",
				Confidence: 82,
				Weight:     8.3,
				Text:       "Common next step: `deeph trace` the same spec before larger runs to inspect stage waits, channels and handoffs.",
			})
		}
		if hasHandoffs && hasTypedIO {
			out = append(out, coachHint{
				ID:         "run.depends_on_ports",
				Kind:       "best-practice",
				Confidence: 74,
				Weight:     7.4,
				Text:       "Use `depends_on_ports` to cut cross-talk and unlock selective stage wait when only some handoffs matter.",
			})
		}
		if hasFileRead && !hasFileReadRange {
			out = append(out, coachHint{
				ID:         "run.file_read_range",
				Kind:       "efficiency",
				Confidence: 79,
				Weight:     8.1,
				Text:       "Swap `file_read` for `file_read_range` in code pipelines; fewer tokens, faster loops, cleaner summaries.",
			})
		}
		if hasDeepSeek && hasTools {
			out = append(out, coachHint{
				ID:         "run.tool_budget",
				Kind:       "best-practice",
				Confidence: 75,
				Weight:     7.7,
				Text:       "Set `tool_max_calls` / `stage_tool_max_calls` in agent metadata to prevent expensive tool storms in parallel stages.",
			})
		}
		if !hasDeepSeek {
			out = append(out, coachHint{
				ID:         "run.provider.deepseek",
				Kind:       "next",
				Confidence: 65,
				Weight:     4.9,
				Text:       "To test real latency/cost behavior, scaffold a provider with `deeph provider add deepseek --set-default`.",
			})
		}
	}

	// Generic low-noise hint, only if nothing else wins.
	out = append(out, coachHint{
		ID:         "generic.command.dict",
		Kind:       "ux",
		Confidence: 60,
		Weight:     2.0,
		Text:       "Explore `deeph command list --json` and `deeph type list --json` to auto-generate team docs or shell helpers.",
	})
	return out
}

func coachTransitionHint(req coachHintRequest, state *coachState) (coachHint, bool) {
	if state == nil {
		return coachHint{}, false
	}
	from := strings.ToLower(strings.TrimSpace(req.CommandPath))
	if from == "" {
		return coachHint{}, false
	}
	next, conf, count, total := coachTopNextCommandForSpec(state, from, req.AgentSpec)
	if next == "" || next == from {
		return coachHint{}, false
	}
	if total < 3 || count < 2 || conf < 65 {
		return coachHint{}, false
	}

	text := fmt.Sprintf("Your common next step after `%s` is `%s` (%d%% of %d local transitions).", from, next, conf, total)
	if from == "run" && next == "trace" {
		if strings.TrimSpace(req.AgentSpec) != "" {
			text = fmt.Sprintf("You usually inspect after `run`: try `deeph trace %q` next (%d%% local pattern).", req.AgentSpec, conf)
		} else {
			text = fmt.Sprintf("You usually inspect after `run`: try `deeph trace` next (%d%% local pattern).", conf)
		}
	} else if from == "chat" && next == "session show" {
		text = fmt.Sprintf("After chat, you often audit the session with `deeph session show <id>` (%d%% local pattern).", conf)
	}
	return coachHint{
		ID:         "transition." + from + "." + strings.ReplaceAll(next, " ", "_"),
		Kind:       "next",
		Text:       text,
		Weight:     8.6 + float64(conf)/100.0,
		Confidence: conf,
	}, true
}

func coachTopNextCommand(state *coachState, from string) (next string, confidence, count, total int) {
	return coachTopNextCommandForSpec(state, from, "")
}

func coachTopNextCommandForSpec(state *coachState, from, spec string) (next string, confidence, count, total int) {
	if scoped := coachScopedTransitionsMap(state, spec); len(scoped) > 0 {
		return coachTopNextCommandFromMap(scoped, from)
	}
	return coachTopNextCommandFromMap(nilIfStateTransitions(state), from)
}

func nilIfStateTransitions(state *coachState) map[string]int {
	if state == nil {
		return nil
	}
	return state.Transitions
}

func coachTopNextCommandFromMap(transitions map[string]int, from string) (next string, confidence, count, total int) {
	if len(transitions) == 0 {
		return "", 0, 0, 0
	}
	from = strings.ToLower(strings.TrimSpace(from))
	if from == "" {
		return "", 0, 0, 0
	}
	prefix := from + "->"
	bestNext := ""
	bestCount := 0
	for key, c := range transitions {
		if c <= 0 || !strings.HasPrefix(key, prefix) {
			continue
		}
		n := strings.TrimPrefix(key, prefix)
		if n == "" {
			continue
		}
		total += c
		if c > bestCount || (c == bestCount && n < bestNext) {
			bestNext = n
			bestCount = c
		}
	}
	if total == 0 || bestNext == "" {
		return "", 0, 0, 0
	}
	conf := int(float64(bestCount)*100/float64(total) + 0.5)
	return bestNext, conf, bestCount, total
}

func coachSpecKey(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	return strings.Join(strings.Fields(spec), " ")
}

func coachScopedTransitionsMap(st *coachState, spec string) map[string]int {
	if st == nil || len(st.ScopedTransitions) == 0 {
		return nil
	}
	key := coachSpecKey(spec)
	if key == "" {
		return nil
	}
	return st.ScopedTransitions[key]
}

func coachScopedPortSignalsMap(st *coachState, spec string) map[string]int {
	if st == nil || len(st.ScopedPortSignals) == 0 {
		return nil
	}
	key := coachSpecKey(spec)
	if key == "" {
		return nil
	}
	return st.ScopedPortSignals[key]
}

func coachEnsureScopedCounter(parent map[string]map[string]int, key string) map[string]int {
	if parent == nil || key == "" {
		return nil
	}
	if m := parent[key]; m != nil {
		return m
	}
	m := map[string]int{}
	parent[key] = m
	return m
}

func coachShouldShowHint(seen, cmdSeen int) bool {
	// First-time and early guidance is valuable; then taper quickly.
	switch seen {
	case 0:
		return true
	case 1:
		return cmdSeen >= 2
	case 2:
		return cmdSeen >= 4
	default:
		return cmdSeen > 0 && cmdSeen%5 == 0 && seen < 6
	}
}

func coachPassesGlobalCooldown(state *coachState) bool {
	if state == nil || state.LastShownAt.IsZero() {
		return true
	}
	return time.Since(state.LastShownAt) >= 90*time.Second
}

func coachEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("DEEPH_COACH")))
	switch v {
	case "", "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func isCharDevice(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func supportsANSIColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := strings.TrimSpace(strings.ToLower(os.Getenv("TERM")))
	if term == "" || term == "dumb" {
		return false
	}
	return true
}

func coachClip(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

func coachStatePath(workspace string) string {
	return filepath.Join(workspace, ".deeph", "coach_state.json")
}

func maybePrintCoachPostRunHint(workspace, commandPath string, plan *runtime.ExecutionPlan, report runtime.ExecutionReport) {
	if !coachEnabled() || !isCharDevice(os.Stderr) {
		return
	}
	if strings.TrimSpace(workspace) == "" {
		return
	}
	st, err := loadCoachState(workspace)
	if err != nil || st == nil {
		return
	}
	hint, ok := chooseCoachPostRunHint(strings.ToLower(strings.TrimSpace(commandPath)), plan, report, st)
	if !ok {
		return
	}
	// Post-run hints can appear a bit more often if they signal real drops/budget hits.
	if !coachPassesPostRunCooldown(st, hint.Kind) {
		return
	}
	printCoachPostRunLine(hint)
	recordCoachHintShownOnly(workspace, hint.ID)
}

func chooseCoachPostRunHint(commandPath string, plan *runtime.ExecutionPlan, report runtime.ExecutionReport, st *coachState) (coachHint, bool) {
	planSpec := ""
	if plan != nil {
		planSpec = plan.Spec
	}
	stView := st
	if st != nil {
		if scopedPorts := coachScopedPortSignalsMap(st, planSpec); len(scopedPorts) > 0 {
			cp := *st
			cp.PortSignals = scopedPorts
			stView = &cp
		}
	}
	type agg struct {
		contextDroppedAgents     int
		contextDroppedItems      int
		contextChannelsDropped   int
		handoffDroppedAgents     int
		handoffDroppedCount      int
		toolBudgetHitAgents      int
		stageToolBudgetHitAgents int
	}
	var a agg
	for _, r := range report.Results {
		if r.ContextDropped > 0 {
			a.contextDroppedAgents++
			a.contextDroppedItems += r.ContextDropped
		}
		if r.ContextChannelsDropped > 0 {
			a.contextChannelsDropped += r.ContextChannelsDropped
		}
		if r.DroppedHandoffs > 0 {
			a.handoffDroppedAgents++
			a.handoffDroppedCount += r.DroppedHandoffs
		}
		if (r.ToolBudgetCallsLimit > 0 && r.ToolBudgetCallsUsed >= r.ToolBudgetCallsLimit) ||
			(r.ToolBudgetExecMSLimit > 0 && r.ToolBudgetExecMSUsed >= r.ToolBudgetExecMSLimit) {
			a.toolBudgetHitAgents++
		}
		if (r.StageToolBudgetCallsLimit > 0 && r.StageToolBudgetCallsUsed >= r.StageToolBudgetCallsLimit) ||
			(r.StageToolBudgetExecMSLimit > 0 && r.StageToolBudgetExecMSUsed >= r.StageToolBudgetExecMSLimit) {
			a.stageToolBudgetHitAgents++
		}
	}
	planByAgent := coachTaskPlanByAgent(plan)
	topHandoffProducers := coachTopResultMetrics(report.Results, func(r runtime.AgentRunResult) int { return r.DroppedHandoffs }, 2)
	topContextChannelAgents := coachTopResultMetrics(report.Results, func(r runtime.AgentRunResult) int { return r.ContextChannelsDropped }, 2)
	topContextDropAgents := coachTopResultMetrics(report.Results, func(r runtime.AgentRunResult) int { return r.ContextDropped }, 2)
	topToolBudgetAgents := coachTopResultMetrics(report.Results, func(r runtime.AgentRunResult) int {
		hit := 0
		if (r.ToolBudgetCallsLimit > 0 && r.ToolBudgetCallsUsed >= r.ToolBudgetCallsLimit) ||
			(r.ToolBudgetExecMSLimit > 0 && r.ToolBudgetExecMSUsed >= r.ToolBudgetExecMSLimit) {
			hit = 1
		}
		return hit
	}, 2)
	topStageBudgetAgents := coachTopResultMetrics(report.Results, func(r runtime.AgentRunResult) int {
		hit := 0
		if (r.StageToolBudgetCallsLimit > 0 && r.StageToolBudgetCallsUsed >= r.StageToolBudgetCallsLimit) ||
			(r.StageToolBudgetExecMSLimit > 0 && r.StageToolBudgetExecMSUsed >= r.StageToolBudgetExecMSLimit) {
			hit = 1
		}
		return hit
	}, 2)

	cands := make([]coachHint, 0, 6)
	if a.handoffDroppedCount > 0 {
		producers := coachMetricAgentNames(topHandoffProducers)
		targetPorts := coachSuggestedTargetPortsForAgents(plan, producers, "handoff")
		targetPorts = coachRankPortRefsByHistory(stView, "handoff_drop", targetPorts)
		historicalHot := coachTopPortSignalsByKind(stView, "handoff_drop", 2)
		text := fmt.Sprintf("Handoff publish dropped %d channel(s). Tune `publish_max_channels` / `publish_max_channel_tokens` on the producer agents.", a.handoffDroppedCount)
		if len(producers) > 0 {
			text = fmt.Sprintf("Handoff publish dropped %d channel(s), mostly %s. Tune `publish_max_channels` / `publish_max_channel_tokens` on those agents.", a.handoffDroppedCount, coachFormatMetricAgents(topHandoffProducers))
		}
		if len(targetPorts) > 0 {
			text += " Also raise `channel_priority` for " + strings.Join(targetPorts, ", ") + "."
		} else if len(historicalHot) > 0 {
			text += " Historical hotspots: " + coachFormatPortSignalHotspots(historicalHot) + "."
		}
		cands = append(cands, coachHint{
			ID:         "postrun.handoff_drop",
			Kind:       "efficiency",
			Confidence: 84,
			Weight:     9.6 + float64(a.handoffDroppedCount)/5.0,
			Text:       text,
		})
	}
	if a.contextChannelsDropped > 0 {
		consumers := coachMetricAgentNames(topContextChannelAgents)
		portHints := coachSuggestedPortsFromTaskPlans(planByAgent, consumers, "priority")
		portHints = coachRankPortRefsByHistory(stView, "context_channel_drop", portHints)
		historicalHot := coachTopPortSignalsByKind(stView, "context_channel_drop", 2)
		text := fmt.Sprintf("Context compile dropped %d channel(s). Tune `context_max_channels`, `context_max_channel_tokens`, and per-port `channel_priority`.", a.contextChannelsDropped)
		if len(consumers) > 0 {
			text = fmt.Sprintf("Context compile dropped %d channel(s), mostly on %s. Tune `context_max_channels` / `context_max_channel_tokens` for those agents.", a.contextChannelsDropped, coachFormatMetricAgents(topContextChannelAgents))
		}
		if len(portHints) > 0 {
			text += " Candidate ports: " + strings.Join(portHints, ", ") + "."
		} else if len(historicalHot) > 0 {
			text += " Historical hotspots: " + coachFormatPortSignalHotspots(historicalHot) + "."
		}
		cands = append(cands, coachHint{
			ID:         "postrun.context_channel_drop",
			Kind:       "efficiency",
			Confidence: 82,
			Weight:     9.1 + float64(a.contextChannelsDropped)/6.0,
			Text:       text,
		})
	}
	if a.contextDroppedItems > 0 {
		text := fmt.Sprintf("Context compiler dropped %d item(s) across %d agent(s). Prefer `summary/*` + `artifact/ref` and tune `context_max_input_tokens`.", a.contextDroppedItems, a.contextDroppedAgents)
		if len(topContextDropAgents) > 0 {
			text = fmt.Sprintf("Context compiler dropped %d item(s), mostly on %s. Increase `context_max_input_tokens` there or reduce raw text handoffs.", a.contextDroppedItems, coachFormatMetricAgents(topContextDropAgents))
			historicalHot := coachTopPortSignalsByKind(stView, "context_drop", 2)
			if len(historicalHot) > 0 {
				text += " Frequent hotspots: " + coachFormatPortSignalHotspots(historicalHot) + "."
			}
		}
		cands = append(cands, coachHint{
			ID:         "postrun.context_drop",
			Kind:       "efficiency",
			Confidence: 78,
			Weight:     8.6 + float64(a.contextDroppedAgents),
			Text:       text,
		})
	}
	if a.stageToolBudgetHitAgents > 0 {
		text := "A stage-level tool budget was hit. Check `stage_tool_max_calls` / `stage_tool_max_exec_ms` and use `file_read_range` to reduce tool pressure."
		if len(topStageBudgetAgents) > 0 {
			stageSet := coachMetricAgentNames(topStageBudgetAgents)
			stageIdxs := coachStageIndexesForAgents(report.Results, stageSet)
			if len(stageIdxs) > 0 {
				text = fmt.Sprintf("Stage-level tool budget was hit (stage %s, agents %s). Tune `stage_tool_max_calls` / `stage_tool_max_exec_ms` and prefer `file_read_range`.", coachFormatIntList(stageIdxs), coachFormatMetricAgents(topStageBudgetAgents))
			}
		}
		cands = append(cands, coachHint{
			ID:         "postrun.stage_tool_budget_hit",
			Kind:       "best-practice",
			Confidence: 80,
			Weight:     9.0 + float64(a.stageToolBudgetHitAgents),
			Text:       text,
		})
	}
	if a.toolBudgetHitAgents > 0 {
		text := "An agent tool budget was hit. Tune `tool_max_calls`, `tool_max_exec_ms`, cache flags, or reduce repetitive tool loops."
		if len(topToolBudgetAgents) > 0 {
			text = fmt.Sprintf("Agent tool budget hit on %s. Tune `tool_max_calls` / `tool_max_exec_ms` and enable cache/locks where deterministic.", coachFormatMetricAgents(topToolBudgetAgents))
		}
		if coachAnyAgentUsesSkill(planByAgent, coachMetricAgentNames(topToolBudgetAgents), "file_read") && !coachAnyAgentUsesSkill(planByAgent, coachMetricAgentNames(topToolBudgetAgents), "file_read_range") {
			text += " Consider replacing `file_read` with `file_read_range` in those agents."
		}
		cands = append(cands, coachHint{
			ID:         "postrun.tool_budget_hit",
			Kind:       "best-practice",
			Confidence: 77,
			Weight:     8.4 + float64(a.toolBudgetHitAgents),
			Text:       text,
		})
	}

	if th, ok := coachTopTransitionPostHint(commandPath, st, plan); ok {
		cands = append(cands, th)
	}
	if len(cands) == 0 {
		return coachHint{}, false
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if c.Weight > best.Weight {
			best = c
		}
	}
	return best, true
}

type coachMetricAgent struct {
	Agent string
	Value int
}

func coachTopResultMetrics(results []runtime.AgentRunResult, f func(runtime.AgentRunResult) int, limit int) []coachMetricAgent {
	if limit <= 0 {
		limit = 1
	}
	out := make([]coachMetricAgent, 0, len(results))
	for _, r := range results {
		v := f(r)
		if v <= 0 {
			continue
		}
		out = append(out, coachMetricAgent{Agent: r.Agent, Value: v})
	}
	if len(out) == 0 {
		return nil
	}
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Value > out[i].Value || (out[j].Value == out[i].Value && out[j].Agent < out[i].Agent) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func coachFormatMetricAgents(ms []coachMetricAgent) string {
	if len(ms) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ms))
	for _, m := range ms {
		if m.Value > 1 {
			parts = append(parts, fmt.Sprintf("`%s` (%d)", m.Agent, m.Value))
		} else {
			parts = append(parts, fmt.Sprintf("`%s`", m.Agent))
		}
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func coachMetricAgentNames(ms []coachMetricAgent) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		if strings.TrimSpace(m.Agent) != "" {
			out = append(out, m.Agent)
		}
	}
	return out
}

func coachTaskPlanByAgent(plan *runtime.ExecutionPlan) map[string]runtime.TaskPlan {
	out := map[string]runtime.TaskPlan{}
	if plan == nil {
		return out
	}
	for _, t := range plan.Tasks {
		out[t.Agent] = t
	}
	return out
}

func coachSuggestedTargetPortsForAgents(plan *runtime.ExecutionPlan, producers []string, mode string) []string {
	if plan == nil || len(producers) == 0 {
		return nil
	}
	producerSet := map[string]struct{}{}
	for _, a := range producers {
		producerSet[a] = struct{}{}
	}
	out := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for _, h := range plan.Handoffs {
		if _, ok := producerSet[h.FromAgent]; !ok {
			continue
		}
		if mode == "handoff" && h.ChannelPriority > 0 {
			continue
		}
		port := h.ToAgent + "." + h.ToPort
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		out = append(out, "`"+port+"`")
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func coachSuggestedPortsFromTaskPlans(planByAgent map[string]runtime.TaskPlan, agents []string, mode string) []string {
	if len(planByAgent) == 0 || len(agents) == 0 {
		return nil
	}
	out := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for _, agent := range agents {
		t, ok := planByAgent[agent]
		if !ok {
			continue
		}
		for _, in := range t.IO.Inputs {
			if mode == "priority" && in.ChannelPriority > 0 {
				continue
			}
			key := t.Agent + "." + in.Name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, "`"+key+"`")
			if len(out) >= 4 {
				return out
			}
		}
	}
	return out
}

func coachAnyAgentUsesSkill(planByAgent map[string]runtime.TaskPlan, agents []string, skill string) bool {
	for _, agent := range agents {
		t, ok := planByAgent[agent]
		if !ok {
			continue
		}
		for _, s := range t.Skills {
			if s == skill {
				return true
			}
		}
	}
	return false
}

func coachStageIndexesForAgents(results []runtime.AgentRunResult, agents []string) []int {
	if len(agents) == 0 {
		return nil
	}
	set := map[string]struct{}{}
	for _, a := range agents {
		set[a] = struct{}{}
	}
	idxSet := map[int]struct{}{}
	out := make([]int, 0, len(agents))
	for _, r := range results {
		if _, ok := set[r.Agent]; !ok {
			continue
		}
		if _, ok := idxSet[r.StageIndex]; ok {
			continue
		}
		idxSet[r.StageIndex] = struct{}{}
		out = append(out, r.StageIndex)
	}
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func coachFormatIntList(xs []int) string {
	if len(xs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, fmt.Sprintf("%d", x))
	}
	return strings.Join(parts, ",")
}

func coachRankPortRefsByHistory(st *coachState, kind string, refs []string) []string {
	if len(refs) <= 1 || st == nil || len(st.PortSignals) == 0 {
		return refs
	}
	out := make([]string, len(refs))
	copy(out, refs)
	score := func(ref string) int {
		ref = strings.Trim(ref, "`")
		return coachPortSignalScore(st, kind, ref)
	}
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			si, sj := score(out[i]), score(out[j])
			if sj > si || (sj == si && out[j] < out[i]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func coachTopPortSignalsByKind(st *coachState, kind string, limit int) []coachCountKV {
	if st == nil || len(st.PortSignals) == 0 {
		return nil
	}
	prefix := kind + ":"
	out := make([]coachCountKV, 0, len(st.PortSignals))
	for k, v := range st.PortSignals {
		if v <= 0 || !strings.HasPrefix(k, prefix) {
			continue
		}
		ref := strings.TrimPrefix(k, prefix)
		if strings.TrimSpace(ref) == "" {
			continue
		}
		out = append(out, coachCountKV{Key: ref, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Key < out[j].Key
		}
		return out[i].Count > out[j].Count
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func coachFormatPortSignalHotspots(items []coachCountKV) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, kv := range items {
		parts = append(parts, fmt.Sprintf("`%s` (%d)", kv.Key, kv.Count))
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func coachPortSignalKey(kind, portRef string) string {
	kind = strings.TrimSpace(kind)
	portRef = strings.Trim(strings.TrimSpace(portRef), "`")
	if kind == "" || portRef == "" {
		return ""
	}
	return kind + ":" + portRef
}

func coachPortSignalScore(st *coachState, kind, portRef string) int {
	if st == nil || len(st.PortSignals) == 0 {
		return 0
	}
	key := coachPortSignalKey(kind, portRef)
	if key == "" {
		return 0
	}
	return st.PortSignals[key]
}

func recordCoachRunSignals(workspace string, plan *runtime.ExecutionPlan, report runtime.ExecutionReport) {
	if strings.TrimSpace(workspace) == "" {
		return
	}
	st, err := loadCoachState(workspace)
	if err != nil || st == nil {
		return
	}
	if st.PortSignals == nil {
		st.PortSignals = map[string]int{}
	}
	if st.ScopedPortSignals == nil {
		st.ScopedPortSignals = map[string]map[string]int{}
	}
	planByAgent := coachTaskPlanByAgent(plan)
	var scopedPortSignals map[string]int
	if plan != nil {
		if specKey := coachSpecKey(plan.Spec); specKey != "" {
			scopedPortSignals = coachEnsureScopedCounter(st.ScopedPortSignals, specKey)
		}
	}
	for _, r := range report.Results {
		if r.Agent == "" {
			continue
		}
		tp, _ := planByAgent[r.Agent]
		if r.ContextChannelsDropped > 0 {
			ports := coachSuggestedPortsFromTaskPlans(planByAgent, []string{r.Agent}, "priority")
			if len(ports) == 0 {
				ports = coachAllInputPortRefs(tp)
			}
			coachAddPortSignalBatch(st, "context_channel_drop", ports, r.ContextChannelsDropped)
			coachAddPortSignalBatchMap(scopedPortSignals, "context_channel_drop", ports, r.ContextChannelsDropped)
		}
		if r.ContextDropped > 0 {
			ports := coachAllInputPortRefs(tp)
			coachAddPortSignalBatch(st, "context_drop", ports, r.ContextDropped)
			coachAddPortSignalBatchMap(scopedPortSignals, "context_drop", ports, r.ContextDropped)
		}
		if r.DroppedHandoffs > 0 && plan != nil {
			ports := coachSuggestedTargetPortsForAgents(plan, []string{r.Agent}, "")
			coachAddPortSignalBatch(st, "handoff_drop", ports, r.DroppedHandoffs)
			coachAddPortSignalBatchMap(scopedPortSignals, "handoff_drop", ports, r.DroppedHandoffs)
		}
	}
	_ = saveCoachState(workspace, st)
}

func coachAllInputPortRefs(tp runtime.TaskPlan) []string {
	if len(tp.IO.Inputs) == 0 {
		return nil
	}
	out := make([]string, 0, len(tp.IO.Inputs))
	for _, in := range tp.IO.Inputs {
		if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(tp.Agent) == "" {
			continue
		}
		out = append(out, "`"+tp.Agent+"."+in.Name+"`")
	}
	return out
}

func coachAddPortSignalBatch(st *coachState, kind string, refs []string, weight int) {
	if st == nil || len(refs) == 0 {
		return
	}
	if st.PortSignals == nil {
		st.PortSignals = map[string]int{}
	}
	if weight <= 0 {
		weight = 1
	}
	if weight > 4 {
		weight = 4
	}
	seen := map[string]struct{}{}
	for i, ref := range refs {
		if i >= 4 {
			break
		}
		ref = strings.Trim(ref, "`")
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		key := coachPortSignalKey(kind, ref)
		if key == "" {
			continue
		}
		st.PortSignals[key] += weight
	}
}

func coachAddPortSignalBatchMap(dst map[string]int, kind string, refs []string, weight int) {
	if len(dst) == 0 && dst == nil {
		return
	}
	if len(refs) == 0 {
		return
	}
	if weight <= 0 {
		weight = 1
	}
	if weight > 4 {
		weight = 4
	}
	seen := map[string]struct{}{}
	for i, ref := range refs {
		if i >= 4 {
			break
		}
		ref = strings.Trim(ref, "`")
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		key := coachPortSignalKey(kind, ref)
		if key == "" {
			continue
		}
		dst[key] += weight
	}
}

func coachTopTransitionPostHint(commandPath string, st *coachState, plan *runtime.ExecutionPlan) (coachHint, bool) {
	spec := ""
	if plan != nil {
		spec = plan.Spec
	}
	next, conf, count, total := coachTopNextCommandForSpec(st, commandPath, spec)
	if next == "" || next == commandPath || total < 3 || count < 2 || conf < 70 {
		return coachHint{}, false
	}
	text := fmt.Sprintf("Typical next step for you after `%s`: `%s` (%d%% local pattern).", commandPath, next, conf)
	if commandPath == "run" && next == "trace" && plan != nil && strings.TrimSpace(plan.Spec) != "" {
		text = fmt.Sprintf("You often inspect after `run`: `deeph trace --json %q` (%d%% local pattern).", plan.Spec, conf)
	}
	return coachHint{
		ID:         "postrun.transition." + commandPath + "." + strings.ReplaceAll(next, " ", "_"),
		Kind:       "next",
		Confidence: conf,
		Weight:     7.2 + float64(conf)/100.0,
		Text:       text,
	}, true
}

func coachPassesPostRunCooldown(st *coachState, kind string) bool {
	if st == nil || st.LastShownAt.IsZero() {
		return true
	}
	minGap := 60 * time.Second
	if kind == "efficiency" {
		minGap = 30 * time.Second
	}
	return time.Since(st.LastShownAt) >= minGap
}

func printCoachPostRunLine(hint coachHint) {
	prefix := "[deepH coach]"
	if supportsANSIColor() {
		prefix = "\x1b[36m[deepH coach]\x1b[0m"
	}
	label := strings.ToUpper(hint.Kind)
	conf := ""
	if hint.Confidence > 0 {
		conf = fmt.Sprintf(" %d%%", hint.Confidence)
	}
	_, _ = fmt.Fprintf(os.Stderr, "%s %s%s | %s\n", prefix, label, conf, hint.Text)
}

func loadCoachState(workspace string) (*coachState, error) {
	path := coachStatePath(workspace)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &coachState{
				Version:           1,
				HintSeen:          map[string]int{},
				CommandSeen:       map[string]int{},
				Transitions:       map[string]int{},
				PortSignals:       map[string]int{},
				ScopedTransitions: map[string]map[string]int{},
				ScopedPortSignals: map[string]map[string]int{},
			}, nil
		}
		return nil, err
	}
	var st coachState
	if err := json.Unmarshal(b, &st); err != nil {
		return &coachState{
			Version:           1,
			HintSeen:          map[string]int{},
			CommandSeen:       map[string]int{},
			Transitions:       map[string]int{},
			PortSignals:       map[string]int{},
			ScopedTransitions: map[string]map[string]int{},
			ScopedPortSignals: map[string]map[string]int{},
		}, nil
	}
	if st.Version == 0 {
		st.Version = 1
	}
	if st.HintSeen == nil {
		st.HintSeen = map[string]int{}
	}
	if st.CommandSeen == nil {
		st.CommandSeen = map[string]int{}
	}
	if st.Transitions == nil {
		st.Transitions = map[string]int{}
	}
	if st.PortSignals == nil {
		st.PortSignals = map[string]int{}
	}
	if st.ScopedTransitions == nil {
		st.ScopedTransitions = map[string]map[string]int{}
	}
	if st.ScopedPortSignals == nil {
		st.ScopedPortSignals = map[string]map[string]int{}
	}
	return &st, nil
}

func saveCoachState(workspace string, st *coachState) error {
	if strings.TrimSpace(workspace) == "" || st == nil {
		return nil
	}
	dir := filepath.Dir(coachStatePath(workspace))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(coachStatePath(workspace), b, 0o644)
}

func recordCoachCommandUse(workspace, commandPath string) {
	st, err := loadCoachState(workspace)
	if err != nil || st == nil {
		return
	}
	key := strings.ToLower(strings.TrimSpace(commandPath))
	if key != "" {
		st.CommandSeen[key]++
	}
	_ = saveCoachState(workspace, st)
}

func recordCoachHintShown(workspace, commandPath, hintID string) {
	st, err := loadCoachState(workspace)
	if err != nil || st == nil {
		return
	}
	cmd := strings.ToLower(strings.TrimSpace(commandPath))
	if cmd != "" {
		st.CommandSeen[cmd]++
	}
	id := strings.TrimSpace(hintID)
	if id != "" {
		st.HintSeen[id]++
	}
	st.LastShownAt = time.Now()
	_ = saveCoachState(workspace, st)
}

func recordCoachHintShownOnly(workspace, hintID string) {
	st, err := loadCoachState(workspace)
	if err != nil || st == nil {
		return
	}
	if st.HintSeen == nil {
		st.HintSeen = map[string]int{}
	}
	id := strings.TrimSpace(hintID)
	if id != "" {
		st.HintSeen[id]++
	}
	st.LastShownAt = time.Now()
	_ = saveCoachState(workspace, st)
}

func recordCoachCommandTransition(workspace, commandPath string, specOpt ...string) {
	st, err := loadCoachState(workspace)
	if err != nil || st == nil {
		return
	}
	if st.Transitions == nil {
		st.Transitions = map[string]int{}
	}
	if st.ScopedTransitions == nil {
		st.ScopedTransitions = map[string]map[string]int{}
	}
	cmd := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(commandPath)), " "))
	if cmd == "" {
		return
	}
	spec := ""
	if len(specOpt) > 0 {
		spec = coachSpecKey(specOpt[0])
	}
	now := time.Now()
	prev := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(st.LastCommand)), " "))
	prevSpec := coachSpecKey(st.LastCommandSpec)
	if prev != "" && prev != cmd {
		if st.LastCommandAt.IsZero() || now.Sub(st.LastCommandAt) <= 30*time.Minute {
			key := prev + "->" + cmd
			st.Transitions[key]++
			if spec != "" && spec == prevSpec {
				scope := coachEnsureScopedCounter(st.ScopedTransitions, spec)
				if scope != nil {
					scope[key]++
				}
			}
		}
	}
	st.LastCommand = cmd
	st.LastCommandSpec = spec
	st.LastCommandAt = now
	_ = saveCoachState(workspace, st)
}
