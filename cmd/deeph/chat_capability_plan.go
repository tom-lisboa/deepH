package main

import (
	"strconv"
	"strings"
)

type chatPendingPlan struct {
	Kind            string         `json:"kind,omitempty"`
	Summary         string         `json:"summary,omitempty"`
	Commands        []deephCommand `json:"commands,omitempty"`
	Followup        *deephCommand  `json:"followup,omitempty"`
	FollowupSummary string         `json:"followup_summary,omitempty"`
}

func cloneChatPendingPlan(plan *chatPendingPlan) *chatPendingPlan {
	if plan == nil {
		return nil
	}
	cp := *plan
	if len(plan.Commands) > 0 {
		cp.Commands = make([]deephCommand, len(plan.Commands))
		for i := range plan.Commands {
			cp.Commands[i] = cloneDeephCommand(plan.Commands[i])
		}
	}
	if plan.Followup != nil {
		followup := cloneDeephCommand(*plan.Followup)
		cp.Followup = &followup
	}
	return &cp
}

func cloneDeephCommand(cmd deephCommand) deephCommand {
	cp := cmd
	cp.Args = append([]string{}, cmd.Args...)
	return cp
}

func maybeBuildGuideCapabilityPlan(workspace string, meta *chatSessionMeta, userMessage string) *chatPendingPlan {
	if meta == nil || !strings.EqualFold(strings.TrimSpace(meta.AgentSpec), "guide") {
		return nil
	}
	norm := normalizeChatLookupText(userMessage)
	if !localGuideLooksCodeRequest(norm, userMessage) {
		return nil
	}
	agents := listWorkspaceAgentNames(workspace)
	if guideSuggestedCodeAgent(agents, norm) != "" {
		return nil
	}

	bootstrap, err := parseChatExecLine(`/exec deeph quickstart --workspace .`, workspace)
	if err != nil {
		return nil
	}
	followup := buildGuideCapabilityFollowup(workspace, norm, userMessage)
	if followup == nil {
		return nil
	}

	summary := "preparar o workspace com o pack minimo de codigo e review"
	if strings.EqualFold(followup.Path, "run") {
		summary += " e depois propor a execucao do agent certo"
	}
	return &chatPendingPlan{
		Kind:            "bootstrap_code_capabilities",
		Summary:         summary,
		Commands:        []deephCommand{bootstrap},
		Followup:        followup,
		FollowupSummary: guideCapabilityFollowupSummary(norm, followup),
	}
}

func buildGuideCapabilityFollowup(workspace, norm, userMessage string) *deephCommand {
	agent := guidePlannedCodeAgent(norm)
	commandLine := `/exec deeph run --workspace . ` + agent + ` ` + strconv.Quote(strings.TrimSpace(userMessage))
	if agent == "coder" {
		commandLine = `/exec deeph edit --workspace . ` + strconv.Quote(strings.TrimSpace(userMessage))
	}
	if agent == "diagnoser" {
		commandLine = `/exec deeph diagnose --workspace . ` + strconv.Quote(strings.TrimSpace(userMessage))
	}
	cmd, err := parseChatExecLine(commandLine, workspace)
	if err != nil {
		return nil
	}
	return &cmd
}

func guidePlannedCodeAgent(norm string) string {
	switch guideDetectCodeIntent(norm) {
	case guideCodeIntentDiagnose:
		return "diagnoser"
	case guideCodeIntentReview:
		return "reviewer"
	default:
		return "coder"
	}
}

func guideCapabilityFollowupSummary(norm string, cmd *deephCommand) string {
	if cmd == nil {
		return "executar o proximo passo proposto"
	}
	switch {
	case strings.Contains(cmd.Display, " diagnose "):
		return "rodar o `diagnoser` com a sua instrucao atual, cruzando o erro com os arquivos mais provaveis do workspace"
	case strings.Contains(cmd.Display, " reviewer "):
		return "rodar o `reviewer` com a sua instrucao atual, lendo os arquivos necessarios para apontar riscos, regressao e gaps de teste"
	default:
		return "rodar o `coder` com a sua instrucao atual, lendo so os arquivos necessarios e aplicando uma edicao focada"
	}
}

func formatGuideCapabilityPlanReply(plan *chatPendingPlan) string {
	if plan == nil || len(plan.Commands) == 0 {
		return ""
	}
	lines := []string{
		"Seu workspace ainda nao tem o pack minimo para editar ou revisar codigo com o `deepH`. Posso preparar isso agora sem sair do chat.",
		"",
		"Isso vai instalar ou completar estas capacidades:",
		"- `coder`: editar codigo com leitura por faixa e escrita segura",
		"- `diagnoser`: analisar erros, panics e saidas falhas com escopo compacto do workspace",
		"- `reviewer`: revisar codigo e apontar riscos concretos",
		"- `review_synth` + `reviewflow`: consolidar revisoes diff-aware",
		"",
		"Comando agora:",
		"```bash",
	}
	for _, cmd := range plan.Commands {
		lines = append(lines, cmd.Display)
	}
	lines = append(lines, "```")
	if plan.Followup != nil {
		lines = append(lines,
			"",
			"Depois disso, eu vou pedir sua confirmacao para este proximo passo:",
			"```bash",
			plan.Followup.Display,
			"```",
		)
	}
	if strings.TrimSpace(plan.FollowupSummary) != "" {
		lines = append(lines,
			"",
			"Para aproveitar melhor o deepH agora:",
			"- Isso vai "+plan.FollowupSummary+".",
		)
	}
	lines = append(lines, "", "Se quiser, responda `sim` e eu executo esse plano aqui no chat.")
	return strings.Join(lines, "\n")
}

func maybeHandlePendingPlanReply(meta *chatSessionMeta, line string) (bool, []chatReply, error) {
	if meta == nil || meta.PendingPlan == nil {
		return false, nil, nil
	}
	switch {
	case chatLooksAffirmative(line):
		plan := cloneChatPendingPlan(meta.PendingPlan)
		meta.PendingPlan = nil
		var last deephCommandReceipt
		for _, cmd := range plan.Commands {
			req := cloneDeephCommand(cmd)
			req.Confirmed = true
			receipt, err := executeChatExecRequest(req)
			last = receipt
			recordLastCommandReceipt(meta, receipt)
			if err != nil {
				return true, []chatReply{{Agent: meta.AgentSpec, Error: err.Error()}}, nil
			}
		}
		if plan.Followup != nil {
			followup := cloneDeephCommand(*plan.Followup)
			meta.PendingExec = &followup
			return true, []chatReply{{
				Agent: meta.AgentSpec,
				Text:  formatGuideCapabilityPlanSuccessReply(&last, &followup, plan.FollowupSummary),
			}}, nil
		}
		text := "Executei o plano proposto."
		if strings.TrimSpace(last.Summary) != "" {
			text = last.Summary
		}
		return true, []chatReply{{Agent: meta.AgentSpec, Text: text}}, nil
	case chatLooksNegative(line):
		meta.PendingPlan = nil
		return true, []chatReply{{Agent: meta.AgentSpec, Text: "Nao executei o plano pendente."}}, nil
	default:
		meta.PendingPlan = nil
		return false, nil, nil
	}
}

func formatGuideCapabilityPlanSuccessReply(receipt *deephCommandReceipt, followup *deephCommand, summary string) string {
	lines := []string{"Preparei o workspace para codigo e review."}
	if receipt != nil && strings.TrimSpace(receipt.Summary) != "" {
		lines[0] = receipt.Summary
	}
	if followup != nil && strings.TrimSpace(followup.Display) != "" {
		lines = append(lines,
			"",
			"Agora posso executar este proximo passo:",
			"```bash",
			followup.Display,
			"```",
		)
	}
	if strings.TrimSpace(summary) != "" {
		lines = append(lines, "", "Isso vai "+summary+".")
	}
	lines = append(lines, "", "Se quiser, responda `sim` e eu executo esse comando aqui no chat.")
	return strings.Join(lines, "\n")
}
