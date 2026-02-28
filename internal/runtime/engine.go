package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"deeph/internal/project"
	"deeph/internal/typesys"
)

type Engine struct {
	workspace    string
	project      *project.Project
	providers    map[string]Provider
	providerCfgs map[string]project.ProviderConfig
	skills       map[string]Skill
	skillCfgs    map[string]project.SkillConfig
}

func New(workspace string, p *project.Project) (*Engine, error) {
	if p == nil {
		return nil, fmt.Errorf("project is nil")
	}
	providers := make(map[string]Provider, len(p.Root.Providers))
	providerCfgs := make(map[string]project.ProviderConfig, len(p.Root.Providers))
	for _, pc := range p.Root.Providers {
		providers[pc.Name] = newProvider(pc)
		providerCfgs[pc.Name] = pc
	}
	skills := make(map[string]Skill, len(p.Skills))
	skillCfgs := make(map[string]project.SkillConfig, len(p.Skills))
	for _, sc := range p.Skills {
		skills[sc.Name] = newSkill(workspace, sc)
		skillCfgs[sc.Name] = sc
	}
	return &Engine{
		workspace:    workspace,
		project:      p,
		providers:    providers,
		providerCfgs: providerCfgs,
		skills:       skills,
		skillCfgs:    skillCfgs,
	}, nil
}

func (e *Engine) Plan(ctx context.Context, agentNames []string, input string) (ExecutionPlan, []Task, error) {
	graph := AgentSpecGraph{
		Raw:    strings.Join(agentNames, "+"),
		Stages: [][]string{append([]string(nil), agentNames...)},
	}
	return e.planGraph(ctx, graph, input)
}

func (e *Engine) PlanSpec(ctx context.Context, agentSpec string, input string) (ExecutionPlan, []Task, error) {
	graph, err := ParseAgentSpecGraph(agentSpec)
	if err != nil {
		return ExecutionPlan{}, nil, err
	}
	return e.planGraph(ctx, graph, input)
}

func (e *Engine) planGraph(ctx context.Context, graph AgentSpecGraph, input string) (ExecutionPlan, []Task, error) {
	_ = ctx
	agentIndex := make(map[string]project.AgentConfig, len(e.project.Agents))
	for _, a := range e.project.Agents {
		agentIndex[a.Name] = a
	}

	flatAgentNames := graph.Flatten()
	plan := ExecutionPlan{
		CreatedAt: time.Now(),
		Parallel:  false,
		Input:     input,
		Tasks:     make([]TaskPlan, 0, len(flatAgentNames)),
		Stages:    []PlanStage{},
		Handoffs:  []TypedHandoffPlan{},
		Spec:      graph.Raw,
	}
	tasks := make([]Task, 0, len(flatAgentNames))
	taskIdxByAgent := make(map[string]int, len(flatAgentNames))
	specStageByAgent := make(map[string]int, len(flatAgentNames))
	specStageByIdx := make([]int, 0, len(flatAgentNames))
	specStageTaskIndexes := make([][]int, len(graph.Stages))

	for stageIdx, stageAgents := range graph.Stages {
		specStageTaskIndexes[stageIdx] = make([]int, 0, len(stageAgents))
		for _, name := range stageAgents {
			a, ok := agentIndex[name]
			if !ok {
				return ExecutionPlan{}, nil, e.unknownAgentError(name, agentIndex)
			}
			providerName := a.Provider
			if providerName == "" {
				providerName = e.project.Root.DefaultProvider
			}
			providerCfg, ok := e.providerCfgs[providerName]
			if !ok {
				return ExecutionPlan{}, nil, fmt.Errorf("agent %q references unknown provider %q", a.Name, providerName)
			}
			agentFile := e.project.AgentFiles[a.Name]
			taskIndex := len(tasks)
			taskIdxByAgent[a.Name] = taskIndex
			specStageByAgent[a.Name] = stageIdx
			tasks = append(tasks, Task{
				Agent:      a,
				AgentFile:  agentFile,
				Provider:   providerCfg,
				SkillNames: append([]string(nil), a.Skills...),
				StageIndex: stageIdx,
			})
			specStageByIdx = append(specStageByIdx, stageIdx)
			plan.Tasks = append(plan.Tasks, TaskPlan{
				Agent:         a.Name,
				AgentFile:     agentFile,
				Provider:      providerCfg.Name,
				ProviderType:  providerCfg.Type,
				Model:         coalesce(a.Model, providerCfg.Model),
				Skills:        append([]string(nil), a.Skills...),
				TimeoutMS:     a.TimeoutMS,
				StartupCalls:  len(a.StartupCalls),
				ContextBudget: e.contextBudgetForTask(a, providerCfg).limitTokens(),
				ContextMoment: string(e.contextMomentForTask(a, providerCfg)),
				StageIndex:    stageIdx,
				IO:            taskIOPlan(a),
			})
			specStageTaskIndexes[stageIdx] = append(specStageTaskIndexes[stageIdx], taskIndex)
		}
	}

	predIdxSets := make([]map[int]struct{}, len(tasks))
	depNameSets := make([]map[string]struct{}, len(tasks))
	for i := range tasks {
		predIdxSets[i] = map[int]struct{}{}
		depNameSets[i] = map[string]struct{}{}
	}
	edgeSeen := map[string]struct{}{}
	addEdge := func(srcIdx, dstIdx int, force bool) {
		if srcIdx < 0 || dstIdx < 0 || srcIdx >= len(tasks) || dstIdx >= len(tasks) || srcIdx == dstIdx {
			return
		}
		links := inferTaskHandoffLinks(tasks[srcIdx], tasks[dstIdx])
		if len(links) == 0 && !force {
			return
		}
		edgeKey := fmt.Sprintf("%d>%d", srcIdx, dstIdx)
		if _, ok := edgeSeen[edgeKey]; ok {
			// Even if the dependency edge already exists, new links may still have been filtered/added
			// by another pass; append only missing links below.
		} else {
			edgeSeen[edgeKey] = struct{}{}
			predIdxSets[dstIdx][srcIdx] = struct{}{}
			depNameSets[dstIdx][tasks[srcIdx].Agent.Name] = struct{}{}
		}
		for _, link := range links {
			tasks[srcIdx].Outgoing = append(tasks[srcIdx].Outgoing, link)
			tasks[dstIdx].Incoming = append(tasks[dstIdx].Incoming, link)
			plan.Handoffs = append(plan.Handoffs, TypedHandoffPlan{
				Channel:         link.Channel,
				FromAgent:       link.FromAgent,
				ToAgent:         link.ToAgent,
				FromPort:        link.FromPort,
				ToPort:          link.ToPort,
				Kind:            link.Kind.String(),
				MergePolicy:     normalizeHandoffMergePolicy(link.MergePolicy),
				ChannelPriority: link.ChannelPriority,
				TargetMaxTokens: link.TargetMaxTokens,
				Required:        link.Required,
			})
		}
	}

	// Stage barriers: previous stage feeds next stage by default.
	for stageIdx := 1; stageIdx < len(specStageTaskIndexes); stageIdx++ {
		prevIdxs := specStageTaskIndexes[stageIdx-1]
		curIdxs := specStageTaskIndexes[stageIdx]
		for _, dstIdx := range curIdxs {
			dstHasExplicitRouting := len(tasks[dstIdx].Agent.DependsOn) > 0 || len(tasks[dstIdx].Agent.DependsOnPorts) > 0
			for _, srcIdx := range prevIdxs {
				addEdge(srcIdx, dstIdx, !dstHasExplicitRouting)
			}
		}
	}

	// Explicit dependencies can connect non-adjacent earlier stages.
	for dstIdx := range tasks {
		dst := tasks[dstIdx]
		for _, depRaw := range dst.Agent.DependsOn {
			depName := strings.TrimSpace(depRaw)
			if depName == "" {
				continue
			}
			srcIdx, ok := taskIdxByAgent[depName]
			if !ok {
				return ExecutionPlan{}, nil, fmt.Errorf("agent %q depends_on %q but it is not included in the current run spec %q", dst.Agent.Name, depName, graph.Raw)
			}
			if specStageByAgent[depName] > specStageByAgent[dst.Agent.Name] {
				return ExecutionPlan{}, nil, fmt.Errorf("agent %q depends_on %q but %q appears in a later spec stage (reorder the run spec or agent stages)", dst.Agent.Name, depName, depName)
			}
			addEdge(srcIdx, dstIdx, true)
		}
		for _, depName := range listPortDependencyAgents(dst.Agent.DependsOnPorts) {
			srcIdx, ok := taskIdxByAgent[depName]
			if !ok {
				return ExecutionPlan{}, nil, fmt.Errorf("agent %q depends_on_ports references %q but it is not included in the current run spec %q", dst.Agent.Name, depName, graph.Raw)
			}
			if specStageByAgent[depName] > specStageByAgent[dst.Agent.Name] {
				return ExecutionPlan{}, nil, fmt.Errorf("agent %q depends_on_ports references %q but %q appears in a later spec stage (reorder the run spec or agent stages)", dst.Agent.Name, depName, depName)
			}
			addEdge(srcIdx, dstIdx, true)
		}
	}

	// Selective stage wait can create cross-stage parallelism when a next-stage task does not depend on
	// some tasks in the immediately previous spec stage.
	for stageIdx := 1; stageIdx < len(specStageTaskIndexes) && !plan.Parallel; stageIdx++ {
		prevIdxs := specStageTaskIndexes[stageIdx-1]
		curIdxs := specStageTaskIndexes[stageIdx]
		for _, dstIdx := range curIdxs {
			for _, srcIdx := range prevIdxs {
				if _, ok := predIdxSets[dstIdx][srcIdx]; !ok {
					plan.Parallel = true
					break
				}
			}
			if plan.Parallel {
				break
			}
		}
	}

	// Topological order and stage leveling (keeps base spec stage, pushes tasks later when needed).
	indegree := make([]int, len(tasks))
	for i := range tasks {
		indegree[i] = len(predIdxSets[i])
	}
	order := make([]int, 0, len(tasks))
	used := make([]bool, len(tasks))
	for len(order) < len(tasks) {
		picked := -1
		for i := range tasks {
			if used[i] || indegree[i] != 0 {
				continue
			}
			picked = i
			break
		}
		if picked == -1 {
			return ExecutionPlan{}, nil, fmt.Errorf("dependency cycle detected in selected agents for spec %q", graph.Raw)
		}
		used[picked] = true
		order = append(order, picked)
		for dstIdx := range tasks {
			if _, ok := predIdxSets[dstIdx][picked]; ok {
				indegree[dstIdx]--
			}
		}
	}

	stageByIdx := append([]int(nil), specStageByIdx...)
	for _, idx := range order {
		stage := stageByIdx[idx]
		for predIdx := range predIdxSets[idx] {
			if s := stageByIdx[predIdx] + 1; s > stage {
				stage = s
			}
		}
		stageByIdx[idx] = stage
	}

	// Compress stage indexes to keep output compact.
	uniqueStages := make([]int, 0, len(stageByIdx))
	stageSeen := map[int]struct{}{}
	for _, s := range stageByIdx {
		if _, ok := stageSeen[s]; ok {
			continue
		}
		stageSeen[s] = struct{}{}
		uniqueStages = append(uniqueStages, s)
	}
	sort.Ints(uniqueStages)
	stageRemap := make(map[int]int, len(uniqueStages))
	for i, s := range uniqueStages {
		stageRemap[s] = i
	}
	stageAgents := make([][]string, len(uniqueStages))
	for idx := range tasks {
		newStage := stageRemap[stageByIdx[idx]]
		tasks[idx].StageIndex = newStage
		plan.Tasks[idx].StageIndex = newStage
		stageAgents[newStage] = append(stageAgents[newStage], tasks[idx].Agent.Name)
	}
	for i, agents := range stageAgents {
		if len(agents) > 1 {
			plan.Parallel = true
		}
		plan.Stages = append(plan.Stages, PlanStage{Index: i, Agents: agents})
	}

	for i := range tasks {
		deps := make([]string, 0, len(depNameSets[i]))
		for name := range depNameSets[i] {
			deps = append(deps, name)
		}
		sort.Strings(deps)
		tasks[i].DependsOn = deps
		plan.Tasks[i].DependsOn = append([]string(nil), deps...)
	}

	// Ensure stable handoff ordering for trace output.
	plan.Handoffs = dedupeTypedHandoffPlans(plan.Handoffs)
	for i := range tasks {
		tasks[i].Incoming = dedupeHandoffLinks(tasks[i].Incoming)
		tasks[i].Outgoing = dedupeHandoffLinks(tasks[i].Outgoing)
	}
	return plan, tasks, nil
}

func (e *Engine) unknownAgentError(name string, agentIndex map[string]project.AgentConfig) error {
	if name == "" {
		return fmt.Errorf("unknown agent %q", name)
	}
	examplePath := filepath.Join(e.workspace, "examples", "agents", name+".yaml")
	if _, err := os.Stat(examplePath); err == nil {
		targetPath := filepath.Join(e.workspace, "agents", name+".yaml")
		return fmt.Errorf("unknown agent %q (tip: copy %s -> %s)", name, examplePath, targetPath)
	}

	available := make([]string, 0, len(agentIndex))
	for agentName := range agentIndex {
		available = append(available, agentName)
	}
	sort.Strings(available)
	if len(available) == 0 {
		return fmt.Errorf("unknown agent %q (no agents found in %s)", name, filepath.Join(e.workspace, "agents"))
	}
	return fmt.Errorf("unknown agent %q (available: %s)", name, strings.Join(available, ", "))
}

func (e *Engine) Run(ctx context.Context, agentNames []string, input string) (ExecutionReport, error) {
	graph := AgentSpecGraph{
		Raw:    strings.Join(agentNames, "+"),
		Stages: [][]string{append([]string(nil), agentNames...)},
	}
	return e.runGraph(ctx, graph, input)
}

func (e *Engine) RunSpec(ctx context.Context, agentSpec string, input string) (ExecutionReport, error) {
	graph, err := ParseAgentSpecGraph(agentSpec)
	if err != nil {
		return ExecutionReport{}, err
	}
	return e.runGraph(ctx, graph, input)
}

func (e *Engine) runGraph(ctx context.Context, graph AgentSpecGraph, input string) (ExecutionReport, error) {
	plan, tasks, err := e.planGraph(ctx, graph, input)
	if err != nil {
		return ExecutionReport{}, err
	}
	report := ExecutionReport{
		StartedAt: time.Now(),
		Parallel:  plan.Parallel,
		Input:     input,
		Results:   make([]AgentRunResult, len(tasks)),
	}
	sharedBus := NewContextBus(input)
	toolBroker := newToolBroker()
	stageToolBudgets := buildStageToolBudgets(tasks)
	sharedBus.AddConstraint("Prefer concise answers and avoid repeating large raw tool outputs.")
	sharedBus.AddConstraint("Use tools only when they materially improve the answer.")
	sharedBus.PutFact("runtime.engine", "deepH", 1.0, "runtime")
	sharedBus.PutFact("runtime.parallel", strconv.FormatBool(plan.Parallel), 1.0, "runtime")
	sharedBus.PutFact("runtime.scheduler", "dag_channels", 1.0, "runtime")

	taskIdxByAgent := make(map[string]int, len(tasks))
	for i, t := range tasks {
		taskIdxByAgent[t.Agent.Name] = i
	}

	if len(tasks) == 0 {
		report.EndedAt = time.Now()
		return report, nil
	}

	successors := make([][]int, len(tasks))
	indegree := make([]int, len(tasks))
	for dstIdx, task := range tasks {
		seenPred := map[int]struct{}{}
		for _, depName := range task.DependsOn {
			srcIdx, ok := taskIdxByAgent[depName]
			if !ok || srcIdx == dstIdx {
				continue
			}
			if _, ok := seenPred[srcIdx]; ok {
				continue
			}
			seenPred[srcIdx] = struct{}{}
			indegree[dstIdx]++
			successors[srcIdx] = append(successors[srcIdx], dstIdx)
		}
	}
	for i := range successors {
		sort.Ints(successors[i])
	}

	type taskResultEvent struct {
		taskIndex int
		result    AgentRunResult
	}
	resultsCh := make(chan taskResultEvent, len(tasks))
	launched := make([]bool, len(tasks))
	launchTask := func(idx int) {
		if idx < 0 || idx >= len(tasks) || launched[idx] {
			return
		}
		launched[idx] = true
		go func(taskIndex int) {
			resultsCh <- taskResultEvent{
				taskIndex: taskIndex,
				result:    e.runTask(ctx, tasks[taskIndex], input, sharedBus, toolBroker, stageToolBudgets[tasks[taskIndex].StageIndex]),
			}
		}(idx)
	}

	launchedCount := 0
	for i := range tasks {
		if indegree[i] == 0 {
			launchTask(i)
			launchedCount++
		}
	}
	if launchedCount == 0 {
		return ExecutionReport{}, fmt.Errorf("no runnable tasks for spec %q (dependency deadlock)", graph.Raw)
	}

	completedCount := 0
	for completedCount < len(tasks) {
		item := <-resultsCh
		report.Results[item.taskIndex] = item.result
		pub := e.publishTaskOutputs(sharedBus, tasks[item.taskIndex], report.Results[item.taskIndex])
		report.Results[item.taskIndex].SentHandoffs = pub.Sent
		report.Results[item.taskIndex].DroppedHandoffs = pub.Dropped
		report.Results[item.taskIndex].HandoffTokens = pub.Tokens
		report.Results[item.taskIndex].SkippedOutputPublish = pub.SkippedUnconsumedOutput
		completedCount++

		for _, succIdx := range successors[item.taskIndex] {
			if indegree[succIdx] > 0 {
				indegree[succIdx]--
			}
			if indegree[succIdx] == 0 && !launched[succIdx] {
				launchTask(succIdx)
				launchedCount++
			}
		}
	}
	report.EndedAt = time.Now()
	return report, nil
}

func (e *Engine) runTask(parent context.Context, task Task, input string, sharedBus *ContextBus, broker *toolBroker, stageBudget *stageToolBudget) AgentRunResult {
	start := time.Now()
	res := AgentRunResult{
		Agent:        task.Agent.Name,
		Provider:     task.Provider.Name,
		ProviderType: task.Provider.Type,
		Model:        coalesce(task.Agent.Model, task.Provider.Model),
		Skills:       append([]string(nil), task.SkillNames...),
		StageIndex:   task.StageIndex,
		DependsOn:    append([]string(nil), task.DependsOn...),
	}
	bus := sharedBus
	if bus == nil {
		bus = NewContextBus(input)
	}
	budget := e.contextBudgetForTask(task.Agent, task.Provider)
	res.ContextBudget = budget.limitTokens()
	moment := e.contextMomentForTask(task.Agent, task.Provider)
	res.ContextMoment = string(moment)
	compileChannels, channelStats := selectTaskCompileChannels(task, budget, moment)
	res.ContextChannelsTotal = channelStats.Total
	res.ContextChannelsUsed = channelStats.Selected
	res.ContextChannelsDropped = channelStats.Dropped
	toolBudget := newTaskToolBudget(task.Agent.Metadata)
	res.ToolBudgetCallsLimit = toolBudget.MaxCalls
	if toolBudget.MaxExec > 0 {
		res.ToolBudgetExecMSLimit = int(toolBudget.MaxExec / time.Millisecond)
	}
	if stageBudget != nil {
		_, _, maxCalls, maxExec := stageBudget.Snapshot()
		res.StageToolBudgetCallsLimit = maxCalls
		if maxExec > 0 {
			res.StageToolBudgetExecMSLimit = int(maxExec / time.Millisecond)
		}
	}
	defer func() {
		res.ToolBudgetCallsUsed = toolBudget.CallsUsed
		res.ToolBudgetExecMSUsed = int(toolBudget.ExecUsed / time.Millisecond)
		if stageBudget != nil {
			callsUsed, execUsed, _, _ := stageBudget.Snapshot()
			res.StageToolBudgetCallsUsed = callsUsed
			res.StageToolBudgetExecMSUsed = int(execUsed / time.Millisecond)
		}
	}()

	ctx := parent
	cancel := func() {}
	if task.Agent.TimeoutMS > 0 {
		ctx, cancel = context.WithTimeout(parent, time.Duration(task.Agent.TimeoutMS)*time.Millisecond)
	}
	defer cancel()

	startupResults := make([]SkillCallResult, 0, len(task.Agent.StartupCalls))
	for _, call := range task.Agent.StartupCalls {
		_, ok := e.skills[call.Skill]
		if !ok {
			startupResults = append(startupResults, SkillCallResult{Skill: call.Skill, Error: "skill not registered"})
			res.StartupCalls = startupResults
			res.Error = fmt.Sprintf("startup call references unknown skill %q", call.Skill)
			res.Duration = time.Since(start)
			return res
		}
		if err := beforeToolCallBudgets(toolBudget, stageBudget, call.Skill); err != nil {
			callRes := SkillCallResult{Skill: call.Skill, Error: err.Error(), Duration: 0}
			startupResults = append(startupResults, callRes)
			bus.RecordSkillCallAtMoment(task.Agent.Name, callRes, ContextMomentDiscovery)
			res.StartupCalls = startupResults
			res.Error = fmt.Sprintf("startup skill %q blocked by tool budget: %v", call.Skill, err)
			res.Duration = time.Since(start)
			return res
		}
		callStart := time.Now()
		out, cacheable, cached, err := e.executeSkillWithBroker(ctx, broker, task.Agent, call.Skill, input, call.Args)
		callRes := SkillCallResult{Skill: call.Skill, Duration: time.Since(callStart), Cacheable: cacheable, Cached: cached}
		afterToolCallBudgets(toolBudget, stageBudget, callRes.Duration)
		if err != nil {
			callRes.Error = err.Error()
			startupResults = append(startupResults, callRes)
			bus.RecordSkillCallAtMoment(task.Agent.Name, callRes, ContextMomentDiscovery)
			res.StartupCalls = startupResults
			res.Error = fmt.Sprintf("startup skill %q failed: %v", call.Skill, err)
			res.Duration = time.Since(start)
			return res
		}
		if err := afterToolExecutionCheckBudgets(toolBudget, stageBudget, call.Skill); err != nil {
			callRes.Error = err.Error()
			startupResults = append(startupResults, callRes)
			bus.RecordSkillCallAtMoment(task.Agent.Name, callRes, ContextMomentDiscovery)
			res.StartupCalls = startupResults
			res.Error = fmt.Sprintf("startup skill %q blocked by tool budget: %v", call.Skill, err)
			res.Duration = time.Since(start)
			return res
		}
		callRes.Result = out
		if callRes.Cacheable {
			if callRes.Cached {
				res.ToolCacheHits++
			} else {
				res.ToolCacheMisses++
			}
		}
		startupResults = append(startupResults, callRes)
		bus.RecordSkillCallAtMoment(task.Agent.Name, callRes, ContextMomentDiscovery)
	}
	res.StartupCalls = startupResults

	compiled := bus.Compile(ContextCompileSpec{
		AgentName: task.Agent.Name,
		Skills:    task.Agent.Skills,
		Budget:    budget,
		Moment:    moment,
		Channels:  compileChannels,
	})
	res.ContextTokens = compiled.EstimatedTokens
	res.ContextVersion = compiled.Version
	res.ContextDropped = compiled.DroppedItems

	provider, ok := e.providers[task.Provider.Name]
	if !ok {
		res.Error = fmt.Sprintf("provider %q not registered", task.Provider.Name)
		res.Duration = time.Since(start)
		return res
	}

	if task.Provider.Type == "deepseek" && len(task.Agent.Skills) > 0 {
		toolResp, toolTrace, toolHits, toolMisses, err := e.runToolLoop(ctx, provider, task, input, bus, compiled, broker, toolBudget, stageBudget)
		res.ToolCalls = toolTrace
		res.ToolCacheHits += toolHits
		res.ToolCacheMisses += toolMisses
		if err != nil {
			res.Error = err.Error()
			res.Duration = time.Since(start)
			return res
		}
		res.Output = toolResp.Text
		if pt, ok := toolResp.Meta["provider_type"].(string); ok && pt != "" {
			res.ProviderType = pt
		}
		if toolResp.Provider != "" {
			res.Provider = toolResp.Provider
		}
		if toolResp.Model != "" {
			res.Model = toolResp.Model
		}
		res.Duration = time.Since(start)
		return res
	}

	llmResp, err := provider.Generate(ctx, LLMRequest{
		AgentName:       task.Agent.Name,
		Model:           coalesce(task.Agent.Model, task.Provider.Model),
		SystemPrompt:    task.Agent.SystemPrompt,
		Input:           compiled.Text,
		AvailableSkills: append([]string(nil), task.Agent.Skills...),
	})
	if err != nil {
		res.Error = err.Error()
		res.Duration = time.Since(start)
		return res
	}
	res.Output = llmResp.Text
	if pt, ok := llmResp.Meta["provider_type"].(string); ok && pt != "" {
		res.ProviderType = pt
	}
	if llmResp.Provider != "" {
		res.Provider = llmResp.Provider
	}
	if llmResp.Model != "" {
		res.Model = llmResp.Model
	}
	res.Duration = time.Since(start)
	return res
}

func (e *Engine) runToolLoop(ctx context.Context, provider Provider, task Task, input string, bus *ContextBus, compiled CompiledContext, broker *toolBroker, toolBudget *taskToolBudget, stageBudget *stageToolBudget) (LLMResponse, []SkillCallResult, int, int, error) {
	tools, err := e.buildToolDefinitions(task.Agent.Skills)
	if err != nil {
		return LLMResponse{}, nil, 0, 0, err
	}
	trace := make([]SkillCallResult, 0)
	cacheHits := 0
	cacheMisses := 0
	if len(tools) == 0 {
		resp, err := provider.Generate(ctx, LLMRequest{
			AgentName:       task.Agent.Name,
			Model:           coalesce(task.Agent.Model, task.Provider.Model),
			SystemPrompt:    task.Agent.SystemPrompt,
			Input:           compiled.Text,
			AvailableSkills: append([]string(nil), task.Agent.Skills...),
		})
		return resp, trace, cacheHits, cacheMisses, err
	}

	messages := e.initialMessages(task.Agent, compiled.Text)
	maxRounds := 8
	if v, ok := metadataInt(task.Agent.Metadata, "max_tool_rounds"); ok && v > 0 {
		maxRounds = v
	}
	repeatedToolLimit := 2
	if v, ok := metadataInt(task.Agent.Metadata, "max_repeated_tool_calls"); ok && v > 0 {
		repeatedToolLimit = v
	}
	seenToolCalls := map[string]int{}
	for round := 0; round < maxRounds; round++ {
		llmResp, err := provider.Generate(ctx, LLMRequest{
			AgentName:       task.Agent.Name,
			Model:           coalesce(task.Agent.Model, task.Provider.Model),
			SystemPrompt:    task.Agent.SystemPrompt,
			Input:           input,
			AvailableSkills: append([]string(nil), task.Agent.Skills...),
			Messages:        append([]ChatMessage(nil), messages...),
			Tools:           tools,
			ToolChoice:      "auto",
		})
		if err != nil {
			return LLMResponse{}, trace, cacheHits, cacheMisses, err
		}

		if len(llmResp.ToolCalls) == 0 {
			return llmResp, trace, cacheHits, cacheMisses, nil
		}

		assistantMsg := ChatMessage{
			Role:             "assistant",
			Content:          llmResp.Text,
			ToolCalls:        append([]LLMToolCall(nil), llmResp.ToolCalls...),
			ReasoningContent: llmResp.ReasoningContent,
		}
		messages = append(messages, assistantMsg)

		for _, tc := range llmResp.ToolCalls {
			key := toolCallKey(tc)
			seenToolCalls[key]++
			if seenToolCalls[key] > repeatedToolLimit {
				callTrace := SkillCallResult{
					Skill:    tc.Name,
					CallID:   tc.ID,
					Args:     tryDecodeToolArgs(tc.Arguments),
					Error:    fmt.Sprintf("repeated tool call blocked after %d attempt(s)", repeatedToolLimit),
					Duration: 0,
				}
				toolMsg := toolErrorMessage(tc, callTrace.Error)
				if bus != nil {
					bus.RecordSkillCall(task.Agent.Name, callTrace)
				}
				trace = append(trace, callTrace)
				messages = append(messages, toolMsg)
				continue
			}
			if err := beforeToolCallBudgets(toolBudget, stageBudget, tc.Name); err != nil {
				callTrace := SkillCallResult{
					Skill:    tc.Name,
					CallID:   tc.ID,
					Args:     tryDecodeToolArgs(tc.Arguments),
					Error:    err.Error(),
					Duration: 0,
				}
				if bus != nil {
					bus.RecordSkillCall(task.Agent.Name, callTrace)
				}
				trace = append(trace, callTrace)
				return LLMResponse{}, trace, cacheHits, cacheMisses, fmt.Errorf("tool budget exceeded for agent %q: %v", task.Agent.Name, err)
			}
			callTrace, toolMsg := e.executeToolCall(ctx, broker, task.Agent, input, tc)
			afterToolCallBudgets(toolBudget, stageBudget, callTrace.Duration)
			if err := afterToolExecutionCheckBudgets(toolBudget, stageBudget, tc.Name); err != nil {
				callTrace.Error = err.Error()
				if bus != nil {
					bus.RecordSkillCall(task.Agent.Name, callTrace)
				}
				trace = append(trace, callTrace)
				return LLMResponse{}, trace, cacheHits, cacheMisses, fmt.Errorf("tool budget exceeded for agent %q: %v", task.Agent.Name, err)
			}
			if callTrace.Cacheable {
				if callTrace.Cached {
					cacheHits++
				} else {
					cacheMisses++
				}
			}
			if bus != nil {
				bus.RecordSkillCall(task.Agent.Name, callTrace)
			}
			trace = append(trace, callTrace)
			messages = append(messages, toolMsg)
		}
	}

	return LLMResponse{}, trace, cacheHits, cacheMisses, fmt.Errorf("tool loop exceeded max rounds (%d)", maxRounds)
}

func (e *Engine) initialMessages(agent project.AgentConfig, compiledContext string) []ChatMessage {
	msgs := make([]ChatMessage, 0, 2)
	if strings.TrimSpace(agent.SystemPrompt) != "" {
		msgs = append(msgs, ChatMessage{
			Role:    "system",
			Content: agent.SystemPrompt,
		})
	}
	if strings.TrimSpace(compiledContext) == "" {
		compiledContext = " "
	}
	msgs = append(msgs, ChatMessage{
		Role:    "user",
		Content: compiledContext,
	})
	return msgs
}

func (e *Engine) executeToolCall(ctx context.Context, broker *toolBroker, agent project.AgentConfig, input string, tc LLMToolCall) (SkillCallResult, ChatMessage) {
	callTrace := SkillCallResult{
		Skill:  tc.Name,
		CallID: tc.ID,
	}

	args := map[string]any{}
	if strings.TrimSpace(tc.Arguments) != "" {
		var decoded any
		if err := json.Unmarshal([]byte(tc.Arguments), &decoded); err != nil {
			callTrace.Error = fmt.Sprintf("invalid tool arguments JSON: %v", err)
			callTrace.Duration = 0
			return callTrace, toolErrorMessage(tc, callTrace.Error)
		}
		obj, ok := decoded.(map[string]any)
		if !ok {
			callTrace.Error = "tool arguments must be a JSON object"
			callTrace.Duration = 0
			return callTrace, toolErrorMessage(tc, callTrace.Error)
		}
		args = obj
	}
	callTrace.Args = args

	skill, ok := e.skills[tc.Name]
	if !ok {
		callTrace.Error = "skill not registered"
		callTrace.Duration = 0
		return callTrace, toolErrorMessage(tc, callTrace.Error)
	}

	start := time.Now()
	_ = skill // resolved above for existence; execution happens via helper.
	result, cacheable, cached, err := e.executeSkillWithBroker(ctx, broker, agent, tc.Name, input, args)
	callTrace.Duration = time.Since(start)
	callTrace.Cacheable = cacheable
	callTrace.Cached = cached
	if err != nil {
		callTrace.Error = err.Error()
		return callTrace, toolErrorMessage(tc, err.Error())
	}
	callTrace.Result = result
	return callTrace, toolResultMessage(tc, result)
}

func (e *Engine) executeSkillWithBroker(ctx context.Context, broker *toolBroker, agent project.AgentConfig, skillName, input string, args map[string]any) (map[string]any, bool, bool, error) {
	skill, ok := e.skills[skillName]
	if !ok {
		return nil, false, false, fmt.Errorf("skill %q not registered", skillName)
	}
	lockKey := e.lockKeyForSkillCall(agent, skillName, args)
	runSkill := func(runCtx context.Context) (map[string]any, error) {
		if broker != nil && lockKey != "" {
			return broker.WithResourceLock(runCtx, lockKey, func(lockedCtx context.Context) (map[string]any, error) {
				return skill.Execute(lockedCtx, SkillExecution{AgentName: agent.Name, Input: input, Args: args})
			})
		}
		return skill.Execute(runCtx, SkillExecution{AgentName: agent.Name, Input: input, Args: args})
	}
	cacheKey, cacheable := e.cacheKeyForSkillCall(agent, skillName, input, args)
	if !cacheable || broker == nil {
		out, err := runSkill(ctx)
		return out, cacheable, false, err
	}
	out, err, cacheHit := broker.Do(ctx, cacheKey, func(runCtx context.Context) (map[string]any, error) {
		return runSkill(runCtx)
	})
	return out, true, cacheHit, err
}

func (e *Engine) cacheKeyForSkillCall(agent project.AgentConfig, skillName, input string, args map[string]any) (string, bool) {
	cfg, ok := e.skillCfgs[skillName]
	if !ok {
		return "", false
	}
	typ := strings.ToLower(strings.TrimSpace(cfg.Type))
	switch typ {
	case "file_read", "file_read_range":
		// Workspace-scoped deterministic reads; safe to dedupe across agents in a run.
		return encodeSkillCacheKey("v1", typ, skillName, args, "")
	case "command_doc":
		return encodeSkillCacheKey("v1", typ, skillName, args, "")
	case "echo":
		// Echo includes input in result, so include it if user explicitly enables cache.
		if !metadataBool(agent.Metadata, "cache_echo_tools") {
			return "", false
		}
		return encodeSkillCacheKey("v1", typ, skillName, args, input)
	case "http":
		if !metadataBool(agent.Metadata, "cache_http_get_tools") {
			return "", false
		}
		method := strings.ToUpper(coalesce(anyString(args["method"]), cfg.Method, "GET"))
		if method != "GET" {
			return "", false
		}
		return encodeSkillCacheKey("v1", typ, skillName, args, "")
	default:
		return "", false
	}
}

func (e *Engine) lockKeyForSkillCall(agent project.AgentConfig, skillName string, args map[string]any) string {
	cfg, ok := e.skillCfgs[skillName]
	if !ok {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "file_read", "file_read_range", "file_write_safe":
		if !metadataBoolDefault(agent.Metadata, "lock_file_tools", true) {
			return ""
		}
		pathVal := strings.TrimSpace(anyString(args["path"]))
		if pathVal == "" {
			return ""
		}
		return "file:" + filepath.Clean(pathVal)
	case "http":
		if !metadataBool(agent.Metadata, "lock_http_host_tools") {
			return ""
		}
		rawURL := strings.TrimSpace(coalesce(anyString(args["url"]), cfg.URL))
		if rawURL == "" {
			return ""
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return ""
		}
		host := strings.ToLower(strings.TrimSpace(u.Host))
		if host == "" {
			return ""
		}
		return "http-host:" + host
	default:
		return ""
	}
}

func encodeSkillCacheKey(version, skillType, skillName string, args map[string]any, input string) (string, bool) {
	argBytes, err := json.Marshal(args)
	if err != nil {
		return "", false
	}
	key := version + "|type=" + skillType + "|name=" + skillName + "|args=" + string(argBytes)
	if strings.TrimSpace(input) != "" {
		key += "|input=" + strings.TrimSpace(input)
	}
	return key, true
}

func toolResultMessage(tc LLMToolCall, result map[string]any) ChatMessage {
	payload := map[string]any{
		"ok":     true,
		"result": result,
	}
	b, _ := json.Marshal(payload)
	return ChatMessage{
		Role:       "tool",
		ToolCallID: tc.ID,
		Name:       tc.Name,
		Content:    string(b),
	}
}

func toolErrorMessage(tc LLMToolCall, message string) ChatMessage {
	payload := map[string]any{
		"ok":    false,
		"error": message,
	}
	b, _ := json.Marshal(payload)
	return ChatMessage{
		Role:       "tool",
		ToolCallID: tc.ID,
		Name:       tc.Name,
		Content:    string(b),
	}
}

func toolCallKey(tc LLMToolCall) string {
	return tc.Name + "|" + strings.TrimSpace(tc.Arguments)
}

func tryDecodeToolArgs(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}

func (e *Engine) buildToolDefinitions(skillNames []string) ([]LLMToolDefinition, error) {
	tools := make([]LLMToolDefinition, 0, len(skillNames))
	for _, name := range skillNames {
		cfg, ok := e.skillCfgs[name]
		if !ok {
			return nil, fmt.Errorf("agent references unknown skill %q", name)
		}
		def := LLMToolDefinition{
			Name:        cfg.Name,
			Description: coalesce(cfg.Description, "Runtime skill"),
			Parameters:  toolParametersSchema(cfg),
		}
		tools = append(tools, def)
	}
	return tools, nil
}

func toolParametersSchema(cfg project.SkillConfig) map[string]any {
	switch cfg.Type {
	case "file_read":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path inside the workspace (relative path only)",
				},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		}
	case "file_read_range":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path inside the workspace (relative path only)",
				},
				"start_line": map[string]any{
					"type":        "integer",
					"description": "1-based start line (inclusive)",
				},
				"end_line": map[string]any{
					"type":        "integer",
					"description": "1-based end line (inclusive)",
				},
				"max_bytes": map[string]any{
					"type":        "integer",
					"description": "Optional override to reduce returned bytes",
				},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		}
	case "file_write_safe":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path inside the workspace (relative path only)",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full text content to write",
				},
				"overwrite": map[string]any{
					"type":        "boolean",
					"description": "Set true to replace an existing file (default false)",
				},
				"create_dirs": map[string]any{
					"type":        "boolean",
					"description": "Create parent directories if missing (default true)",
				},
				"create_if_missing": map[string]any{
					"type":        "boolean",
					"description": "Allow creating a new file if it does not exist (default true)",
				},
				"expected_existing_sha1": map[string]any{
					"type":        "string",
					"description": "Optional guard: fail if current file sha1 differs",
				},
				"max_bytes": map[string]any{
					"type":        "integer",
					"description": "Optional lower max size for content (safety cap)",
				},
			},
			"required":             []string{"path", "content"},
			"additionalProperties": false,
		}
	case "http":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Target URL",
				},
				"method": map[string]any{
					"type":        "string",
					"description": "HTTP method (GET, POST, ...)",
				},
				"body": map[string]any{
					"type":        "string",
					"description": "Optional request body",
				},
			},
			"required": []string{"url"},
		}
	case "echo":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "Optional note to echo back",
				},
			},
		}
	case "command_doc":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Exact deeph command path when known, such as chat, quickstart, provider add or kit add",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Fallback free-text query for deeph command lookup",
				},
				"category": map[string]any{
					"type":        "string",
					"description": "Optional command category filter such as workspace, execution, providers or sessions",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Optional small cap for fuzzy matches",
				},
			},
			"additionalProperties": false,
		}
	default:
		return map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		}
	}
}

func (e *Engine) contextBudgetForTask(agent project.AgentConfig, provider project.ProviderConfig) ContextBudget {
	b := DefaultContextBudget()

	// Keep default budgets conservative for lower token spend.
	switch provider.Type {
	case "deepseek":
		b.MaxInputTokens = 1100
	default:
		b.MaxInputTokens = 900
	}
	if strings.Contains(strings.ToLower(coalesce(agent.Model, provider.Model)), "reasoner") {
		b.MaxInputTokens += 200
	}

	if v, ok := metadataInt(agent.Metadata, "context_max_input_tokens"); ok && v > 0 {
		b.MaxInputTokens = v
	}
	if v, ok := metadataInt(agent.Metadata, "context_max_recent_events"); ok && v > 0 {
		b.MaxRecentEvents = v
	}
	if v, ok := metadataInt(agent.Metadata, "context_max_artifacts"); ok && v > 0 {
		b.MaxArtifacts = v
	}
	if v, ok := metadataInt(agent.Metadata, "context_max_facts"); ok && v > 0 {
		b.MaxFacts = v
	}
	return b
}

func (e *Engine) contextMomentForTask(agent project.AgentConfig, provider project.ProviderConfig) ContextMoment {
	if v, ok := agent.Metadata["context_moment"]; ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "plan":
			return ContextMomentPlan
		case "discovery":
			return ContextMomentDiscovery
		case "tool_loop", "tools":
			return ContextMomentToolLoop
		case "synthesis", "answer":
			return ContextMomentSynthesis
		case "validate", "validation":
			return ContextMomentValidate
		}
	}
	if provider.Type == "deepseek" && len(agent.Skills) > 0 {
		return ContextMomentToolLoop
	}
	if strings.Contains(strings.ToLower(agent.Name), "review") || strings.Contains(strings.ToLower(agent.Name), "lint") {
		return ContextMomentValidate
	}
	if len(agent.Skills) > 0 {
		return ContextMomentDiscovery
	}
	return ContextMomentSynthesis
}

type taskToolBudget struct {
	MaxCalls  int
	MaxExec   time.Duration
	CallsUsed int
	ExecUsed  time.Duration
}

func newTaskToolBudget(meta map[string]string) *taskToolBudget {
	b := &taskToolBudget{}
	if v, ok := metadataInt(meta, "tool_max_calls"); ok && v > 0 {
		b.MaxCalls = v
	}
	if v, ok := metadataInt(meta, "tool_max_exec_ms"); ok && v > 0 {
		b.MaxExec = time.Duration(v) * time.Millisecond
	}
	return b
}

func (b *taskToolBudget) RollbackBeforeCall() {
	if b == nil {
		return
	}
	if b.CallsUsed > 0 {
		b.CallsUsed--
	}
}

func (b *taskToolBudget) BeforeCall(skill string) error {
	if b == nil {
		return nil
	}
	if b.MaxCalls > 0 && b.CallsUsed >= b.MaxCalls {
		return fmt.Errorf("tool_max_calls exceeded (%d) before %s", b.MaxCalls, strings.TrimSpace(skill))
	}
	b.CallsUsed++
	return nil
}

func (b *taskToolBudget) AfterCall(d time.Duration) {
	if b == nil {
		return
	}
	if d < 0 {
		d = 0
	}
	b.ExecUsed += d
}

func (b *taskToolBudget) AfterExecutionCheck(skill string) error {
	if b == nil {
		return nil
	}
	if b.MaxExec > 0 && b.ExecUsed > b.MaxExec {
		return fmt.Errorf("tool_max_exec_ms exceeded (%dms) after %s", b.MaxExec/time.Millisecond, strings.TrimSpace(skill))
	}
	return nil
}

type stageToolBudget struct {
	StageIndex int
	MaxCalls   int
	MaxExec    time.Duration

	mu        sync.Mutex
	CallsUsed int
	ExecUsed  time.Duration
}

func buildStageToolBudgets(tasks []Task) map[int]*stageToolBudget {
	if len(tasks) == 0 {
		return nil
	}
	type acc struct {
		minCalls int
		minExec  time.Duration
	}
	accByStage := map[int]*acc{}
	for _, t := range tasks {
		stage := t.StageIndex
		a := accByStage[stage]
		if a == nil {
			a = &acc{}
			accByStage[stage] = a
		}
		if v, ok := metadataInt(t.Agent.Metadata, "stage_tool_max_calls"); ok && v > 0 {
			a.minCalls = minPositiveInt(a.minCalls, v)
		}
		if v, ok := metadataInt(t.Agent.Metadata, "stage_tool_max_exec_ms"); ok && v > 0 {
			d := time.Duration(v) * time.Millisecond
			a.minExec = minPositiveDuration(a.minExec, d)
		}
	}
	out := map[int]*stageToolBudget{}
	for stage, a := range accByStage {
		if a == nil || (a.minCalls <= 0 && a.minExec <= 0) {
			continue
		}
		out[stage] = &stageToolBudget{
			StageIndex: stage,
			MaxCalls:   a.minCalls,
			MaxExec:    a.minExec,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (b *stageToolBudget) BeforeCall(skill string) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.MaxCalls > 0 && b.CallsUsed >= b.MaxCalls {
		return fmt.Errorf("stage_tool_max_calls exceeded (%d) on stage=%d before %s", b.MaxCalls, b.StageIndex, strings.TrimSpace(skill))
	}
	b.CallsUsed++
	return nil
}

func (b *stageToolBudget) RollbackBeforeCall() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.CallsUsed > 0 {
		b.CallsUsed--
	}
}

func (b *stageToolBudget) AfterCall(d time.Duration) {
	if b == nil {
		return
	}
	if d < 0 {
		d = 0
	}
	b.mu.Lock()
	b.ExecUsed += d
	b.mu.Unlock()
}

func (b *stageToolBudget) AfterExecutionCheck(skill string) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.MaxExec > 0 && b.ExecUsed > b.MaxExec {
		return fmt.Errorf("stage_tool_max_exec_ms exceeded (%dms) on stage=%d after %s", b.MaxExec/time.Millisecond, b.StageIndex, strings.TrimSpace(skill))
	}
	return nil
}

func (b *stageToolBudget) Snapshot() (callsUsed int, execUsed time.Duration, maxCalls int, maxExec time.Duration) {
	if b == nil {
		return 0, 0, 0, 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.CallsUsed, b.ExecUsed, b.MaxCalls, b.MaxExec
}

func beforeToolCallBudgets(taskBudget *taskToolBudget, stageBudget *stageToolBudget, skill string) error {
	if taskBudget != nil {
		if err := taskBudget.BeforeCall(skill); err != nil {
			return err
		}
	}
	if stageBudget != nil {
		if err := stageBudget.BeforeCall(skill); err != nil {
			if taskBudget != nil {
				taskBudget.RollbackBeforeCall()
			}
			return err
		}
	}
	return nil
}

func afterToolCallBudgets(taskBudget *taskToolBudget, stageBudget *stageToolBudget, d time.Duration) {
	if taskBudget != nil {
		taskBudget.AfterCall(d)
	}
	if stageBudget != nil {
		stageBudget.AfterCall(d)
	}
}

func afterToolExecutionCheckBudgets(taskBudget *taskToolBudget, stageBudget *stageToolBudget, skill string) error {
	if taskBudget != nil {
		if err := taskBudget.AfterExecutionCheck(skill); err != nil {
			return err
		}
	}
	if stageBudget != nil {
		if err := stageBudget.AfterExecutionCheck(skill); err != nil {
			return err
		}
	}
	return nil
}

func minPositiveInt(cur, next int) int {
	if next <= 0 {
		return cur
	}
	if cur <= 0 || next < cur {
		return next
	}
	return cur
}

func minPositiveDuration(cur, next time.Duration) time.Duration {
	if next <= 0 {
		return cur
	}
	if cur <= 0 || next < cur {
		return next
	}
	return cur
}

func metadataInt(meta map[string]string, key string) (int, bool) {
	if meta == nil {
		return 0, false
	}
	raw, ok := meta[key]
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return n, true
}

func metadataBool(meta map[string]string, key string) bool {
	return metadataBoolDefault(meta, key, false)
}

func metadataBoolDefault(meta map[string]string, key string, fallback bool) bool {
	if meta == nil {
		return fallback
	}
	raw, ok := meta[key]
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func (e *Engine) ListAgents() []string {
	out := make([]string, 0, len(e.project.Agents))
	for _, a := range e.project.Agents {
		out = append(out, a.Name)
	}
	sort.Strings(out)
	return out
}

func (e *Engine) ListSkills() []string {
	out := make([]string, 0, len(e.skills))
	for name := range e.skills {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func taskIOPlan(a project.AgentConfig) TaskIOPlan {
	toPorts := func(ports []project.IOPortConfig, isInput bool) []TypedPortPlan {
		if len(ports) == 0 {
			return nil
		}
		out := make([]TypedPortPlan, 0, len(ports))
		for _, p := range ports {
			name := strings.TrimSpace(p.Name)
			if name == "" {
				continue
			}
			rawKinds := p.Accepts
			if !isInput {
				rawKinds = p.Produces
			}
			kinds := make([]string, 0, len(rawKinds))
			seen := map[string]struct{}{}
			for _, raw := range rawKinds {
				if k, ok := typesys.NormalizeKind(raw); ok {
					ks := k.String()
					if _, ok := seen[ks]; ok {
						continue
					}
					seen[ks] = struct{}{}
					kinds = append(kinds, ks)
				}
			}
			out = append(out, TypedPortPlan{
				Name:            name,
				Kinds:           kinds,
				MergePolicy:     normalizeHandoffMergePolicy(p.MergePolicy),
				ChannelPriority: p.ChannelPriority,
				Required:        p.Required,
				MaxTokens:       p.MaxTokens,
			})
		}
		return out
	}
	return TaskIOPlan{
		Inputs:  toPorts(a.IO.Inputs, true),
		Outputs: toPorts(a.IO.Outputs, false),
	}
}

type publishTaskOutputsResult struct {
	Sent                    int
	Dropped                 int
	Tokens                  int
	SkippedUnconsumedOutput bool
}

type publishTaskOutputCandidate struct {
	Link       TypedHandoffLink
	Value      typesys.TypedValue
	TokensCost int
	Priority   float64
}

func (e *Engine) publishTaskOutputs(bus *ContextBus, task Task, res AgentRunResult) publishTaskOutputsResult {
	out := publishTaskOutputsResult{}
	if bus == nil || res.Error != "" {
		return out
	}
	output := strings.TrimSpace(res.Output)
	if output == "" {
		return out
	}
	if len(task.Outgoing) == 0 {
		if shouldPublishUnconsumedOutput(task.Agent.Metadata) {
			bus.RecordAgentOutput(task.Agent.Name, output)
		} else {
			out.SkippedUnconsumedOutput = true
		}
		return out
	}

	maxChannels := 0
	if v, ok := metadataInt(task.Agent.Metadata, "publish_max_channels"); ok && v > 0 {
		maxChannels = v
	}
	maxTokens := 0
	if v, ok := metadataInt(task.Agent.Metadata, "publish_max_channel_tokens"); ok && v > 0 {
		maxTokens = v
	}

	previewCache := map[string]typesys.TypedValue{}
	cands := make([]publishTaskOutputCandidate, 0, len(task.Outgoing))
	for _, link := range task.Outgoing {
		if link.Kind == "" {
			continue
		}
		cacheKey := link.FromPort + "|" + link.Kind.String() + "|" + strings.TrimSpace(link.Channel)
		tv, ok := previewCache[cacheKey]
		if !ok {
			tv = e.previewAgentOutputTypedValue(task, res, link)
			previewCache[cacheKey] = tv
		}
		if tv.Kind == "" && tv.RefID == "" && strings.TrimSpace(tv.InlineText) == "" {
			continue
		}
		tokenCost := tv.TokensHint
		if tokenCost <= 0 {
			tokenCost = estimateTokens(strings.TrimSpace(tv.InlineText))
			if tokenCost <= 0 {
				tokenCost = estimateTokens(fmt.Sprint(tv.Meta))
			}
			if tokenCost <= 0 {
				tokenCost = 1
			}
		}
		if link.TargetMaxTokens > 0 && tokenCost > link.TargetMaxTokens {
			tokenCost = link.TargetMaxTokens
		}
		cands = append(cands, publishTaskOutputCandidate{
			Link:       link,
			Value:      tv,
			TokensCost: maxInt(tokenCost, 1),
			Priority:   publishChannelPriority(link, tv),
		})
	}
	if len(cands) == 0 {
		return out
	}

	sort.SliceStable(cands, func(i, j int) bool {
		di := cands[i].Priority / float64(maxInt(cands[i].TokensCost, 1))
		dj := cands[j].Priority / float64(maxInt(cands[j].TokensCost, 1))
		if di != dj {
			return di > dj
		}
		if cands[i].Priority != cands[j].Priority {
			return cands[i].Priority > cands[j].Priority
		}
		if cands[i].Link.Required != cands[j].Link.Required {
			return cands[i].Link.Required && !cands[j].Link.Required
		}
		return cands[i].Link.Channel < cands[j].Link.Channel
	})

	usedTokens := 0
	sentChannels := 0
	finalCache := map[string]typesys.TypedValue{}
	for _, c := range cands {
		if maxChannels > 0 && sentChannels >= maxChannels {
			out.Dropped++
			continue
		}
		if maxTokens > 0 && usedTokens+c.TokensCost > maxTokens {
			// Never drop a required channel when nothing has been published yet; publish one to avoid dead input.
			if !(c.Link.Required && sentChannels == 0) {
				out.Dropped++
				continue
			}
		}
		cacheKey := c.Link.FromPort + "|" + c.Link.Kind.String() + "|" + strings.TrimSpace(c.Link.Channel)
		tv, ok := finalCache[cacheKey]
		if !ok {
			tv = e.buildAgentOutputTypedValue(bus, task, res, c.Link)
			finalCache[cacheKey] = tv
		}
		if tv.Kind == "" && tv.RefID == "" && strings.TrimSpace(tv.InlineText) == "" {
			out.Dropped++
			continue
		}
		bus.RecordAgentHandoff(c.Link, tv)
		sentChannels++
		usedTokens += c.TokensCost
	}
	out.Sent = sentChannels
	out.Tokens = usedTokens
	return out
}

func publishChannelPriority(link TypedHandoffLink, tv typesys.TypedValue) float64 {
	kind := tv.Kind
	if kind == "" {
		kind = link.Kind
	}
	score := scoreTypeAndMoment(kind, ContextMomentSynthesis, ContextMomentSynthesis, DefaultContextWeightProfile())
	if link.Required {
		score += 5.0
	}
	if link.TargetMaxTokens > 0 {
		score += 0.4
	}
	if link.ChannelPriority > 0 {
		score += link.ChannelPriority
	}
	if strings.TrimSpace(link.Channel) != "" {
		score += 0.1
	}
	if score <= 0 {
		score = 0.5
	}
	return score
}

func taskIncomingChannels(task Task) []string {
	if len(task.Incoming) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(task.Incoming))
	for _, in := range task.Incoming {
		ch := strings.TrimSpace(in.Channel)
		if ch == "" {
			continue
		}
		if _, ok := seen[ch]; ok {
			continue
		}
		seen[ch] = struct{}{}
		out = append(out, ch)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

type taskCompileChannelStats struct {
	Total    int
	Selected int
	Dropped  int
}

type taskCompileChannelCandidate struct {
	Channel    string
	ToPort     string
	Kind       typesys.Kind
	Required   bool
	Priority   float64
	TokensCost int
}

func selectTaskCompileChannels(task Task, budget ContextBudget, moment ContextMoment) ([]string, taskCompileChannelStats) {
	all := taskIncomingChannels(task)
	stats := taskCompileChannelStats{Total: len(all), Selected: len(all)}
	if len(all) == 0 {
		return nil, stats
	}

	maxChannels := 0
	if v, ok := metadataInt(task.Agent.Metadata, "context_max_channels"); ok && v > 0 {
		maxChannels = v
	}
	maxChannelTokens := 0
	if v, ok := metadataInt(task.Agent.Metadata, "context_max_channel_tokens"); ok && v > 0 {
		maxChannelTokens = v
	}
	if maxChannels <= 0 && maxChannelTokens <= 0 {
		return all, stats
	}

	weights := DefaultContextWeightProfile()
	byChannel := map[string]taskCompileChannelCandidate{}
	for _, link := range task.Incoming {
		ch := strings.TrimSpace(link.Channel)
		if ch == "" {
			continue
		}
		c := taskCompileChannelCandidate{
			Channel:    ch,
			ToPort:     strings.TrimSpace(link.ToPort),
			Kind:       link.Kind,
			Required:   link.Required,
			Priority:   compileChannelPriority(link, moment, weights),
			TokensCost: compileChannelTokenCost(link),
		}
		if prev, ok := byChannel[ch]; ok {
			if c.Priority > prev.Priority {
				prev.Priority = c.Priority
			}
			if c.Required {
				prev.Required = true
			}
			if prev.ToPort == "" && c.ToPort != "" {
				prev.ToPort = c.ToPort
			}
			if prev.TokensCost <= 0 || (c.TokensCost > 0 && c.TokensCost < prev.TokensCost) {
				prev.TokensCost = c.TokensCost
			}
			byChannel[ch] = prev
			continue
		}
		byChannel[ch] = c
	}
	if len(byChannel) == 0 {
		stats.Selected = 0
		stats.Dropped = stats.Total
		return nil, stats
	}

	cands := make([]taskCompileChannelCandidate, 0, len(byChannel))
	for _, c := range byChannel {
		if c.TokensCost <= 0 {
			c.TokensCost = 1
		}
		cands = append(cands, c)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		di := cands[i].Priority / float64(maxInt(cands[i].TokensCost, 1))
		dj := cands[j].Priority / float64(maxInt(cands[j].TokensCost, 1))
		if di != dj {
			return di > dj
		}
		if cands[i].Priority != cands[j].Priority {
			return cands[i].Priority > cands[j].Priority
		}
		if cands[i].Required != cands[j].Required {
			return cands[i].Required && !cands[j].Required
		}
		return cands[i].Channel < cands[j].Channel
	})

	selected := map[string]struct{}{}
	selectedList := make([]string, 0, len(cands))
	usedTokens := 0
	usedCount := 0
	add := func(c taskCompileChannelCandidate) bool {
		if _, ok := selected[c.Channel]; ok {
			return true
		}
		if maxChannels > 0 && usedCount >= maxChannels {
			return false
		}
		if maxChannelTokens > 0 && usedTokens+c.TokensCost > maxChannelTokens {
			return false
		}
		selected[c.Channel] = struct{}{}
		selectedList = append(selectedList, c.Channel)
		usedCount++
		usedTokens += c.TokensCost
		return true
	}

	// First pass: keep at least one channel for each required input port when possible.
	bestRequiredByPort := map[string]taskCompileChannelCandidate{}
	for _, c := range cands {
		if !c.Required || strings.TrimSpace(c.ToPort) == "" {
			continue
		}
		prev, ok := bestRequiredByPort[c.ToPort]
		if !ok || c.Priority > prev.Priority {
			bestRequiredByPort[c.ToPort] = c
		}
	}
	requiredPorts := make([]string, 0, len(bestRequiredByPort))
	for port := range bestRequiredByPort {
		requiredPorts = append(requiredPorts, port)
	}
	sort.Strings(requiredPorts)
	for _, port := range requiredPorts {
		c := bestRequiredByPort[port]
		if add(c) {
			continue
		}
		// If the caps are too tight, allow one required channel through to avoid empty required input.
		if len(selected) == 0 || (maxChannels > 0 && usedCount >= maxChannels) || (maxChannelTokens > 0 && usedTokens+c.TokensCost > maxChannelTokens) {
			selected[c.Channel] = struct{}{}
			selectedList = append(selectedList, c.Channel)
			usedCount++
			usedTokens += c.TokensCost
		}
	}

	for _, c := range cands {
		_ = add(c)
	}

	sort.Strings(selectedList)
	stats.Selected = len(selectedList)
	stats.Dropped = maxInt(stats.Total-stats.Selected, 0)
	return selectedList, stats
}

func compileChannelPriority(link TypedHandoffLink, moment ContextMoment, weights ContextWeightProfile) float64 {
	score := scoreTypeAndMoment(defaultKind(link.Kind, typesys.KindMessageAgent), ContextMomentSynthesis, moment, weights)
	if link.Required {
		score += 4.0
	}
	if link.ChannelPriority > 0 {
		score += link.ChannelPriority
	}
	if link.TargetMaxTokens > 0 {
		score += 0.2
	}
	if score <= 0 {
		score = 0.5
	}
	return score
}

func compileChannelTokenCost(link TypedHandoffLink) int {
	if link.TargetMaxTokens > 0 {
		return maxInt(link.TargetMaxTokens, 1)
	}
	kind := defaultKind(link.Kind, typesys.KindMessageAgent)
	s := kind.String()
	switch {
	case strings.HasPrefix(s, "summary/"), strings.HasPrefix(s, "memory/"), strings.HasPrefix(s, "diagnostic/"):
		return 48
	case strings.HasPrefix(s, "artifact/"), strings.HasPrefix(s, "code/"), strings.HasPrefix(s, "json/"), strings.HasPrefix(s, "data/"):
		return 80
	case strings.HasPrefix(s, "tool/"):
		return 64
	default:
		return 56
	}
}

func shouldPublishUnconsumedOutput(meta map[string]string) bool {
	if meta == nil {
		return false
	}
	raw, ok := meta["publish_unconsumed_output"]
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (e *Engine) previewAgentOutputTypedValue(task Task, res AgentRunResult, link TypedHandoffLink) typesys.TypedValue {
	raw := strings.TrimSpace(res.Output)
	if raw == "" {
		return typesys.TypedValue{}
	}
	kind := link.Kind
	if kind == "" {
		kind = typesys.KindMessageAgent
	}
	summary := trim(summarizeRawByKind(kind, raw), 220)
	if summary == "" {
		summary = trim(raw, 220)
	}
	tokensHint := estimateTokens(summary)

	if shouldInlineHandoff(kind, raw) {
		inline := raw
		if strings.HasPrefix(kind.String(), "summary/") {
			inline = summary
		} else {
			inline = trim(raw, 320)
		}
		return typesys.TypedValue{
			Kind:       kind,
			InlineText: inline,
			Bytes:      len(raw),
			TokensHint: estimateTokens(inline),
		}
	}
	return typesys.TypedValue{
		Kind:       kind,
		InlineText: summary,
		Bytes:      len(raw),
		TokensHint: tokensHint,
	}
}

func (e *Engine) buildAgentOutputTypedValue(bus *ContextBus, task Task, res AgentRunResult, link TypedHandoffLink) typesys.TypedValue {
	raw := strings.TrimSpace(res.Output)
	if raw == "" {
		return typesys.TypedValue{}
	}
	kind := link.Kind
	if kind == "" {
		kind = typesys.KindMessageAgent
	}

	summary := trim(summarizeRawByKind(kind, raw), 220)
	if summary == "" {
		summary = trim(raw, 220)
	}
	tokensHint := estimateTokens(summary)

	if shouldInlineHandoff(kind, raw) {
		inline := raw
		if strings.HasPrefix(kind.String(), "summary/") {
			inline = summary
		} else {
			inline = trim(raw, 320)
		}
		return typesys.TypedValue{
			Kind:       kind,
			InlineText: inline,
			Bytes:      len(raw),
			TokensHint: estimateTokens(inline),
			Meta: map[string]string{
				"from_agent": task.Agent.Name,
				"from_port":  coalesce(strings.TrimSpace(link.FromPort), "output"),
				"summary":    summary,
			},
		}
	}

	if bus == nil {
		return typesys.TypedValue{
			Kind:       kind,
			InlineText: summary,
			Bytes:      len(raw),
			TokensHint: tokensHint,
		}
	}

	art := bus.PutScopedArtifact(
		kind,
		ContextMomentSynthesis,
		task.Agent.Name+"."+coalesce(strings.TrimSpace(link.FromPort), "output"),
		raw,
		summary,
		strings.TrimSpace(link.ToAgent),
		strings.TrimSpace(link.Channel),
	)
	if art.ID == "" {
		return typesys.TypedValue{
			Kind:       kind,
			InlineText: summary,
			Bytes:      len(raw),
			TokensHint: tokensHint,
		}
	}

	meta := map[string]string{
		"from_agent": task.Agent.Name,
		"from_port":  coalesce(strings.TrimSpace(link.FromPort), "output"),
		"hash":       art.Hash,
	}
	if summary != "" {
		meta["summary"] = summary
	}
	return typesys.TypedValue{
		Kind:       kind,
		RefID:      art.ID,
		Bytes:      art.Bytes,
		TokensHint: tokensHint,
		Meta:       meta,
	}
}

func shouldInlineHandoff(kind typesys.Kind, raw string) bool {
	if raw == "" {
		return true
	}
	switch {
	case strings.HasPrefix(kind.String(), "summary/"):
		return true
	case kind == typesys.KindMessageAgent,
		kind == typesys.KindTextPlain,
		kind == typesys.KindTextMarkdown,
		kind == typesys.KindTextPrompt:
		return len(raw) <= 320
	default:
		return false
	}
}
