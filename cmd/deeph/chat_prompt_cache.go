package main

import (
	"fmt"
	"strings"
)

type chatPromptCache struct {
	operationalKey    string
	operationalBlock  string
	workingSetKey     string
	workingSetBlock   string
	memoryKey         string
	memoryBlock       string
	historyKey        string
	historyTailBlock  string
	operationalBuilds int
	workingSetBuilds  int
	memoryBuilds      int
	historyBuilds     int
}

func buildChatTurnInputCached(meta *chatSessionMeta, memory *chatSessionMemory, entries []chatSessionEntry, userMessage string, historyTurns, historyTokens int, cache *chatPromptCache) string {
	if cache == nil {
		return buildChatTurnInput(meta, memory, entries, userMessage, historyTurns, historyTokens)
	}
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		return ""
	}

	primer := buildChatCommandPrimer(meta, userMessage)
	operationalBlock := cache.cachedOperationalBlock(meta)
	workingSetBlock := cache.cachedWorkingSetBlock(memory)
	memoryBlock, historyTailBlock := cache.cachedMemoryAndTail(memory, entries, historyTurns, historyTokens)
	if operationalBlock == "" && workingSetBlock == "" && memoryBlock == "" && historyTailBlock == "" && primer == "" {
		return userMessage
	}

	lines := make([]string, 0, 20)
	lines = append(lines, "[chat_session]")
	if meta != nil {
		lines = append(lines, "session_id: "+meta.ID)
		if strings.TrimSpace(meta.AgentSpec) != "" {
			lines = append(lines, "agent_spec: "+meta.AgentSpec)
		}
	}
	if operationalBlock != "" {
		lines = append(lines, operationalBlock)
	}
	if runtimeRules := buildChatRuntimeRules(meta); runtimeRules != "" {
		lines = append(lines, runtimeRules)
	}
	if workingSetBlock != "" {
		lines = append(lines, workingSetBlock)
	}
	if memoryBlock != "" {
		lines = append(lines, memoryBlock)
	}
	if primer != "" {
		lines = append(lines, primer)
	}
	if historyTailBlock != "" {
		lines = append(lines, historyTailBlock)
	}
	lines = append(lines, "current_user_message:")
	lines = append(lines, userMessage)
	lines = append(lines, "instruction: continue the conversation, prefer compact memory over replaying old turns, and avoid repeating previous answers.")
	return strings.Join(lines, "\n")
}

func (c *chatPromptCache) cachedOperationalBlock(meta *chatSessionMeta) string {
	key := chatOperationalBlockKey(meta)
	if key == c.operationalKey {
		return c.operationalBlock
	}
	c.operationalKey = key
	c.operationalBlock = strings.Join(buildChatOperationalState(meta), "\n")
	c.operationalBuilds++
	return c.operationalBlock
}

func (c *chatPromptCache) cachedWorkingSetBlock(memory *chatSessionMemory) string {
	key := chatWorkingSetBlockKey(memory)
	if key == c.workingSetKey {
		return c.workingSetBlock
	}
	c.workingSetKey = key
	c.workingSetBlock = buildChatWorkingSetBlock(memory)
	c.workingSetBuilds++
	return c.workingSetBlock
}

func (c *chatPromptCache) cachedMemoryAndTail(memory *chatSessionMemory, entries []chatSessionEntry, historyTurns, historyTokens int) (string, string) {
	memoryBudget, tailBudget := splitChatHistoryBudget(memory, historyTokens)
	memoryKey := chatMemoryBlockKey(memory, memoryBudget)
	if memoryKey != c.memoryKey {
		c.memoryKey = memoryKey
		c.memoryBlock = buildChatMemoryBlock(memory, memoryBudget)
		c.memoryBuilds++
	}

	historyKey := chatHistoryTailKey(entries, memory, historyTurns, tailBudget)
	if historyKey != c.historyKey {
		c.historyKey = historyKey
		c.historyTailBlock = buildChatHistoryTailBlock(entries, memory, historyTurns, tailBudget)
		c.historyBuilds++
	}
	return c.memoryBlock, c.historyTailBlock
}

func chatOperationalBlockKey(meta *chatSessionMeta) string {
	if meta == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(meta.ID)
	b.WriteString("|")
	b.WriteString(meta.AgentSpec)
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%d", meta.Turns))
	if meta.PendingPlan != nil {
		b.WriteString("|plan:")
		b.WriteString(meta.PendingPlan.Summary)
		if len(meta.PendingPlan.Commands) > 0 {
			b.WriteString("|plan_first:")
			b.WriteString(meta.PendingPlan.Commands[0].Display)
		}
		if meta.PendingPlan.Followup != nil {
			b.WriteString("|plan_followup:")
			b.WriteString(meta.PendingPlan.Followup.Display)
		}
	}
	if meta.PendingExec != nil {
		b.WriteString("|pending:")
		b.WriteString(meta.PendingExec.Display)
	}
	if meta.LastCommandReceipt != nil {
		b.WriteString("|last:")
		b.WriteString(meta.LastCommandReceipt.Command.Display)
		b.WriteString(fmt.Sprintf("|ok:%v", meta.LastCommandReceipt.Success))
		b.WriteString("|next:")
		b.WriteString(meta.LastCommandReceipt.Next)
		b.WriteString("|err:")
		b.WriteString(meta.LastCommandReceipt.Error)
	}
	return b.String()
}

func chatWorkingSetBlockKey(memory *chatSessionMemory) string {
	if memory == nil {
		return ""
	}
	ws := memory.WorkingSet
	return strings.Join([]string{
		strings.Join(ws.ActiveTopics, "\x1f"),
		strings.Join(ws.OpenLoops, "\x1f"),
		strings.Join(ws.PinnedCommands, "\x1f"),
	}, "\x1e")
}

func chatMemoryBlockKey(memory *chatSessionMemory, budget int) string {
	if memory == nil || budget <= 0 {
		return fmt.Sprintf("budget:%d", budget)
	}
	lastStart, lastEnd := 0, 0
	if n := len(memory.Episodes); n > 0 {
		lastStart = memory.Episodes[n-1].StartTurn
		lastEnd = memory.Episodes[n-1].EndTurn
	}
	return fmt.Sprintf("budget:%d|compact:%d|episodes:%d|last:%d-%d", budget, memory.CompactedThroughTurn, len(memory.Episodes), lastStart, lastEnd)
}

func chatHistoryTailKey(entries []chatSessionEntry, memory *chatSessionMemory, historyTurns, historyTokens int) string {
	rawTailTurns := historyTurns
	if memory != nil && memory.RawTailTurns > 0 && (rawTailTurns <= 0 || memory.RawTailTurns < rawTailTurns) {
		rawTailTurns = memory.RawTailTurns
	}
	lastTurn := 0
	if len(entries) > 0 {
		lastTurn = entries[len(entries)-1].Turn
	}
	return fmt.Sprintf("entries:%d|last_turn:%d|turns:%d|tokens:%d", len(entries), lastTurn, rawTailTurns, historyTokens)
}

func splitChatHistoryBudget(memory *chatSessionMemory, historyTokens int) (memoryBudget, tailBudget int) {
	if historyTokens <= 0 {
		return 0, historyTokens
	}
	if memory == nil || len(memory.Episodes) == 0 {
		return 0, historyTokens
	}
	memoryBudget = historyTokens / 2
	if memoryBudget < 120 {
		memoryBudget = min(historyTokens, 120)
	}
	tailBudget = historyTokens - memoryBudget
	if tailBudget < 120 {
		tailBudget = min(historyTokens, 120)
		memoryBudget = max(0, historyTokens-tailBudget)
	}
	return memoryBudget, tailBudget
}

func buildChatHistoryTailBlock(entries []chatSessionEntry, memory *chatSessionMemory, historyTurns, historyTokens int) string {
	rawTailTurns := historyTurns
	if memory != nil && memory.RawTailTurns > 0 && (rawTailTurns <= 0 || memory.RawTailTurns < rawTailTurns) {
		rawTailTurns = memory.RawTailTurns
	}
	selected := selectChatHistoryEntries(entries, rawTailTurns, historyTokens)
	if len(selected) == 0 {
		return ""
	}
	lines := []string{"history_tail:"}
	for _, e := range selected {
		label := e.Role
		if e.Agent != "" {
			label += "(" + e.Agent + ")"
		}
		lines = append(lines, "- "+label+": "+clipLine(e.Text, 420))
	}
	return strings.Join(lines, "\n")
}
