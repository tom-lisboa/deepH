package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"deeph/internal/catalog"
	"deeph/internal/project"
	"deeph/internal/scaffold"
)

func cmdQuickstart(args []string) error {
	fs := flag.NewFlagSet("quickstart", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	agentName := fs.String("agent", "guide", "starter agent name")
	provider := fs.String("provider", "", "starter agent provider override")
	model := fs.String("model", "", "starter agent model override")
	withEcho := fs.Bool("with-echo", true, "install echo skill template")
	withDeepSeek := fs.Bool("deepseek", false, "add deepseek provider scaffold and set it as default")
	force := fs.Bool("force", false, "overwrite starter files when they already exist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*agentName) == "" {
		return fmt.Errorf("--agent cannot be empty")
	}

	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	if err := scaffold.InitWorkspace(abs); err != nil {
		return err
	}
	fmt.Printf("Workspace ready at %s\n", abs)

	if *withEcho {
		created, err := ensureSkillTemplate(abs, "echo", *force)
		if err != nil {
			return err
		}
		if created {
			fmt.Println("Installed skill template: echo")
		} else {
			fmt.Println("Skill template already present: echo")
		}
	}
	for _, skillName := range []string{"file_read_range", "file_write_safe"} {
		created, err := ensureSkillTemplate(abs, skillName, *force)
		if err != nil {
			return err
		}
		if created {
			fmt.Printf("Installed skill template: %s\n", skillName)
		} else {
			fmt.Printf("Skill template already present: %s\n", skillName)
		}
	}
	if strings.EqualFold(strings.TrimSpace(*agentName), "guide") {
		created, err := ensureSkillTemplate(abs, "command_doc", *force)
		if err != nil {
			return err
		}
		if created {
			fmt.Println("Installed skill template: command_doc")
		} else {
			fmt.Println("Skill template already present: command_doc")
		}
	}

	selectedProvider := strings.TrimSpace(*provider)
	selectedModel := strings.TrimSpace(*model)

	if *withDeepSeek {
		if selectedProvider == "" {
			selectedProvider = "deepseek"
		}
		if selectedModel == "" {
			selectedModel = "deepseek-chat"
		}
		if err := ensureDeepSeekProvider(abs, selectedProvider, selectedModel, *force); err != nil {
			return err
		}
	} else if selectedProvider == "" {
		if p, err := project.Load(abs); err == nil {
			selectedProvider = strings.TrimSpace(p.Root.DefaultProvider)
		}
	}

	if selectedModel == "" {
		if strings.EqualFold(selectedProvider, "deepseek") {
			selectedModel = "deepseek-chat"
		} else {
			selectedModel = "mock-small"
		}
	}

	agentCreated, err := ensureAgentTemplate(abs, strings.TrimSpace(*agentName), selectedProvider, selectedModel, *force)
	if err != nil {
		return err
	}
	if agentCreated {
		fmt.Printf("Created agent template: %s\n", filepath.Join(abs, "agents", strings.TrimSpace(*agentName)+".yaml"))
	} else {
		fmt.Printf("Agent already present: %s\n", filepath.Join(abs, "agents", strings.TrimSpace(*agentName)+".yaml"))
	}
	if strings.EqualFold(strings.TrimSpace(*agentName), "guide") {
		if err := ensureStarterCodePack(abs, selectedProvider, selectedModel, *force); err != nil {
			return err
		}
	}

	if err := cmdValidate([]string{"--workspace", abs}); err != nil {
		return err
	}
	saveStudioRecent(abs, strings.TrimSpace(*agentName), "")

	fmt.Println("Quickstart complete.")
	fmt.Println("Try:")
	fmt.Printf("  deeph run %s \"hello\"\n", strings.TrimSpace(*agentName))
	if strings.EqualFold(strings.TrimSpace(*agentName), "guide") {
		fmt.Println("  deeph diagnose \"paste the error or stack trace\"")
		fmt.Println("  deeph review")
		fmt.Println("  deeph edit \"analyze and update the relevant file\"")
	}
	if strings.EqualFold(selectedProvider, "deepseek") {
		fmt.Println("  (set DEEPSEEK_API_KEY in your shell before running)")
	}
	return nil
}

func ensureSkillTemplate(workspace, skillName string, force bool) (bool, error) {
	tmpl, err := catalog.Get(skillName)
	if err != nil {
		return false, err
	}
	skillsDir := filepath.Join(workspace, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return false, err
	}
	outPath := filepath.Join(skillsDir, tmpl.Filename)
	if !force {
		if _, err := os.Stat(outPath); err == nil {
			return false, nil
		} else if err != nil && !os.IsNotExist(err) {
			return false, err
		}
	}
	if err := os.WriteFile(outPath, []byte(tmpl.Content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func ensureAgentTemplate(workspace, name, provider, model string, force bool) (bool, error) {
	outPath := filepath.Join(workspace, "agents", name+".yaml")
	if !force {
		if _, err := os.Stat(outPath); err == nil {
			return false, nil
		} else if err != nil && !os.IsNotExist(err) {
			return false, err
		}
	}
	create := scaffold.CreateAgentFile
	if strings.EqualFold(name, "guide") {
		create = scaffold.CreateGuideStarterFile
	}
	if _, err := create(workspace, scaffold.AgentTemplateOptions{
		Name:        name,
		Provider:    strings.TrimSpace(provider),
		Model:       strings.TrimSpace(model),
		Description: "Starter agent generated by deeph quickstart",
		Force:       force,
	}); err != nil {
		return false, err
	}
	return true, nil
}

func ensureDeepSeekProvider(workspace, providerName, model string, force bool) error {
	p, err := project.Load(workspace)
	if err != nil {
		return err
	}

	for i := range p.Root.Providers {
		cfg := p.Root.Providers[i]
		if cfg.Name != providerName {
			continue
		}
		if strings.TrimSpace(cfg.Type) != "deepseek" {
			return fmt.Errorf("provider %q exists with type %q (expected deepseek)", providerName, cfg.Type)
		}
		changed := false
		if strings.TrimSpace(p.Root.DefaultProvider) != providerName {
			p.Root.DefaultProvider = providerName
			changed = true
		}
		if force && strings.TrimSpace(model) != "" && strings.TrimSpace(cfg.Model) != strings.TrimSpace(model) {
			p.Root.Providers[i].Model = strings.TrimSpace(model)
			changed = true
		}
		if changed {
			if err := project.SaveRootConfig(workspace, p.Root); err != nil {
				return err
			}
			fmt.Printf("Updated provider %q as default in %s\n", providerName, filepath.Join(workspace, project.RootConfigFile))
		} else {
			fmt.Printf("Provider already present: %q\n", providerName)
		}
		return nil
	}

	providerArgs := []string{
		"--workspace", workspace,
		"--name", providerName,
		"--model", model,
		"--set-default",
	}
	if force {
		providerArgs = append(providerArgs, "--force")
	}
	providerArgs = append(providerArgs, "deepseek")
	return cmdProviderAdd(providerArgs)
}

func ensureStarterCodePack(workspace, provider, model string, force bool) error {
	starters := []struct {
		name   string
		create func(string, scaffold.AgentTemplateOptions) (string, error)
	}{
		{name: "coder", create: scaffold.CreateCoderStarterFile},
		{name: "diagnoser", create: scaffold.CreateDiagnoserStarterFile},
		{name: "reviewer", create: scaffold.CreateReviewerStarterFile},
		{name: "review_synth", create: scaffold.CreateReviewSynthStarterFile},
	}
	for _, starter := range starters {
		path := filepath.Join(workspace, "agents", starter.name+".yaml")
		if _, err := starter.create(workspace, scaffold.AgentTemplateOptions{
			Name:        starter.name,
			Provider:    strings.TrimSpace(provider),
			Model:       strings.TrimSpace(model),
			Description: "Starter agent generated by deeph quickstart",
			Force:       force,
		}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			fmt.Printf("Agent already present: %s\n", path)
		} else {
			fmt.Printf("Created agent template: %s\n", path)
		}
	}
	crewPath := filepath.Join(workspace, "crews", "reviewflow.yaml")
	if _, err := scaffold.CreateReviewflowCrewFile(workspace, force); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Printf("Crew already present: %s\n", crewPath)
	} else {
		fmt.Printf("Created crew template: %s\n", crewPath)
	}
	return nil
}
