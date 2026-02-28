package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var agentNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type AgentTemplateOptions struct {
	Name        string
	Provider    string
	Model       string
	Description string
	Force       bool
}

func CreateAgentFile(workspace string, opts AgentTemplateOptions) (string, error) {
	return createAgentFile(workspace, opts, renderAgentTemplate(opts))
}

func CreateGuideStarterFile(workspace string, opts AgentTemplateOptions) (string, error) {
	return createAgentFile(workspace, opts, renderGuideStarterTemplate(opts))
}

func createAgentFile(workspace string, opts AgentTemplateOptions, content string) (string, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return "", fmt.Errorf("agent name is required")
	}
	if !agentNamePattern.MatchString(name) {
		return "", fmt.Errorf("invalid agent name %q (use letters, numbers, _ or -)", name)
	}

	agentsDir := filepath.Join(workspace, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", agentsDir, err)
	}
	outPath := filepath.Join(agentsDir, name+".yaml")
	if !opts.Force {
		if _, err := os.Stat(outPath); err == nil {
			return "", fmt.Errorf("%s already exists (use --force to overwrite)", outPath)
		}
	}

	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", outPath, err)
	}
	return outPath, nil
}

func renderAgentTemplate(opts AgentTemplateOptions) string {
	name := strings.TrimSpace(opts.Name)
	desc := strings.TrimSpace(opts.Description)
	if desc == "" {
		desc = "User-defined agent"
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = "mock-small"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", yamlScalar(desc))
	if p := strings.TrimSpace(opts.Provider); p != "" {
		fmt.Fprintf(&b, "provider: %s\n", p)
	} else {
		b.WriteString("# provider: your_provider_name  # optional if deeph.yaml has default_provider\n")
	}
	fmt.Fprintf(&b, "model: %s\n", model)
	b.WriteString("system_prompt: |\n")
	fmt.Fprintf(&b, "  You are the %s agent.\n", name)
	b.WriteString("  State assumptions clearly and keep the answer practical.\n")
	b.WriteString("skills: []\n")
	b.WriteString("# depends_on: [planner]   # optional explicit DAG dependency (must appear in an earlier run stage)\n")
	b.WriteString("# depends_on_ports:\n")
	b.WriteString("#   brief: [planner.summary, coder.summary]  # optional per-input routing (agent or agent.port)\n")
	b.WriteString("# io:\n")
	b.WriteString("#   inputs:\n")
	b.WriteString("#     - name: source\n")
	b.WriteString("#       accepts: [code/go, code/ts, text/plain]\n")
	b.WriteString("#       merge_policy: latest   # auto | latest | append2 | append3 | append4 (inputs only)\n")
	b.WriteString("#       channel_priority: 3    # optional publish selection bias under channel budget (inputs only)\n")
	b.WriteString("#       required: true\n")
	b.WriteString("#     - name: ask\n")
	b.WriteString("#       accepts: [text/plain]\n")
	b.WriteString("#   outputs:\n")
	b.WriteString("#     - name: answer\n")
	b.WriteString("#       produces: [text/markdown]\n")
	b.WriteString("# startup_calls:\n")
	b.WriteString("#   - skill: echo\n")
	b.WriteString("#     args:\n")
	b.WriteString("#       note: \"hello from startup_calls\"\n")
	b.WriteString("# metadata:\n")
	b.WriteString("#   strict_types: \"true\"\n")
	b.WriteString("#   context_max_input_tokens: \"900\"\n")
	b.WriteString("#   context_max_channels: \"8\"           # optional cap for incoming channels used in context compile\n")
	b.WriteString("#   context_max_channel_tokens: \"320\"   # optional proxy budget for selected incoming channels in compile\n")
	b.WriteString("#   publish_max_channels: \"8\"          # optional channel publish budget per agent run\n")
	b.WriteString("#   publish_max_channel_tokens: \"240\"   # optional budget for handoff facts/channels\n")
	b.WriteString("#   publish_unconsumed_output: \"true\"   # publish final output into shared context even with no downstream consumers\n")
	b.WriteString("#   tool_max_calls: \"8\"                 # optional tool budget per agent run (startup_calls + tool loop)\n")
	b.WriteString("#   tool_max_exec_ms: \"1500\"            # optional cumulative tool execution time budget per agent run\n")
	b.WriteString("#   stage_tool_max_calls: \"12\"          # optional shared budget across all agents in the same stage (strictest wins)\n")
	b.WriteString("#   stage_tool_max_exec_ms: \"2500\"      # optional shared exec time budget across the same stage (strictest wins)\n")
	b.WriteString("#   lock_file_tools: \"true\"             # default true; serialize file_read/file_read_range per path\n")
	b.WriteString("#   lock_http_host_tools: \"true\"        # optional; serialize HTTP tools per host (off by default)\n")
	b.WriteString("#   cache_http_get_tools: \"true\"        # optional shared tool broker cache for HTTP GET only\n")
	b.WriteString("#   cache_echo_tools: \"true\"            # optional (usually not needed)\n")
	return b.String()
}

func renderGuideStarterTemplate(opts AgentTemplateOptions) string {
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = "mock-small"
	}

	var b strings.Builder
	b.WriteString("name: guide\n")
	fmt.Fprintf(&b, "description: %s\n", yamlScalar("Starter guide agent generated by deeph quickstart"))
	if p := strings.TrimSpace(opts.Provider); p != "" {
		fmt.Fprintf(&b, "provider: %s\n", p)
	} else {
		b.WriteString("# provider: your_provider_name  # optional if deeph.yaml has default_provider\n")
	}
	fmt.Fprintf(&b, "model: %s\n", model)
	b.WriteString("system_prompt: |\n")
	b.WriteString("  You are the guide agent for a deepH workspace.\n")
	b.WriteString("  Give the user the exact deeph command or the smallest exact command sequence needed for the task.\n")
	b.WriteString("  Be direct, practical and concise.\n")
	b.WriteString("  When the user asks how to do something, put the command first.\n")
	b.WriteString("  Mention --workspace when the user is clearly outside a project folder.\n")
	b.WriteString("  Do not claim that you executed anything unless the user explicitly says they already ran it.\n")
	b.WriteString("  Use the command_doc skill only when the exact command path, flags or workflow details are uncertain.\n")
	b.WriteString("  Do not call command_doc on every turn.\n")
	b.WriteString("  If a file must be edited, say exactly which file should be changed.\n")
	b.WriteString("skills:\n")
	b.WriteString("  - command_doc\n")
	b.WriteString("metadata:\n")
	b.WriteString("  tool_max_calls: \"1\"\n")
	b.WriteString("  max_tool_rounds: \"2\"\n")
	b.WriteString("  max_repeated_tool_calls: \"1\"\n")
	b.WriteString("  context_max_input_tokens: \"900\"\n")
	return b.String()
}

func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
