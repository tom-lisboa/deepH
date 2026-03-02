package main

import "deeph/internal/runtime"

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

	if len(line) > 0 && line[0] == '/' {
		if meta != nil {
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
			meta.PendingExec = derivePendingExecFromGuideText(workspace, localText)
			if meta.PendingExec != nil {
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
		meta.PendingExec = nil
	}
	return chatRoute{Kind: chatRouteLLM}, nil
}
