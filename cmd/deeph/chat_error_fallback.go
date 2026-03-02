package main

import "strings"

func maybeBuildChatErrorFallback(workspace string, meta *chatSessionMeta, userMessage string, err error) []chatReply {
	if err == nil || meta == nil || !strings.EqualFold(strings.TrimSpace(meta.AgentSpec), "guide") {
		return nil
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return nil
	}
	if strings.Contains(strings.ToLower(msg), "tool budget exceeded") {
		norm := normalizeChatLookupText(userMessage)
		if out, ok := maybeAnswerGuideCodeWorkflow(workspace, norm, userMessage); ok {
			return []chatReply{{Agent: meta.AgentSpec, Text: out}}
		}
		text := formatLocalGuideReply(
			"O `guide` nao conseguiu confirmar a resposta porque estourou o budget de ferramentas desta rodada.",
			[]string{
				"deeph command list",
			},
			[]string{
				"Se a pergunta for sobre codigo, use `deeph review --workspace .` ou um agent de codigo com `deeph run --workspace . <agent> \"...\"`.",
				"Se a pergunta for sobre CLI, reformule com o comando ou fluxo desejado para reduzir a necessidade de lookup.",
			},
		)
		return []chatReply{{Agent: meta.AgentSpec, Text: text}}
	}
	return nil
}
