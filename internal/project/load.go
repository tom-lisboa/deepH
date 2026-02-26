package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const RootConfigFile = "deeph.yaml"

func Load(workspace string) (*Project, error) {
	rootPath := filepath.Join(workspace, RootConfigFile)
	b, err := os.ReadFile(rootPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rootPath, err)
	}

	var root RootConfig
	if err := yaml.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", rootPath, err)
	}
	if root.Version == 0 {
		root.Version = 1
	}

	agents, agentFiles, err := loadDir[AgentConfig](filepath.Join(workspace, "agents"))
	if err != nil {
		return nil, err
	}
	skills, skillFiles, err := loadDir[SkillConfig](filepath.Join(workspace, "skills"))
	if err != nil {
		return nil, err
	}

	return &Project{
		Root:       root,
		Agents:     agents,
		Skills:     skills,
		AgentFiles: agentFiles,
		SkillFiles: skillFiles,
	}, nil
}

func loadDir[T any](dir string) ([]T, map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, map[string]string{}, nil
		}
		return nil, nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	items := make([]T, 0, len(entries))
	files := map[string]string{}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", path, err)
		}
		var item T
		if err := yaml.Unmarshal(b, &item); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", path, err)
		}
		items = append(items, item)
		if named, ok := any(item).(interface{ GetName() string }); ok {
			files[named.GetName()] = path
		}
	}
	return items, files, nil
}

func (a AgentConfig) GetName() string { return a.Name }
func (s SkillConfig) GetName() string { return s.Name }
