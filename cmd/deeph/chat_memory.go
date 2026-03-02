package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

type chatSessionMemory struct {
	CompactedThroughTurn int                  `json:"compacted_through_turn,omitempty"`
	RawTailTurns         int                  `json:"raw_tail_turns,omitempty"`
	WorkingSet           chatWorkingSet       `json:"working_set,omitempty"`
	Episodes             []chatSessionEpisode `json:"episodes,omitempty"`
}

type chatWorkingSet struct {
	ActiveTopics   []string `json:"active_topics,omitempty"`
	OpenLoops      []string `json:"open_loops,omitempty"`
	PinnedCommands []string `json:"pinned_commands,omitempty"`
}

type chatSessionEpisode struct {
	StartTurn         int      `json:"start_turn"`
	EndTurn           int      `json:"end_turn"`
	UserGoals         []string `json:"user_goals,omitempty"`
	AssistantOutcomes []string `json:"assistant_outcomes,omitempty"`
	Commands          []string `json:"commands,omitempty"`
}

type chatMemoryConfig struct {
	RawTailTurns          int
	EpisodeSpanTurns      int
	WorkingSetTurns       int
	MaxGoalsPerEpisode    int
	MaxOutcomesPerEpisode int
	MaxCommandsPerEpisode int
	MaxActiveTopics       int
	MaxOpenLoops          int
	MaxPinnedCommands     int
}

var (
	defaultChatMemoryConfig = chatMemoryConfig{
		RawTailTurns:          2,
		EpisodeSpanTurns:      3,
		WorkingSetTurns:       4,
		MaxGoalsPerEpisode:    4,
		MaxOutcomesPerEpisode: 4,
		MaxCommandsPerEpisode: 6,
		MaxActiveTopics:       5,
		MaxOpenLoops:          4,
		MaxPinnedCommands:     6,
	}
	chatCommandPattern = regexp.MustCompile("`?(deeph(?:\\s+[a-zA-Z0-9_./:@=-]+)+)`?")
)

func chatSessionMemoryPath(workspace, id string) string {
	return filepath.Join(chatSessionsDir(workspace), id+".memory.json")
}

func loadChatSessionMemory(workspace, id string) (*chatSessionMemory, error) {
	path := chatSessionMemoryPath(workspace, id)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var memory chatSessionMemory
	if err := json.Unmarshal(b, &memory); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if memory.RawTailTurns <= 0 {
		memory.RawTailTurns = defaultChatMemoryConfig.RawTailTurns
	}
	return &memory, nil
}

func saveChatSessionMemory(workspace, id string, memory *chatSessionMemory) error {
	if strings.TrimSpace(id) == "" || memory == nil {
		return nil
	}
	if err := os.MkdirAll(chatSessionsDir(workspace), 0o755); err != nil {
		return err
	}
	if memory.RawTailTurns <= 0 {
		memory.RawTailTurns = defaultChatMemoryConfig.RawTailTurns
	}
	b, err := json.MarshalIndent(memory, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session memory: %w", err)
	}
	path := chatSessionMemoryPath(workspace, id)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func loadOrBuildChatSessionMemory(workspace string, meta *chatSessionMeta, entriesByTurn map[int][]chatSessionEntry, cfg chatMemoryConfig) (*chatSessionMemory, bool) {
	if meta == nil {
		return newChatSessionMemory(cfg), false
	}
	memory, err := loadChatSessionMemory(workspace, meta.ID)
	if err != nil && !os.IsNotExist(err) {
		return rebuildChatSessionMemory(entriesByTurn, meta.Turns, cfg), true
	}
	if memory == nil {
		memory = newChatSessionMemory(cfg)
	}
	changed := advanceChatSessionMemory(memory, entriesByTurn, meta.Turns, cfg)
	if refreshChatWorkingSet(memory, meta, entriesByTurn, meta.Turns, cfg) {
		changed = true
	}
	if changed {
		return memory, true
	}
	return memory, false
}

func newChatSessionMemory(cfg chatMemoryConfig) *chatSessionMemory {
	return &chatSessionMemory{
		RawTailTurns: cfg.RawTailTurns,
	}
}

func rebuildChatSessionMemory(entriesByTurn map[int][]chatSessionEntry, totalTurns int, cfg chatMemoryConfig) *chatSessionMemory {
	memory := newChatSessionMemory(cfg)
	advanceChatSessionMemory(memory, entriesByTurn, totalTurns, cfg)
	return memory
}

func advanceChatSessionMemory(memory *chatSessionMemory, entriesByTurn map[int][]chatSessionEntry, totalTurns int, cfg chatMemoryConfig) bool {
	if memory == nil {
		return false
	}
	if memory.RawTailTurns != cfg.RawTailTurns || memory.CompactedThroughTurn > totalTurns {
		rebuilt := rebuildChatSessionMemory(entriesByTurn, totalTurns, cfg)
		*memory = *rebuilt
		return true
	}
	targetTurn := totalTurns - cfg.RawTailTurns
	if targetTurn <= memory.CompactedThroughTurn {
		return false
	}
	for turn := memory.CompactedThroughTurn + 1; turn <= targetTurn; turn++ {
		compactChatTurnIntoMemory(memory, turn, entriesByTurn[turn], cfg)
	}
	return true
}

func compactChatTurnIntoMemory(memory *chatSessionMemory, turn int, entries []chatSessionEntry, cfg chatMemoryConfig) {
	if turn <= 0 || memory == nil {
		return
	}
	digest := digestChatTurn(entries)
	lastIdx := len(memory.Episodes) - 1
	if lastIdx < 0 || memory.Episodes[lastIdx].EndTurn-memory.Episodes[lastIdx].StartTurn+1 >= cfg.EpisodeSpanTurns {
		memory.Episodes = append(memory.Episodes, chatSessionEpisode{
			StartTurn: turn,
			EndTurn:   turn,
		})
		lastIdx = len(memory.Episodes) - 1
	}
	ep := &memory.Episodes[lastIdx]
	if ep.StartTurn == 0 {
		ep.StartTurn = turn
	}
	if turn > ep.EndTurn {
		ep.EndTurn = turn
	}
	mergeUniqueClipped(&ep.UserGoals, digest.UserGoals, cfg.MaxGoalsPerEpisode, 140)
	mergeUniqueClipped(&ep.AssistantOutcomes, digest.AssistantOutcomes, cfg.MaxOutcomesPerEpisode, 160)
	mergeUniqueClipped(&ep.Commands, digest.Commands, cfg.MaxCommandsPerEpisode, 120)
	memory.CompactedThroughTurn = turn
}

type chatTurnDigest struct {
	UserGoals         []string
	AssistantOutcomes []string
	Commands          []string
}

func digestChatTurn(entries []chatSessionEntry) chatTurnDigest {
	var digest chatTurnDigest
	for _, entry := range entries {
		text := strings.TrimSpace(entry.Text)
		if text == "" {
			continue
		}
		switch entry.Role {
		case "user":
			mergeUniqueClipped(&digest.UserGoals, []string{clipLine(text, 140)}, 2, 140)
		case "assistant":
			mergeUniqueClipped(&digest.AssistantOutcomes, []string{clipLine(firstSentence(text), 160)}, 2, 160)
		}
		mergeUniqueClipped(&digest.Commands, extractChatCommands(text), 3, 120)
	}
	return digest
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	for _, sep := range []string{". ", "! ", "? "} {
		if idx := strings.Index(s, sep); idx >= 0 {
			return strings.TrimSpace(s[:idx+1])
		}
	}
	return s
}

func extractChatCommands(text string) []string {
	matches := chatCommandPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		cmd := clipLine(match[1], 120)
		if cmd == "" {
			continue
		}
		out = append(out, cmd)
	}
	return uniqueStrings(out)
}

func mergeUniqueClipped(dst *[]string, src []string, limit, clip int) {
	if dst == nil || limit <= 0 {
		return
	}
	seen := make(map[string]struct{}, len(*dst))
	for _, item := range *dst {
		seen[item] = struct{}{}
	}
	for _, item := range src {
		item = clipLine(item, clip)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		*dst = append(*dst, item)
		seen[item] = struct{}{}
		if len(*dst) >= limit {
			return
		}
	}
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func indexChatEntriesByTurn(entries []chatSessionEntry) map[int][]chatSessionEntry {
	if len(entries) == 0 {
		return map[int][]chatSessionEntry{}
	}
	index := make(map[int][]chatSessionEntry, len(entries))
	for _, entry := range entries {
		index[entry.Turn] = append(index[entry.Turn], entry)
	}
	return index
}

func appendIndexedChatEntries(index map[int][]chatSessionEntry, entries []chatSessionEntry) map[int][]chatSessionEntry {
	if index == nil {
		index = map[int][]chatSessionEntry{}
	}
	for _, entry := range entries {
		index[entry.Turn] = append(index[entry.Turn], entry)
	}
	return index
}

func buildChatMemoryBlock(memory *chatSessionMemory, maxTokens int) string {
	if memory == nil || len(memory.Episodes) == 0 || maxTokens <= 0 {
		return ""
	}
	type renderedEpisode struct {
		episode chatSessionEpisode
		block   string
		tokens  int
	}
	rendered := make([]renderedEpisode, 0, len(memory.Episodes))
	for _, episode := range memory.Episodes {
		block := renderChatEpisode(episode)
		if block == "" {
			continue
		}
		rendered = append(rendered, renderedEpisode{
			episode: episode,
			block:   block,
			tokens:  estimateChatTokens(block),
		})
	}
	if len(rendered) == 0 {
		return ""
	}
	used := estimateChatTokens("[chat_memory]\n")
	selected := make([]renderedEpisode, 0, len(rendered))
	for i := len(rendered) - 1; i >= 0; i-- {
		ep := rendered[i]
		if len(selected) > 0 && used+ep.tokens > maxTokens {
			break
		}
		selected = append(selected, ep)
		used += ep.tokens
	}
	if len(selected) == 0 {
		selected = append(selected, rendered[len(rendered)-1])
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].episode.StartTurn < selected[j].episode.StartTurn
	})
	lines := []string{
		"[chat_memory]",
		fmt.Sprintf("compacted_through_turn: %d", memory.CompactedThroughTurn),
	}
	for _, ep := range selected {
		lines = append(lines, ep.block)
	}
	return strings.Join(lines, "\n")
}

func buildChatWorkingSetBlock(memory *chatSessionMemory) string {
	if memory == nil {
		return ""
	}
	ws := memory.WorkingSet
	if len(ws.ActiveTopics) == 0 && len(ws.OpenLoops) == 0 && len(ws.PinnedCommands) == 0 {
		return ""
	}
	lines := []string{"[chat_working_set]"}
	if len(ws.ActiveTopics) > 0 {
		lines = append(lines, "active_topics: "+strings.Join(ws.ActiveTopics, " | "))
	}
	if len(ws.OpenLoops) > 0 {
		lines = append(lines, "open_loops:")
		for _, loop := range ws.OpenLoops {
			lines = append(lines, "- "+loop)
		}
	}
	if len(ws.PinnedCommands) > 0 {
		lines = append(lines, "pinned_commands:")
		for _, cmd := range ws.PinnedCommands {
			lines = append(lines, "- "+cmd)
		}
	}
	return strings.Join(lines, "\n")
}

func renderChatEpisode(episode chatSessionEpisode) string {
	lines := []string{fmt.Sprintf("- episode turns=%d-%d", episode.StartTurn, episode.EndTurn)}
	if len(episode.UserGoals) > 0 {
		lines = append(lines, "  goals: "+strings.Join(episode.UserGoals, " | "))
	}
	if len(episode.AssistantOutcomes) > 0 {
		lines = append(lines, "  outcomes: "+strings.Join(episode.AssistantOutcomes, " | "))
	}
	if len(episode.Commands) > 0 {
		lines = append(lines, "  commands: "+strings.Join(episode.Commands, " | "))
	}
	if len(lines) == 1 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func estimateChatTokens(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	tok := (len(s) + 3) / 4
	if tok <= 0 {
		return 1
	}
	return tok
}

func refreshChatWorkingSet(memory *chatSessionMemory, meta *chatSessionMeta, entriesByTurn map[int][]chatSessionEntry, totalTurns int, cfg chatMemoryConfig) bool {
	if memory == nil {
		return false
	}
	next := computeChatWorkingSet(meta, entriesByTurn, totalTurns, cfg)
	if reflect.DeepEqual(memory.WorkingSet, next) {
		return false
	}
	memory.WorkingSet = next
	return true
}

func computeChatWorkingSet(meta *chatSessionMeta, entriesByTurn map[int][]chatSessionEntry, totalTurns int, cfg chatMemoryConfig) chatWorkingSet {
	var ws chatWorkingSet
	startTurn := max(1, totalTurns-cfg.WorkingSetTurns+1)
	for turn := totalTurns; turn >= startTurn; turn-- {
		digest := digestChatTurn(entriesByTurn[turn])
		appendRecentUniqueClipped(&ws.ActiveTopics, digest.UserGoals, cfg.MaxActiveTopics, 120)
		appendRecentUniqueClipped(&ws.PinnedCommands, digest.Commands, cfg.MaxPinnedCommands, 120)
	}

	if meta != nil {
		if meta.PendingPlan != nil {
			if strings.TrimSpace(meta.PendingPlan.Summary) != "" {
				appendRecentUniqueClipped(&ws.OpenLoops, []string{"confirm plan: " + meta.PendingPlan.Summary}, cfg.MaxOpenLoops, 140)
			}
			if len(meta.PendingPlan.Commands) > 0 && strings.TrimSpace(meta.PendingPlan.Commands[0].Display) != "" {
				appendRecentUniqueClipped(&ws.PinnedCommands, []string{meta.PendingPlan.Commands[0].Display}, cfg.MaxPinnedCommands, 120)
			}
			if meta.PendingPlan.Followup != nil && strings.TrimSpace(meta.PendingPlan.Followup.Display) != "" {
				appendRecentUniqueClipped(&ws.OpenLoops, []string{"planned next step: " + meta.PendingPlan.Followup.Display}, cfg.MaxOpenLoops, 140)
			}
		}
		if meta.PendingExec != nil && strings.TrimSpace(meta.PendingExec.Display) != "" {
			appendRecentUniqueClipped(&ws.OpenLoops, []string{"confirm command: " + meta.PendingExec.Display}, cfg.MaxOpenLoops, 140)
			appendRecentUniqueClipped(&ws.PinnedCommands, []string{meta.PendingExec.Display}, cfg.MaxPinnedCommands, 120)
		}
		if meta.LastCommandReceipt != nil {
			receipt := meta.LastCommandReceipt
			if strings.TrimSpace(receipt.Command.Display) != "" {
				appendRecentUniqueClipped(&ws.PinnedCommands, []string{receipt.Command.Display}, cfg.MaxPinnedCommands, 120)
			}
			if strings.TrimSpace(receipt.Next) != "" {
				appendRecentUniqueClipped(&ws.OpenLoops, []string{"next step: " + receipt.Next}, cfg.MaxOpenLoops, 140)
				appendRecentUniqueClipped(&ws.PinnedCommands, []string{receipt.Next}, cfg.MaxPinnedCommands, 120)
			}
			if strings.TrimSpace(receipt.Error) != "" {
				appendRecentUniqueClipped(&ws.OpenLoops, []string{"resolve command error: " + clipLine(receipt.Error, 120)}, cfg.MaxOpenLoops, 140)
			}
		}
	}

	return ws
}

func appendRecentUniqueClipped(dst *[]string, src []string, limit, clip int) {
	if dst == nil || limit <= 0 {
		return
	}
	seen := make(map[string]struct{}, len(*dst))
	for _, item := range *dst {
		seen[item] = struct{}{}
	}
	for _, item := range src {
		item = clipLine(item, clip)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		*dst = append(*dst, item)
		seen[item] = struct{}{}
		if len(*dst) >= limit {
			return
		}
	}
}
