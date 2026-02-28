package main

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"deeph/internal/project"
)

type studioSessionOption struct {
	ID   string
	Spec string
}

func listWorkspaceAgentNames(workspace string) []string {
	p, err := project.Load(workspace)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(p.Agents))
	for _, agent := range p.Agents {
		name := strings.TrimSpace(agent.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func listWorkspaceSessionOptions(workspace string, limit int) []studioSessionOption {
	metas, err := listChatSessionMetas(workspace)
	if err != nil || len(metas) == 0 {
		return nil
	}
	if limit > 0 && len(metas) > limit {
		metas = metas[:limit]
	}
	out := make([]studioSessionOption, 0, len(metas))
	for _, meta := range metas {
		out = append(out, studioSessionOption{
			ID:   meta.ID,
			Spec: meta.AgentSpec,
		})
	}
	return out
}

func promptAgentSpec(reader *bufio.Reader, workspace, label, def string) (string, error) {
	agents := listWorkspaceAgentNames(workspace)
	if len(agents) == 0 {
		return promptLine(reader, label, def)
	}
	fmt.Println("Available agents:")
	for i, name := range agents {
		fmt.Printf("  %d) %s\n", i+1, name)
	}
	answer, err := promptLine(reader, label+" (name or number)", def)
	if err != nil {
		return "", err
	}
	return resolvePickerInput(answer, agents, def), nil
}

func promptSessionID(reader *bufio.Reader, workspace, label, def string) (string, error) {
	sessions := listWorkspaceSessionOptions(workspace, 5)
	if len(sessions) == 0 {
		return promptLine(reader, label, def)
	}
	fmt.Println("Recent sessions:")
	for i, sess := range sessions {
		spec := strings.TrimSpace(sess.Spec)
		if spec == "" {
			spec = "unknown"
		}
		fmt.Printf("  %d) %s (%s)\n", i+1, sess.ID, spec)
	}
	options := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		options = append(options, sess.ID)
	}
	answer, err := promptLine(reader, label+" (blank, id or number)", def)
	if err != nil {
		return "", err
	}
	return resolvePickerInput(answer, options, def), nil
}

func promptWorkspaceChoice(reader *bufio.Reader, currentWorkspace, lastWorkspace string) (string, error) {
	options := make([]string, 0, 2)
	seen := map[string]struct{}{}
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		key := strings.ToLower(abs)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		options = append(options, abs)
	}
	add(currentWorkspace)
	add(lastWorkspace)
	if len(options) > 0 {
		fmt.Println("Workspace options:")
		for i, option := range options {
			fmt.Printf("  %d) %s\n", i+1, option)
		}
	}
	answer, err := promptLine(reader, "Workspace (path or number)", currentWorkspace)
	if err != nil {
		return "", err
	}
	selected := resolvePickerInput(answer, options, currentWorkspace)
	return filepath.Abs(selected)
}

func resolvePickerInput(answer string, options []string, def string) string {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return def
	}
	if idx, err := strconv.Atoi(answer); err == nil {
		if idx >= 1 && idx <= len(options) {
			return options[idx-1]
		}
	}
	return answer
}

