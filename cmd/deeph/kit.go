package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"deeph/internal/catalog"
	"deeph/internal/project"
)

func cmdKit(args []string) error {
	if len(args) == 0 {
		return errors.New("kit requires a subcommand: list or add")
	}
	switch args[0] {
	case "list":
		return cmdKitList(args[1:])
	case "add":
		return cmdKitAdd(args[1:])
	default:
		return fmt.Errorf("unknown kit subcommand %q", args[0])
	}
}

func cmdKitList(args []string) error {
	fs := flag.NewFlagSet("kit list", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("kit list does not accept positional arguments")
	}
	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "kit list")
	kits := catalog.ListKits()
	if len(kits) == 0 {
		fmt.Println("No kits registered.")
		return nil
	}
	for _, k := range kits {
		fmt.Printf("- %s: %s (skills=%d files=%d", k.Name, k.Description, len(k.RequiredSkills), len(k.Files))
		if strings.TrimSpace(k.ProviderType) != "" {
			fmt.Printf(" provider=%s", k.ProviderType)
		}
		fmt.Printf(")\n")
	}
	return nil
}

func cmdKitAdd(args []string) error {
	fs := flag.NewFlagSet("kit add", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	force := fs.Bool("force", false, "overwrite existing files/skills when content differs")
	providerName := fs.String("provider-name", "deepseek", "provider name to scaffold when kit requires deepseek")
	model := fs.String("model", "deepseek-chat", "provider model used when scaffolding deepseek")
	setDefaultProvider := fs.Bool("set-default-provider", true, "set scaffoled provider as default_provider")
	skipProvider := fs.Bool("skip-provider", false, "do not scaffold provider configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return errors.New("kit add requires <name|git-url[#manifest.yaml]>")
	}

	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	rootPath := filepath.Join(abs, project.RootConfigFile)
	if _, err := os.Stat(rootPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workspace not initialized: %s not found (run `deeph init` first)", rootPath)
		}
		return err
	}

	kit, sourceLabel, err := resolveKitTemplate(strings.TrimSpace(rest[0]))
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "kit add", kit.Name)

	p, err := project.Load(abs)
	if err != nil {
		return err
	}

	skillStats := installStats{}
	fileStats := installStats{}
	sort.Strings(kit.RequiredSkills)
	for _, skillName := range kit.RequiredSkills {
		status, outPath, err := installCatalogSkillTemplate(abs, skillName, *force)
		if err != nil {
			return err
		}
		skillStats.bump(status)
		if status != "unchanged" {
			fmt.Printf("skill[%s]: %s (%s)\n", skillName, status, outPath)
		}
	}

	for _, f := range kit.Files {
		status, outPath, err := writeKitFile(abs, f.Path, f.Content, *force)
		if err != nil {
			return err
		}
		fileStats.bump(status)
		if status != "unchanged" {
			fmt.Printf("file[%s]: %s (%s)\n", f.Path, status, outPath)
		}
	}

	providerMsg := "skipped"
	providerChanged := false
	if !*skipProvider && strings.EqualFold(strings.TrimSpace(kit.ProviderType), "deepseek") {
		if strings.TrimSpace(*providerName) == "" {
			return errors.New("--provider-name cannot be empty")
		}
		msg, changed, err := ensureKitDeepseekProvider(&p.Root, strings.TrimSpace(*providerName), strings.TrimSpace(*model), *setDefaultProvider)
		if err != nil {
			return err
		}
		providerMsg = msg
		providerChanged = changed
	}
	if providerChanged {
		if err := project.SaveRootConfig(abs, p.Root); err != nil {
			return err
		}
	}

	reloaded, err := project.Load(abs)
	if err != nil {
		return err
	}
	verr := project.Validate(reloaded)
	printValidation(verr)
	if verr != nil && verr.HasErrors() {
		return verr
	}

	fmt.Printf("Installed kit %q in %s\n", kit.Name, abs)
	fmt.Printf("  source: %s\n", sourceLabel)
	fmt.Printf("  skills: created=%d updated=%d unchanged=%d skipped=%d\n", skillStats.Created, skillStats.Updated, skillStats.Unchanged, skillStats.Skipped)
	fmt.Printf("  files:  created=%d updated=%d unchanged=%d skipped=%d\n", fileStats.Created, fileStats.Updated, fileStats.Unchanged, fileStats.Skipped)
	fmt.Printf("  provider: %s\n", providerMsg)
	fmt.Println("Next steps:")
	fmt.Println("  1. deeph validate")
	fmt.Printf("  2. deeph crew list\n")
	fmt.Printf("  3. deeph run --multiverse 0 @%s \"sua tarefa\"\n", guessKitCrewName(kit))
	return nil
}

type installStats struct {
	Created   int
	Updated   int
	Unchanged int
	Skipped   int
}

func (s *installStats) bump(status string) {
	switch status {
	case "created":
		s.Created++
	case "updated":
		s.Updated++
	case "skipped":
		s.Skipped++
	default:
		s.Unchanged++
	}
}

func installCatalogSkillTemplate(workspace, name string, force bool) (status string, outPath string, err error) {
	tmpl, err := catalog.Get(name)
	if err != nil {
		return "", "", err
	}
	skillsDir := filepath.Join(workspace, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return "", "", err
	}
	outPath = filepath.Join(skillsDir, tmpl.Filename)
	return writeTextFileWithStatus(outPath, tmpl.Content, force)
}

func writeKitFile(workspace, relPath, content string, force bool) (status string, outPath string, err error) {
	outPath, err = secureJoin(workspace, relPath)
	if err != nil {
		return "", "", err
	}
	parent := filepath.Dir(outPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", "", err
	}
	return writeTextFileWithStatus(outPath, content, force)
}

func writeTextFileWithStatus(path, content string, force bool) (status string, outPath string, err error) {
	outPath = path
	newBytes := []byte(content)
	oldBytes, readErr := os.ReadFile(path)
	if readErr == nil {
		if string(oldBytes) == content {
			return "unchanged", outPath, nil
		}
		if !force {
			return "skipped", outPath, nil
		}
		if err := os.WriteFile(path, newBytes, 0o644); err != nil {
			return "", "", err
		}
		return "updated", outPath, nil
	}
	if !os.IsNotExist(readErr) {
		return "", "", readErr
	}
	if err := os.WriteFile(path, newBytes, 0o644); err != nil {
		return "", "", err
	}
	return "created", outPath, nil
}

func secureJoin(workspace, relPath string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(relPath))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("invalid relative path %q", relPath)
	}
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("kit file path must be relative, got %q", relPath)
	}
	dst := filepath.Join(workspace, clean)
	rel, err := filepath.Rel(workspace, dst)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("kit file path escapes workspace: %q", relPath)
	}
	return dst, nil
}

func ensureKitDeepseekProvider(root *project.RootConfig, name, model string, setDefault bool) (msg string, changed bool, err error) {
	if strings.TrimSpace(model) == "" {
		model = "deepseek-chat"
	}
	cfg := project.ProviderConfig{
		Name:      name,
		Type:      "deepseek",
		BaseURL:   "https://api.deepseek.com",
		APIKeyEnv: "DEEPSEEK_API_KEY",
		Model:     model,
		TimeoutMS: 30000,
	}
	idx := -1
	for i := range root.Providers {
		if root.Providers[i].Name == name {
			idx = i
			break
		}
	}
	action := "kept"
	if idx < 0 {
		root.Providers = append(root.Providers, cfg)
		idx = len(root.Providers) - 1
		action = "added"
		changed = true
	}

	existing := root.Providers[idx]
	if strings.TrimSpace(existing.Type) == "" {
		existing.Type = "deepseek"
		changed = true
	}
	if existing.Type != "deepseek" {
		return "", false, fmt.Errorf("provider %q exists with type %q (expected deepseek)", name, existing.Type)
	}
	if strings.TrimSpace(existing.BaseURL) == "" {
		existing.BaseURL = cfg.BaseURL
		changed = true
	}
	if strings.TrimSpace(existing.APIKeyEnv) == "" {
		existing.APIKeyEnv = cfg.APIKeyEnv
		changed = true
	}
	if strings.TrimSpace(existing.Model) == "" {
		existing.Model = cfg.Model
		changed = true
	}
	if existing.TimeoutMS <= 0 {
		existing.TimeoutMS = cfg.TimeoutMS
		changed = true
	}
	root.Providers[idx] = existing

	defaultInfo := ""
	if setDefault && strings.TrimSpace(root.DefaultProvider) != name {
		root.DefaultProvider = name
		changed = true
		defaultInfo = " + default_provider"
	}
	return fmt.Sprintf("%s deepseek provider %q%s", action, name, defaultInfo), changed, nil
}

func guessKitCrewName(kit catalog.KitTemplate) string {
	for _, f := range kit.Files {
		if strings.HasPrefix(f.Path, "crews/") && strings.HasSuffix(f.Path, ".yaml") {
			base := filepath.Base(f.Path)
			return strings.TrimSuffix(base, ".yaml")
		}
	}
	return "reviewpack"
}
