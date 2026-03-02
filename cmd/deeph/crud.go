package main

import (
	"bufio"
	"encoding/json"
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
	DBKind      string
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

type crudWorkspaceConfig struct {
	Version     int         `json:"version"`
	Entity      string      `json:"entity"`
	Fields      []crudField `json:"fields"`
	DBKind      string      `json:"db_kind"`
	DB          string      `json:"db"`
	Backend     string      `json:"backend"`
	Frontend    string      `json:"frontend,omitempty"`
	BackendOnly bool        `json:"backend_only"`
	Containers  bool        `json:"containers"`
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
	mode := fs.String("mode", "", "crud mode: backend or fullstack")
	dbKind := fs.String("db-kind", "", "database kind: relational or document")
	db := fs.String("db", "", "database engine, ex.: postgres or mongodb")
	entity := fs.String("entity", "", "entity/table name for the CRUD")
	fields := fs.String("fields", "", "comma-separated fields, ex.: nome:text,cidade:text")
	containers := fs.Bool("containers", true, "ask for Docker Compose and containerized local run")
	noPrompt := fs.Bool("no-prompt", false, "skip the interactive CRUD setup wizard")
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

	cfg := defaultCRUDWorkspaceConfig()
	if saved, ok, err := loadCRUDWorkspaceConfig(abs); err != nil {
		return err
	} else if ok {
		cfg = saved
	}
	if flagProvided(args, "mode") {
		cfg.BackendOnly = normalizeCRUDMode(*mode) == "backend"
	}
	if flagProvided(args, "db-kind") {
		cfg.DBKind = normalizeCRUDDBKind(*dbKind)
	}
	if flagProvided(args, "db") {
		cfg.DB = normalizeCRUDDB(*db)
	}
	if flagProvided(args, "entity") {
		cfg.Entity = strings.TrimSpace(*entity)
	}
	if flagProvided(args, "fields") {
		parsedFields, err := parseCRUDFields(*fields)
		if err != nil {
			return err
		}
		cfg.Fields = parsedFields
	}
	if flagProvided(args, "containers") {
		cfg.Containers = *containers
	}
	cfg = normalizeCRUDWorkspaceConfig(cfg)
	if !*noPrompt && isInteractiveTerminal(os.Stdin) {
		reader := bufio.NewReader(os.Stdin)
		nextCfg, err := promptCRUDWorkspaceConfig(reader, cfg)
		if err != nil {
			return err
		}
		cfg = nextCfg
	}
	if err := saveCRUDWorkspaceConfig(abs, cfg); err != nil {
		return err
	}

	opts := crudPromptOptionsFromConfig(abs, cfg)
	fmt.Println("CRUD starter ready.")
	fmt.Println("Saved CRUD workspace profile:")
	fmt.Printf("  - mode: %s\n", crudModeLabel(cfg.BackendOnly))
	fmt.Printf("  - data model: %s\n", displayCRUDDataModelName(cfg.DBKind))
	fmt.Printf("  - backend: %s\n", displayCRUDStackName(cfg.Backend))
	if !cfg.BackendOnly {
		fmt.Printf("  - frontend: %s\n", displayCRUDStackName(cfg.Frontend))
	}
	fmt.Printf("  - database: %s\n", displayCRUDStackName(cfg.DB))
	fmt.Printf("  - entity: %s\n", cfg.Entity)
	fmt.Printf("  - crew: %s\n", chooseCRUDCrew(opts))
	fmt.Println("Next steps:")
	fmt.Printf("  1. deeph crud trace --workspace %q\n", abs)
	fmt.Printf("  2. deeph crud run --workspace %q\n", abs)
	fmt.Printf("  3. if needed, inspect the crew with: deeph crew show --workspace %q %s\n", abs, chooseCRUDCrew(opts))
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
		"crew:" + chooseCRUDCrew(opts),
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
		"crew:" + chooseCRUDCrew(opts),
		buildCRUDPrompt(opts),
	})
}

func parseCRUDPromptOptions(cmdName string, args []string) (crudPromptOptions, error) {
	fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	entity := fs.String("entity", "", "entity/table name for the CRUD")
	fields := fs.String("fields", "", "comma-separated fields, ex.: nome:text,cidade:text or nome,cidade")
	dbKind := fs.String("db-kind", "", "database kind: relational or document")
	db := fs.String("db", "", "database engine for the CRUD")
	backend := fs.String("backend", "", "backend stack (default go)")
	frontend := fs.String("frontend", "", "frontend stack (default next)")
	backendOnly := fs.Bool("backend-only", false, "generate only backend + infra, skip frontend")
	mode := fs.String("mode", "", "crud mode: backend or fullstack")
	containers := fs.Bool("containers", true, "ask for Docker Compose and containerized local run")
	if err := fs.Parse(args); err != nil {
		return crudPromptOptions{}, err
	}
	if len(fs.Args()) != 0 {
		return crudPromptOptions{}, fmt.Errorf("%s does not accept positional arguments", cmdName)
	}

	abs, err := filepath.Abs(strings.TrimSpace(*workspace))
	if err != nil {
		return crudPromptOptions{}, err
	}
	cfg := defaultCRUDWorkspaceConfig()
	if saved, ok, err := loadCRUDWorkspaceConfig(abs); err != nil {
		return crudPromptOptions{}, err
	} else if ok {
		cfg = saved
	}
	if flagProvided(args, "entity") {
		cfg.Entity = strings.TrimSpace(*entity)
	}
	if flagProvided(args, "fields") {
		parsedFields, err := parseCRUDFields(*fields)
		if err != nil {
			return crudPromptOptions{}, err
		}
		cfg.Fields = parsedFields
	}
	if flagProvided(args, "db-kind") {
		cfg.DBKind = normalizeCRUDDBKind(*dbKind)
	}
	if flagProvided(args, "db") {
		cfg.DB = normalizeCRUDDB(*db)
	}
	if flagProvided(args, "backend") {
		cfg.Backend = strings.TrimSpace(*backend)
	}
	if flagProvided(args, "frontend") {
		cfg.Frontend = strings.TrimSpace(*frontend)
	}
	if flagProvided(args, "backend-only") && *backendOnly {
		cfg.BackendOnly = true
	}
	if flagProvided(args, "mode") {
		cfg.BackendOnly = normalizeCRUDMode(*mode) == "backend"
	}
	if flagProvided(args, "containers") {
		cfg.Containers = *containers
	}
	cfg = normalizeCRUDWorkspaceConfig(cfg)
	if strings.TrimSpace(cfg.Entity) == "" {
		return crudPromptOptions{}, errors.New("CRUD entity is empty; run `deeph crud init` or pass --entity")
	}
	if len(cfg.Fields) == 0 {
		return crudPromptOptions{}, errors.New("CRUD fields are empty; run `deeph crud init` or pass --fields")
	}
	return crudPromptOptionsFromConfig(abs, cfg), nil
}

func crudPromptOptionsFromConfig(workspace string, cfg crudWorkspaceConfig) crudPromptOptions {
	return crudPromptOptions{
		Entity:      cfg.Entity,
		Fields:      cfg.Fields,
		DBKind:      cfg.DBKind,
		DB:          cfg.DB,
		Backend:     cfg.Backend,
		Frontend:    cfg.Frontend,
		BackendOnly: cfg.BackendOnly,
		Containers:  cfg.Containers,
		Workspace:   workspace,
	}
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
	dbKind := normalizeCRUDDBKind(opts.DBKind)
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
		fmt.Sprintf("Modelo de dados obrigatorio: %s.", displayCRUDDataModelName(dbKind)),
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
		"- tabela final de rotas HTTP com metodo, path e payload",
		"- persistencia no banco",
		"- migration SQL ou equivalente",
		"- Dockerfile e docker-compose quando aplicavel",
		"- README com comandos exatos para subir e testar",
		"- smoke test simples com exemplos de create, list, update e delete",
	)
	if dbKind == "document" {
		lines = append(lines, "Para banco nao relacional, modele colecoes, chaves e indices com foco em CRUD simples e previsivel.")
	} else {
		lines = append(lines, "Para banco relacional, modele tabela, chaves, constraints e migration inicial de forma explicita.")
	}
	if strings.EqualFold(backend, "go") {
		lines = append(lines, "No backend Go, prefira estrutura simples de handler/service/repository e SQL explicito sem ORM pesado.")
		lines = append(lines, "O server deve subir localmente de forma clara e previsivel.")
	}
	if !opts.BackendOnly && strings.EqualFold(frontend, "next") {
		lines = append(lines, "No frontend Next.js, prefira uma UI simples de lista + formulario para criar, editar e remover registros.")
	}
	lines = append(lines, "Responda e implemente pensando que o usuario quer evoluir junto com o deepH, com arquivos claros e comandos reutilizaveis.")
	return strings.Join(lines, "\n")
}

func chooseCRUDCrew(opts crudPromptOptions) string {
	dbKind := normalizeCRUDDBKind(opts.DBKind)
	switch {
	case dbKind == "document" && opts.BackendOnly:
		return "crud_backend_document"
	case dbKind == "document":
		return "crud_fullstack_document"
	case opts.BackendOnly:
		return "crud_backend_relational"
	default:
		return "crud_fullstack_relational"
	}
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

func displayCRUDDataModelName(kind string) string {
	switch normalizeCRUDDBKind(kind) {
	case "document":
		return "nao relacional"
	default:
		return "relacional"
	}
}

func defaultCRUDWorkspaceConfig() crudWorkspaceConfig {
	fields, _ := parseCRUDFields("nome:text,cidade:text")
	return normalizeCRUDWorkspaceConfig(crudWorkspaceConfig{
		Version:     1,
		Entity:      "people",
		Fields:      fields,
		DBKind:      "relational",
		DB:          "postgres",
		Backend:     "go",
		Frontend:    "next",
		BackendOnly: true,
		Containers:  true,
	})
}

func normalizeCRUDWorkspaceConfig(cfg crudWorkspaceConfig) crudWorkspaceConfig {
	cfg.Version = 1
	cfg.Entity = strings.TrimSpace(cfg.Entity)
	cfg.DBKind = normalizeCRUDDBKind(cfg.DBKind)
	cfg.DB = normalizeCRUDDB(cfg.DB)
	cfg.Backend = coalesce(cfg.Backend, "go")
	cfg.Frontend = coalesce(cfg.Frontend, "next")
	if cfg.DBKind == "document" && cfg.DB == "" {
		cfg.DB = "mongodb"
	}
	if cfg.DBKind == "relational" && cfg.DB == "" {
		cfg.DB = "postgres"
	}
	if cfg.BackendOnly {
		cfg.Frontend = ""
	}
	if len(cfg.Fields) == 0 {
		cfg.Fields, _ = parseCRUDFields("nome:text,cidade:text")
	}
	return cfg
}

func normalizeCRUDMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fullstack", "full", "frontend":
		return "fullstack"
	default:
		return "backend"
	}
}

func normalizeCRUDDBKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "document", "non-relational", "nonrelational", "nosql", "nao-relacional", "nao relacional":
		return "document"
	default:
		return "relational"
	}
}

func normalizeCRUDDB(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "postgresql":
		return "postgres"
	case "mongo":
		return "mongodb"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func crudModeLabel(backendOnly bool) string {
	if backendOnly {
		return "backend-only"
	}
	return "fullstack"
}

func promptCRUDWorkspaceConfig(reader *bufio.Reader, cfg crudWorkspaceConfig) (crudWorkspaceConfig, error) {
	modeDefault := crudModeLabel(cfg.BackendOnly)
	modeRaw, err := promptLine(reader, "CRUD mode (backend-only/fullstack)", modeDefault)
	if err != nil {
		return cfg, err
	}
	cfg.BackendOnly = normalizeCRUDMode(modeRaw) == "backend"

	dbKindRaw, err := promptLine(reader, "Database kind (relational/document)", displayCRUDDataModelName(cfg.DBKind))
	if err != nil {
		return cfg, err
	}
	cfg.DBKind = normalizeCRUDDBKind(dbKindRaw)
	dbDefault := cfg.DB
	if cfg.DBKind == "document" && strings.TrimSpace(dbDefault) == "" {
		dbDefault = "mongodb"
	}
	if cfg.DBKind == "relational" && strings.TrimSpace(dbDefault) == "" {
		dbDefault = "postgres"
	}
	dbRaw, err := promptLine(reader, "Database engine", dbDefault)
	if err != nil {
		return cfg, err
	}
	cfg.DB = normalizeCRUDDB(dbRaw)

	entity, err := promptLine(reader, "Main entity", cfg.Entity)
	if err != nil {
		return cfg, err
	}
	cfg.Entity = strings.TrimSpace(entity)

	fieldDefault := crudFieldsString(cfg.Fields)
	fieldRaw, err := promptLine(reader, "Fields (ex.: nome:text,cidade:text)", fieldDefault)
	if err != nil {
		return cfg, err
	}
	fields, err := parseCRUDFields(fieldRaw)
	if err != nil {
		return cfg, err
	}
	cfg.Fields = fields

	containerDefault := "yes"
	if !cfg.Containers {
		containerDefault = "no"
	}
	containerRaw, err := promptLine(reader, "Use containers locally? (yes/no)", containerDefault)
	if err != nil {
		return cfg, err
	}
	cfg.Containers = parseCRUDYesNo(containerRaw, cfg.Containers)
	cfg.Backend = "go"
	if !cfg.BackendOnly {
		cfg.Frontend = "next"
	}
	return normalizeCRUDWorkspaceConfig(cfg), nil
}

func parseCRUDYesNo(raw string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "y", "yes", "sim", "s", "true":
		return true
	case "n", "no", "nao", "não", "false":
		return false
	default:
		return fallback
	}
}

func crudFieldsString(fields []crudField) string {
	if len(fields) == 0 {
		return ""
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			continue
		}
		typ := coalesce(f.Type, "text")
		out = append(out, fmt.Sprintf("%s:%s", name, typ))
	}
	return strings.Join(out, ",")
}

func crudConfigPath(workspace string) string {
	return filepath.Join(workspace, ".deeph", "crud.json")
}

func loadCRUDWorkspaceConfig(workspace string) (crudWorkspaceConfig, bool, error) {
	path := crudConfigPath(workspace)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return crudWorkspaceConfig{}, false, nil
		}
		return crudWorkspaceConfig{}, false, err
	}
	var cfg crudWorkspaceConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return crudWorkspaceConfig{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return normalizeCRUDWorkspaceConfig(cfg), true, nil
}

func saveCRUDWorkspaceConfig(workspace string, cfg crudWorkspaceConfig) error {
	path := crudConfigPath(workspace)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(normalizeCRUDWorkspaceConfig(cfg), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func flagProvided(args []string, name string) bool {
	prefix := "--" + strings.TrimLeft(strings.TrimSpace(name), "-")
	for _, arg := range args {
		if arg == prefix || strings.HasPrefix(arg, prefix+"=") {
			return true
		}
	}
	return false
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
