package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"deeph/internal/typesys"
	"gopkg.in/yaml.v3"
)

type crewConfig struct {
	Name        string         `yaml:"name" json:"name"`
	Description string         `yaml:"description" json:"description,omitempty"`
	Spec        string         `yaml:"spec" json:"spec"`
	Universes   []crewUniverse `yaml:"universes" json:"universes,omitempty"`
}

type crewUniverse struct {
	Name            string   `yaml:"name" json:"name"`
	Description     string   `yaml:"description" json:"description,omitempty"`
	Spec            string   `yaml:"spec" json:"spec"`
	InputPrefix     string   `yaml:"input_prefix" json:"input_prefix,omitempty"`
	InputSuffix     string   `yaml:"input_suffix" json:"input_suffix,omitempty"`
	DependsOn       []string `yaml:"depends_on" json:"depends_on,omitempty"`
	InputPort       string   `yaml:"input_port" json:"input_port,omitempty"`
	OutputPort      string   `yaml:"output_port" json:"output_port,omitempty"`
	OutputKind      string   `yaml:"output_kind" json:"output_kind,omitempty"`
	MergePolicy     string   `yaml:"merge_policy" json:"merge_policy,omitempty"`
	HandoffMaxChars int      `yaml:"handoff_max_chars" json:"handoff_max_chars,omitempty"`
}

func cmdCrew(args []string) error {
	if len(args) == 0 {
		return errors.New("crew requires a subcommand: list or show")
	}
	switch args[0] {
	case "list":
		return cmdCrewList(args[1:])
	case "show":
		return cmdCrewShow(args[1:])
	default:
		return fmt.Errorf("unknown crew subcommand %q", args[0])
	}
}

func cmdCrewList(args []string) error {
	fs := flag.NewFlagSet("crew list", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("crew list does not accept positional arguments")
	}
	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "crew list")
	crews, err := listCrewConfigs(abs)
	if err != nil {
		return err
	}
	if len(crews) == 0 {
		fmt.Printf("No crews found in %s\n", filepath.Join(abs, "crews"))
		return nil
	}
	for _, c := range crews {
		u := ""
		if len(c.Universes) > 0 {
			u = fmt.Sprintf(" universes=%d", len(c.Universes))
		}
		desc := strings.TrimSpace(c.Description)
		if desc != "" {
			fmt.Printf("- %s%s: %s\n", c.Name, u, desc)
		} else {
			fmt.Printf("- %s%s spec=%q\n", c.Name, u, c.Spec)
		}
	}
	return nil
}

func cmdCrewShow(args []string) error {
	fs := flag.NewFlagSet("crew show", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return errors.New("crew show requires <name>")
	}
	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	crew, path, err := loadCrewConfig(abs, rest[0])
	if err != nil {
		return err
	}
	recordCoachCommandTransition(abs, "crew show", crew.Spec)
	fmt.Printf("crew: %s\n", crew.Name)
	fmt.Printf("file: %s\n", path)
	if crew.Description != "" {
		fmt.Printf("description: %s\n", crew.Description)
	}
	fmt.Printf("spec: %s\n", crew.Spec)
	if len(crew.Universes) == 0 {
		fmt.Println("universes: (none)")
		return nil
	}
	fmt.Printf("universes: %d\n", len(crew.Universes))
	for i, u := range crew.Universes {
		fmt.Printf("- [%d] %s spec=%q\n", i, u.Name, u.Spec)
		if strings.TrimSpace(u.Description) != "" {
			fmt.Printf("    desc: %s\n", u.Description)
		}
		if strings.TrimSpace(u.InputPrefix) != "" {
			fmt.Printf("    input_prefix: %q\n", u.InputPrefix)
		}
		if strings.TrimSpace(u.InputSuffix) != "" {
			fmt.Printf("    input_suffix: %q\n", u.InputSuffix)
		}
		if len(u.DependsOn) > 0 {
			fmt.Printf("    depends_on: %v\n", u.DependsOn)
		}
		if strings.TrimSpace(u.InputPort) != "" || strings.TrimSpace(u.OutputPort) != "" {
			in := strings.TrimSpace(u.InputPort)
			out := strings.TrimSpace(u.OutputPort)
			if in == "" {
				in = "context"
			}
			if out == "" {
				out = "result"
			}
			fmt.Printf("    ports: %s <- %s\n", in, out)
		}
		if strings.TrimSpace(u.OutputKind) != "" {
			fmt.Printf("    output_kind: %s\n", u.OutputKind)
		}
		if strings.TrimSpace(u.MergePolicy) != "" {
			fmt.Printf("    merge_policy: %s\n", u.MergePolicy)
		}
		if u.HandoffMaxChars > 0 {
			fmt.Printf("    handoff_max_chars: %d\n", u.HandoffMaxChars)
		}
	}
	return nil
}

func crewDir(workspace string) string {
	return filepath.Join(workspace, "crews")
}

func listCrewConfigs(workspace string) ([]crewConfig, error) {
	ents, err := os.ReadDir(crewDir(workspace))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]crewConfig, 0, len(ents))
	for _, ent := range ents {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !(strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			continue
		}
		c, _, err := loadCrewConfigByPath(filepath.Join(crewDir(workspace), name))
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func loadCrewConfig(workspace, name string) (crewConfig, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return crewConfig{}, "", errors.New("crew name is empty")
	}
	if strings.ContainsAny(name, `/\`) {
		return crewConfig{}, "", fmt.Errorf("invalid crew name %q", name)
	}
	candidates := []string{
		filepath.Join(crewDir(workspace), name+".yaml"),
		filepath.Join(crewDir(workspace), name+".yml"),
	}
	for _, p := range candidates {
		c, path, err := loadCrewConfigByPath(p)
		if err == nil {
			return c, path, nil
		}
		if !os.IsNotExist(err) {
			return crewConfig{}, "", err
		}
	}
	return crewConfig{}, "", fmt.Errorf("unknown crew %q (tip: create %s/%s.yaml)", name, crewDir(workspace), name)
}

func loadCrewConfigByPath(path string) (crewConfig, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return crewConfig{}, "", err
	}
	var c crewConfig
	if err := yaml.Unmarshal(b, &c); err != nil {
		return crewConfig{}, "", fmt.Errorf("parse %s: %w", path, err)
	}
	if strings.TrimSpace(c.Name) == "" {
		base := filepath.Base(path)
		c.Name = strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml")
	}
	if strings.TrimSpace(c.Spec) == "" && len(c.Universes) > 0 {
		c.Spec = strings.TrimSpace(c.Universes[0].Spec)
	}
	if strings.TrimSpace(c.Spec) == "" {
		return crewConfig{}, "", fmt.Errorf("crew %q missing spec", c.Name)
	}
	for i := range c.Universes {
		if strings.TrimSpace(c.Universes[i].Name) == "" {
			c.Universes[i].Name = fmt.Sprintf("u%d", i+1)
		}
		if strings.TrimSpace(c.Universes[i].Spec) == "" {
			c.Universes[i].Spec = c.Spec
		}
		if strings.TrimSpace(c.Universes[i].InputPort) == "" {
			c.Universes[i].InputPort = "context"
		}
		if strings.TrimSpace(c.Universes[i].OutputPort) == "" {
			c.Universes[i].OutputPort = "result"
		}
		if strings.TrimSpace(c.Universes[i].OutputKind) == "" {
			c.Universes[i].OutputKind = string(typesys.KindSummaryText)
		} else if k, ok := typesys.NormalizeKind(c.Universes[i].OutputKind); ok {
			c.Universes[i].OutputKind = k.String()
		} else {
			return crewConfig{}, "", fmt.Errorf("crew %q universe %q has unknown output_kind %q", c.Name, c.Universes[i].Name, c.Universes[i].OutputKind)
		}
		if strings.TrimSpace(c.Universes[i].MergePolicy) == "" {
			c.Universes[i].MergePolicy = "append"
		}
	}
	return c, path, nil
}

func resolveAgentSpecOrCrew(workspace, raw string) (resolvedSpec string, crew *crewConfig, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, errors.New("empty agent spec")
	}
	if strings.HasPrefix(raw, "crew:") {
		name := strings.TrimSpace(strings.TrimPrefix(raw, "crew:"))
		c, _, err := loadCrewConfig(workspace, name)
		if err != nil {
			return "", nil, err
		}
		return c.Spec, &c, nil
	}
	if strings.HasPrefix(raw, "@") && len(raw) > 1 {
		name := strings.TrimSpace(strings.TrimPrefix(raw, "@"))
		c, _, err := loadCrewConfig(workspace, name)
		if err != nil {
			return "", nil, err
		}
		return c.Spec, &c, nil
	}
	return raw, nil, nil
}
