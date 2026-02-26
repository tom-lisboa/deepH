package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"deeph/internal/project"
	"deeph/internal/runtime"
	"deeph/internal/typesys"
)

type multiverseUniverse struct {
	ID              string   `json:"id"`
	Label           string   `json:"label,omitempty"`
	Spec            string   `json:"spec"`
	Input           string   `json:"input,omitempty"`
	Source          string   `json:"source,omitempty"`
	CrewName        string   `json:"crew_name,omitempty"`
	CrewPath        string   `json:"crew_path,omitempty"`
	Index           int      `json:"index"`
	InputNote       string   `json:"input_note,omitempty"`
	DependsOn       []string `json:"depends_on,omitempty"`
	InputPort       string   `json:"input_port,omitempty"`
	OutputPort      string   `json:"output_port,omitempty"`
	OutputKind      string   `json:"output_kind,omitempty"`
	MergePolicy     string   `json:"merge_policy,omitempty"`
	HandoffMaxChars int      `json:"handoff_max_chars,omitempty"`
}

type multiverseTraceBranch struct {
	Universe multiverseUniverse    `json:"universe"`
	Plan     runtime.ExecutionPlan `json:"plan"`
	Error    string                `json:"error,omitempty"`
}

type multiverseRunBranch struct {
	Universe              multiverseUniverse      `json:"universe"`
	Report                runtime.ExecutionReport `json:"report,omitempty"`
	Error                 string                  `json:"error,omitempty"`
	DurationMS            int64                   `json:"duration_ms"`
	IncomingChannels      []string                `json:"incoming_channels,omitempty"`
	IncomingContributions int                     `json:"incoming_contributions,omitempty"`
	InputAugmented        bool                    `json:"input_augmented,omitempty"`
	InputAugmentNote      string                  `json:"input_augment_note,omitempty"`
}

type multiverseJudgeRun struct {
	Spec      string
	Report    runtime.ExecutionReport
	Error     string
	Decision  *multiverseJudgeDecision
	RawOutput string
}

type multiverseJudgeDecision struct {
	Winner      string   `json:"winner,omitempty"`
	Rationale   string   `json:"rationale,omitempty"`
	Differences []string `json:"differences,omitempty"`
	Risks       []string `json:"risks,omitempty"`
	FollowUp    []string `json:"follow_up,omitempty"`
	Format      string   `json:"-"`
}

type multiverseUniverseHandoff struct {
	FromID      string `json:"from_id"`
	FromLabel   string `json:"from_label,omitempty"`
	FromPort    string `json:"from_port"`
	ToID        string `json:"to_id"`
	ToLabel     string `json:"to_label,omitempty"`
	ToPort      string `json:"to_port"`
	Kind        string `json:"kind"`
	Channel     string `json:"channel"`
	MergePolicy string `json:"merge_policy,omitempty"`
	MaxChars    int    `json:"max_chars,omitempty"`

	fromIndex int
	toIndex   int
}

type multiverseOrchestrationPlan struct {
	Scheduler  string                      `json:"scheduler"`
	Handoffs   []multiverseUniverseHandoff `json:"handoffs,omitempty"`
	indegree   []int
	dependents [][]int
	incoming   map[int][]multiverseUniverseHandoff
}

func buildMultiverseUniverses(workspace string, rawArg, resolvedSpec, input string, mvCount int, crew *crewConfig) ([]multiverseUniverse, error) {
	if mvCount == 1 && (crew == nil || len(crew.Universes) == 0) {
		return nil, nil
	}
	if mvCount < 0 {
		return nil, fmt.Errorf("--multiverse must be >= 0")
	}

	if crew != nil && len(crew.Universes) > 0 {
		limit := len(crew.Universes)
		if mvCount > 0 && mvCount < limit {
			limit = mvCount
		}
		if mvCount == 1 {
			limit = 1
		}
		if mvCount == 0 {
			// all universes from crew
		}
		if limit <= 0 {
			return nil, fmt.Errorf("crew %q has no universes", crew.Name)
		}
		out := make([]multiverseUniverse, 0, limit)
		for i := 0; i < limit; i++ {
			u := crew.Universes[i]
			spec := strings.TrimSpace(u.Spec)
			if spec == "" {
				spec = resolvedSpec
			}
			branchInput, note := applyCrewUniverseInput(input, u)
			label := strings.TrimSpace(u.Name)
			if label == "" {
				label = fmt.Sprintf("u%d", i+1)
			}
			out = append(out, multiverseUniverse{
				ID:              fmt.Sprintf("u%d", i+1),
				Label:           label,
				Spec:            spec,
				Input:           branchInput,
				Source:          "crew.universes",
				CrewName:        crew.Name,
				Index:           i,
				InputNote:       note,
				DependsOn:       trimNonEmpty(u.DependsOn),
				InputPort:       strings.TrimSpace(u.InputPort),
				OutputPort:      strings.TrimSpace(u.OutputPort),
				OutputKind:      strings.TrimSpace(u.OutputKind),
				MergePolicy:     strings.TrimSpace(u.MergePolicy),
				HandoffMaxChars: u.HandoffMaxChars,
			})
		}
		if len(out) <= 1 {
			return nil, nil
		}
		return out, nil
	}

	if mvCount == 0 {
		return nil, fmt.Errorf("--multiverse=0 requires a crew with `universes`")
	}
	if mvCount < 2 {
		return nil, nil
	}
	out := make([]multiverseUniverse, 0, mvCount)
	for i := 0; i < mvCount; i++ {
		id := fmt.Sprintf("u%d", i+1)
		out = append(out, multiverseUniverse{
			ID:              id,
			Label:           id,
			Spec:            resolvedSpec,
			Input:           input,
			Source:          "clone",
			CrewName:        "",
			Index:           i,
			InputPort:       "context",
			OutputPort:      "result",
			OutputKind:      string(typesys.KindSummaryText),
			MergePolicy:     "append",
			HandoffMaxChars: 260,
		})
	}
	_ = workspace
	_ = rawArg
	return out, nil
}

func applyCrewUniverseInput(base string, u crewUniverse) (string, string) {
	base = strings.TrimSpace(base)
	p := strings.TrimSpace(u.InputPrefix)
	s := strings.TrimSpace(u.InputSuffix)
	if p == "" && s == "" {
		return base, ""
	}
	lines := make([]string, 0, 3)
	noteParts := make([]string, 0, 2)
	if p != "" {
		lines = append(lines, p)
		noteParts = append(noteParts, "prefix")
	}
	if base != "" {
		lines = append(lines, base)
	}
	if s != "" {
		lines = append(lines, s)
		noteParts = append(noteParts, "suffix")
	}
	note := ""
	if len(noteParts) > 0 {
		note = "crew universe input " + strings.Join(noteParts, "+")
	}
	return strings.Join(lines, "\n"), note
}

func trimNonEmpty(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeNotes(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case strings.Contains(a, b):
		return a
	default:
		return a + "; " + b
	}
}

func normalizeMultiverseUniverse(u multiverseUniverse) multiverseUniverse {
	if strings.TrimSpace(u.InputPort) == "" {
		u.InputPort = "context"
	}
	if strings.TrimSpace(u.OutputPort) == "" {
		u.OutputPort = "result"
	}
	if strings.TrimSpace(u.OutputKind) == "" {
		u.OutputKind = string(typesys.KindSummaryText)
	} else if k, ok := typesys.NormalizeKind(u.OutputKind); ok {
		u.OutputKind = k.String()
	} else {
		u.OutputKind = string(typesys.KindSummaryText)
	}
	if strings.TrimSpace(u.MergePolicy) == "" {
		u.MergePolicy = "append"
	}
	u.MergePolicy = strings.ToLower(strings.TrimSpace(u.MergePolicy))
	switch u.MergePolicy {
	case "append", "latest":
	default:
		u.MergePolicy = "append"
	}
	if u.HandoffMaxChars <= 0 {
		u.HandoffMaxChars = 260
	}
	u.DependsOn = trimNonEmpty(u.DependsOn)
	return u
}

func universeRefKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func universeDisplayName(u multiverseUniverse) string {
	if strings.TrimSpace(u.Label) != "" && strings.TrimSpace(u.Label) != strings.TrimSpace(u.ID) {
		return strings.TrimSpace(u.Label) + " (" + strings.TrimSpace(u.ID) + ")"
	}
	if strings.TrimSpace(u.Label) != "" {
		return strings.TrimSpace(u.Label)
	}
	return strings.TrimSpace(u.ID)
}

func planMultiverseOrchestration(universes []multiverseUniverse) (*multiverseOrchestrationPlan, error) {
	n := len(universes)
	mv := &multiverseOrchestrationPlan{
		Scheduler:  "parallel",
		incoming:   make(map[int][]multiverseUniverseHandoff, n),
		indegree:   make([]int, n),
		dependents: make([][]int, n),
	}
	if n == 0 {
		return mv, nil
	}
	byRef := make(map[string]int, n*2)
	for i := range universes {
		u := normalizeMultiverseUniverse(universes[i])
		universes[i] = u
		if strings.TrimSpace(u.ID) == "" {
			return nil, fmt.Errorf("multiverse universe[%d] missing id", i)
		}
		if _, ok := byRef[universeRefKey(u.ID)]; ok {
			return nil, fmt.Errorf("duplicate multiverse universe id %q", u.ID)
		}
		byRef[universeRefKey(u.ID)] = i
		if lbl := strings.TrimSpace(u.Label); lbl != "" {
			k := universeRefKey(lbl)
			if j, ok := byRef[k]; ok && j != i {
				return nil, fmt.Errorf("duplicate multiverse universe label %q", lbl)
			}
			byRef[k] = i
		}
	}
	seenEdge := map[string]struct{}{}
	for toIdx, u := range universes {
		if len(u.DependsOn) == 0 {
			continue
		}
		mv.Scheduler = "dag_channels"
		for _, ref := range u.DependsOn {
			fromIdx, ok := byRef[universeRefKey(ref)]
			if !ok {
				return nil, fmt.Errorf("universe %q depends_on unknown universe %q", universeDisplayName(u), ref)
			}
			if fromIdx == toIdx {
				return nil, fmt.Errorf("universe %q cannot depend on itself", universeDisplayName(u))
			}
			edgeKey := fmt.Sprintf("%d->%d", fromIdx, toIdx)
			if _, ok := seenEdge[edgeKey]; ok {
				continue
			}
			seenEdge[edgeKey] = struct{}{}
			from := universes[fromIdx]
			kind := strings.TrimSpace(from.OutputKind)
			if kind == "" {
				kind = string(typesys.KindSummaryText)
			}
			h := multiverseUniverseHandoff{
				FromID:      from.ID,
				FromLabel:   from.Label,
				FromPort:    from.OutputPort,
				ToID:        u.ID,
				ToLabel:     u.Label,
				ToPort:      u.InputPort,
				Kind:        kind,
				Channel:     fmt.Sprintf("%s.%s->%s.%s#%s", from.ID, from.OutputPort, u.ID, u.InputPort, kind),
				MergePolicy: u.MergePolicy,
				MaxChars:    u.HandoffMaxChars,
				fromIndex:   fromIdx,
				toIndex:     toIdx,
			}
			mv.Handoffs = append(mv.Handoffs, h)
			mv.incoming[toIdx] = append(mv.incoming[toIdx], h)
			mv.dependents[fromIdx] = append(mv.dependents[fromIdx], toIdx)
			mv.indegree[toIdx]++
		}
	}
	// Cycle check (Kahn).
	ind := append([]int(nil), mv.indegree...)
	q := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if ind[i] == 0 {
			q = append(q, i)
		}
	}
	visited := 0
	for len(q) > 0 {
		i := q[0]
		q = q[1:]
		visited++
		for _, j := range mv.dependents[i] {
			ind[j]--
			if ind[j] == 0 {
				q = append(q, j)
			}
		}
	}
	if visited != n {
		return nil, fmt.Errorf("multiverse universe dependency cycle detected")
	}
	return mv, nil
}

func buildMultiverseUniverseInput(u multiverseUniverse, mvPlan *multiverseOrchestrationPlan, done []bool, branches []multiverseRunBranch) (string, string, []string, int) {
	u = normalizeMultiverseUniverse(u)
	base := strings.TrimSpace(u.Input)
	if mvPlan == nil {
		return base, "", nil, 0
	}
	in := mvPlan.incoming[u.Index]
	if len(in) == 0 {
		return base, "", nil, 0
	}
	selected := make([]multiverseUniverseHandoff, 0, len(in))
	switch u.MergePolicy {
	case "latest":
		for i := len(in) - 1; i >= 0; i-- {
			h := in[i]
			if done == nil || (h.fromIndex >= 0 && h.fromIndex < len(done) && done[h.fromIndex]) {
				selected = append(selected, h)
				break
			}
		}
	default: // append
		for _, h := range in {
			if done == nil || (h.fromIndex >= 0 && h.fromIndex < len(done) && done[h.fromIndex]) {
				selected = append(selected, h)
			}
		}
	}
	if len(selected) == 0 || len(branches) == 0 {
		return base, "", nil, 0
	}
	lines := []string{
		"[multiverse_handoffs]",
		"kind: context/compiled",
		fmt.Sprintf("target: %s", universeDisplayName(u)),
	}
	channels := make([]string, 0, len(selected))
	contribs := 0
	for _, h := range selected {
		if h.fromIndex < 0 || h.fromIndex >= len(branches) {
			continue
		}
		br := branches[h.fromIndex]
		if strings.TrimSpace(br.Universe.ID) == "" {
			continue
		}
		channels = append(channels, h.Channel)
		contribs++
		lines = append(lines, "- channel: "+h.Channel)
		lines = append(lines, "  kind: "+h.Kind)
		lines = append(lines, "  from: "+quoteYAMLInline(universeDisplayName(br.Universe)))
		if br.Error != "" {
			lines = append(lines, "  status: error")
			lines = append(lines, "  error: "+quoteYAMLInline(br.Error))
			continue
		}
		lines = append(lines, "  status: ok")
		sinks := multiverseSinkReplies(br.Report)
		fp := multiverseOutputHash(sinks)
		if len(fp) > 12 {
			fp = fp[:12]
		}
		lines = append(lines, "  sink_fingerprint: "+fp)
		lines = append(lines, "  sink_outputs:")
		if len(sinks) == 0 {
			lines = append(lines, "    - agent: none")
			lines = append(lines, "      text: \"\"")
			continue
		}
		for _, s := range sinks {
			lines = append(lines, "    - agent: "+s.Agent)
			if s.Error != "" {
				lines = append(lines, "      error: "+quoteYAMLInline(s.Error))
				continue
			}
			text := multiverseNormalizeJudgeSinkText(strings.TrimSpace(s.Text), h.MaxChars)
			lines = append(lines, "      text: |")
			for _, ln := range strings.Split(text, "\n") {
				lines = append(lines, "        "+ln)
			}
		}
	}
	if contribs == 0 {
		return base, "", nil, 0
	}
	compiled := strings.Join(lines, "\n")
	combined := compiled
	if base != "" {
		combined = base + "\n\n" + compiled
	}
	note := fmt.Sprintf("multiverse channels=%d merge=%s", len(channels), u.MergePolicy)
	return combined, note, channels, contribs
}

func traceMultiverse(ctx context.Context, workspace string, p *project.Project, universes []multiverseUniverse) ([]multiverseTraceBranch, *multiverseOrchestrationPlan, error) {
	mvPlan, err := planMultiverseOrchestration(universes)
	if err != nil {
		return nil, nil, err
	}
	eng, err := runtime.New(workspace, p)
	if err != nil {
		return nil, nil, err
	}
	out := make([]multiverseTraceBranch, 0, len(universes))
	for _, u := range universes {
		input, note, _, _ := buildMultiverseUniverseInput(u, mvPlan, nil, nil)
		plan, _, err := eng.PlanSpec(ctx, u.Spec, input)
		b := multiverseTraceBranch{Universe: u}
		if note != "" {
			b.Universe.InputNote = mergeNotes(b.Universe.InputNote, note)
		}
		if err != nil {
			b.Error = err.Error()
		} else {
			b.Plan = plan
		}
		out = append(out, b)
	}
	return out, mvPlan, nil
}

func runMultiverse(ctx context.Context, workspace string, p *project.Project, universes []multiverseUniverse) ([]multiverseRunBranch, *multiverseOrchestrationPlan, error) {
	mvPlan, err := planMultiverseOrchestration(universes)
	if err != nil {
		return nil, nil, err
	}
	out := make([]multiverseRunBranch, len(universes))
	if len(universes) == 0 {
		return out, mvPlan, nil
	}
	type mvDone struct {
		idx    int
		branch multiverseRunBranch
	}
	doneCh := make(chan mvDone, len(universes))
	started := make([]bool, len(universes))
	done := make([]bool, len(universes))
	remaining := append([]int(nil), mvPlan.indegree...)
	var mu sync.Mutex

	startUniverse := func(i int) {
		if started[i] {
			return
		}
		started[i] = true
		u := universes[i]
		// Snapshot current completed branches for deterministic input compilation.
		mu.Lock()
		completed := make([]multiverseRunBranch, len(out))
		copy(completed, out)
		completedDone := make([]bool, len(done))
		copy(completedDone, done)
		mu.Unlock()

		go func(i int, u multiverseUniverse, snapshot []multiverseRunBranch, snapshotDone []bool) {
			start := time.Now()
			br := multiverseRunBranch{Universe: u}
			input, note, chans, contribs := buildMultiverseUniverseInput(u, mvPlan, snapshotDone, snapshot)
			if note != "" {
				br.Universe.InputNote = mergeNotes(br.Universe.InputNote, note)
				br.InputAugmentNote = note
			}
			br.IncomingChannels = chans
			br.IncomingContributions = contribs
			br.InputAugmented = len(chans) > 0

			eng, err := runtime.New(workspace, p)
			if err != nil {
				br.Error = err.Error()
				br.DurationMS = time.Since(start).Milliseconds()
				doneCh <- mvDone{idx: i, branch: br}
				return
			}
			report, err := eng.RunSpec(ctx, u.Spec, input)
			br.DurationMS = time.Since(start).Milliseconds()
			if err != nil {
				br.Error = err.Error()
			} else {
				br.Report = report
			}
			doneCh <- mvDone{idx: i, branch: br}
		}(i, u, completed, completedDone)
	}

	for i := range universes {
		if remaining[i] == 0 {
			startUniverse(i)
		}
	}
	completedCount := 0
	for completedCount < len(universes) {
		select {
		case <-ctx.Done():
			return nil, mvPlan, ctx.Err()
		case res := <-doneCh:
			mu.Lock()
			out[res.idx] = res.branch
			if !done[res.idx] {
				done[res.idx] = true
				completedCount++
			}
			next := append([]int(nil), mvPlan.dependents[res.idx]...)
			ready := make([]int, 0, len(next))
			for _, j := range next {
				if remaining[j] > 0 {
					remaining[j]--
				}
				if remaining[j] == 0 && !started[j] {
					ready = append(ready, j)
				}
			}
			mu.Unlock()
			for _, j := range ready {
				startUniverse(j)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Universe.Index < out[j].Universe.Index })
	return out, mvPlan, nil
}

func runMultiverseJudge(ctx context.Context, workspace string, p *project.Project, judgeSpec string, sourceSpec string, sourceInput string, branches []multiverseRunBranch, maxCharsPerBranch int) multiverseJudgeRun {
	out := multiverseJudgeRun{Spec: strings.TrimSpace(judgeSpec)}
	if out.Spec == "" {
		out.Error = "judge agent spec is empty"
		return out
	}
	eng, err := runtime.New(workspace, p)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	prompt := buildMultiverseJudgePrompt(sourceSpec, sourceInput, branches, maxCharsPerBranch)
	report, err := eng.RunSpec(ctx, out.Spec, prompt)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Report = report
	out.RawOutput = multiverseJudgeLastOutput(report)
	if d, ok := parseMultiverseJudgeDecision(out.RawOutput); ok {
		out.Decision = d
	}
	return out
}

func printMultiverseTraceText(workspace string, rawArg string, mvPlan *multiverseOrchestrationPlan, branches []multiverseTraceBranch) {
	fmt.Printf("Multiverse Trace (%s)\n", workspace)
	fmt.Printf("  branches: %d\n", len(branches))
	scheduler := "parallel"
	if mvPlan != nil && strings.TrimSpace(mvPlan.Scheduler) != "" {
		scheduler = mvPlan.Scheduler
	}
	fmt.Printf("  scheduler: %s\n", scheduler)
	if strings.TrimSpace(rawArg) != "" {
		fmt.Printf("  source: %q\n", rawArg)
	}
	if mvPlan != nil && len(mvPlan.Handoffs) > 0 {
		fmt.Println("  universe_handoffs:")
		for _, h := range mvPlan.Handoffs {
			merge := ""
			if h.MergePolicy != "" && h.MergePolicy != "append" {
				merge = " merge=" + h.MergePolicy
			}
			maxChars := ""
			if h.MaxChars > 0 {
				maxChars = fmt.Sprintf(" max_chars=%d", h.MaxChars)
			}
			fmt.Printf("    - %s.%s -> %s.%s kind=%s channel=%s%s%s\n", h.FromID, h.FromPort, h.ToID, h.ToPort, h.Kind, h.Channel, merge, maxChars)
		}
	}
	for _, b := range branches {
		label := b.Universe.ID
		if strings.TrimSpace(b.Universe.Label) != "" && b.Universe.Label != b.Universe.ID {
			label += "/" + b.Universe.Label
		}
		fmt.Printf("\n[%s] spec=%q", label, b.Universe.Spec)
		if b.Universe.CrewName != "" {
			fmt.Printf(" crew=%s", b.Universe.CrewName)
		}
		if b.Universe.Source != "" {
			fmt.Printf(" source=%s", b.Universe.Source)
		}
		fmt.Println()
		if b.Universe.InputNote != "" {
			fmt.Printf("  input_note: %s\n", b.Universe.InputNote)
		}
		if strings.TrimSpace(b.Universe.OutputKind) != "" {
			fmt.Printf("  output_kind=%s\n", b.Universe.OutputKind)
		}
		if len(b.Universe.DependsOn) > 0 {
			fmt.Printf("  depends_on=%v ports=%s<-%s merge=%s\n", b.Universe.DependsOn, b.Universe.InputPort, b.Universe.OutputPort, b.Universe.MergePolicy)
		}
		if b.Error != "" {
			fmt.Printf("  error: %s\n", b.Error)
			continue
		}
		fmt.Printf("  tasks=%d stages=%d handoffs=%d parallel=%v\n", len(b.Plan.Tasks), len(b.Plan.Stages), len(b.Plan.Handoffs), b.Plan.Parallel)
	}
}

func printMultiverseRunText(workspace string, rawArg string, mvPlan *multiverseOrchestrationPlan, branches []multiverseRunBranch) {
	fmt.Printf("Multiverse Run (%s)\n", workspace)
	fmt.Printf("  branches: %d\n", len(branches))
	if mvPlan != nil && strings.TrimSpace(mvPlan.Scheduler) != "" {
		fmt.Printf("  scheduler: %s\n", mvPlan.Scheduler)
	}
	if strings.TrimSpace(rawArg) != "" {
		fmt.Printf("  source: %q\n", rawArg)
	}
	if mvPlan != nil && len(mvPlan.Handoffs) > 0 {
		fmt.Printf("  universe_handoffs: %d\n", len(mvPlan.Handoffs))
	}
	type outputSig struct {
		Hash  string
		Count int
		IDs   []string
	}
	sigMap := map[string]*outputSig{}
	for _, b := range branches {
		label := b.Universe.ID
		if strings.TrimSpace(b.Universe.Label) != "" && b.Universe.Label != b.Universe.ID {
			label += "/" + b.Universe.Label
		}
		fmt.Printf("\n[%s] spec=%q duration=%dms\n", label, b.Universe.Spec, b.DurationMS)
		if b.Universe.InputNote != "" {
			fmt.Printf("  input_note: %s\n", b.Universe.InputNote)
		}
		if strings.TrimSpace(b.Universe.OutputKind) != "" {
			fmt.Printf("  output_kind=%s\n", b.Universe.OutputKind)
		}
		if len(b.Universe.DependsOn) > 0 {
			fmt.Printf("  depends_on=%v\n", b.Universe.DependsOn)
		}
		if b.InputAugmented {
			fmt.Printf("  universe_channels=%d contributions=%d\n", len(b.IncomingChannels), b.IncomingContributions)
			for _, ch := range b.IncomingChannels {
				fmt.Printf("    - %s\n", ch)
			}
		}
		if b.Error != "" {
			fmt.Printf("  error: %s\n", b.Error)
			continue
		}
		fmt.Printf("  report: parallel=%v agents=%d total=%s\n", b.Report.Parallel, len(b.Report.Results), b.Report.EndedAt.Sub(b.Report.StartedAt).Round(time.Millisecond))
		sinks := multiverseSinkReplies(b.Report)
		if len(sinks) == 0 {
			fmt.Println("  output: (none)")
			continue
		}
		for _, s := range sinks {
			head := fmt.Sprintf("  [%s]", s.Agent)
			if s.Error != "" {
				fmt.Printf("%s error: %s\n", head, s.Error)
				continue
			}
			txt := strings.TrimSpace(s.Text)
			fmt.Printf("%s %s\n", head, clipLine(txt, 320))
		}
		hash := multiverseOutputHash(sinks)
		sig := sigMap[hash]
		if sig == nil {
			sig = &outputSig{Hash: hash}
			sigMap[hash] = sig
		}
		sig.Count++
		sig.IDs = append(sig.IDs, label)
	}
	if len(sigMap) > 0 {
		sigs := make([]*outputSig, 0, len(sigMap))
		for _, s := range sigMap {
			sigs = append(sigs, s)
		}
		sort.Slice(sigs, func(i, j int) bool {
			if sigs[i].Count == sigs[j].Count {
				return sigs[i].Hash < sigs[j].Hash
			}
			return sigs[i].Count > sigs[j].Count
		})
		fmt.Println("\nConsensus (sink output fingerprint):")
		for _, s := range sigs {
			short := s.Hash
			if len(short) > 12 {
				short = short[:12]
			}
			fmt.Printf("- %s count=%d branches=%v\n", short, s.Count, s.IDs)
		}
	}
}

func printMultiverseJudgeText(j multiverseJudgeRun) {
	if strings.TrimSpace(j.Spec) == "" {
		return
	}
	fmt.Printf("\nJudge (%s)\n", j.Spec)
	if j.Error != "" {
		fmt.Printf("  error: %s\n", j.Error)
		return
	}
	if len(j.Report.Results) == 0 {
		fmt.Println("  output: (none)")
		return
	}
	last := j.Report.Results[len(j.Report.Results)-1]
	if last.Error != "" {
		fmt.Printf("  error: %s\n", last.Error)
		return
	}
	fmt.Printf("  duration: %s\n", j.Report.EndedAt.Sub(j.Report.StartedAt).Round(time.Millisecond))
	if j.Decision != nil && j.Decision.meaningful() {
		fmt.Printf("  structured: yes (%s)\n", j.Decision.Format)
		if j.Decision.Winner != "" {
			fmt.Printf("  winner: %s\n", j.Decision.Winner)
		}
		if j.Decision.Rationale != "" {
			fmt.Printf("  rationale: %s\n", clipLine(j.Decision.Rationale, 700))
		}
		if len(j.Decision.Differences) > 0 {
			fmt.Println("  differences:")
			for _, it := range j.Decision.Differences {
				fmt.Printf("    - %s\n", clipLine(it, 240))
			}
		}
		if len(j.Decision.Risks) > 0 {
			fmt.Println("  risks:")
			for _, it := range j.Decision.Risks {
				fmt.Printf("    - %s\n", clipLine(it, 240))
			}
		}
		if len(j.Decision.FollowUp) > 0 {
			fmt.Println("  follow_up:")
			for _, it := range j.Decision.FollowUp {
				fmt.Printf("    - %s\n", clipLine(it, 240))
			}
		}
		return
	}
	raw := strings.TrimSpace(j.RawOutput)
	if raw == "" {
		raw = multiverseJudgeLastOutput(j.Report)
	}
	if raw == "" {
		fmt.Println("  output: (none)")
		return
	}
	fmt.Printf("  output:\n%s\n", multiverseFormatJudgeOutput(raw))
}

func buildMultiverseJudgePrompt(sourceSpec string, sourceInput string, branches []multiverseRunBranch, maxCharsPerBranch int) string {
	if maxCharsPerBranch <= 0 {
		maxCharsPerBranch = 600
	}
	lines := make([]string, 0, len(branches)*8+16)
	lines = append(lines, "[multiverse_compare_request]")
	lines = append(lines, "type: plan/summary")
	if strings.TrimSpace(sourceSpec) != "" {
		lines = append(lines, "source_spec: "+sourceSpec)
	}
	if strings.TrimSpace(sourceInput) != "" {
		lines = append(lines, "goal:")
		lines = append(lines, clipLine(sourceInput, 600))
	}
	lines = append(lines, "instruction:")
	lines = append(lines, "Compare multiverse branch outputs and choose the best branch for the goal.")
	lines = append(lines, "Be explicit about tradeoffs, risks, and why other branches were not selected.")
	lines = append(lines, "Return JSON when possible (preferred), otherwise use sections with exact labels.")
	lines = append(lines, `Preferred JSON shape: {"winner":"u1","rationale":"...","differences":["..."],"risks":["..."],"follow_up":["..."]}`)
	lines = append(lines, "If uncertain, set winner to a branch id or label and explain uncertainty in rationale.")
	lines = append(lines, "branches:")

	for _, b := range branches {
		label := b.Universe.ID
		if strings.TrimSpace(b.Universe.Label) != "" {
			label = b.Universe.Label + " (" + b.Universe.ID + ")"
		}
		lines = append(lines, "- id: "+b.Universe.ID)
		lines = append(lines, "  label: "+label)
		lines = append(lines, "  spec: "+quoteYAMLInline(b.Universe.Spec))
		if b.Universe.InputNote != "" {
			lines = append(lines, "  input_note: "+quoteYAMLInline(b.Universe.InputNote))
		}
		lines = append(lines, fmt.Sprintf("  duration_ms: %d", b.DurationMS))
		if b.Error != "" {
			lines = append(lines, "  status: error")
			lines = append(lines, "  error: "+quoteYAMLInline(b.Error))
			continue
		}
		lines = append(lines, "  status: ok")
		sinks := multiverseSinkReplies(b.Report)
		lines = append(lines, "  sink_outputs:")
		if len(sinks) == 0 {
			lines = append(lines, "    - agent: none")
			lines = append(lines, "      text: \"\"")
			continue
		}
		for _, s := range sinks {
			lines = append(lines, "    - agent: "+s.Agent)
			if s.Error != "" {
				lines = append(lines, "      error: "+quoteYAMLInline(s.Error))
				continue
			}
			text := multiverseNormalizeJudgeSinkText(strings.TrimSpace(s.Text), maxCharsPerBranch)
			lines = append(lines, "      text: |")
			for _, ln := range strings.Split(text, "\n") {
				lines = append(lines, "        "+ln)
			}
		}
		fingerprint := multiverseOutputHash(sinks)
		if len(fingerprint) > 12 {
			fingerprint = fingerprint[:12]
		}
		lines = append(lines, "  sink_fingerprint: "+fingerprint)
	}
	lines = append(lines, "return_type_hint: summary/text")
	return strings.Join(lines, "\n")
}

func quoteYAMLInline(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func multiverseNormalizeJudgeSinkText(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[mock-provider]") {
		// Local mock returns an expanded debug line; judge only needs a compact proxy.
		if maxChars > 240 {
			maxChars = 240
		}
	}
	return clipLine(s, maxChars)
}

func multiverseFormatJudgeOutput(s string) string {
	if strings.HasPrefix(strings.TrimSpace(s), "[mock-provider]") {
		return clipLine(s, 700)
	}
	return s
}

func multiverseJudgeLastOutput(report runtime.ExecutionReport) string {
	if len(report.Results) == 0 {
		return ""
	}
	last := report.Results[len(report.Results)-1]
	if last.Error != "" {
		return ""
	}
	return strings.TrimSpace(last.Output)
}

func (d *multiverseJudgeDecision) meaningful() bool {
	if d == nil {
		return false
	}
	return strings.TrimSpace(d.Winner) != "" ||
		strings.TrimSpace(d.Rationale) != "" ||
		len(d.Differences) > 0 ||
		len(d.Risks) > 0 ||
		len(d.FollowUp) > 0
}

func parseMultiverseJudgeDecision(raw string) (*multiverseJudgeDecision, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	if d, ok := parseMultiverseJudgeDecisionJSON(raw); ok && d.meaningful() {
		return d, true
	}
	if d, ok := parseMultiverseJudgeDecisionSections(raw); ok && d.meaningful() {
		return d, true
	}
	return nil, false
}

func parseMultiverseJudgeDecisionJSON(raw string) (*multiverseJudgeDecision, bool) {
	candidates := []string{}
	if s, ok := stripFence(raw); ok {
		candidates = append(candidates, s)
	}
	candidates = append(candidates, raw)
	candidates = append(candidates, extractJSONObjects(raw)...)
	seen := map[string]struct{}{}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		d, ok := parseJudgeJSONCandidate(c)
		if ok {
			d.Format = "json"
			return d, true
		}
	}
	return nil, false
}

func parseJudgeJSONCandidate(s string) (*multiverseJudgeDecision, bool) {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, false
	}
	d := &multiverseJudgeDecision{}
	d.Winner = firstString(m,
		"winner", "winner_id", "selected_branch", "selected", "best_branch", "branch",
	)
	d.Rationale = firstString(m, "rationale", "reason", "why")
	d.Differences = firstStringList(m, "differences", "diffs", "tradeoffs")
	d.Risks = firstStringList(m, "risks", "risk")
	d.FollowUp = firstStringList(m, "follow_up", "followup", "next_steps", "followups")
	return d, d.meaningful()
}

func parseMultiverseJudgeDecisionSections(raw string) (*multiverseJudgeDecision, bool) {
	raw = normalizeNewlines(raw)
	lines := strings.Split(raw, "\n")
	d := &multiverseJudgeDecision{Format: "sections"}
	section := ""
	appendSection := func(name, val string) {
		val = cleanJudgeText(val)
		if val == "" {
			return
		}
		switch name {
		case "winner":
			if d.Winner == "" {
				d.Winner = val
			}
		case "rationale":
			if d.Rationale == "" {
				d.Rationale = val
			} else if !strings.EqualFold(d.Rationale, val) {
				d.Rationale = strings.TrimSpace(d.Rationale + " " + val)
			}
		case "differences":
			d.Differences = appendUniqueStrings(d.Differences, splitJudgeListValue(val)...)
		case "risks":
			d.Risks = appendUniqueStrings(d.Risks, splitJudgeListValue(val)...)
		case "follow_up":
			d.FollowUp = appendUniqueStrings(d.FollowUp, splitJudgeListValue(val)...)
		}
	}

	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "```") {
			continue
		}
		if sec, ok := parseJudgeHeading(t); ok {
			section = sec
			continue
		}
		if k, v, ok := parseJudgeKeyValue(t); ok {
			section = k
			appendSection(k, v)
			continue
		}
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") || isNumberedBullet(t) {
			if section != "" {
				appendSection(section, stripBulletPrefix(t))
			}
			continue
		}
		if section != "" {
			appendSection(section, t)
		}
	}
	if !d.meaningful() {
		return nil, false
	}
	return d, true
}

func parseJudgeHeading(line string) (string, bool) {
	line = strings.TrimSpace(strings.TrimLeft(line, "#"))
	line = strings.TrimSpace(strings.TrimRight(line, ":"))
	if line == "" {
		return "", false
	}
	switch normalizeJudgeKey(line) {
	case "winner":
		return "winner", true
	case "rationale":
		return "rationale", true
	case "differences":
		return "differences", true
	case "risks":
		return "risks", true
	case "followup":
		return "follow_up", true
	}
	return "", false
}

func parseJudgeKeyValue(line string) (string, string, bool) {
	i := strings.Index(line, ":")
	if i <= 0 {
		return "", "", false
	}
	key := normalizeJudgeKey(line[:i])
	val := strings.TrimSpace(line[i+1:])
	switch key {
	case "winner":
		return "winner", val, true
	case "rationale":
		return "rationale", val, true
	case "differences":
		return "differences", val, true
	case "risks":
		return "risks", val, true
	case "followup":
		return "follow_up", val, true
	default:
		return "", "", false
	}
}

func normalizeJudgeKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	repl := strings.NewReplacer("_", "", "-", "", " ", "")
	return repl.Replace(s)
}

func cleanJudgeText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	return strings.TrimSpace(s)
}

func splitJudgeListValue(v string) []string {
	v = cleanJudgeText(v)
	if v == "" {
		return nil
	}
	if strings.Contains(v, ";") {
		parts := strings.Split(v, ";")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := cleanJudgeText(p); t != "" {
				out = append(out, t)
			}
		}
		if len(out) > 1 {
			return out
		}
	}
	return []string{v}
}

func appendUniqueStrings(dst []string, src ...string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, s := range dst {
		seen[s] = struct{}{}
	}
	for _, s := range src {
		s = cleanJudgeText(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		dst = append(dst, s)
	}
	return dst
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case string:
				if s := cleanJudgeText(x); s != "" {
					return s
				}
			case float64:
				return strconv.FormatFloat(x, 'f', -1, 64)
			case bool:
				return strconv.FormatBool(x)
			}
		}
	}
	return ""
}

func firstStringList(m map[string]any, keys ...string) []string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case []any:
			out := make([]string, 0, len(x))
			for _, it := range x {
				switch y := it.(type) {
				case string:
					if s := cleanJudgeText(y); s != "" {
						out = append(out, s)
					}
				case float64:
					out = append(out, strconv.FormatFloat(y, 'f', -1, 64))
				case bool:
					out = append(out, strconv.FormatBool(y))
				}
			}
			if len(out) > 0 {
				return out
			}
		case string:
			if out := splitJudgeListValue(x); len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func stripFence(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "```") {
		return "", false
	}
	lines := strings.Split(normalizeNewlines(raw), "\n")
	if len(lines) < 3 {
		return "", false
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		return "", false
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(last, "```") {
		return "", false
	}
	return strings.Join(lines[1:len(lines)-1], "\n"), true
}

func extractJSONObjects(raw string) []string {
	raw = normalizeNewlines(raw)
	out := []string{}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '{' {
			continue
		}
		if s, end, ok := scanJSONObject(raw, i); ok {
			out = append(out, s)
			i = end - 1
		}
	}
	return out
}

func scanJSONObject(s string, start int) (string, int, bool) {
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], i + 1, true
			}
			if depth < 0 {
				return "", 0, false
			}
		}
	}
	return "", 0, false
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func isNumberedBullet(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i+1 >= len(s) {
		return false
	}
	return s[i] == '.' && s[i+1] == ' '
}

func stripBulletPrefix(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "- "):
		return strings.TrimSpace(s[2:])
	case strings.HasPrefix(s, "* "):
		return strings.TrimSpace(s[2:])
	case isNumberedBullet(s):
		i := strings.Index(s, ". ")
		if i >= 0 && i+2 <= len(s) {
			return strings.TrimSpace(s[i+2:])
		}
	}
	return s
}

type multiverseSinkReply struct {
	Agent string
	Text  string
	Error string
}

func multiverseSinkReplies(report runtime.ExecutionReport) []multiverseSinkReply {
	if len(report.Results) == 0 {
		return nil
	}
	// Sink heuristic: stages with the max stage index are likely outputs of interest.
	maxStage := report.Results[0].StageIndex
	for _, r := range report.Results {
		if r.StageIndex > maxStage {
			maxStage = r.StageIndex
		}
	}
	out := make([]multiverseSinkReply, 0, 2)
	for _, r := range report.Results {
		if r.StageIndex != maxStage {
			continue
		}
		out = append(out, multiverseSinkReply{Agent: r.Agent, Text: r.Output, Error: r.Error})
	}
	if len(out) == 0 {
		last := report.Results[len(report.Results)-1]
		out = append(out, multiverseSinkReply{Agent: last.Agent, Text: last.Output, Error: last.Error})
	}
	return out
}

func multiverseOutputHash(sinks []multiverseSinkReply) string {
	h := sha1.New()
	enc := json.NewEncoder(h)
	_ = enc.Encode(sinks)
	return hex.EncodeToString(h.Sum(nil))
}
