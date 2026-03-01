package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"deeph/internal/project"
)

const defaultCRUDCrewName = "crud_fullstack_multiverse"

type crudPromptOptions struct {
	Entity      string
	Fields      []crudField
	DB          string
	Backend     string
	Frontend    string
	BackendOnly bool
	Containers  bool
	Workspace   string
}

type crudField struct {
	Name string
	Type string
}

func cmdCRUD(args []string) error {
	if len(args) == 0 {
		return errors.New("crud requires a subcommand: init, prompt, trace or run")
	}
	switch args[0] {
	case "init":
		return cmdCRUDInit(args[1:])
	case "prompt":
		return cmdCRUDPrompt(args[1:])
	case "trace":
		return cmdCRUDTrace(args[1:])
	case "run":
		return cmdCRUDRun(args[1:])
	default:
		return fmt.Errorf("unknown crud subcommand %q", args[0])
	}
}

func cmdCRUDInit(args []string) error {
	fs := flag.NewFlagSet("crud init", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	force := fs.Bool("force", false, "overwrite changed kit files when content differs")
	providerName := fs.String("provider-name", "deepseek", "provider name to scaffold for the CRUD kit")
	model := fs.String("model", "deepseek-chat", "provider model used when scaffolding deepseek")
	setDefaultProvider := fs.Bool("set-default-provider", true, "set scaffolded provider as default_provider")
	skipProvider := fs.Bool("skip-provider", false, "do not scaffold provider configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("crud init does not accept positional arguments")
	}

	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	if !workspaceHasRootConfig(abs) {
		if err := cmdInit([]string{"--workspace", abs}); err != nil {
			return err
		}
	}

	kitArgs := []string{
		"--workspace", abs,
		"--provider-name", strings.TrimSpace(*providerName),
		"--model", strings.TrimSpace(*model),
	}
	if *force {
		kitArgs = append(kitArgs, "--force")
	}
	if *setDefaultProvider {
		kitArgs = append(kitArgs, "--set-default-provider")
	}
	if *skipProvider {
		kitArgs = append(kitArgs, "--skip-provider")
	}
	kitArgs = append(kitArgs, "crud-next-multiverse")
	if err := cmdKitAdd(kitArgs); err != nil {
		return err
	}

	fmt.Println("CRUD starter ready.")
	fmt.Println("Opinionated defaults:")
	fmt.Println("  - backend: Go")
	fmt.Println("  - frontend: Next.js")
	fmt.Println("  - database: Postgres")
	fmt.Println("  - orchestration: crew:crud_fullstack_multiverse")
	fmt.Println("Next steps:")
	fmt.Printf("  1. deeph crud trace --workspace %q --entity people --fields nome:text,cidade:text\n", abs)
	fmt.Printf("  2. deeph crud run --workspace %q --entity people --fields nome:text,cidade:text\n", abs)
	fmt.Printf("  3. if needed, inspect the crew with: deeph crew show --workspace %q %s\n", abs, defaultCRUDCrewName)
	return nil
}

func cmdCRUDPrompt(args []string) error {
	opts, err := parseCRUDPromptOptions("crud prompt", args)
	if err != nil {
		return err
	}
	fmt.Println(buildCRUDPrompt(opts))
	return nil
}

func cmdCRUDTrace(args []string) error {
	opts, err := parseCRUDPromptOptions("crud trace", args)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(opts.Workspace)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "crud trace")
	return cmdTrace([]string{
		"--workspace", abs,
		"--multiverse", "0",
		"crew:" + defaultCRUDCrewName,
		buildCRUDPrompt(opts),
	})
}

func cmdCRUDRun(args []string) error {
	opts, err := parseCRUDPromptOptions("crud run", args)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(opts.Workspace)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "crud run")
	return cmdRun([]string{
		"--workspace", abs,
		"--multiverse", "0",
		"crew:" + defaultCRUDCrewName,
		buildCRUDPrompt(opts),
	})
}

func parseCRUDPromptOptions(cmdName string, args []string) (crudPromptOptions, error) {
	fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	entity := fs.String("entity", "people", "entity/table name for the CRUD")
	fields := fs.String("fields", "nome:text,cidade:text", "comma-separated fields, ex.: nome:text,cidade:text or nome,cidade")
	db := fs.String("db", "postgres", "database engine for the CRUD")
	backend := fs.String("backend", "go", "backend stack (default go)")
	frontend := fs.String("frontend", "next", "frontend stack (default next)")
	backendOnly := fs.Bool("backend-only", false, "generate only backend + infra, skip frontend")
	containers := fs.Bool("containers", true, "ask for Docker Compose and containerized local run")
	if err := fs.Parse(args); err != nil {
		return crudPromptOptions{}, err
	}
	if len(fs.Args()) != 0 {
		return crudPromptOptions{}, fmt.Errorf("%s does not accept positional arguments", cmdName)
	}
	parsedFields, err := parseCRUDFields(*fields)
	if err != nil {
		return crudPromptOptions{}, err
	}
	opts := crudPromptOptions{
		Entity:      strings.TrimSpace(*entity),
		Fields:      parsedFields,
		DB:          strings.TrimSpace(*db),
		Backend:     strings.TrimSpace(*backend),
		Frontend:    strings.TrimSpace(*frontend),
		BackendOnly: *backendOnly,
		Containers:  *containers,
		Workspace:   strings.TrimSpace(*workspace),
	}
	if opts.Entity == "" {
		return crudPromptOptions{}, errors.New("--entity cannot be empty")
	}
	if opts.DB == "" {
		return crudPromptOptions{}, errors.New("--db cannot be empty")
	}
	if opts.Backend == "" {
		opts.Backend = "go"
	}
	if opts.Frontend == "" && !opts.BackendOnly {
		opts.Frontend = "next"
	}
	if opts.BackendOnly {
		opts.Frontend = ""
	}
	return opts, nil
}

func parseCRUDFields(raw string) ([]crudField, error) {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]crudField, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		name := item
		typ := "text"
		if strings.Contains(item, ":") {
			left, right, _ := strings.Cut(item, ":")
			name = strings.TrimSpace(left)
			typ = strings.TrimSpace(right)
		}
		if name == "" {
			return nil, fmt.Errorf("invalid field %q", part)
		}
		if typ == "" {
			typ = "text"
		}
		out = append(out, crudField{Name: name, Type: typ})
	}
	if len(out) == 0 {
		return nil, errors.New("at least one field is required in --fields")
	}
	return out, nil
}

func buildCRUDPrompt(opts crudPromptOptions) string {
	entity := strings.TrimSpace(opts.Entity)
	if entity == "" {
		entity = "people"
	}
	backend := coalesce(strings.TrimSpace(opts.Backend), "go")
	db := coalesce(strings.TrimSpace(opts.DB), "postgres")
	frontend := strings.TrimSpace(opts.Frontend)

	lines := []string{
		fmt.Sprintf("Crie um CRUD para a entidade %s.", entity),
		fmt.Sprintf("Backend obrigatorio em %s.", displayCRUDStackName(backend)),
	}
	if opts.BackendOnly {
		lines = append(lines, "Gere apenas backend e infra local. Nao gere frontend.")
	} else {
		lines = append(lines, fmt.Sprintf("Frontend obrigatorio em %s.", displayCRUDStackName(coalesce(frontend, "next"))))
	}
	lines = append(lines,
		fmt.Sprintf("Banco obrigatorio: %s.", displayCRUDStackName(db)),
		"Use uma estrutura inicial simples, clara e pronta para evolucao pelo usuario.",
	)
	if opts.Containers {
		lines = append(lines, "Tudo deve rodar localmente com containers usando Docker Compose.")
	}
	lines = append(lines,
		"Campos obrigatorios da tabela:",
		"- id",
	)
	for _, f := range opts.Fields {
		lines = append(lines, fmt.Sprintf("- %s (%s)", f.Name, f.Type))
	}
	lines = append(lines,
		"Entregue:",
		"- rotas CRUD completas",
		"- persistencia no banco",
		"- migration SQL ou equivalente",
		"- Dockerfile e docker-compose quando aplicavel",
		"- README com comandos exatos para subir e testar",
		"- smoke test simples com exemplos de create, list, update e delete",
	)
	if strings.EqualFold(backend, "go") {
		lines = append(lines, "No backend Go, prefira estrutura simples de handler/service/repository e SQL explicito sem ORM pesado.")
	}
	if !opts.BackendOnly && strings.EqualFold(frontend, "next") {
		lines = append(lines, "No frontend Next.js, prefira uma UI simples de lista + formulario para criar, editar e remover registros.")
	}
	lines = append(lines, "Responda e implemente pensando que o usuario quer evoluir junto com o deepH, com arquivos claros e comandos reutilizaveis.")
	return strings.Join(lines, "\n")
}

func displayCRUDStackName(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "go", "golang":
		return "Go"
	case "next", "nextjs", "next.js":
		return "Next.js"
	case "postgres", "postgresql":
		return "Postgres"
	case "mysql":
		return "MySQL"
	case "sqlite":
		return "SQLite"
	case "mongo", "mongodb":
		return "MongoDB"
	default:
		return strings.TrimSpace(s)
	}
}

func workspaceHasRootConfig(workspace string) bool {
	rootPath := filepath.Join(workspace, project.RootConfigFile)
	_, err := os.Stat(rootPath)
	return err == nil
}

func coalesce(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}
