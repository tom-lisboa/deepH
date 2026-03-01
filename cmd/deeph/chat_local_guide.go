package main

import (
	"fmt"
	"strconv"
	"strings"

	"deeph/internal/commanddoc"
)

func maybeAnswerGuideLocally(meta *chatSessionMeta, userMessage string) (string, bool) {
	if meta == nil || !strings.EqualFold(strings.TrimSpace(meta.AgentSpec), "guide") {
		return "", false
	}
	norm := normalizeChatLookupText(userMessage)
	if !localGuideLooksOperational(norm) {
		return "", false
	}
	if out, ok := localGuideRecipeReply(norm); ok {
		return out, true
	}
	if out, ok := localGuideCommandReply(norm, userMessage); ok {
		return out, true
	}
	return "", false
}

func localGuideLooksOperational(norm string) bool {
	if norm == "" {
		return false
	}
	if containsAny(norm, "deeph", "workspace", "quickstart", "provider", "deepseek", "agent", "crew", "multiverse", "skill", "validate", "trace", "chat", "session", "kit", "backend", "crud", "graphql", "graph ql") {
		return true
	}
	if containsAny(norm, "qual comando", "quais comandos", "como eu", "como faco", "como faço", "como uso", "como criar", "como rodar", "como configuro", "como configurar", "o que eu uso") {
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
			"Para CRUD com base pronta, o melhor caminho no momento e usar o kit multiverse e adaptar o prompt para o seu dominio:",
			[]string{
				"deeph kit add crud-next-multiverse --provider-name deepseek --model deepseek-chat --set-default-provider",
				"deeph crew show crud_fullstack_multiverse",
				"deeph trace --multiverse 0 \"crew:crud_fullstack_multiverse\" " + strconv.Quote("crie um backend CRUD para "+subject),
				"deeph run --multiverse 0 \"crew:crud_fullstack_multiverse\" " + strconv.Quote("crie um backend CRUD para "+subject),
			},
			[]string{
				"Se voce quiser so backend, deixe isso explicito no prompt para o crew nao gastar energia com frontend.",
				"Depois da primeira geracao, refine com um agent dedicado para infra, testes ou banco se precisar.",
			},
		), true
	case containsAny(norm, "backend", "beck end", "back end", "api", "servidor"):
		return formatLocalGuideReply(
			"Hoje nao existe `deeph create backend`. Para criar um backend dentro do seu workspace, o fluxo mais direto e este:",
			[]string{
				"deeph agent create backend_builder",
				"deeph trace backend_builder " + strconv.Quote("crie um backend Go para "+subject),
				"deeph run backend_builder " + strconv.Quote("crie um backend Go para "+subject),
			},
			[]string{
				"Use `trace` antes de `run` quando voce quiser revisar o caminho sem gastar token desnecessario.",
				"Se o backend for CRUD, banco ou containers, descreva isso no prompt em vez de esperar um comando magico do CLI.",
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
