package main

import (
	"context"
	"fmt"
	"sync"

	"deeph/internal/runtime"
)

type chatSessionActor struct {
	events chan chatSessionActorEvent
	done   chan struct{}
	once   sync.Once
}

type chatSessionActorEvent struct {
	line         string
	turnResp     chan chatSessionActorTurnResult
	snapshotResp chan chatSessionActorSnapshot
}

type chatSessionActorTurnResult struct {
	Done bool
}

type chatSessionActorSnapshot struct {
	Meta    chatSessionMeta
	Entries []chatSessionEntry
}

type chatSessionActorConfig struct {
	Workspace     string
	ShowTrace     bool
	ShowCoach     bool
	HistoryTurns  int
	HistoryTokens int
	Plan          runtime.ExecutionPlan
	Tasks         []runtime.Task
	SinkIdxs      []int
	Engine        *runtime.Engine
}

type chatSessionActorState struct {
	cfg           chatSessionActorConfig
	meta          *chatSessionMeta
	entries       []chatSessionEntry
	entriesByTurn map[int][]chatSessionEntry
	memory        *chatSessionMemory
	promptCache   *chatPromptCache
}

func newChatSessionActor(cfg chatSessionActorConfig, meta *chatSessionMeta, entries []chatSessionEntry) *chatSessionActor {
	actor := &chatSessionActor{
		events: make(chan chatSessionActorEvent),
		done:   make(chan struct{}),
	}
	state := chatSessionActorState{
		cfg:           cfg,
		meta:          cloneChatSessionMeta(meta),
		entries:       cloneChatSessionEntries(entries),
		entriesByTurn: indexChatEntriesByTurn(entries),
		promptCache:   &chatPromptCache{},
	}
	state.memory, _ = loadOrBuildChatSessionMemory(cfg.Workspace, state.meta, state.entriesByTurn, defaultChatMemoryConfig)
	_ = saveChatSessionMemory(cfg.Workspace, state.meta.ID, state.memory)
	go actor.loop(state)
	return actor
}

func (a *chatSessionActor) loop(state chatSessionActorState) {
	defer close(a.done)
	for ev := range a.events {
		switch {
		case ev.snapshotResp != nil:
			ev.snapshotResp <- chatSessionActorSnapshot{
				Meta:    *cloneChatSessionMeta(state.meta),
				Entries: cloneChatSessionEntries(state.entries),
			}
		case ev.turnResp != nil:
			ev.turnResp <- state.processLine(ev.line)
		}
	}
}

func (a *chatSessionActor) ProcessLine(line string) chatSessionActorTurnResult {
	resp := make(chan chatSessionActorTurnResult)
	a.events <- chatSessionActorEvent{
		line:     line,
		turnResp: resp,
	}
	return <-resp
}

func (a *chatSessionActor) Snapshot() chatSessionActorSnapshot {
	resp := make(chan chatSessionActorSnapshot)
	a.events <- chatSessionActorEvent{
		snapshotResp: resp,
	}
	return <-resp
}

func (a *chatSessionActor) Close() {
	a.once.Do(func() {
		close(a.events)
		<-a.done
	})
}

func (s *chatSessionActorState) processLine(line string) chatSessionActorTurnResult {
	route, err := routeChatTurn(s.cfg.Workspace, s.meta, s.entries, line, s.cfg.Plan, s.cfg.Tasks, s.cfg.SinkIdxs)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return chatSessionActorTurnResult{Done: route.Done}
	}
	if route.Kind == chatRouteHandled {
		if len(route.Replies) > 0 {
			route.Replies = sanitizeChatReplies(s.meta, route.Replies)
			printChatReplies(route.Replies)
			before := len(s.entries)
			persistChatTurn(s.cfg.Workspace, s.meta, &s.entries, line, route.Replies)
			s.afterPersist(before)
		}
		return chatSessionActorTurnResult{Done: route.Done}
	}

	if s.cfg.ShowTrace {
		printCompactChatPlan(s.cfg.Plan, s.cfg.SinkIdxs)
	}
	if s.cfg.Engine == nil {
		fmt.Println("error: chat engine unavailable")
		return chatSessionActorTurnResult{}
	}

	input := buildChatTurnInputCached(s.meta, s.memory, s.entries, line, s.cfg.HistoryTurns, s.cfg.HistoryTokens, s.promptCache)
	ctx := context.Background()
	stopCoach := func() {}
	if s.cfg.ShowCoach {
		stopCoach = startCoachHint(ctx, coachHintRequest{
			Workspace:   s.cfg.Workspace,
			CommandPath: "chat",
			AgentSpec:   s.meta.AgentSpec,
			Input:       input,
			Plan:        &s.cfg.Plan,
			Tasks:       s.cfg.Tasks,
			InChat:      true,
			ShowTrace:   s.cfg.ShowTrace,
			SessionID:   s.meta.ID,
			Turn:        s.meta.Turns + 1,
		})
	}

	report, err := s.cfg.Engine.RunSpec(ctx, s.meta.AgentSpec, input)
	stopCoach()
	if err != nil {
		if replies := maybeBuildChatErrorFallback(s.cfg.Workspace, s.meta, line, err); len(replies) > 0 {
			s.meta.PendingPlan = maybeBuildGuideCapabilityPlan(s.cfg.Workspace, s.meta, line)
			if s.meta.PendingPlan != nil {
				s.meta.PendingExec = nil
			} else if len(replies) == 1 {
				s.meta.PendingExec = derivePendingExecFromGuideText(s.cfg.Workspace, replies[0].Text)
			}
			replies = sanitizeChatReplies(s.meta, replies)
			printChatReplies(replies)
			before := len(s.entries)
			persistChatTurn(s.cfg.Workspace, s.meta, &s.entries, line, replies)
			s.afterPersist(before)
			return chatSessionActorTurnResult{}
		}
		fmt.Printf("error: %v\n", err)
		return chatSessionActorTurnResult{}
	}
	recordCoachRunSignals(s.cfg.Workspace, &s.cfg.Plan, report)

	replies := collectChatReplies(report, s.cfg.SinkIdxs)
	if len(replies) == 0 {
		fmt.Println("assistant> (no output)")
		return chatSessionActorTurnResult{}
	}
	replies = sanitizeChatReplies(s.meta, replies)
	printChatReplies(replies)
	if s.cfg.ShowCoach && s.meta.Turns == 0 {
		maybePrintCoachPostRunHint(s.cfg.Workspace, "chat", &s.cfg.Plan, report)
	}
	before := len(s.entries)
	persistChatTurn(s.cfg.Workspace, s.meta, &s.entries, line, replies)
	s.afterPersist(before)
	return chatSessionActorTurnResult{}
}

func (s *chatSessionActorState) afterPersist(previousEntries int) {
	if len(s.entries) > previousEntries {
		s.entriesByTurn = appendIndexedChatEntries(s.entriesByTurn, s.entries[previousEntries:])
	}
	if s.memory == nil {
		s.memory = newChatSessionMemory(defaultChatMemoryConfig)
	}
	changed := advanceChatSessionMemory(s.memory, s.entriesByTurn, s.meta.Turns, defaultChatMemoryConfig)
	if refreshChatWorkingSet(s.memory, s.meta, s.entriesByTurn, s.meta.Turns, defaultChatMemoryConfig) {
		changed = true
	}
	if changed {
		if err := saveChatSessionMemory(s.cfg.Workspace, s.meta.ID, s.memory); err != nil {
			fmt.Printf("warning: failed to save session memory: %v\n", err)
		}
	}
}

func cloneChatSessionMeta(meta *chatSessionMeta) *chatSessionMeta {
	if meta == nil {
		return &chatSessionMeta{}
	}
	cp := *meta
	cp.PendingPlan = cloneChatPendingPlan(meta.PendingPlan)
	if meta.PendingExec != nil {
		pending := *meta.PendingExec
		pending.Args = append([]string{}, meta.PendingExec.Args...)
		cp.PendingExec = &pending
	}
	if meta.LastCommandReceipt != nil {
		receipt := *meta.LastCommandReceipt
		receipt.Command.Args = append([]string{}, meta.LastCommandReceipt.Command.Args...)
		cp.LastCommandReceipt = &receipt
	}
	return &cp
}

func cloneChatSessionEntries(entries []chatSessionEntry) []chatSessionEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]chatSessionEntry, len(entries))
	copy(out, entries)
	return out
}
