package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"deeph/internal/commanddoc"
	"deeph/internal/runtime"
)

func cmdChat(args []string) error {
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	sessionID := fs.String("session", "", "session id (resume if exists, create if missing)")
	historyTurns := fs.Int("history-turns", 8, "max recent user turns to include in chat history context")
	historyTokens := fs.Int("history-tokens", 900, "approx token budget for serialized chat history context")
	showTrace := fs.Bool("trace", false, "show compact plan summary before each turn")
	showCoach := fs.Bool("coach", true, "show occasional semantic tips while waiting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()

	p, abs, verr, err := loadAndValidate(*workspace)
	if err != nil {
		return err
	}
	printValidation(verr)
	if verr != nil && verr.HasErrors() {
		return verr
	}

	var requestedSpec string
	if len(rest) > 0 {
		requestedSpec = strings.TrimSpace(rest[0])
	}
	meta, entries, created, err := openOrCreateChatSession(abs, strings.TrimSpace(*sessionID), requestedSpec)
	if err != nil {
		return err
	}
	if strings.TrimSpace(meta.AgentSpec) == "" {
		return errors.New("chat requires an agent spec (ex.: `deeph chat guide` or `deeph chat --session mysession guide`)")
	}
	saveStudioRecent(abs, meta.AgentSpec, meta.ID)
	recordCoachCommandTransition(abs, "chat", meta.AgentSpec)

	eng, err := runtime.New(abs, p)
	if err != nil {
		return err
	}

	plan, tasks, err := eng.PlanSpec(context.Background(), meta.AgentSpec, "")
	if err != nil {
		return err
	}
	sinkIdxs := chatSinkTaskIndexes(tasks)

	if created {
		fmt.Printf("Chat session created: %s\n", meta.ID)
	} else {
		fmt.Printf("Chat session resumed: %s\n", meta.ID)
	}
	fmt.Printf("Workspace: %s\n", abs)
	fmt.Printf("Agent spec: %s\n", meta.AgentSpec)
	fmt.Printf("History entries loaded: %d\n", len(entries))
	fmt.Println("Commands: /help, /history, /trace, /exit")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for {
		fmt.Print("you> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			fmt.Println("")
			return nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			done, err := handleChatSlashCommand(line, abs, meta, entries, plan, tasks, sinkIdxs)
			if err != nil {
				fmt.Printf("error: %v\n", err)
			}
			if done {
				return nil
			}
			continue
		}
		if localText, ok := maybeAnswerGuideLocally(meta, line); ok {
			replies := []chatReply{{Agent: meta.AgentSpec, Text: localText}}
			printChatReplies(replies)
			persistChatTurn(abs, meta, &entries, line, replies)
			continue
		}

		if *showTrace {
			printCompactChatPlan(plan, sinkIdxs)
		}

		input := buildChatTurnInput(meta, entries, line, *historyTurns, *historyTokens)
		ctx := context.Background()
		stopCoach := func() {}
		if *showCoach {
			stopCoach = startCoachHint(ctx, coachHintRequest{
				Workspace:   abs,
				CommandPath: "chat",
				AgentSpec:   meta.AgentSpec,
				Input:       input,
				Plan:        &plan,
				Tasks:       tasks,
				InChat:      true,
				ShowTrace:   *showTrace,
				SessionID:   meta.ID,
				Turn:        meta.Turns + 1,
			})
		}
		report, err := eng.RunSpec(ctx, meta.AgentSpec, input)
		stopCoach()
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		recordCoachRunSignals(abs, &plan, report)

		replies := collectChatReplies(report, sinkIdxs)
		if len(replies) == 0 {
			fmt.Println("assistant> (no output)")
			continue
		}
		printChatReplies(replies)
		if *showCoach && meta.Turns == 0 {
			maybePrintCoachPostRunHint(abs, "chat", &plan, report)
		}
		persistChatTurn(abs, meta, &entries, line, replies)
	}
}

func cmdSession(args []string) error {
	if len(args) == 0 {
		return errors.New("session requires a subcommand: list or show")
	}
	switch args[0] {
	case "list":
		return cmdSessionList(args[1:])
	case "show":
		return cmdSessionShow(args[1:])
	default:
		return fmt.Errorf("unknown session subcommand %q", args[0])
	}
}

func cmdSessionList(args []string) error {
	fs := flag.NewFlagSet("session list", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("session list does not accept positional arguments")
	}
	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	metas, err := listChatSessionMetas(abs)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "session list")
	if len(metas) == 0 {
		fmt.Printf("No chat sessions found in %s\n", filepath.Join(abs, "sessions"))
		return nil
	}
	sort.Slice(metas, func(i, j int) bool {
		if metas[i].UpdatedAt.Equal(metas[j].UpdatedAt) {
			return metas[i].ID < metas[j].ID
		}
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})
	for _, m := range metas {
		fmt.Printf("- %s turns=%d updated_at=%s spec=%q\n", m.ID, m.Turns, m.UpdatedAt.Format(time.RFC3339), m.AgentSpec)
	}
	return nil
}

func cmdSessionShow(args []string) error {
	fs := flag.NewFlagSet("session show", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	tail := fs.Int("tail", 20, "number of recent entries to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return errors.New("session show requires <id>")
	}
	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	meta, err := loadChatSessionMeta(abs, rest[0])
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "session show")
	entries, err := loadChatSessionEntries(abs, meta.ID)
	if err != nil {
		return err
	}
	fmt.Printf("session: %s\n", meta.ID)
	fmt.Printf("agent_spec: %s\n", meta.AgentSpec)
	fmt.Printf("created_at: %s\n", meta.CreatedAt.Format(time.RFC3339))
	fmt.Printf("updated_at: %s\n", meta.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("turns: %d\n", meta.Turns)
	if *tail > 0 && len(entries) > *tail {
		entries = entries[len(entries)-*tail:]
	}
	if len(entries) == 0 {
		fmt.Println("(no entries)")
		return nil
	}
	fmt.Println("entries:")
	for _, e := range entries {
		label := e.Role
		if e.Agent != "" {
			label += ":" + e.Agent
		}
		fmt.Printf("- [%s] turn=%d %s\n", e.CreatedAt.Format(time.RFC3339), e.Turn, label)
		fmt.Printf("  %s\n", indentForSessionShow(strings.TrimSpace(e.Text)))
	}
	return nil
}

type chatReply struct {
	Agent string
	Text  string
	Error string
	Stage int
	Index int
}

const (
	chatDisplayReplyMaxChars     = 1200
	chatDisplayReplyMaxCharsMock = 320
)

func collectChatReplies(report runtime.ExecutionReport, sinkIdxs []int) []chatReply {
	replies := make([]chatReply, 0, max(1, len(sinkIdxs)))
	if len(sinkIdxs) > 0 {
		for _, idx := range sinkIdxs {
			if idx < 0 || idx >= len(report.Results) {
				continue
			}
			r := report.Results[idx]
			replies = append(replies, chatReply{
				Agent: r.Agent,
				Text:  r.Output,
				Error: r.Error,
				Stage: r.StageIndex,
				Index: idx,
			})
		}
	}
	if len(replies) == 0 && len(report.Results) > 0 {
		last := report.Results[len(report.Results)-1]
		replies = append(replies, chatReply{
			Agent: last.Agent,
			Text:  last.Output,
			Error: last.Error,
			Stage: last.StageIndex,
			Index: len(report.Results) - 1,
		})
	}
	return replies
}

func printChatReplies(replies []chatReply) {
	if len(replies) == 1 {
		r := replies[0]
		if r.Error != "" {
			fmt.Printf("assistant(%s)> error: %s\n", r.Agent, r.Error)
			return
		}
		fmt.Printf("assistant(%s)> %s\n", r.Agent, formatChatReplyForTerminal(r.Text))
		return
	}
	fmt.Printf("assistant> %d outputs\n", len(replies))
	for _, r := range replies {
		if r.Error != "" {
			fmt.Printf("[%s] error: %s\n", r.Agent, r.Error)
			continue
		}
		fmt.Printf("[%s]\n%s\n", r.Agent, formatChatReplyForTerminal(r.Text))
	}
}

func formatChatReplyForTerminal(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(empty)"
	}
	limit := chatDisplayReplyMaxChars
	if strings.HasPrefix(s, "[mock-provider]") {
		limit = chatDisplayReplyMaxCharsMock
	}
	if len(s) <= limit {
		return s
	}
	clipped := s[:limit]
	clipped = strings.TrimRight(clipped, " \n\t")
	extra := len(s) - len(clipped)
	if extra < 0 {
		extra = 0
	}
	return clipped + fmt.Sprintf("\n... [clipped %d chars, full text saved in session history]", extra)
}

func chatSinkTaskIndexes(tasks []runtime.Task) []int {
	if len(tasks) == 0 {
		return nil
	}
	out := make([]int, 0, len(tasks))
	for i, t := range tasks {
		if len(t.Outgoing) == 0 {
			out = append(out, i)
		}
	}
	if len(out) == 0 {
		out = append(out, len(tasks)-1)
	}
	return out
}

func printCompactChatPlan(plan runtime.ExecutionPlan, sinkIdxs []int) {
	fmt.Printf("[trace] spec=%q tasks=%d stages=%d parallel=%v sinks=%v\n", plan.Spec, len(plan.Tasks), len(plan.Stages), plan.Parallel, sinkIdxs)
}

func handleChatSlashCommand(line, workspace string, meta *chatSessionMeta, entries []chatSessionEntry, plan runtime.ExecutionPlan, tasks []runtime.Task, sinkIdxs []int) (done bool, err error) {
	cmd := strings.TrimSpace(line)
	switch {
	case cmd == "/exit", cmd == "/quit":
		fmt.Println("bye")
		return true, nil
	case cmd == "/help":
		fmt.Println("Slash commands:")
		fmt.Println("  /help    show this help")
		fmt.Println("  /history show recent session entries")
		fmt.Println("  /trace   show compact execution plan summary")
		fmt.Println("  /exit    end chat session")
		return false, nil
	case cmd == "/trace":
		printCompactChatPlan(plan, sinkIdxs)
		return false, nil
	case cmd == "/history":
		if len(entries) == 0 {
			fmt.Println("(no history)")
			return false, nil
		}
		start := 0
		if len(entries) > 12 {
			start = len(entries) - 12
		}
		for _, e := range entries[start:] {
			label := e.Role
			if e.Agent != "" {
				label += ":" + e.Agent
			}
			fmt.Printf("- turn=%d %s: %s\n", e.Turn, label, clipLine(e.Text, 180))
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown slash command %q", cmd)
	}
}

func buildChatTurnInput(meta *chatSessionMeta, entries []chatSessionEntry, userMessage string, historyTurns, historyTokens int) string {
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		return ""
	}
	selected := selectChatHistoryEntries(entries, historyTurns, historyTokens)
	primer := buildChatCommandPrimer(meta, userMessage)
	if len(selected) == 0 && primer == "" {
		return userMessage
	}
	lines := make([]string, 0, len(selected)+16)
	lines = append(lines, "[chat_session]")
	lines = append(lines, "session_id: "+meta.ID)
	if strings.TrimSpace(meta.AgentSpec) != "" {
		lines = append(lines, "agent_spec: "+meta.AgentSpec)
	}
	if primer != "" {
		lines = append(lines, primer)
	}
	lines = append(lines, "history:")
	for _, e := range selected {
		label := e.Role
		if e.Agent != "" {
			label += "(" + e.Agent + ")"
		}
		lines = append(lines, "- "+label+": "+clipLine(e.Text, 420))
	}
	lines = append(lines, "current_user_message:")
	lines = append(lines, userMessage)
	lines = append(lines, "instruction: continue the conversation, reuse prior context when relevant, avoid repeating previous answers.")
	return strings.Join(lines, "\n")
}

type chatCommandIntent struct {
	Keywords []string
	Paths    []string
}

var chatCommandIntents = []chatCommandIntent{
	{Keywords: []string{"quickstart", "workspace novo", "novo workspace", "iniciar workspace", "init workspace", "bootstrap"}, Paths: []string{"quickstart", "init", "validate"}},
	{Keywords: []string{"workspace", "projeto", "project"}, Paths: []string{"quickstart", "validate", "studio"}},
	{Keywords: []string{"provider", "deepseek", "api key", "modelo", "model"}, Paths: []string{"provider add", "provider list"}},
	{Keywords: []string{"skill", "ferramenta", "tool"}, Paths: []string{"skill list", "skill add"}},
	{Keywords: []string{"agent", "agente"}, Paths: []string{"agent create", "run", "chat"}},
	{Keywords: []string{"crew", "multiverse", "universo"}, Paths: []string{"crew list", "crew show", "trace", "run"}},
	{Keywords: []string{"trace", "plano", "plan", "handoff", "channel", "channels"}, Paths: []string{"trace", "run"}},
	{Keywords: []string{"run", "rodar", "executar", "execucao", "execução"}, Paths: []string{"run", "trace"}},
	{Keywords: []string{"chat", "conversa"}, Paths: []string{"chat", "session list", "session show"}},
	{Keywords: []string{"session", "sessao", "sessão", "history", "historico", "histórico"}, Paths: []string{"session list", "session show", "chat"}},
	{Keywords: []string{"kit", "starter"}, Paths: []string{"kit list", "kit add"}},
	{Keywords: []string{"validate", "validar"}, Paths: []string{"validate"}},
	{Keywords: []string{"command", "commands", "comando", "comandos", "cli", "ajuda", "help"}, Paths: []string{"command list", "command explain", "help"}},
	{Keywords: []string{"type", "types", "tipo", "tipos"}, Paths: []string{"type list", "type explain"}},
	{Keywords: []string{"coach", "hint", "dica"}, Paths: []string{"coach stats", "coach reset"}},
	{Keywords: []string{"update", "upgrade", "atualizar"}, Paths: []string{"update"}},
}

func buildChatCommandPrimer(meta *chatSessionMeta, userMessage string) string {
	if !chatShouldInjectCommandPrimer(meta, userMessage) {
		return ""
	}
	docs := chatRelevantCommandDocs(userMessage)
	if len(docs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(docs)*2+6)
	lines = append(lines, "[deeph_command_primer]")
	lines = append(lines, "When the request is about operating deepH, prefer exact deepH-native commands before generic shell commands.")
	for _, doc := range docs {
		lines = append(lines, fmt.Sprintf("- deeph %s: %s", doc.Path, doc.Summary))
		if len(doc.Usage) > 0 {
			lines = append(lines, "  usage: "+doc.Usage[0])
		}
	}
	norm := normalizeChatLookupText(userMessage)
	if strings.Contains(norm, "crew") && (strings.Contains(norm, "powershell") || strings.Contains(norm, "windows")) {
		lines = append(lines, "note: on PowerShell, prefer 'crew:name' instead of @name when running or tracing crews.")
	}
	lines = append(lines, "instruction: answer with the smallest exact `deeph ...` command sequence that solves the user's deepH workflow question.")
	return strings.Join(lines, "\n")
}

func chatShouldInjectCommandPrimer(meta *chatSessionMeta, userMessage string) bool {
	norm := normalizeChatLookupText(userMessage)
	if norm == "" {
		return false
	}
	for _, kw := range []string{
		"deeph",
		"workspace",
		"agent",
		"agente",
		"skill",
		"crew",
		"multiverse",
		"universo",
		"provider",
		"deepseek",
		"trace",
		"validate",
		"quickstart",
		"studio",
		"kit",
		"command",
		"comando",
		"session",
		"sessao",
	} {
		if strings.Contains(norm, kw) {
			return true
		}
	}
	if meta != nil && strings.EqualFold(strings.TrimSpace(meta.AgentSpec), "guide") {
		for _, kw := range []string{"como", "how", "help", "ajuda", "rodar", "executar", "criar", "create", "listar", "list", "mostrar", "show", "configurar", "setup"} {
			if strings.Contains(norm, kw) {
				return true
			}
		}
	}
	return false
}

func chatRelevantCommandDocs(userMessage string) []commanddoc.Doc {
	norm := normalizeChatLookupText(userMessage)
	if norm == "" {
		return nil
	}
	scores := map[string]int{}
	addPath := func(path string, score int) {
		path = commanddoc.NormalizePath(path)
		if path == "" {
			return
		}
		scores[path] += score
	}

	for _, doc := range commanddoc.Dictionary() {
		path := commanddoc.NormalizePath(doc.Path)
		if strings.Contains(norm, path) {
			addPath(doc.Path, 20)
		}
	}

	for _, intent := range chatCommandIntents {
		matched := false
		for _, kw := range intent.Keywords {
			if strings.Contains(norm, normalizeChatLookupText(kw)) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		for i, path := range intent.Paths {
			addPath(path, 12-i)
		}
	}

	if len(scores) == 0 {
		for _, path := range []string{"command list", "command explain", "run", "trace"} {
			addPath(path, 1)
		}
	}

	type scoredDoc struct {
		doc   commanddoc.Doc
		score int
	}
	list := make([]scoredDoc, 0, len(scores))
	for path, score := range scores {
		doc, ok := commanddoc.Lookup(path)
		if !ok {
			continue
		}
		list = append(list, scoredDoc{doc: doc, score: score})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].score == list[j].score {
			return list[i].doc.Path < list[j].doc.Path
		}
		return list[i].score > list[j].score
	})
	if len(list) > 4 {
		list = list[:4]
	}
	out := make([]commanddoc.Doc, 0, len(list))
	for _, item := range list {
		out = append(out, item.doc)
	}
	return out
}

func normalizeChatLookupText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	repl := strings.NewReplacer(
		"á", "a",
		"à", "a",
		"ã", "a",
		"â", "a",
		"é", "e",
		"ê", "e",
		"í", "i",
		"ó", "o",
		"ô", "o",
		"õ", "o",
		"ú", "u",
		"ç", "c",
	)
	s = repl.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func selectChatHistoryEntries(entries []chatSessionEntry, maxTurns, maxTokens int) []chatSessionEntry {
	if len(entries) == 0 {
		return nil
	}
	start := 0
	if maxTurns > 0 {
		userTurnsSeen := 0
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i].Role == "user" {
				userTurnsSeen++
				if userTurnsSeen > maxTurns {
					start = i + 1
					break
				}
			}
		}
	}
	cands := entries[start:]
	if maxTokens <= 0 {
		out := make([]chatSessionEntry, len(cands))
		copy(out, cands)
		return out
	}
	used := 0
	startByBudget := len(cands)
	for i := len(cands) - 1; i >= 0; i-- {
		line := cands[i].Role + ":" + cands[i].Agent + ":" + clipLine(cands[i].Text, 420)
		tok := (len(strings.TrimSpace(line)) + 3) / 4
		if tok <= 0 {
			tok = 1
		}
		if used+tok > maxTokens {
			break
		}
		used += tok
		startByBudget = i
	}
	if startByBudget >= len(cands) {
		return nil
	}
	out := make([]chatSessionEntry, len(cands[startByBudget:]))
	copy(out, cands[startByBudget:])
	return out
}

func clipLine(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n"))
	s = strings.Join(strings.Fields(s), " ")
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func indentForSessionShow(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func persistChatTurn(workspace string, meta *chatSessionMeta, entries *[]chatSessionEntry, userLine string, replies []chatReply) {
	if meta == nil {
		return
	}
	meta.Turns++
	now := time.Now()
	meta.UpdatedAt = now

	toAppend := make([]chatSessionEntry, 0, 1+len(replies))
	toAppend = append(toAppend, chatSessionEntry{
		Turn:      meta.Turns,
		Role:      "user",
		Text:      userLine,
		CreatedAt: now,
	})
	for _, r := range replies {
		text := strings.TrimSpace(r.Text)
		if text == "" && r.Error != "" {
			text = "error: " + r.Error
		}
		if text == "" {
			continue
		}
		toAppend = append(toAppend, chatSessionEntry{
			Turn:      meta.Turns,
			Role:      "assistant",
			Agent:     r.Agent,
			Text:      text,
			CreatedAt: now,
		})
	}
	if err := appendChatSessionEntries(workspace, meta.ID, toAppend); err != nil {
		fmt.Printf("warning: failed to append session history: %v\n", err)
	} else if entries != nil {
		*entries = append(*entries, toAppend...)
	}
	if err := saveChatSessionMeta(workspace, meta); err != nil {
		fmt.Printf("warning: failed to save session metadata: %v\n", err)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
