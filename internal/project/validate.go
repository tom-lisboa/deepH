package project

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"deeph/internal/typesys"
)

var validProviderTypes = map[string]struct{}{
	"mock":      {},
	"http":      {},
	"openai":    {},
	"deepseek":  {},
	"anthropic": {},
	"ollama":    {},
}

var validSkillTypes = map[string]struct{}{
	"echo":            {},
	"file_read":       {},
	"file_read_range": {},
	"file_write_safe": {},
	"http":            {},
}

func Validate(p *Project) *ValidationError {
	issues := []Issue{}
	if p == nil {
		issues = append(issues, Issue{Level: IssueError, Path: RootConfigFile, Message: "project is nil"})
		return &ValidationError{Issues: issues}
	}

	if p.Root.Version <= 0 {
		issues = append(issues, Issue{Level: IssueError, Path: RootConfigFile, Field: "version", Message: "must be >= 1"})
	}

	providerByName := map[string]ProviderConfig{}
	for i, pc := range p.Root.Providers {
		path := fmt.Sprintf("%s.providers[%d]", RootConfigFile, i)
		if strings.TrimSpace(pc.Name) == "" {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "name", Message: "is required"})
			continue
		}
		if _, exists := providerByName[pc.Name]; exists {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "name", Message: "duplicate provider name"})
		}
		providerByName[pc.Name] = pc
		if _, ok := validProviderTypes[pc.Type]; !ok {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "type", Message: "unsupported provider type"})
		}
		if pc.TimeoutMS < 0 {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "timeout_ms", Message: "must be >= 0"})
		}
		if pc.Type == "http" && strings.TrimSpace(pc.BaseURL) == "" {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "base_url", Message: "is required for http provider"})
		}
		if pc.Type == "deepseek" && strings.TrimSpace(pc.APIKeyEnv) == "" {
			issues = append(issues, Issue{Level: IssueWarning, Path: path, Field: "api_key_env", Message: "empty value defaults to DEEPSEEK_API_KEY at runtime"})
		}
		if pc.Type == "deepseek" && strings.TrimSpace(pc.BaseURL) == "" {
			issues = append(issues, Issue{Level: IssueWarning, Path: path, Field: "base_url", Message: "empty value defaults to https://api.deepseek.com at runtime"})
		}
	}

	if p.Root.DefaultProvider != "" {
		if _, ok := providerByName[p.Root.DefaultProvider]; !ok {
			issues = append(issues, Issue{Level: IssueError, Path: RootConfigFile, Field: "default_provider", Message: "references unknown provider"})
		}
	}

	skillByName := map[string]SkillConfig{}
	seenSkillFiles := map[string]struct{}{}
	for _, sc := range p.Skills {
		path := p.SkillFiles[sc.Name]
		if path == "" {
			path = filepath.Join("skills", sc.Name+".yaml")
		}
		path = filepath.Clean(path)
		if strings.TrimSpace(sc.Name) == "" {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "name", Message: "is required"})
			continue
		}
		if _, ok := seenSkillFiles[sc.Name]; ok {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "name", Message: "duplicate skill name"})
		}
		seenSkillFiles[sc.Name] = struct{}{}
		skillByName[sc.Name] = sc
		if _, ok := validSkillTypes[sc.Type]; !ok {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "type", Message: "unsupported skill type"})
		}
		if sc.TimeoutMS < 0 {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "timeout_ms", Message: "must be >= 0"})
		}
		if sc.Type == "http" && strings.TrimSpace(sc.Method) == "" {
			issues = append(issues, Issue{Level: IssueWarning, Path: path, Field: "method", Message: "empty method defaults to GET"})
		}
		if sc.Type == "file_read" || sc.Type == "file_read_range" || sc.Type == "file_write_safe" {
			if maxBytes, ok := intParam(sc.Params, "max_bytes"); ok && maxBytes <= 0 {
				issues = append(issues, Issue{Level: IssueError, Path: path, Field: "params.max_bytes", Message: "must be > 0"})
			}
		}
	}

	seenAgents := map[string]struct{}{}
	agentByName := map[string]AgentConfig{}
	for _, ac := range p.Agents {
		path := p.AgentFiles[ac.Name]
		if path == "" {
			path = filepath.Join("agents", ac.Name+".yaml")
		}
		if strings.TrimSpace(ac.Name) == "" {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "name", Message: "is required"})
			continue
		}
		if _, ok := seenAgents[ac.Name]; ok {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "name", Message: "duplicate agent name"})
		}
		seenAgents[ac.Name] = struct{}{}
		agentByName[ac.Name] = ac
		if ac.TimeoutMS < 0 {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "timeout_ms", Message: "must be >= 0"})
		}
		providerName := ac.Provider
		if providerName == "" {
			providerName = p.Root.DefaultProvider
		}
		if providerName == "" {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "provider", Message: "is required when no default_provider is set"})
		} else if _, ok := providerByName[providerName]; !ok {
			issues = append(issues, Issue{Level: IssueError, Path: path, Field: "provider", Message: "references unknown provider"})
		}
		for i, skill := range ac.Skills {
			if _, ok := skillByName[skill]; !ok {
				issues = append(issues, Issue{Level: IssueError, Path: path, Field: fmt.Sprintf("skills[%d]", i), Message: "references unknown skill"})
			}
		}
		seenDeps := map[string]struct{}{}
		for i, dep := range ac.DependsOn {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				issues = append(issues, Issue{Level: IssueError, Path: path, Field: fmt.Sprintf("depends_on[%d]", i), Message: "dependency name cannot be empty"})
				continue
			}
			if dep == ac.Name {
				issues = append(issues, Issue{Level: IssueError, Path: path, Field: fmt.Sprintf("depends_on[%d]", i), Message: "agent cannot depend on itself"})
			}
			if _, ok := seenDeps[dep]; ok {
				issues = append(issues, Issue{Level: IssueWarning, Path: path, Field: fmt.Sprintf("depends_on[%d]", i), Message: "duplicate dependency entry"})
				continue
			}
			seenDeps[dep] = struct{}{}
		}
		for i, call := range ac.StartupCalls {
			if strings.TrimSpace(call.Skill) == "" {
				issues = append(issues, Issue{Level: IssueError, Path: path, Field: fmt.Sprintf("startup_calls[%d].skill", i), Message: "is required"})
				continue
			}
			if _, ok := skillByName[call.Skill]; !ok {
				issues = append(issues, Issue{Level: IssueError, Path: path, Field: fmt.Sprintf("startup_calls[%d].skill", i), Message: "references unknown skill"})
			}
		}
		validateAgentIO(&issues, path, ac)
		validateAgentDependsOnPorts(&issues, path, ac)
		if metadataBool(ac.Metadata, "strict_types") && len(ac.IO.Inputs) == 0 && len(ac.IO.Outputs) == 0 {
			issues = append(issues, Issue{Level: IssueWarning, Path: path, Field: "metadata.strict_types", Message: "strict_types=true but io.inputs/io.outputs are empty"})
		}
	}
	for _, ac := range p.Agents {
		path := p.AgentFiles[ac.Name]
		if path == "" {
			path = filepath.Join("agents", ac.Name+".yaml")
		}
		for i, dep := range ac.DependsOn {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			if _, ok := agentByName[dep]; !ok {
				issues = append(issues, Issue{Level: IssueError, Path: path, Field: fmt.Sprintf("depends_on[%d]", i), Message: "references unknown agent"})
			}
		}
		validateAgentDependsOnPortsRefs(&issues, path, ac, agentByName)
	}
	for _, cycle := range findAgentDependencyCycles(agentByName) {
		if len(cycle) == 0 {
			continue
		}
		root := cycle[0]
		path := p.AgentFiles[root]
		if path == "" {
			path = filepath.Join("agents", root+".yaml")
		}
		issues = append(issues, Issue{
			Level:   IssueError,
			Path:    path,
			Field:   "depends_on",
			Message: "dependency cycle detected: " + strings.Join(cycle, " -> "),
		})
	}

	if len(p.Agents) > 0 && len(p.Root.Providers) == 0 {
		issues = append(issues, Issue{Level: IssueError, Path: RootConfigFile, Field: "providers", Message: "at least one provider is required when agents exist"})
	}

	if len(issues) == 0 {
		return nil
	}
	return &ValidationError{Issues: issues}
}

func findAgentDependencyCycles(agentByName map[string]AgentConfig) [][]string {
	if len(agentByName) == 0 {
		return nil
	}
	const (
		stateUnvisited = 0
		stateVisiting  = 1
		stateDone      = 2
	)
	state := map[string]int{}
	stack := make([]string, 0, len(agentByName))
	indexInStack := map[string]int{}
	cycles := make([][]string, 0)
	seenCycleKeys := map[string]struct{}{}

	var visit func(string)
	visit = func(name string) {
		switch state[name] {
		case stateDone:
			return
		case stateVisiting:
			return
		}
		state[name] = stateVisiting
		indexInStack[name] = len(stack)
		stack = append(stack, name)

		for _, dep := range listAgentDependencyNames(agentByName[name]) {
			if dep == "" {
				continue
			}
			if _, ok := agentByName[dep]; !ok {
				continue
			}
			if state[dep] == stateVisiting {
				start := indexInStack[dep]
				cycle := append([]string(nil), stack[start:]...)
				cycle = append(cycle, dep)
				key := strings.Join(cycle, "->")
				if _, ok := seenCycleKeys[key]; !ok {
					seenCycleKeys[key] = struct{}{}
					cycles = append(cycles, cycle)
				}
				continue
			}
			visit(dep)
		}

		stack = stack[:len(stack)-1]
		delete(indexInStack, name)
		state[name] = stateDone
	}

	names := make([]string, 0, len(agentByName))
	for name := range agentByName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		visit(name)
	}
	return cycles
}

func metadataBool(meta map[string]string, key string) bool {
	if meta == nil {
		return false
	}
	v, ok := meta[key]
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func validateAgentIO(issues *[]Issue, path string, ac AgentConfig) {
	validateIOPorts(issues, path, "io.inputs", ac.IO.Inputs, true)
	validateIOPorts(issues, path, "io.outputs", ac.IO.Outputs, false)
}

func validateAgentDependsOnPorts(issues *[]Issue, path string, ac AgentConfig) {
	if len(ac.DependsOnPorts) == 0 {
		return
	}
	inputPortNames := map[string]struct{}{}
	for _, p := range ac.IO.Inputs {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		inputPortNames[name] = struct{}{}
	}
	if len(ac.IO.Inputs) == 0 {
		*issues = append(*issues, Issue{
			Level:   IssueWarning,
			Path:    path,
			Field:   "depends_on_ports",
			Message: "defined without io.inputs; port-level routing will be limited (consider adding io.inputs)",
		})
	}

	keys := make([]string, 0, len(ac.DependsOnPorts))
	for port := range ac.DependsOnPorts {
		keys = append(keys, port)
	}
	sort.Strings(keys)
	for _, rawPort := range keys {
		port := strings.TrimSpace(rawPort)
		field := fmt.Sprintf("depends_on_ports.%s", rawPort)
		if port == "" {
			*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: field, Message: "input port key cannot be empty"})
			continue
		}
		if len(inputPortNames) > 0 {
			if _, ok := inputPortNames[port]; !ok {
				*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: field, Message: "references unknown io.inputs port"})
			}
		}
		selectors := ac.DependsOnPorts[rawPort]
		if len(selectors) == 0 {
			*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: field, Message: "must contain at least one selector (agent or agent.port)"})
			continue
		}
		seenSel := map[string]struct{}{}
		for i, selRaw := range selectors {
			sel, err := parseAgentPortSelector(selRaw)
			if err != nil {
				*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: fmt.Sprintf("%s[%d]", field, i), Message: err.Error()})
				continue
			}
			if sel.Agent == ac.Name {
				*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: fmt.Sprintf("%s[%d]", field, i), Message: "agent cannot depend on itself via depends_on_ports"})
			}
			key := sel.Agent + "." + sel.Port
			if _, ok := seenSel[key]; ok {
				*issues = append(*issues, Issue{Level: IssueWarning, Path: path, Field: fmt.Sprintf("%s[%d]", field, i), Message: "duplicate selector"})
				continue
			}
			seenSel[key] = struct{}{}
		}
	}
}

func validateAgentDependsOnPortsRefs(issues *[]Issue, path string, ac AgentConfig, agentByName map[string]AgentConfig) {
	if len(ac.DependsOnPorts) == 0 {
		return
	}
	for rawPort, selectors := range ac.DependsOnPorts {
		for i, selRaw := range selectors {
			sel, err := parseAgentPortSelector(selRaw)
			if err != nil {
				continue
			}
			srcAgent, ok := agentByName[sel.Agent]
			if !ok {
				*issues = append(*issues, Issue{
					Level:   IssueError,
					Path:    path,
					Field:   fmt.Sprintf("depends_on_ports.%s[%d]", rawPort, i),
					Message: "references unknown agent",
				})
				continue
			}
			if sel.Port == "" {
				continue
			}
			if len(srcAgent.IO.Outputs) == 0 {
				*issues = append(*issues, Issue{
					Level:   IssueWarning,
					Path:    path,
					Field:   fmt.Sprintf("depends_on_ports.%s[%d]", rawPort, i),
					Message: "references source port but upstream agent has no io.outputs contract",
				})
				continue
			}
			if !agentHasOutputPort(srcAgent, sel.Port) {
				*issues = append(*issues, Issue{
					Level:   IssueError,
					Path:    path,
					Field:   fmt.Sprintf("depends_on_ports.%s[%d]", rawPort, i),
					Message: "references unknown upstream io.outputs port",
				})
			}
		}
	}
}

func listAgentDependencyNames(ac AgentConfig) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ac.DependsOn)+len(ac.DependsOnPorts))
	for _, depRaw := range ac.DependsOn {
		dep := strings.TrimSpace(depRaw)
		if dep == "" {
			continue
		}
		if _, ok := seen[dep]; ok {
			continue
		}
		seen[dep] = struct{}{}
		out = append(out, dep)
	}
	for _, selectors := range ac.DependsOnPorts {
		for _, selRaw := range selectors {
			sel, err := parseAgentPortSelector(selRaw)
			if err != nil {
				continue
			}
			if sel.Agent == "" {
				continue
			}
			if _, ok := seen[sel.Agent]; ok {
				continue
			}
			seen[sel.Agent] = struct{}{}
			out = append(out, sel.Agent)
		}
	}
	sort.Strings(out)
	return out
}

type agentPortSelector struct {
	Agent string
	Port  string
}

func parseAgentPortSelector(raw string) (agentPortSelector, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return agentPortSelector{}, fmt.Errorf("selector cannot be empty (use agent or agent.port)")
	}
	if strings.Contains(raw, " ") {
		return agentPortSelector{}, fmt.Errorf("selector %q must not contain spaces", raw)
	}
	parts := strings.Split(raw, ".")
	switch len(parts) {
	case 1:
		if parts[0] == "" {
			return agentPortSelector{}, fmt.Errorf("invalid selector %q", raw)
		}
		return agentPortSelector{Agent: parts[0]}, nil
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return agentPortSelector{}, fmt.Errorf("invalid selector %q (expected agent.port)", raw)
		}
		return agentPortSelector{Agent: parts[0], Port: parts[1]}, nil
	default:
		return agentPortSelector{}, fmt.Errorf("invalid selector %q (expected agent or agent.port)", raw)
	}
}

func agentHasOutputPort(ac AgentConfig, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, p := range ac.IO.Outputs {
		if strings.TrimSpace(p.Name) == want {
			return true
		}
	}
	return false
}

func validateIOPorts(issues *[]Issue, path, fieldPrefix string, ports []IOPortConfig, isInput bool) {
	seen := map[string]struct{}{}
	for i, p := range ports {
		portField := fmt.Sprintf("%s[%d]", fieldPrefix, i)
		name := strings.TrimSpace(p.Name)
		if name == "" {
			*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: portField + ".name", Message: "is required"})
			continue
		}
		if _, ok := seen[name]; ok {
			*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: portField + ".name", Message: "duplicate port name"})
		}
		seen[name] = struct{}{}
		if p.MaxTokens < 0 {
			*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: portField + ".max_tokens", Message: "must be >= 0"})
		}
		if math.IsNaN(p.ChannelPriority) || math.IsInf(p.ChannelPriority, 0) {
			*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: portField + ".channel_priority", Message: "must be a finite number"})
		}
		if p.ChannelPriority < 0 {
			*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: portField + ".channel_priority", Message: "must be >= 0"})
		}
		if p.ChannelPriority > 0 && !isInput {
			*issues = append(*issues, Issue{Level: IssueWarning, Path: path, Field: portField + ".channel_priority", Message: "channel_priority is only used for io.inputs handoff publish ordering"})
		}
		if strings.TrimSpace(p.MergePolicy) != "" {
			if !isInput {
				*issues = append(*issues, Issue{Level: IssueWarning, Path: path, Field: portField + ".merge_policy", Message: "merge_policy is only used for io.inputs handoff merge"})
			} else if _, ok := normalizePortMergePolicy(p.MergePolicy); !ok {
				*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: portField + ".merge_policy", Message: "unsupported merge_policy (use auto|latest|append2|append3|append4)"})
			}
		}

		kinds := p.Accepts
		kindField := ".accepts"
		if !isInput {
			kinds = p.Produces
			kindField = ".produces"
		}
		if len(kinds) == 0 {
			*issues = append(*issues, Issue{Level: IssueWarning, Path: path, Field: portField + kindField, Message: "empty list means no explicit type contract"})
			continue
		}
		for j, raw := range kinds {
			if _, ok := typesys.NormalizeKind(raw); !ok {
				*issues = append(*issues, Issue{Level: IssueError, Path: path, Field: fmt.Sprintf("%s%s[%d]", portField, kindField, j), Message: "unknown type kind"})
			}
		}
	}
}

func normalizePortMergePolicy(raw string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "", "auto":
		return "auto", true
	case "latest":
		return "latest", true
	case "append", "append3":
		return "append3", true
	case "append2":
		return "append2", true
	case "append4":
		return "append4", true
	default:
		return "", false
	}
}

func intParam(params map[string]any, key string) (int, bool) {
	if params == nil {
		return 0, false
	}
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
