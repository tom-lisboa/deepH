package main

import (
	"fmt"
	"strconv"
	"strings"

	"deeph/internal/commanddoc"
)

func maybeAnswerGuideLocally(workspace string, meta *chatSessionMeta, userMessage string) (string, bool) {
	if meta == nil || !strings.EqualFold(strings.TrimSpace(meta.AgentSpec), "guide") {
		return "", false
	}
	norm := normalizeChatLookupText(userMessage)
	if out, ok := maybeAnswerGuideCodeWorkflow(workspace, norm, userMessage); ok {
		return out, true
	}
	if !localGuideLooksOperational(norm) {
		return "", false
	}
	if out, ok := maybeAnswerGuideOperational(workspace, meta, userMessage); ok {
		return out, true
	}
	if out, ok := localGuideRecipeReply(norm); ok {
		return out, true
	}
	if out, ok := localGuideCommandReply(norm, userMessage); ok {
		return out, true
	}
	return "", false
}

func maybeAnswerGuideCodeWorkflow(workspace, norm, userMessage string) (string, bool) {
	if !localGuideLooksCodeRequest(norm, userMessage) {
		return "", false
	}
	agents := listWorkspaceAgentNames(workspace)
	if agent := guideSuggestedCodeAgent(agents, norm); agent != "" {
		actionSummary := "ler os arquivos necessarios e responder de forma objetiva"
		command := fmt.Sprintf("deeph run --workspace . %s %s", agent, strconv.Quote(strings.TrimSpace(userMessage)))
		switch strings.ToLower(strings.TrimSpace(agent)) {
		case "coder":
			actionSummary = "ler os arquivos necessarios, aplicar uma edicao focada e resumir riscos residuais"
			command = fmt.Sprintf("deeph edit --workspace . %s", strconv.Quote(strings.TrimSpace(userMessage)))
		case "diagnoser":
			actionSummary = "analisar o erro, separar evidencia de hipotese e apontar a correcao minima mais segura"
			command = fmt.Sprintf("deeph diagnose --workspace . %s", strconv.Quote(strings.TrimSpace(userMessage)))
		}
		return formatLocalGuideReply(
			"O `guide` deste chat e focado na operacao do `deepH`. Para analisar ou implementar codigo de verdade no workspace, o caminho certo agora e executar o agent certo com a sua instrucao atual.",
			[]string{
				command,
			},
			[]string{
				fmt.Sprintf("Isso roda o agent `%s` com a sua instrucao atual, sem reabrir outro chat.", agent),
				"Se voce responder `sim` aqui no chat, eu executo esse comando para voce agora.",
				"Esse passo vai " + actionSummary + ".",
				"Se quiser revisar apenas mudancas locais em Git, use tambem `deeph review --workspace .`.",
			},
		), true
	}
	if plan := maybeBuildGuideCapabilityPlan(workspace, &chatSessionMeta{AgentSpec: "guide"}, userMessage); plan != nil {
		return formatGuideCapabilityPlanReply(plan), true
	}
	return "", false
}

func localGuideLooksCodeRequest(norm, raw string) bool {
	if norm == "" {
		return false
	}
	if containsAny(norm, "qual comando", "quais comandos", "como eu", "como faco", "como faço", "como uso", "como rodar", "como configurar", "provider", "deepseek", "workspace", "crew", "session", "chat", "trace", "validate", "agent", "agente") {
		return false
	}
	if containsAny(norm, "analise", "analisar", "analisa", "revise", "review", "explique", "explica", "sugira", "sugerir", "implemente", "implementa", "adicione", "adicionar", "funcao", "função", "function", "arquivo", "file", "codigo", "código", "main.go", "main.py", "panic", "stack trace", "stderr", "erro", "error", "falhou", "build failed", "test failed") {
		return true
	}
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "/") && strings.Contains(raw, ".") {
		return true
	}
	return false
}

func guideSuggestedCodeAgent(agents []string, norm string) string {
	containsAgent := func(candidates ...string) string {
		for _, candidate := range candidates {
			for _, agent := range agents {
				if strings.EqualFold(strings.TrimSpace(agent), candidate) {
					return agent
				}
			}
		}
		return ""
	}
	switch guideDetectCodeIntent(norm) {
	case guideCodeIntentEdit:
		if agent := containsAgent("coder", "builder", "implementer"); agent != "" {
			return agent
		}
		return ""
	case guideCodeIntentDiagnose:
		if agent := containsAgent("diagnoser", "reviewer", "coder"); agent != "" {
			return agent
		}
		return ""
	case guideCodeIntentReview:
		if agent := containsAgent("reviewer", "review_synth", "coder"); agent != "" {
			return agent
		}
	default:
		if agent := containsAgent("coder", "diagnoser", "reviewer", "review_synth"); agent != "" {
			return agent
		}
	}
	return ""
}

type guideCodeIntent string

const (
	guideCodeIntentEdit     guideCodeIntent = "edit"
	guideCodeIntentDiagnose guideCodeIntent = "diagnose"
	guideCodeIntentReview   guideCodeIntent = "review"
	guideCodeIntentAnalyze  guideCodeIntent = "analyze"
)

func guideDetectCodeIntent(norm string) guideCodeIntent {
	editSignals := containsAny(norm,
		"implemente", "implementa", "crie", "adicionar", "adicione", "editar", "edite", "edita",
		"altere", "altera", "corrija", "corrige", "refatore", "refatora", "refactor", "escreva", "write",
		"com implementacao", "com implementação", "implementacao completa", "implementação completa",
	)
	reviewSignals := containsAny(norm,
		"review", "revise", "revisar", "revisao", "revisão", "regressao", "regressão",
		"riscos", "risk", "bug", "bugs", "falha", "falhas", "teste faltando", "testes faltando",
		"gaps de teste", "missing tests", "diff", "mudancas locais", "mudanças locais", "pull request", "pr",
	)
	diagnoseSignals := containsAny(norm,
		"panic", "stack trace", "traceback", "stderr", "exception", "erro", "error", "falhou",
		"build failed", "test failed", "compilation failed", "compilacao falhou", "compilação falhou",
		"nil pointer", "segmentation fault", "segfault", "undefined", "syntax error",
	)
	switch {
	case editSignals:
		return guideCodeIntentEdit
	case diagnoseSignals:
		return guideCodeIntentDiagnose
	case reviewSignals:
		return guideCodeIntentReview
	default:
		return guideCodeIntentAnalyze
	}
}

func localGuideLooksOperational(norm string) bool {
	if norm == "" {
		return false
	}
	if containsAny(norm, "deeph", "workspace", "quickstart", "provider", "deepseek", "agent", "crew", "multiverse", "skill", "validate", "trace", "chat", "session", "kit", "backend", "crud", "graphql", "graph ql", "docker", "compose", "container", "saas", "projeto", "app", "produto", "workflow", "fluxo", "pipeline") {
		return true
	}
	if containsAny(norm, "qual comando", "quais comandos", "como eu", "como faco", "como faço", "como uso", "como criar", "como rodar", "como configuro", "como configurar", "o que eu uso", "proximo passo", "próximo passo", "como comeco", "como começo") {
		return true
	}
	return false
}

func localGuideRecipeReply(norm string) (string, bool) {
	subject := localGuideSubject(norm)

	switch {
	case containsAny(norm, "powershell", "windows") && containsAny(norm, "crew", "multiverse", "universo"):
		return formatLocalGuideReply(
			"No PowerShell, evite `@crew`. O caminho mais seguro e este:",
			[]string{
				"deeph crew list",
				"deeph crew show <nome-da-crew>",
				"deeph run --multiverse 0 'crew:<nome-da-crew>' \"sua tarefa\"",
			},
			[]string{
				"Use `crew:nome` em vez de `@nome` no PowerShell.",
				"Se quiser revisar o plano antes de gastar token, rode `deeph trace --multiverse 0 'crew:<nome-da-crew>' \"sua tarefa\"`.",
			},
		), true
	case containsAny(norm, "docker", "compose", "container", "containers") && containsAny(norm, "crud", "backend", "api", "server", "servidor"):
		return formatLocalGuideReply(
			"Para operar o CRUD localmente sem ficar lembrando `docker compose`, use os comandos do proprio `deepH`:",
			[]string{
				"deeph crud up",
				"deeph crud smoke",
				"deeph crud down",
			},
			[]string{
				"`deeph crud up` procura o compose do workspace e sobe os containers.",
				"`deeph crud smoke` roda o script gerado ou faz um smoke HTTP padrao do CRUD.",
				"Se a URL da API nao for detectada sozinha, passe `--base-url http://127.0.0.1:8080`.",
			},
		), true
	case containsAny(norm, "graphql", "graph ql"):
		return formatLocalGuideReply(
			"Hoje nao existe um comando nativo como `deeph create graphql-backend`. O caminho mais proximo e criar um agent focado e mandar ele gerar o backend:",
			[]string{
				"deeph agent create graphql_backend",
				"deeph trace graphql_backend " + strconv.Quote("crie um backend GraphQL em Go para "+subject),
				"deeph run graphql_backend " + strconv.Quote("crie um backend GraphQL em Go para "+subject),
			},
			[]string{
				"Se esse backend tiver banco e CRUD, diga isso no prompt: Postgres, schema, resolvers, migrations e Docker.",
				"Use `deeph validate` sempre que editar agents ou skills antes de rodar de novo.",
			},
		), true
	case containsAny(norm, "crud") || (containsAny(norm, "postgres", "mysql", "sqlite", "mongo", "mongodb", "banco") && containsAny(norm, "backend", "api", "cadastro")):
		return formatLocalGuideReply(
			"Para CRUD com a UX mais direta no `deepH`, use o fluxo opinativo `deeph crud`:",
			[]string{
				"deeph crud init",
				"deeph crud trace --entity people --fields nome:text,cidade:text",
				"deeph crud run --entity people --fields nome:text,cidade:text",
				"deeph crud up",
				"deeph crud smoke",
			},
			[]string{
				"O fluxo `deeph crud` ja assume os defaults opinativos: backend em Go, frontend em Next.js e banco em Postgres.",
				"Se voce quiser so backend, use `deeph crud run --backend-only --entity people --fields nome:text,cidade:text`.",
				"Depois de gerar o projeto, use `deeph crud up` para subir os containers e `deeph crud down` para derrubar o ambiente.",
				"Depois da primeira geracao, refine o dominio trocando `people` pelos agregados reais de " + subject + ".",
			},
		), true
	case containsAny(norm, "backend", "beck end", "back end", "api", "servidor"):
		return formatLocalGuideReply(
			"Hoje nao existe `deeph create backend`. Se a sua ideia e um backend CRUD com a stack opinativa do produto, o fluxo mais direto e este:",
			[]string{
				"deeph crud init",
				"deeph crud trace --backend-only --entity people --fields nome:text,cidade:text",
				"deeph crud run --backend-only --entity people --fields nome:text,cidade:text",
				"deeph crud up",
				"deeph crud smoke",
			},
			[]string{
				"Esse fluxo ja assume backend em Go, banco Postgres e pode subir com containers.",
				"Se o seu dominio nao for `people`, troque a entidade e os campos para refletir " + subject + ".",
			},
		), true
	case containsAny(norm, "provider", "deepseek", "api key"):
		return formatLocalGuideReply(
			"Para configurar o provider DeepSeek no workspace atual, use:",
			[]string{
				"deeph provider add --name deepseek --model deepseek-chat --set-default --force deepseek",
				"deeph validate",
			},
			[]string{
				"Garanta que `DEEPSEEK_API_KEY` esteja setada no shell antes de rodar agents reais.",
				"Se quiser confirmar a configuracao, rode `deeph provider list`.",
			},
		), true
	}

	return "", false
}

func localGuideCommandReply(norm, userMessage string) (string, bool) {
	if !containsAny(norm, "qual comando", "quais comandos", "como eu", "como faco", "como faço", "como uso", "como rodar", "como listar", "como mostrar", "como validar", "ajuda", "help") {
		return "", false
	}
	docs := chatRelevantCommandDocs(userMessage)
	if len(docs) == 0 {
		return "", false
	}
	if len(docs) > 3 {
		docs = docs[:3]
	}
	cmds := make([]string, 0, len(docs))
	tips := make([]string, 0, 3)
	seenTips := map[string]struct{}{}
	for _, doc := range docs {
		cmd := localGuideBestDocCommand(doc)
		if cmd != "" {
			cmds = append(cmds, cmd)
		}
		for _, tip := range localGuideDocTips(doc) {
			if _, ok := seenTips[tip]; ok {
				continue
			}
			seenTips[tip] = struct{}{}
			tips = append(tips, tip)
		}
	}
	if len(cmds) == 0 {
		return "", false
	}
	intro := "Os comandos mais proximos para isso, no `deepH`, sao estes:"
	if len(cmds) == 1 {
		intro = fmt.Sprintf("O comando certo no `deepH` para isso e `%s`.", cmds[0])
	}
	return formatLocalGuideReply(intro, cmds, tips), true
}

func localGuideBestDocCommand(doc commanddoc.Doc) string {
	if len(doc.Examples) > 0 {
		return doc.Examples[0]
	}
	if len(doc.Usage) > 0 {
		return doc.Usage[0]
	}
	return ""
}

func localGuideDocTips(doc commanddoc.Doc) []string {
	switch commanddoc.NormalizePath(doc.Path) {
	case "validate":
		return []string{"Rode `deeph validate` sempre que editar `deeph.yaml`, `agents/*.yaml`, `skills/*.yaml` ou `crews/*.yaml`."}
	case "trace":
		return []string{"Use `deeph trace` antes de `run` quando quiser revisar stages, channels e handoffs sem gastar execucao completa."}
	case "run":
		return []string{"Se a tarefa envolver crew com universos, combine com `--multiverse 0`."}
	case "provider add":
		return []string{"Depois de configurar provider, valide com `deeph validate`."}
	case "kit add":
		return []string{"Depois de instalar um kit, confira o que entrou com `deeph crew show` ou abrindo os arquivos em `agents/` e `crews/`."}
	case "crew show":
		return []string{"No PowerShell, prefira `crew:nome` em vez de `@nome` para executar a crew."}
	case "chat":
		return []string{"Dentro do chat, use `/trace` e `/history` para inspecionar o contexto sem sair da sessao."}
	}
	return nil
}

func localGuideSubject(norm string) string {
	switch {
	case strings.Contains(norm, "futebol"):
		return "seu projeto de futebol"
	case strings.Contains(norm, "ecommerce"):
		return "seu projeto de ecommerce"
	case strings.Contains(norm, "crm"):
		return "seu projeto de CRM"
	default:
		return "seu projeto"
	}
}

func containsAny(s string, items ...string) bool {
	for _, item := range items {
		if strings.Contains(s, normalizeChatLookupText(item)) {
			return true
		}
	}
	return false
}

func formatLocalGuideReply(intro string, commands, tips []string) string {
	lines := make([]string, 0, len(commands)+len(tips)+10)
	if strings.TrimSpace(intro) != "" {
		lines = append(lines, strings.TrimSpace(intro), "")
	}
	if len(commands) > 0 {
		lines = append(lines, "Comando agora:")
		lines = append(lines, "```bash")
		for _, cmd := range commands {
			lines = append(lines, cmd)
		}
		lines = append(lines, "```")
	}
	if len(tips) > 0 {
		lines = append(lines, "", "Para aproveitar melhor o deepH agora:")
		for _, tip := range tips {
			lines = append(lines, "- "+tip)
		}
	}
	return strings.Join(lines, "\n")
}
