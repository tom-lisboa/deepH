package main

import (
	"fmt"
	"strings"
	"time"

	"deeph/internal/runtime"
)

func printTracePlanText(workspace string, plan runtime.ExecutionPlan) {
	fmt.Printf("Trace (%s)\n", workspace)
	fmt.Printf("  created_at: %s\n", plan.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  parallel: %v\n", plan.Parallel)
	fmt.Println("  scheduler: dag_channels (dependency-driven, selective stage wait)")
	if strings.TrimSpace(plan.Spec) != "" {
		fmt.Printf("  spec: %q\n", plan.Spec)
	}
	fmt.Printf("  input: %q\n", plan.Input)
	if len(plan.Stages) > 1 {
		for _, s := range plan.Stages {
			fmt.Printf("  stage[%d]: agents=%v\n", s.Index, s.Agents)
		}
		if len(plan.Handoffs) > 0 {
			fmt.Println("  handoffs:")
			for _, h := range plan.Handoffs {
				req := ""
				if h.Required {
					req = " required=true"
				}
				ch := ""
				if strings.TrimSpace(h.Channel) != "" {
					ch = " channel=" + h.Channel
				}
				merge := ""
				if strings.TrimSpace(h.MergePolicy) != "" && h.MergePolicy != "auto" {
					merge = " merge=" + h.MergePolicy
				}
				chPrio := ""
				if h.ChannelPriority > 0 {
					chPrio = fmt.Sprintf(" channel_priority=%.2f", h.ChannelPriority)
				}
				maxTok := ""
				if h.TargetMaxTokens > 0 {
					maxTok = fmt.Sprintf(" max_tokens=%d", h.TargetMaxTokens)
				}
				fmt.Printf("    - %s.%s -> %s.%s kind=%s%s%s%s%s%s\n", h.FromAgent, h.FromPort, h.ToAgent, h.ToPort, h.Kind, ch, merge, chPrio, maxTok, req)
			}
		}
	}
	for i, t := range plan.Tasks {
		fmt.Printf("  task[%d]: stage=%d agent=%s provider=%s(%s) model=%s skills=%v startup_calls=%d context_budget=%dt context_moment=%s\n", i, t.StageIndex, t.Agent, t.Provider, t.ProviderType, t.Model, t.Skills, t.StartupCalls, t.ContextBudget, t.ContextMoment)
		if len(t.DependsOn) > 0 {
			fmt.Printf("           depends_on=%v\n", t.DependsOn)
		}
		if len(t.IO.Inputs) > 0 || len(t.IO.Outputs) > 0 {
			fmt.Printf("           io.inputs=%d io.outputs=%d\n", len(t.IO.Inputs), len(t.IO.Outputs))
			for _, in := range t.IO.Inputs {
				if (in.MergePolicy != "" && in.MergePolicy != "auto") || in.ChannelPriority > 0 || in.MaxTokens > 0 {
					fmt.Printf("           input[%s] kinds=%v", in.Name, in.Kinds)
					if in.MergePolicy != "" && in.MergePolicy != "auto" {
						fmt.Printf(" merge=%s", in.MergePolicy)
					}
					if in.ChannelPriority > 0 {
						fmt.Printf(" channel_priority=%.2f", in.ChannelPriority)
					}
					if in.MaxTokens > 0 {
						fmt.Printf(" max_tokens=%d", in.MaxTokens)
					}
					fmt.Println()
				}
			}
		}
		if t.ProviderType == "deepseek" && len(t.Skills) > 0 && t.ContextMoment == "tool_loop" {
			fmt.Println("           tool_loop=enabled (deepseek chat completions -> skills)")
		}
		if t.AgentFile != "" {
			fmt.Printf("           source=%s\n", t.AgentFile)
		}
	}
}

func printRunReportText(plan runtime.ExecutionPlan, report runtime.ExecutionReport) {
	fmt.Printf("Run started=%s parallel=%v scheduler=dag_channels input=%q\n", report.StartedAt.Format(time.RFC3339), report.Parallel, report.Input)
	for _, r := range report.Results {
		fmt.Printf("\n[%s] stage=%d provider=%s(%s) model=%s duration=%s context=%d/%dt dropped=%d version=%d moment=%s\n", r.Agent, r.StageIndex, r.Provider, r.ProviderType, r.Model, r.Duration.Round(time.Millisecond), r.ContextTokens, r.ContextBudget, r.ContextDropped, r.ContextVersion, r.ContextMoment)
		if len(r.DependsOn) > 0 {
			fmt.Printf("  depends_on=%v\n", r.DependsOn)
		}
		if r.ContextChannelsTotal > 0 {
			fmt.Printf("  context_channels=%d/%d dropped=%d\n", r.ContextChannelsUsed, r.ContextChannelsTotal, r.ContextChannelsDropped)
		}
		if r.ToolCacheHits > 0 || r.ToolCacheMisses > 0 {
			fmt.Printf("  tool_cache hits=%d misses=%d\n", r.ToolCacheHits, r.ToolCacheMisses)
		}
		if r.ToolBudgetCallsLimit > 0 || r.ToolBudgetExecMSLimit > 0 {
			callLimit := "unlimited"
			if r.ToolBudgetCallsLimit > 0 {
				callLimit = fmt.Sprintf("%d", r.ToolBudgetCallsLimit)
			}
			execLimit := "unlimited"
			if r.ToolBudgetExecMSLimit > 0 {
				execLimit = fmt.Sprintf("%dms", r.ToolBudgetExecMSLimit)
			}
			fmt.Printf("  tool_budget calls=%d/%s exec_ms=%d/%s\n", r.ToolBudgetCallsUsed, callLimit, r.ToolBudgetExecMSUsed, execLimit)
		}
		if r.StageToolBudgetCallsLimit > 0 || r.StageToolBudgetExecMSLimit > 0 {
			callLimit := "unlimited"
			if r.StageToolBudgetCallsLimit > 0 {
				callLimit = fmt.Sprintf("%d", r.StageToolBudgetCallsLimit)
			}
			execLimit := "unlimited"
			if r.StageToolBudgetExecMSLimit > 0 {
				execLimit = fmt.Sprintf("%dms", r.StageToolBudgetExecMSLimit)
			}
			fmt.Printf("  stage_tool_budget calls=%d/%s exec_ms=%d/%s\n", r.StageToolBudgetCallsUsed, callLimit, r.StageToolBudgetExecMSUsed, execLimit)
		}
		if r.SentHandoffs > 0 {
			fmt.Printf("  handoffs_sent=%d\n", r.SentHandoffs)
		}
		if r.HandoffTokens > 0 || r.DroppedHandoffs > 0 {
			fmt.Printf("  handoff_publish tokens=%d dropped=%d\n", r.HandoffTokens, r.DroppedHandoffs)
		}
		if r.SkippedOutputPublish {
			fmt.Println("  handoff_publish skipped_unconsumed_output=true")
		}
		if len(r.StartupCalls) > 0 {
			for _, c := range r.StartupCalls {
				if c.Error != "" {
					fmt.Printf("  startup_call %s failed (%s): %s\n", c.Skill, c.Duration.Round(time.Millisecond), c.Error)
				} else {
					fmt.Printf("  startup_call %s ok (%s)\n", c.Skill, c.Duration.Round(time.Millisecond))
				}
			}
		}
		if len(r.ToolCalls) > 0 {
			for _, c := range r.ToolCalls {
				if c.Error != "" {
					fmt.Printf("  tool_call %s", c.Skill)
					if c.CallID != "" {
						fmt.Printf(" id=%s", c.CallID)
					}
					fmt.Printf(" failed (%s): %s\n", c.Duration.Round(time.Millisecond), c.Error)
					continue
				}
				fmt.Printf("  tool_call %s", c.Skill)
				if c.CallID != "" {
					fmt.Printf(" id=%s", c.CallID)
				}
				if len(c.Args) > 0 {
					fmt.Printf(" args=%v", c.Args)
				}
				if c.Cached {
					fmt.Printf(" cached=true")
				}
				if c.Cacheable && !c.Cached {
					fmt.Printf(" cacheable=true")
				}
				fmt.Printf(" ok (%s)\n", c.Duration.Round(time.Millisecond))
			}
		}
		if r.Error != "" {
			fmt.Printf("  error: %s\n", r.Error)
			continue
		}
		fmt.Println(r.Output)
	}
	fmt.Printf("\nFinished in %s\n", report.EndedAt.Sub(report.StartedAt).Round(time.Millisecond))
}
