package runtime

import (
	"fmt"
	"sort"
	"strings"

	"deeph/internal/project"
	"deeph/internal/typesys"
)

func ParseAgentSpecGraph(spec string) (AgentSpecGraph, error) {
	raw := strings.TrimSpace(spec)
	if raw == "" {
		return AgentSpecGraph{}, fmt.Errorf("empty agent spec")
	}

	stageParts := strings.Split(raw, ">")
	stages := make([][]string, 0, len(stageParts))
	seen := map[string]struct{}{}
	for stageIdx, part := range stageParts {
		part = strings.TrimSpace(part)
		if part == "" {
			return AgentSpecGraph{}, fmt.Errorf("invalid agent spec %q: empty stage near '>'", spec)
		}
		names := strings.Split(part, "+")
		stage := make([]string, 0, len(names))
		for _, n := range names {
			name := strings.TrimSpace(n)
			if name == "" {
				return AgentSpecGraph{}, fmt.Errorf("invalid agent spec %q: empty agent near '+'", spec)
			}
			if strings.ContainsAny(name, " >+") {
				return AgentSpecGraph{}, fmt.Errorf("invalid agent name %q in spec %q", name, spec)
			}
			if _, ok := seen[name]; ok {
				return AgentSpecGraph{}, fmt.Errorf("duplicate agent %q in spec %q", name, spec)
			}
			seen[name] = struct{}{}
			stage = append(stage, name)
		}
		if len(stage) == 0 {
			return AgentSpecGraph{}, fmt.Errorf("invalid agent spec %q: stage[%d] is empty", spec, stageIdx)
		}
		stages = append(stages, stage)
	}
	return AgentSpecGraph{Raw: raw, Stages: stages}, nil
}

func (g AgentSpecGraph) Flatten() []string {
	total := 0
	for _, s := range g.Stages {
		total += len(s)
	}
	out := make([]string, 0, total)
	for _, s := range g.Stages {
		out = append(out, s...)
	}
	return out
}

func SplitAgentSpec(spec string) []string {
	g, err := ParseAgentSpecGraph(spec)
	if err == nil {
		return g.Flatten()
	}
	// Backward-compatible fallback for malformed input (caller will fail on unknown agent later).
	parts := strings.FieldsFunc(spec, func(r rune) bool { return r == '+' || r == '>' })
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func inferTaskHandoffLinks(from, to Task) []TypedHandoffLink {
	outPorts := normalizeOutputPorts(from.Agent)
	inPorts := normalizeInputPorts(to.Agent)
	portSelectors := parsePortDependencySelectors(to.Agent.DependsOnPorts)

	// If neither side has explicit contracts, keep a lightweight generic handoff.
	if len(outPorts) == 0 && len(inPorts) == 0 {
		if !portSelectorAllows(portSelectors, "input", from.Agent.Name, "output") && len(portSelectors) > 0 {
			return nil
		}
		return []TypedHandoffLink{{
			Channel:     handoffChannelID(from.Agent.Name, "output", to.Agent.Name, "input", typesys.KindMessageAgent),
			FromAgent:   from.Agent.Name,
			ToAgent:     to.Agent.Name,
			FromPort:    "output",
			ToPort:      "input",
			Kind:        typesys.KindMessageAgent,
			MergePolicy: "auto",
		}}
	}

	links := make([]TypedHandoffLink, 0, 8)
	usedInput := map[string]struct{}{}
	usedOutputKind := map[string]struct{}{}

	for _, in := range inPorts {
		allowedSelectors, constrained := portSelectors[in.Name]
		matchFound := false
		for _, want := range in.Kinds {
			for _, out := range outPorts {
				for _, have := range out.Kinds {
					if !kindsCompatible(have, want) {
						continue
					}
					if constrained && !portSelectorSliceAllows(allowedSelectors, from.Agent.Name, out.Name) {
						continue
					}
					key := out.Name + "|" + in.Name + "|" + have.String()
					if _, ok := usedOutputKind[key]; ok {
						continue
					}
					usedOutputKind[key] = struct{}{}
					links = append(links, TypedHandoffLink{
						Channel:         handoffChannelID(from.Agent.Name, out.Name, to.Agent.Name, in.Name, have),
						FromAgent:       from.Agent.Name,
						ToAgent:         to.Agent.Name,
						FromPort:        out.Name,
						ToPort:          in.Name,
						Kind:            have,
						MergePolicy:     in.MergePolicy,
						ChannelPriority: in.ChannelPriority,
						TargetMaxTokens: in.MaxTokens,
						Required:        in.Required,
					})
					matchFound = true
					break
				}
				if matchFound {
					break
				}
			}
			if matchFound {
				break
			}
		}
		if matchFound {
			usedInput[in.Name] = struct{}{}
			continue
		}

		// Fallback: if the input accepts agent/text and we have any output, feed a generic agent message.
		if acceptsMessageLike(in.Kinds) {
			fallbackPort := pickFallbackOutputPortName(outPorts)
			if constrained {
				if p, ok := pickSelectorOutputPort(allowedSelectors, from.Agent.Name, outPorts); ok {
					fallbackPort = p
				}
				if !portSelectorSliceAllows(allowedSelectors, from.Agent.Name, fallbackPort) {
					continue
				}
			}
			links = append(links, TypedHandoffLink{
				Channel:         handoffChannelID(from.Agent.Name, fallbackPort, to.Agent.Name, in.Name, typesys.KindMessageAgent),
				FromAgent:       from.Agent.Name,
				ToAgent:         to.Agent.Name,
				FromPort:        fallbackPort,
				ToPort:          in.Name,
				Kind:            typesys.KindMessageAgent,
				MergePolicy:     in.MergePolicy,
				ChannelPriority: in.ChannelPriority,
				TargetMaxTokens: in.MaxTokens,
				Required:        in.Required,
			})
			usedInput[in.Name] = struct{}{}
		}
	}

	// If target has no explicit inputs but source has explicit outputs, expose a generic message handoff.
	if len(inPorts) == 0 && len(outPorts) > 0 && len(links) == 0 {
		if len(portSelectors) > 0 && !portSelectorAllows(portSelectors, "input", from.Agent.Name, outPorts[0].Name) {
			return nil
		}
		links = append(links, TypedHandoffLink{
			Channel:     handoffChannelID(from.Agent.Name, outPorts[0].Name, to.Agent.Name, "input", typesys.KindMessageAgent),
			FromAgent:   from.Agent.Name,
			ToAgent:     to.Agent.Name,
			FromPort:    outPorts[0].Name,
			ToPort:      "input",
			Kind:        typesys.KindMessageAgent,
			MergePolicy: "auto",
		})
	}

	return dedupeHandoffLinks(links)
}

type portDepSelector struct {
	Agent string
	Port  string
}

func parsePortDependencySelectors(raw map[string][]string) map[string][]portDepSelector {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string][]portDepSelector, len(raw))
	for rawPort, refs := range raw {
		port := strings.TrimSpace(rawPort)
		if port == "" {
			continue
		}
		selectors := make([]portDepSelector, 0, len(refs))
		for _, ref := range refs {
			sel, ok := parsePortDepSelector(ref)
			if !ok {
				continue
			}
			selectors = append(selectors, sel)
		}
		if len(selectors) > 0 {
			out[port] = selectors
		}
	}
	return out
}

func parsePortDepSelector(raw string) (portDepSelector, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, " ") {
		return portDepSelector{}, false
	}
	parts := strings.Split(raw, ".")
	switch len(parts) {
	case 1:
		return portDepSelector{Agent: parts[0]}, parts[0] != ""
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return portDepSelector{}, false
		}
		return portDepSelector{Agent: parts[0], Port: parts[1]}, true
	default:
		return portDepSelector{}, false
	}
}

func portSelectorAllows(selectorsByPort map[string][]portDepSelector, toPort, fromAgent, fromPort string) bool {
	if len(selectorsByPort) == 0 {
		return true
	}
	sels, ok := selectorsByPort[toPort]
	if !ok || len(sels) == 0 {
		return true
	}
	return portSelectorSliceAllows(sels, fromAgent, fromPort)
}

func portSelectorSliceAllows(sels []portDepSelector, fromAgent, fromPort string) bool {
	if len(sels) == 0 {
		return true
	}
	for _, sel := range sels {
		if sel.Agent != fromAgent {
			continue
		}
		if sel.Port == "" || sel.Port == fromPort {
			return true
		}
	}
	return false
}

func pickSelectorOutputPort(sels []portDepSelector, fromAgent string, outPorts []normalizedPort) (string, bool) {
	if len(sels) == 0 {
		return "", false
	}
	for _, sel := range sels {
		if sel.Agent != fromAgent || sel.Port == "" {
			continue
		}
		for _, out := range outPorts {
			if out.Name == sel.Port {
				return out.Name, true
			}
		}
	}
	return "", false
}

func listPortDependencyAgents(raw map[string][]string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, refs := range raw {
		for _, ref := range refs {
			sel, ok := parsePortDepSelector(ref)
			if !ok || sel.Agent == "" {
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

type normalizedPort struct {
	Name            string
	Kinds           []typesys.Kind
	MergePolicy     string
	ChannelPriority float64
	Required        bool
	MaxTokens       int
}

func normalizeInputPorts(agent project.AgentConfig) []normalizedPort {
	if len(agent.IO.Inputs) == 0 {
		return nil
	}
	out := make([]normalizedPort, 0, len(agent.IO.Inputs))
	for _, p := range agent.IO.Inputs {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		kinds := normalizeKinds(p.Accepts)
		if len(kinds) == 0 {
			kinds = []typesys.Kind{typesys.KindMessageAgent}
		}
		out = append(out, normalizedPort{
			Name:            name,
			Kinds:           kinds,
			MergePolicy:     normalizeHandoffMergePolicy(p.MergePolicy),
			ChannelPriority: p.ChannelPriority,
			Required:        p.Required,
			MaxTokens:       p.MaxTokens,
		})
	}
	return out
}

func normalizeOutputPorts(agent project.AgentConfig) []normalizedPort {
	if len(agent.IO.Outputs) == 0 {
		return nil
	}
	out := make([]normalizedPort, 0, len(agent.IO.Outputs))
	for _, p := range agent.IO.Outputs {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		kinds := normalizeKinds(p.Produces)
		if len(kinds) == 0 {
			kinds = []typesys.Kind{typesys.KindMessageAgent}
		}
		out = append(out, normalizedPort{
			Name:            name,
			Kinds:           kinds,
			MergePolicy:     normalizeHandoffMergePolicy(p.MergePolicy),
			ChannelPriority: p.ChannelPriority,
			Required:        p.Required,
			MaxTokens:       p.MaxTokens,
		})
	}
	return out
}

func normalizeKinds(raw []string) []typesys.Kind {
	if len(raw) == 0 {
		return nil
	}
	out := make([]typesys.Kind, 0, len(raw))
	seen := map[typesys.Kind]struct{}{}
	for _, r := range raw {
		k, ok := typesys.NormalizeKind(r)
		if !ok {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func kindsCompatible(have, want typesys.Kind) bool {
	if have == "" || want == "" {
		return false
	}
	if have == want {
		return true
	}
	// Generic inter-agent handoff can satisfy text/plain inputs.
	if have == typesys.KindMessageAgent && want == typesys.KindTextPlain {
		return true
	}
	if have == typesys.KindTextPlain && want == typesys.KindMessageAgent {
		return true
	}
	// artifact/ref is a generic transport for large typed payloads.
	if have == typesys.KindArtifactRef && strings.HasPrefix(want.String(), "artifact/") {
		return true
	}
	return false
}

func acceptsMessageLike(kinds []typesys.Kind) bool {
	for _, k := range kinds {
		if k == typesys.KindMessageAgent || k == typesys.KindTextPlain || k == typesys.KindTextMarkdown {
			return true
		}
	}
	return false
}

func pickFallbackOutputPortName(out []normalizedPort) string {
	if len(out) == 0 {
		return "output"
	}
	return out[0].Name
}

func dedupeHandoffLinks(in []TypedHandoffLink) []TypedHandoffLink {
	if len(in) <= 1 {
		return in
	}
	out := make([]TypedHandoffLink, 0, len(in))
	seen := map[string]struct{}{}
	for _, l := range in {
		key := l.Channel + "|" + l.FromAgent + "|" + l.ToAgent + "|" + l.FromPort + "|" + l.ToPort + "|" + l.Kind.String() + "|" + l.MergePolicy + "|" + fmt.Sprintf("%.6f", l.ChannelPriority) + "|" + fmt.Sprintf("%d", l.TargetMaxTokens)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, l)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ToAgent != out[j].ToAgent {
			return out[i].ToAgent < out[j].ToAgent
		}
		if out[i].ToPort != out[j].ToPort {
			return out[i].ToPort < out[j].ToPort
		}
		if out[i].FromAgent != out[j].FromAgent {
			return out[i].FromAgent < out[j].FromAgent
		}
		if out[i].FromPort != out[j].FromPort {
			return out[i].FromPort < out[j].FromPort
		}
		if out[i].Channel != out[j].Channel {
			return out[i].Channel < out[j].Channel
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].MergePolicy != out[j].MergePolicy {
			return out[i].MergePolicy < out[j].MergePolicy
		}
		if out[i].ChannelPriority != out[j].ChannelPriority {
			return out[i].ChannelPriority > out[j].ChannelPriority
		}
		return out[i].TargetMaxTokens < out[j].TargetMaxTokens
	})
	return out
}

func dedupeTypedHandoffPlans(in []TypedHandoffPlan) []TypedHandoffPlan {
	if len(in) <= 1 {
		return in
	}
	out := make([]TypedHandoffPlan, 0, len(in))
	seen := map[string]struct{}{}
	for _, h := range in {
		key := h.Channel + "|" + h.FromAgent + "|" + h.ToAgent + "|" + h.FromPort + "|" + h.ToPort + "|" + h.Kind + "|" + h.MergePolicy + "|" + fmt.Sprintf("%.6f", h.ChannelPriority) + "|" + fmt.Sprintf("%d", h.TargetMaxTokens)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, h)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a := out[i]
		b := out[j]
		if a.FromAgent != b.FromAgent {
			return a.FromAgent < b.FromAgent
		}
		if a.ToAgent != b.ToAgent {
			return a.ToAgent < b.ToAgent
		}
		if a.ToPort != b.ToPort {
			return a.ToPort < b.ToPort
		}
		if a.FromPort != b.FromPort {
			return a.FromPort < b.FromPort
		}
		if a.Channel != b.Channel {
			return a.Channel < b.Channel
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.MergePolicy != b.MergePolicy {
			return a.MergePolicy < b.MergePolicy
		}
		if a.ChannelPriority != b.ChannelPriority {
			return a.ChannelPriority > b.ChannelPriority
		}
		if a.TargetMaxTokens != b.TargetMaxTokens {
			return a.TargetMaxTokens < b.TargetMaxTokens
		}
		if a.Required != b.Required {
			return a.Required && !b.Required
		}
		return false
	})
	return out
}

func handoffChannelID(fromAgent, fromPort, toAgent, toPort string, kind typesys.Kind) string {
	fromAgent = strings.TrimSpace(fromAgent)
	fromPort = strings.TrimSpace(fromPort)
	toAgent = strings.TrimSpace(toAgent)
	toPort = strings.TrimSpace(toPort)
	if fromPort == "" {
		fromPort = "output"
	}
	if toPort == "" {
		toPort = "input"
	}
	kindStr := kind.String()
	if kindStr == "" {
		kindStr = "message/agent"
	}
	return fmt.Sprintf("%s.%s->%s.%s#%s", fromAgent, fromPort, toAgent, toPort, kindStr)
}

func normalizeHandoffMergePolicy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return "auto"
	case "latest":
		return "latest"
	case "append", "append3":
		return "append3"
	case "append2":
		return "append2"
	case "append4":
		return "append4"
	default:
		return "auto"
	}
}
