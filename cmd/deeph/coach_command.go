package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func cmdCoach(args []string) error {
	if len(args) == 0 {
		return errors.New("coach requires a subcommand: stats or reset")
	}
	switch args[0] {
	case "stats":
		return cmdCoachStats(args[1:])
	case "reset":
		return cmdCoachReset(args[1:])
	default:
		return fmt.Errorf("unknown coach subcommand %q", args[0])
	}
}

type coachCountKV struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

func cmdCoachStats(args []string) error {
	fs := flag.NewFlagSet("coach stats", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	top := fs.Int("top", 10, "max rows per section")
	scope := fs.String("scope", "", "agent spec scope (workflow-specific coach signals)")
	kind := fs.String("kind", "", "port signal kind filter: handoff_drop|context_channel_drop|context_drop")
	jsonOut := fs.Bool("json", false, "print stats as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("coach stats does not accept positional arguments")
	}
	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "coach stats")
	st, err := loadCoachState(abs)
	if err != nil {
		return err
	}
	if st == nil {
		return errors.New("coach state unavailable")
	}
	if *top <= 0 {
		*top = 10
	}

	scopeKey := coachSpecKey(*scope)
	cmds := coachSortedCounts(st.CommandSeen, *top)
	hints := coachSortedCounts(st.HintSeen, *top)
	transitionsMap := st.Transitions
	if scopeKey != "" {
		if scoped := coachScopedTransitionsMap(st, scopeKey); scoped != nil {
			transitionsMap = scoped
		} else {
			transitionsMap = map[string]int{}
		}
	}
	portSignalsMap := st.PortSignals
	if scopeKey != "" {
		if scoped := coachScopedPortSignalsMap(st, scopeKey); scoped != nil {
			portSignalsMap = scoped
		} else {
			portSignalsMap = map[string]int{}
		}
	}
	trans := coachSortedCounts(transitionsMap, *top)
	portSignalsKind := strings.TrimSpace(*kind)
	portSignals := coachPortSignalsStatsRows(portSignalsMap, portSignalsKind, *top)
	type coachStatsPayload struct {
		Workspace       string         `json:"workspace"`
		Path            string         `json:"path"`
		Version         int            `json:"version"`
		LastShownAt     *time.Time     `json:"last_shown_at,omitempty"`
		LastCommand     string         `json:"last_command,omitempty"`
		LastCommandSpec string         `json:"last_command_spec,omitempty"`
		LastCommandAt   *time.Time     `json:"last_command_at,omitempty"`
		Scope           string         `json:"scope,omitempty"`
		PortSignalsKind string         `json:"port_signals_kind,omitempty"`
		Commands        []coachCountKV `json:"commands"`
		Hints           []coachCountKV `json:"hints"`
		Transitions     []coachCountKV `json:"transitions"`
		PortSignals     []coachCountKV `json:"port_signals,omitempty"`
		TopTransitions  []coachCountKV `json:"top_transitions_by_source,omitempty"`
	}
	payload := coachStatsPayload{
		Workspace:       abs,
		Path:            coachStatePath(abs),
		Version:         st.Version,
		LastCommand:     st.LastCommand,
		LastCommandSpec: st.LastCommandSpec,
		Scope:           scopeKey,
		PortSignalsKind: portSignalsKind,
		Commands:        cmds,
		Hints:           hints,
		Transitions:     trans,
		PortSignals:     portSignals,
	}
	if !st.LastShownAt.IsZero() {
		t := st.LastShownAt
		payload.LastShownAt = &t
	}
	if !st.LastCommandAt.IsZero() {
		t := st.LastCommandAt
		payload.LastCommandAt = &t
	}
	if scopeKey != "" {
		tmp := *st
		tmp.Transitions = transitionsMap
		payload.TopTransitions = coachTopTransitionsBySource(&tmp, *top)
	} else {
		payload.TopTransitions = coachTopTransitionsBySource(st, *top)
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	fmt.Printf("coach state: %s\n", payload.Path)
	fmt.Printf("workspace: %s\n", payload.Workspace)
	fmt.Printf("version: %d\n", payload.Version)
	if payload.Scope != "" {
		fmt.Printf("scope: %s\n", payload.Scope)
	}
	if payload.LastCommand != "" {
		fmt.Printf("last_command: %s", payload.LastCommand)
		if payload.LastCommandSpec != "" {
			fmt.Printf(" [spec=%s]", payload.LastCommandSpec)
		}
		if payload.LastCommandAt != nil {
			fmt.Printf(" (%s)", payload.LastCommandAt.Format(time.RFC3339))
		}
		fmt.Println()
	}
	if payload.LastShownAt != nil {
		fmt.Printf("last_hint_shown_at: %s\n", payload.LastShownAt.Format(time.RFC3339))
	}
	fmt.Println("commands:")
	coachPrintCountList(cmds)
	fmt.Println("hints:")
	coachPrintCountList(hints)
	fmt.Println("transitions:")
	coachPrintCountList(trans)
	fmt.Println("port_signals:")
	if payload.PortSignalsKind != "" {
		fmt.Printf("  kind=%s\n", payload.PortSignalsKind)
	}
	coachPrintCountList(portSignals)
	if len(payload.TopTransitions) > 0 {
		fmt.Println("top next-step patterns:")
		for _, kv := range payload.TopTransitions {
			fmt.Printf("- %s (%d)\n", kv.Key, kv.Count)
		}
	}
	return nil
}

func cmdCoachReset(args []string) error {
	fs := flag.NewFlagSet("coach reset", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	yes := fs.Bool("yes", false, "confirm reset")
	resetAll := fs.Bool("all", false, "reset all coach state (explicit alias)")
	resetHints := fs.Bool("hints", false, "reset hint counters only")
	resetTransitions := fs.Bool("transitions", false, "reset command transition learning only")
	resetCommands := fs.Bool("commands", false, "reset command usage counters only")
	resetPorts := fs.Bool("ports", false, "reset port signal counters only")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("coach reset does not accept positional arguments")
	}
	if !*yes {
		return errors.New("coach reset requires --yes")
	}
	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	if *resetAll && (*resetHints || *resetTransitions || *resetCommands || *resetPorts) {
		return errors.New("coach reset: use --all alone, or use partial flags without --all")
	}
	selected := *resetAll || *resetHints || *resetTransitions || *resetCommands || *resetPorts
	path := coachStatePath(abs)
	if !selected || *resetAll {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Printf("Coach state reset: %s\n", path)
		return nil
	}
	st, err := loadCoachState(abs)
	if err != nil {
		return err
	}
	if st == nil {
		st = &coachState{Version: 1}
	}
	if *resetHints {
		st.HintSeen = map[string]int{}
	}
	if *resetTransitions {
		st.Transitions = map[string]int{}
		st.ScopedTransitions = map[string]map[string]int{}
		st.LastCommand = ""
		st.LastCommandSpec = ""
		st.LastCommandAt = time.Time{}
	}
	if *resetCommands {
		st.CommandSeen = map[string]int{}
	}
	if *resetPorts {
		st.PortSignals = map[string]int{}
	}
	if err := saveCoachState(abs, st); err != nil {
		return err
	}
	parts := make([]string, 0, 4)
	if *resetHints {
		parts = append(parts, "hints")
	}
	if *resetTransitions {
		parts = append(parts, "transitions")
	}
	if *resetCommands {
		parts = append(parts, "commands")
	}
	if *resetPorts {
		parts = append(parts, "ports")
	}
	fmt.Printf("Coach state reset (%s): %s\n", strings.Join(parts, ", "), path)
	return nil
}

func coachSortedCounts(m map[string]int, limit int) []coachCountKV {
	if len(m) == 0 {
		return nil
	}
	out := make([]coachCountKV, 0, len(m))
	for k, v := range m {
		if v <= 0 || strings.TrimSpace(k) == "" {
			continue
		}
		out = append(out, coachCountKV{Key: k, Count: v})
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

func coachPortSignalsStatsRows(m map[string]int, kind string, limit int) []coachCountKV {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return coachSortedCounts(m, limit)
	}
	prefix := kind + ":"
	filtered := make([]coachCountKV, 0, len(m))
	for k, v := range m {
		if v <= 0 || !strings.HasPrefix(k, prefix) {
			continue
		}
		key := strings.TrimPrefix(k, prefix)
		if strings.TrimSpace(key) == "" {
			key = k
		}
		filtered = append(filtered, coachCountKV{Key: key, Count: v})
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Count == filtered[j].Count {
			return filtered[i].Key < filtered[j].Key
		}
		return filtered[i].Count > filtered[j].Count
	})
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func coachPrintCountList(items []coachCountKV) {
	if len(items) == 0 {
		fmt.Println("- (none)")
		return
	}
	for _, kv := range items {
		fmt.Printf("- %s (%d)\n", kv.Key, kv.Count)
	}
}

func coachTopTransitionsBySource(st *coachState, limit int) []coachCountKV {
	if st == nil || len(st.Transitions) == 0 {
		return nil
	}
	type rec struct {
		from  string
		to    string
		count int
		total int
	}
	bestBySource := map[string]rec{}
	totalBySource := map[string]int{}
	for key, c := range st.Transitions {
		if c <= 0 {
			continue
		}
		parts := strings.SplitN(key, "->", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			continue
		}
		from, to := parts[0], parts[1]
		totalBySource[from] += c
		cur := bestBySource[from]
		if c > cur.count || (c == cur.count && (cur.to == "" || to < cur.to)) {
			bestBySource[from] = rec{from: from, to: to, count: c}
		}
	}
	out := make([]coachCountKV, 0, len(bestBySource))
	for from, r := range bestBySource {
		total := totalBySource[from]
		if total > 0 {
			r.total = total
		}
		conf := 0
		if r.total > 0 {
			conf = int(float64(r.count)*100/float64(r.total) + 0.5)
		}
		out = append(out, coachCountKV{
			Key:   fmt.Sprintf("%s -> %s (%d%% of %d)", r.from, r.to, conf, r.total),
			Count: r.count,
		})
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
