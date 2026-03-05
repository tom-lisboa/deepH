package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"deeph/internal/runtime"
)

type chatRouteKind string

const (
	chatRouteHandled chatRouteKind = "handled"
	chatRouteLLM     chatRouteKind = "llm"
)

type chatRoute struct {
	Kind    chatRouteKind
	Replies []chatReply
	Done    bool
}

// routeChatTurn handles deterministic chat paths before the LLM/runtime path.
func routeChatTurn(workspace string, meta *chatSessionMeta, entries []chatSessionEntry, line string, plan runtime.ExecutionPlan, tasks []runtime.Task, sinkIdxs []int) (chatRoute, error) {
	if meta != nil && meta.PendingPlan != nil {
		handled, replies, err := maybeHandlePendingPlanReply(meta, line)
		if err != nil {
			return chatRoute{}, err
		}
		if handled {
			return chatRoute{
				Kind:    chatRouteHandled,
				Replies: replies,
			}, nil
		}
	}

	if meta != nil && meta.PendingExec != nil {
		handled, replies, err := maybeHandlePendingExecReply(meta, line)
		if err != nil {
			return chatRoute{}, err
		}
		if handled {
			return chatRoute{
				Kind:    chatRouteHandled,
				Replies: replies,
			}, nil
		}
	}

	if handled, replies, err := maybeHandleDirectDeephCommand(workspace, meta, line); handled {
		if err != nil {
			return chatRoute{}, err
		}
		return chatRoute{
			Kind:    chatRouteHandled,
			Replies: replies,
		}, nil
	}

	if len(line) > 0 && line[0] == '/' {
		if shouldTreatSlashLikePathInput(line) {
			if meta != nil {
				meta.PendingPlan = nil
				meta.PendingExec = nil
			}
			return chatRoute{Kind: chatRouteLLM}, nil
		}
		if meta != nil {
			meta.PendingPlan = nil
			meta.PendingExec = nil
		}
		done, err := handleChatSlashCommand(line, workspace, meta, entries, plan, tasks, sinkIdxs)
		return chatRoute{
			Kind: chatRouteHandled,
			Done: done,
		}, err
	}

	if localText, ok := maybeAnswerGuideLocally(workspace, meta, line); ok {
		if meta != nil {
			meta.PendingPlan = maybeBuildGuideCapabilityPlan(workspace, meta, line)
			if meta.PendingPlan != nil {
				meta.PendingExec = nil
			} else {
				meta.PendingExec = derivePendingExecFromGuideText(workspace, localText)
			}
			if meta.PendingPlan == nil && meta.PendingExec != nil {
				localText = appendGuideExecCallToAction(localText)
			}
		}
		agent := ""
		if meta != nil {
			agent = meta.AgentSpec
		}
		return chatRoute{
			Kind: chatRouteHandled,
			Replies: []chatReply{{
				Agent: agent,
				Text:  localText,
			}},
		}, nil
	}

	if meta != nil {
		meta.PendingPlan = nil
		meta.PendingExec = nil
	}
	return chatRoute{Kind: chatRouteLLM}, nil
}

func maybeHandleDirectDeephCommand(workspace string, meta *chatSessionMeta, line string) (bool, []chatReply, error) {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	if trimmed == "" || (!strings.HasPrefix(lower, "deeph ") && lower != "deeph") {
		return false, nil, nil
	}

	req, err := parseChatExecLine(trimmed, workspace)
	if err != nil {
		return true, []chatReply{{
			Agent: chatReplyAgent(meta),
			Error: err.Error(),
		}}, nil
	}
	if meta != nil {
		meta.PendingPlan = nil
	}
	if chatExecRequiresConfirm(req.Path) && !req.Confirmed {
		if meta != nil {
			copyReq := req
			copyReq.Args = append([]string{}, req.Args...)
			meta.PendingExec = &copyReq
		}
		return true, []chatReply{{
			Agent: chatReplyAgent(meta),
			Text:  fmt.Sprintf("Comando detectado: `%s`.\nResponda `sim` para confirmar ou `nao` para cancelar.", req.Display),
		}}, nil
	}
	if meta != nil {
		meta.PendingExec = nil
	}

	receipt, execErr := executeChatExecRequest(req)
	recordLastCommandReceipt(meta, receipt)
	if execErr != nil {
		return true, []chatReply{{
			Agent: chatReplyAgent(meta),
			Error: execErr.Error(),
		}}, nil
	}
	return true, []chatReply{{
		Agent: chatReplyAgent(meta),
		Text:  receipt.Summary,
	}}, nil
}

func chatReplyAgent(meta *chatSessionMeta) string {
	if meta == nil {
		return ""
	}
	return meta.AgentSpec
}

func shouldTreatSlashLikePathInput(line string) bool {
	token := strings.TrimSpace(line)
	if token == "" || token[0] != '/' {
		return false
	}
	if i := strings.IndexAny(token, " \t"); i >= 0 {
		token = token[:i]
	}
	switch token {
	case "/help", "/mode", "/status", "/history", "/trace", "/exec", "/exit", "/quit":
		return false
	}
	if !filepath.IsAbs(token) {
		return false
	}
	if _, err := os.Stat(token); err == nil {
		return true
	}
	// If it looks like a deep absolute path, prefer treating it as user content
	// instead of returning an unknown slash command error.
	return strings.Contains(strings.TrimPrefix(token, "/"), "/")
}
