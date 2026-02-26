package project

import (
	"fmt"
)

type RootConfig struct {
	Version         int              `yaml:"version"`
	DefaultProvider string           `yaml:"default_provider"`
	Providers       []ProviderConfig `yaml:"providers"`
}

type ProviderConfig struct {
	Name      string            `yaml:"name"`
	Type      string            `yaml:"type"`
	BaseURL   string            `yaml:"base_url"`
	APIKeyEnv string            `yaml:"api_key_env"`
	Model     string            `yaml:"model"`
	Headers   map[string]string `yaml:"headers"`
	TimeoutMS int               `yaml:"timeout_ms"`
}

type AgentConfig struct {
	Name           string              `yaml:"name"`
	Description    string              `yaml:"description"`
	Provider       string              `yaml:"provider"`
	Model          string              `yaml:"model"`
	SystemPrompt   string              `yaml:"system_prompt"`
	Skills         []string            `yaml:"skills"`
	DependsOn      []string            `yaml:"depends_on"`
	DependsOnPorts map[string][]string `yaml:"depends_on_ports"`
	IO             AgentIOConfig       `yaml:"io"`
	StartupCalls   []SkillCall         `yaml:"startup_calls"`
	TimeoutMS      int                 `yaml:"timeout_ms"`
	Metadata       map[string]string   `yaml:"metadata"`
}

type AgentIOConfig struct {
	Inputs  []IOPortConfig `yaml:"inputs"`
	Outputs []IOPortConfig `yaml:"outputs"`
}

type IOPortConfig struct {
	Name        string   `yaml:"name"`
	Accepts     []string `yaml:"accepts"`
	Produces    []string `yaml:"produces"`
	MergePolicy string   `yaml:"merge_policy"`
	// ChannelPriority biases publish selection for handoffs targeting this input port.
	// Higher values win earlier under publish budget pressure.
	ChannelPriority float64 `yaml:"channel_priority"`
	Required        bool    `yaml:"required"`
	MaxTokens       int     `yaml:"max_tokens"`
	Description     string  `yaml:"description"`
}

type SkillCall struct {
	Skill string         `yaml:"skill"`
	Args  map[string]any `yaml:"args"`
}

type SkillConfig struct {
	Name        string            `yaml:"name"`
	Type        string            `yaml:"type"`
	Description string            `yaml:"description"`
	Method      string            `yaml:"method"`
	URL         string            `yaml:"url"`
	Headers     map[string]string `yaml:"headers"`
	TimeoutMS   int               `yaml:"timeout_ms"`
	Params      map[string]any    `yaml:"params"`
}

type Project struct {
	Root       RootConfig
	Agents     []AgentConfig
	Skills     []SkillConfig
	AgentFiles map[string]string
	SkillFiles map[string]string
}

type IssueLevel string

const (
	IssueError   IssueLevel = "error"
	IssueWarning IssueLevel = "warning"
)

type Issue struct {
	Level   IssueLevel
	Path    string
	Field   string
	Message string
}

func (i Issue) String() string {
	if i.Field == "" {
		return fmt.Sprintf("[%s] %s: %s", i.Level, i.Path, i.Message)
	}
	return fmt.Sprintf("[%s] %s (%s): %s", i.Level, i.Path, i.Field, i.Message)
}

type ValidationError struct {
	Issues []Issue
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed with %d issue(s)", len(e.Issues))
}

func (e *ValidationError) HasErrors() bool {
	if e == nil {
		return false
	}
	for _, it := range e.Issues {
		if it.Level == IssueError {
			return true
		}
	}
	return false
}
