package main

import (
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

type guideOperatorIntent string

const (
	guideOperatorUnknown  guideOperatorIntent = ""
	guideOperatorWorkflow guideOperatorIntent = "workflow"
	guideOperatorUp       guideOperatorIntent = "up"
	guideOperatorSmoke    guideOperatorIntent = "smoke"
	guideOperatorDown     guideOperatorIntent = "down"
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
	case containsAny(norm, "crud", "cadastro"):
		return guideOperatorWorkflow
	case containsAny(norm, "backend", "beck end", "back end", "api", "servidor"):
		return guideOperatorWorkflow
	case containsAny(norm, "saas", "app", "projeto", "produto") && containsAny(norm, "como", "comeco", "comeco", "comecar", "começar", "criar", "iniciar", "passo", "agora"):
		return guideOperatorWorkflow
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
	case guideOperatorWorkflow:
		return buildGuideOperatorWorkflowReply(norm, state), true
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
	if strings.TrimSpace(state.CoachNext) != "" {
		out = append(out, fmt.Sprintf("padrao local: o proximo passo costuma ser `%s` (%d%%)", state.CoachNext, state.CoachConfidence))
	}
	return out
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
