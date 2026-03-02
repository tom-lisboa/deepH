package main

import (
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"

	"deeph/internal/project"
)

type guideOperatorIntent string

const (
	guideOperatorUnknown guideOperatorIntent = ""
	guideOperatorCRUD    guideOperatorIntent = "crud"
	guideOperatorUp      guideOperatorIntent = "up"
	guideOperatorSmoke   guideOperatorIntent = "smoke"
	guideOperatorDown    guideOperatorIntent = "down"
	guideOperatorAgent   guideOperatorIntent = "agent"
	guideOperatorFlow    guideOperatorIntent = "flow"
)

type guideWorkspaceState struct {
	Workspace       string
	HasRootConfig   bool
	HasCRUDProfile  bool
	CRUDConfig      crudWorkspaceConfig
	ComposeFile     string
	BaseURL         string
	SmokeScript     string
	DockerAvailable bool
	DockerLabel     string
	Agents          []string
	DefaultProvider string
	LastCommand     string
	CoachNext       string
	CoachConfidence int
}

type guideProbeResult struct {
	Kind       string
	Config     crudWorkspaceConfig
	OK         bool
	Text       string
	Text2      string
	Items      []string
	Confidence int
}

type guideOperatorReply struct {
	Intro    string
	Commands []string
	What     []string
	Next     []string
	Context  []string
	Notes    []string
}

func maybeAnswerGuideOperational(workspace string, meta *chatSessionMeta, userMessage string) (string, bool) {
	if meta == nil || !strings.EqualFold(strings.TrimSpace(meta.AgentSpec), "guide") {
		return "", false
	}
	norm := normalizeChatLookupText(userMessage)
	intent := detectGuideOperatorIntent(norm)
	if intent == guideOperatorUnknown {
		return "", false
	}
	state := probeGuideWorkspaceState(workspace)
	reply, ok := buildGuideOperatorReply(norm, state, intent)
	if !ok {
		return "", false
	}
	return formatGuideOperatorReply(reply), true
}

func detectGuideOperatorIntent(norm string) guideOperatorIntent {
	switch {
	case containsAny(norm, "derrubar", "parar", "desligar", "down", "stop") && containsAny(norm, "crud", "docker", "compose", "container", "api", "backend", "server", "servidor"):
		return guideOperatorDown
	case containsAny(norm, "smoke", "testar", "teste", "validar", "validacao", "validação", "health", "rotas", "rota") && containsAny(norm, "crud", "api", "backend", "docker", "compose", "container", "server", "servidor"):
		return guideOperatorSmoke
	case containsAny(norm, "subir", "levantar", "rodar", "iniciar", "start", "up") && containsAny(norm, "crud", "docker", "compose", "container", "api", "backend", "server", "servidor"):
		return guideOperatorUp
	case containsAny(norm, "agent", "agente") && containsAny(norm, "criar", "cria", "create", "novo", "new", "montar", "gerar"):
		return guideOperatorAgent
	case containsAny(norm, "workflow", "fluxo", "crew", "pipeline", "multiverse", "universo") && containsAny(norm, "criar", "cria", "create", "montar", "fazer", "comecar", "começar", "iniciar"):
		return guideOperatorFlow
	case containsAny(norm, "crud", "cadastro"):
		return guideOperatorCRUD
	case containsAny(norm, "backend", "beck end", "back end", "api", "servidor"):
		return guideOperatorCRUD
	case containsAny(norm, "saas", "app", "projeto", "produto") && containsAny(norm, "como", "comeco", "comeco", "comecar", "começar", "criar", "iniciar", "passo", "agora"):
		return guideOperatorCRUD
	default:
		return guideOperatorUnknown
	}
}

func probeGuideWorkspaceState(workspace string) guideWorkspaceState {
	workspace = strings.TrimSpace(workspace)
	state := guideWorkspaceState{
		Workspace:     workspace,
		HasRootConfig: workspaceHasRootConfig(workspace),
	}
	ch := make(chan guideProbeResult, 5)
	pending := 0

	pending++
	go func() {
		cfg, ok, err := loadCRUDWorkspaceConfig(workspace)
		if err != nil {
			ch <- guideProbeResult{Kind: "crud"}
			return
		}
		ch <- guideProbeResult{Kind: "crud", Config: cfg, OK: ok}
	}()

	pending++
	go func() {
		composePath, err := findCRUDComposeFile(workspace)
		if err != nil {
			ch <- guideProbeResult{Kind: "compose"}
			return
		}
		ch <- guideProbeResult{
			Kind:  "compose",
			OK:    true,
			Text:  composePath,
			Text2: detectCRUDBaseURLFromComposeFile(composePath),
		}
	}()

	pending++
	go func() {
		scriptPath, ok := findCRUDSmokeScript(workspace, "", goruntime.GOOS)
		ch <- guideProbeResult{Kind: "smoke", OK: ok, Text: scriptPath}
	}()

	pending++
	go func() {
		rt, err := resolveCRUDComposeRuntime()
		if err != nil {
			ch <- guideProbeResult{Kind: "docker"}
			return
		}
		ch <- guideProbeResult{Kind: "docker", OK: true, Text: rt.Label}
	}()

	pending++
	go func() {
		agents := listWorkspaceAgentNames(workspace)
		out := guideProbeResult{Kind: "agents", Items: agents}
		if state.HasRootConfig {
			if p, err := project.Load(workspace); err == nil {
				out.OK = true
				out.Text = strings.TrimSpace(p.Root.DefaultProvider)
				if len(p.Agents) > 0 {
					names := make([]string, 0, len(p.Agents))
					for _, a := range p.Agents {
						if name := strings.TrimSpace(a.Name); name != "" {
							names = append(names, name)
						}
					}
					sort.Strings(names)
					out.Items = names
				}
			}
		}
		ch <- out
	}()

	pending++
	go func() {
		st, err := loadCoachState(workspace)
		if err != nil || st == nil {
			ch <- guideProbeResult{Kind: "coach"}
			return
		}
		next, conf, count, total := coachTopNextCommand(st, st.LastCommand)
		out := guideProbeResult{
			Kind: "coach",
			Text: strings.TrimSpace(st.LastCommand),
		}
		if next != "" && total >= 3 && count >= 2 && conf >= 65 {
			out.OK = true
			out.Text2 = next
			out.Confidence = conf
		}
		ch <- out
	}()

	for i := 0; i < pending; i++ {
		res := <-ch
		switch res.Kind {
		case "crud":
			state.HasCRUDProfile = res.OK
			if res.OK {
				state.CRUDConfig = res.Config
			}
		case "compose":
			if res.OK {
				state.ComposeFile = res.Text
				state.BaseURL = res.Text2
			}
		case "smoke":
			if res.OK {
				state.SmokeScript = res.Text
			}
		case "docker":
			state.DockerAvailable = res.OK
			state.DockerLabel = res.Text
		case "agents":
			state.Agents = res.Items
			if res.OK {
				state.DefaultProvider = res.Text
			}
		case "coach":
			state.LastCommand = res.Text
			if res.OK {
				state.CoachNext = res.Text2
				state.CoachConfidence = res.Confidence
			}
		}
	}
	return state
}

func buildGuideOperatorReply(norm string, state guideWorkspaceState, intent guideOperatorIntent) (guideOperatorReply, bool) {
	switch intent {
	case guideOperatorUp:
		return buildGuideOperatorUpReply(state), true
	case guideOperatorSmoke:
		return buildGuideOperatorSmokeReply(state), true
	case guideOperatorDown:
		return buildGuideOperatorDownReply(state), true
	case guideOperatorCRUD:
		return buildGuideOperatorWorkflowReply(norm, state), true
	case guideOperatorAgent:
		return buildGuideOperatorAgentReply(norm, state), true
	case guideOperatorFlow:
		return buildGuideOperatorFlowReply(state), true
	default:
		return guideOperatorReply{}, false
	}
}

func buildGuideOperatorWorkflowReply(norm string, state guideWorkspaceState) guideOperatorReply {
	subject := localGuideSubject(norm)
	if !state.HasCRUDProfile {
		cmd := guideCRUDInitCommand(norm)
		return guideOperatorReply{
			Intro: "O caminho mais direto para transformar isso em um CRUD operavel dentro do `deepH` e iniciar o fluxo opinativo agora.",
			Commands: []string{
				cmd,
			},
			What: []string{
				"o wizard vai perguntar modo, banco, entidade e campos",
				"o perfil do CRUD sera salvo em `.deeph/crud.json`",
				"o fluxo vai assumir backend em Go e, se voce quiser fullstack, frontend em Next.js",
			},
			Next: []string{
				guideCRUDRunCommand(norm, state),
			},
			Context: buildGuideOperatorContext(state),
			Notes: []string{
				"Se o foco agora for so API, deixe o modo em `backend-only`.",
				"Depois da geracao, use `deeph crud up` e `deeph crud smoke` para subir e validar " + subject + ".",
			},
		}
	}

	if strings.TrimSpace(state.ComposeFile) == "" {
		runCmd := guideCRUDRunCommand(norm, state)
		return guideOperatorReply{
			Intro: "O workspace ja tem perfil de CRUD salvo. O proximo passo com mais acao agora e gerar o projeto.",
			Commands: []string{
				runCmd,
			},
			What: []string{
				"o `deepH` vai escolher o crew certo a partir do perfil salvo",
				"vai gerar rotas CRUD, backend em Go, persistencia e infra local",
				"se o modo for fullstack, tambem gera a base em Next.js",
			},
			Next: []string{
				"deeph crud up --workspace .",
			},
			Context: buildGuideOperatorContext(state),
			Notes: []string{
				"Se quiser revisar antes de gerar, rode `deeph crud trace --workspace .`.",
			},
		}
	}

	if state.LastCommand == "crud up" || state.CoachNext == "crud smoke" {
		return buildGuideOperatorSmokeReply(state)
	}
	if state.LastCommand == "crud smoke" || state.CoachNext == "crud down" {
		return buildGuideOperatorDownReply(state)
	}
	return buildGuideOperatorUpReply(state)
}

func buildGuideOperatorAgentReply(norm string, state guideWorkspaceState) guideOperatorReply {
	name := guideExtractAgentName(norm)
	if !state.HasRootConfig {
		next := "deeph agent create --workspace ."
		if name != "" {
			next += " " + name
		} else {
			next += " <nome-do-agent>"
		}
		return guideOperatorReply{
			Intro: "Antes de criar agents, inicialize o workspace para o `deepH` reconhecer a estrutura do projeto.",
			Commands: []string{
				"deeph init --workspace .",
			},
			What: []string{
				"o `deepH` vai criar `deeph.yaml` e a estrutura basica do workspace",
				"depois disso, o comando `agent create` ja pode escrever o template do agent em `agents/`",
			},
			Next: []string{
				next,
			},
			Notes: []string{
				"Eu nao vou estilizar o YAML no chat. Depois de criar o arquivo, abra-o no VS Code e personalize `system_prompt`, `skills` e `io`.",
			},
		}
	}

	if name == "" {
		return guideOperatorReply{
			Intro: "Para criar um agent de forma direta, eu so preciso do nome.",
			Commands: []string{
				"deeph agent create --workspace . <nome-do-agent>",
			},
			What: []string{
				"o `deepH` vai criar um template em `agents/<nome-do-agent>.yaml`",
				"o template nao sera estilizado pelo chat; a ideia e voce abrir o arquivo no VS Code e ajustar o YAML",
			},
			Next: []string{
				"deeph validate --workspace .",
			},
			Context: buildGuideOperatorContext(state),
			Notes: []string{
				"Se quiser, me diga algo como `cria um agent reviewer` ou `cria um agent backend_builder`.",
			},
		}
	}

	if guideHasAgent(state.Agents, name) {
		return guideOperatorReply{
			Intro: fmt.Sprintf("O agent `%s` ja existe neste workspace.", name),
			Commands: []string{
				fmt.Sprintf("deeph validate --workspace ."),
			},
			What: []string{
				fmt.Sprintf("o arquivo `agents/%s.yaml` ja deve existir", name),
				"o passo certo agora e abrir esse YAML no VS Code e personalizar o agent",
			},
			Next: []string{
				fmt.Sprintf("deeph run --workspace . %s \"teste\"", name),
			},
			Context: buildGuideOperatorContext(state),
			Notes: []string{
				fmt.Sprintf("Depois de editar `agents/%s.yaml`, rode `deeph validate --workspace .`.", name),
			},
		}
	}

	reply := guideOperatorReply{
		Intro: fmt.Sprintf("O comando certo agora e criar o template do agent `%s`.", name),
		Commands: []string{
			fmt.Sprintf("deeph agent create --workspace . %s", name),
		},
		What: []string{
			fmt.Sprintf("o `deepH` vai criar `agents/%s.yaml`", name),
			"o arquivo sera um template inicial, sem o chat tentar estilizar seu YAML por voce",
		},
		Next: []string{
			"deeph validate --workspace .",
		},
		Context: buildGuideOperatorContext(state),
		Notes: []string{
			fmt.Sprintf("Depois de criar, abra `agents/%s.yaml` no VS Code e ajuste `system_prompt`, `skills` e `io`.", name),
		},
	}
	if strings.TrimSpace(state.DefaultProvider) == "" {
		reply.Notes = append(reply.Notes, "Se voce quiser um provider padrao no workspace, configure antes ou depois com `deeph provider add --name deepseek --model deepseek-chat --set-default --force deepseek`.")
	}
	return reply
}

func buildGuideOperatorFlowReply(state guideWorkspaceState) guideOperatorReply {
	if !state.HasRootConfig {
		return guideOperatorReply{
			Intro: "Antes de montar um workflow, inicialize o workspace do `deepH`.",
			Commands: []string{
				"deeph init --workspace .",
			},
			What: []string{
				"o `deepH` vai criar a base do workspace",
				"depois disso, voce pode criar agents e testar workflows inline sem escrever crew YAML ainda",
			},
			Next: []string{
				"deeph agent create --workspace . planner",
			},
			Notes: []string{
				"Para comecar workflows, eu recomendaria criar os primeiros agents antes de pensar em `crews/*.yaml`.",
			},
		}
	}

	if len(state.Agents) < 2 {
		nextAgent := guideSuggestedNextAgent(state.Agents)
		return guideOperatorReply{
			Intro: "Para montar um workflow de verdade, o workspace ainda precisa de mais agents.",
			Commands: []string{
				fmt.Sprintf("deeph agent create --workspace . %s", nextAgent),
			},
			What: []string{
				fmt.Sprintf("o `deepH` vai criar `agents/%s.yaml`", nextAgent),
				"depois disso, voce pode abrir o YAML no VS Code e personalizar o papel desse agent no fluxo",
			},
			Next: []string{
				"deeph validate --workspace .",
			},
			Context: buildGuideOperatorContext(state),
			Notes: []string{
				"Assim que houver pelo menos dois agents, voce ja pode testar um workflow inline com `trace` e `run`.",
			},
		}
	}

	spec := guideSuggestedWorkflowSpec(state.Agents)
	return guideOperatorReply{
		Intro: "Voce ja tem agents suficientes para comecar um workflow sem precisar abrir YAML de crew agora.",
		Commands: []string{
			fmt.Sprintf("deeph trace --workspace . %q \"sua tarefa\"", spec),
		},
		What: []string{
			fmt.Sprintf("o `deepH` vai inspecionar o workflow inline `%s`", spec),
			"voce vai ver stages, canais e handoffs antes de executar o fluxo completo",
		},
		Next: []string{
			fmt.Sprintf("deeph run --workspace . %q \"sua tarefa\"", spec),
		},
		Context: buildGuideOperatorContext(state),
		Notes: []string{
			"Se depois voce quiser persistir esse fluxo como crew, ai sim vale abrir `crews/*.yaml` no VS Code.",
		},
	}
}

func buildGuideOperatorUpReply(state guideWorkspaceState) guideOperatorReply {
	reply := guideOperatorReply{
		Intro: "O comando operacional certo agora e subir a stack local do CRUD.",
		Commands: []string{
			"deeph crud up --workspace .",
		},
		What: []string{
			"o `deepH` vai localizar o arquivo de compose dentro do workspace",
			"vai usar `docker compose` ou `docker-compose` para subir os containers",
			"vai tentar esperar a API responder antes de te devolver o proximo passo",
		},
		Next: []string{
			"deeph crud smoke --workspace .",
		},
		Context: buildGuideOperatorContext(state),
	}
	if !state.HasCRUDProfile {
		reply.Intro = "Antes de subir containers, o workspace ainda precisa ser inicializado como CRUD."
		reply.Commands = []string{guideCRUDInitCommand("")}
		reply.What = []string{
			"o wizard vai salvar a configuracao do CRUD no workspace",
			"depois disso, voce podera gerar o projeto com `deeph crud run`",
		}
		reply.Next = []string{"deeph crud run --workspace .", "deeph crud up --workspace ."}
		return reply
	}
	if strings.TrimSpace(state.ComposeFile) == "" {
		reply.Intro = "Ainda nao encontrei um ambiente gerado para subir. Gere o CRUD primeiro."
		reply.Commands = []string{guideCRUDRunCommand("", state)}
		reply.What = []string{
			"o `deepH` vai gerar o projeto CRUD a partir do perfil salvo",
			"quando o compose existir, voce podera subir a stack com `deeph crud up`",
		}
		reply.Next = []string{"deeph crud up --workspace ."}
	}
	if !state.DockerAvailable {
		reply.Notes = append(reply.Notes, "Docker Compose nao foi detectado nesta maquina. Instale Docker Desktop ou Docker Engine com Compose antes de rodar `deeph crud up`.")
	}
	return reply
}

func buildGuideOperatorSmokeReply(state guideWorkspaceState) guideOperatorReply {
	reply := guideOperatorReply{
		Intro: "O melhor passo agora e validar o CRUD com o smoke test do proprio `deepH`.",
		Commands: []string{
			"deeph crud smoke --workspace .",
		},
		What: []string{
			"o `deepH` tenta rodar primeiro o script gerado do projeto",
			"se nao encontrar script, cai para um smoke HTTP embutido com create, list, read, update e delete",
			"se a URL da API nao for detectada, voce ainda pode informar `--base-url`",
		},
		Next: []string{
			"deeph crud down --workspace .",
		},
		Context: buildGuideOperatorContext(state),
	}
	if !state.HasCRUDProfile {
		reply.Intro = "Antes de validar o CRUD, o workspace ainda precisa ser inicializado."
		reply.Commands = []string{guideCRUDInitCommand("")}
		reply.What = []string{
			"o wizard vai salvar entidade, campos, modo e banco",
			"depois disso, gere o projeto e suba a stack antes do smoke",
		}
		reply.Next = []string{"deeph crud run --workspace .", "deeph crud up --workspace .", "deeph crud smoke --workspace ."}
		return reply
	}
	if strings.TrimSpace(state.ComposeFile) == "" {
		reply.Intro = "Ainda nao encontrei um ambiente gerado para validar. Gere o CRUD antes do smoke."
		reply.Commands = []string{guideCRUDRunCommand("", state)}
		reply.What = []string{
			"o `deepH` vai gerar o projeto CRUD",
			"depois disso, suba a stack com `deeph crud up` e rode o smoke",
		}
		reply.Next = []string{"deeph crud up --workspace .", "deeph crud smoke --workspace ."}
	}
	return reply
}

func buildGuideOperatorDownReply(state guideWorkspaceState) guideOperatorReply {
	reply := guideOperatorReply{
		Intro: "Para encerrar o ambiente local do CRUD, o comando certo e este.",
		Commands: []string{
			"deeph crud down --workspace .",
		},
		What: []string{
			"o `deepH` vai localizar o compose do workspace",
			"vai derrubar os containers e remover orfaos da stack",
		},
		Context: buildGuideOperatorContext(state),
	}
	if strings.TrimSpace(state.ComposeFile) == "" {
		reply.Intro = "No workspace atual eu ainda nao detectei um compose para derrubar."
		reply.Commands = []string{"deeph crud run --workspace ."}
		reply.What = []string{
			"gere o CRUD primeiro para que o ambiente local tenha compose e scripts operacionais",
		}
		reply.Next = []string{"deeph crud up --workspace .", "deeph crud down --workspace ."}
	}
	return reply
}

func guideCRUDInitCommand(norm string) string {
	cmd := "deeph crud init --workspace ."
	if guideWantsBackendOnly(norm) {
		cmd += " --mode backend"
	} else if guideWantsFullstack(norm) {
		cmd += " --mode fullstack"
	}
	if guideWantsDocumentDB(norm) {
		cmd += " --db-kind document --db mongodb"
	}
	return cmd
}

func guideCRUDRunCommand(norm string, state guideWorkspaceState) string {
	cmd := "deeph crud run --workspace ."
	if guideWantsBackendOnly(norm) && !state.CRUDConfig.BackendOnly {
		cmd += " --mode backend"
	} else if guideWantsFullstack(norm) && state.HasCRUDProfile && state.CRUDConfig.BackendOnly {
		cmd += " --mode fullstack"
	}
	if guideWantsDocumentDB(norm) && strings.TrimSpace(state.CRUDConfig.DBKind) != "document" {
		cmd += " --db-kind document --db mongodb"
	}
	return cmd
}

func guideWantsBackendOnly(norm string) bool {
	return containsAny(norm, "backend", "beck end", "back end", "api", "servidor") && !guideWantsFullstack(norm)
}

func guideWantsFullstack(norm string) bool {
	return containsAny(norm, "fullstack", "frontend", "front end", "front-end", "next")
}

func guideWantsDocumentDB(norm string) bool {
	return containsAny(norm, "document", "mongo", "mongodb", "nosql", "nao relacional", "não relacional")
}

func buildGuideOperatorContext(state guideWorkspaceState) []string {
	out := make([]string, 0, 5)
	if len(state.Agents) > 0 {
		out = append(out, fmt.Sprintf("%d agent(s) detectado(s): `%s`", len(state.Agents), strings.Join(state.Agents, "`, `")))
	}
	if state.HasCRUDProfile {
		entity := strings.TrimSpace(state.CRUDConfig.Entity)
		if entity == "" {
			entity = "people"
		}
		out = append(out, fmt.Sprintf("perfil CRUD detectado para a entidade `%s`", entity))
	}
	if strings.TrimSpace(state.ComposeFile) != "" {
		out = append(out, fmt.Sprintf("compose detectado em `%s`", guideWorkspaceDisplayPath(state.Workspace, state.ComposeFile)))
	}
	if strings.TrimSpace(state.SmokeScript) != "" {
		out = append(out, fmt.Sprintf("script de smoke detectado em `%s`", guideWorkspaceDisplayPath(state.Workspace, state.SmokeScript)))
	}
	if strings.TrimSpace(state.LastCommand) != "" {
		out = append(out, fmt.Sprintf("ultimo comando observado pelo coach: `%s`", state.LastCommand))
	}
	if strings.TrimSpace(state.DefaultProvider) != "" {
		out = append(out, fmt.Sprintf("default_provider detectado: `%s`", state.DefaultProvider))
	}
	if strings.TrimSpace(state.CoachNext) != "" {
		out = append(out, fmt.Sprintf("padrao local: o proximo passo costuma ser `%s` (%d%%)", state.CoachNext, state.CoachConfidence))
	}
	return out
}

func guideExtractAgentName(norm string) string {
	tokens := strings.Fields(norm)
	for i := 0; i < len(tokens); i++ {
		tok := strings.TrimSpace(tokens[i])
		switch tok {
		case "agent", "agente":
			if i+2 < len(tokens) && containsAny(tokens[i+1], "chamado", "named", "nome") {
				if name := sanitizeGuideAgentName(tokens[i+2]); name != "" {
					return name
				}
			}
			if i+1 < len(tokens) {
				if name := sanitizeGuideAgentName(tokens[i+1]); name != "" {
					return name
				}
			}
		}
	}
	for i := 1; i < len(tokens); i++ {
		if tokens[i] == "agent" || tokens[i] == "agente" {
			if name := sanitizeGuideAgentName(tokens[i-1]); name != "" {
				return name
			}
		}
	}
	return ""
}

func sanitizeGuideAgentName(raw string) string {
	raw = strings.Trim(raw, " ,.:;!?\"'`()[]{}")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	switch raw {
	case "para", "de", "do", "da", "e", "ou", "um", "uma", "novo", "nova", "workflow", "crew", "backend", "frontend", "crud":
		return ""
	}
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return ""
		}
	}
	return raw
}

func guideHasAgent(agents []string, name string) bool {
	for _, agent := range agents {
		if strings.EqualFold(strings.TrimSpace(agent), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func guideSuggestedNextAgent(agents []string) string {
	order := []string{"planner", "coder", "reviewer"}
	for _, candidate := range order {
		if !guideHasAgent(agents, candidate) {
			return candidate
		}
	}
	return "agent_" + fmt.Sprintf("%d", len(agents)+1)
}

func guideSuggestedWorkflowSpec(agents []string) string {
	preferred := make([]string, 0, 3)
	for _, candidate := range []string{"planner", "coder", "reviewer"} {
		if guideHasAgent(agents, candidate) {
			preferred = append(preferred, candidate)
		}
	}
	if len(preferred) >= 2 {
		return strings.Join(preferred, ">")
	}
	if len(agents) >= 3 {
		return strings.Join(agents[:3], ">")
	}
	return strings.Join(agents[:2], ">")
}

func guideWorkspaceDisplayPath(workspace, path string) string {
	workspace = strings.TrimSpace(workspace)
	path = strings.TrimSpace(path)
	if workspace == "" || path == "" {
		return path
	}
	rel, err := filepath.Rel(workspace, path)
	if err == nil && strings.TrimSpace(rel) != "" && rel != "." {
		return filepath.ToSlash(rel)
	}
	return path
}

func formatGuideOperatorReply(reply guideOperatorReply) string {
	lines := make([]string, 0, 24)
	if strings.TrimSpace(reply.Intro) != "" {
		lines = append(lines, strings.TrimSpace(reply.Intro), "")
	}
	if len(reply.Commands) > 0 {
		lines = append(lines, "Comando agora:", "```bash")
		lines = append(lines, reply.Commands...)
		lines = append(lines, "```")
	}
	if len(reply.What) > 0 {
		lines = append(lines, "", "O que vai acontecer:")
		for _, item := range reply.What {
			lines = append(lines, "- "+strings.TrimSpace(item))
		}
	}
	if len(reply.Next) > 0 {
		lines = append(lines, "", "Proximo passo:", "```bash")
		lines = append(lines, reply.Next...)
		lines = append(lines, "```")
	}
	if len(reply.Context) > 0 {
		lines = append(lines, "", "Contexto detectado:")
		for _, item := range reply.Context {
			lines = append(lines, "- "+strings.TrimSpace(item))
		}
	}
	if len(reply.Notes) > 0 {
		lines = append(lines, "", "Notas:")
		for _, item := range reply.Notes {
			lines = append(lines, "- "+strings.TrimSpace(item))
		}
	}
	return strings.Join(lines, "\n")
}
