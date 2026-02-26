package runtime

import (
	"context"
	"time"

	"deeph/internal/project"
	"deeph/internal/typesys"
)

type LLMRequest struct {
	AgentName       string
	Model           string
	SystemPrompt    string
	Input           string
	AvailableSkills []string
	StartupResults  []SkillCallResult
	Messages        []ChatMessage
	Tools           []LLMToolDefinition
	ToolChoice      string
}

type LLMResponse struct {
	Text             string
	Provider         string
	Model            string
	Meta             map[string]any
	FinishReason     string
	ReasoningContent string
	ToolCalls        []LLMToolCall
}

type ChatMessage struct {
	Role             string
	Content          string
	Name             string
	ToolCallID       string
	ToolCalls        []LLMToolCall
	ReasoningContent string
}

type LLMToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
	Strict      bool
}

type LLMToolCall struct {
	ID        string
	Type      string
	Name      string
	Arguments string
}

type Provider interface {
	Name() string
	Generate(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

type SkillExecution struct {
	AgentName string
	Input     string
	Args      map[string]any
}

type SkillCallResult struct {
	Skill     string         `json:"skill"`
	Result    map[string]any `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
	Duration  time.Duration  `json:"duration"`
	CallID    string         `json:"call_id,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
	Cached    bool           `json:"cached,omitempty"`
	Cacheable bool           `json:"cacheable,omitempty"`
}

type Skill interface {
	Name() string
	Description() string
	Execute(ctx context.Context, exec SkillExecution) (map[string]any, error)
}

type Task struct {
	Agent      project.AgentConfig
	AgentFile  string
	Provider   project.ProviderConfig
	SkillNames []string
	StageIndex int
	DependsOn  []string
	Incoming   []TypedHandoffLink
	Outgoing   []TypedHandoffLink
}

type ExecutionPlan struct {
	CreatedAt time.Time
	Parallel  bool
	Input     string
	Tasks     []TaskPlan
	Stages    []PlanStage
	Handoffs  []TypedHandoffPlan
	Spec      string
}

type TaskPlan struct {
	Agent         string
	AgentFile     string
	Provider      string
	ProviderType  string
	Model         string
	Skills        []string
	TimeoutMS     int
	StartupCalls  int
	ContextBudget int
	ContextMoment string
	StageIndex    int
	DependsOn     []string
	IO            TaskIOPlan
}

type TaskIOPlan struct {
	Inputs  []TypedPortPlan
	Outputs []TypedPortPlan
}

type TypedPortPlan struct {
	Name            string
	Kinds           []string
	MergePolicy     string
	ChannelPriority float64
	Required        bool
	MaxTokens       int
}

type PlanStage struct {
	Index  int
	Agents []string
}

type TypedHandoffPlan struct {
	Channel         string
	FromAgent       string
	ToAgent         string
	FromPort        string
	ToPort          string
	Kind            string
	MergePolicy     string
	ChannelPriority float64
	TargetMaxTokens int
	Required        bool
}

type TypedHandoffLink struct {
	Channel         string
	FromAgent       string
	ToAgent         string
	FromPort        string
	ToPort          string
	Kind            typesys.Kind
	MergePolicy     string
	ChannelPriority float64
	TargetMaxTokens int
	Required        bool
}

type AgentSpecGraph struct {
	Raw    string
	Stages [][]string
}

type AgentRunResult struct {
	Agent                      string
	Provider                   string
	ProviderType               string
	Model                      string
	Skills                     []string
	Output                     string
	StartupCalls               []SkillCallResult
	ToolCalls                  []SkillCallResult
	ToolCacheHits              int
	ToolCacheMisses            int
	ToolBudgetCallsUsed        int
	ToolBudgetCallsLimit       int
	ToolBudgetExecMSUsed       int
	ToolBudgetExecMSLimit      int
	StageToolBudgetCallsUsed   int
	StageToolBudgetCallsLimit  int
	StageToolBudgetExecMSUsed  int
	StageToolBudgetExecMSLimit int
	ContextTokens              int
	ContextBudget              int
	ContextVersion             uint64
	ContextDropped             int
	ContextChannelsTotal       int
	ContextChannelsUsed        int
	ContextChannelsDropped     int
	ContextMoment              string
	StageIndex                 int
	DependsOn                  []string
	SentHandoffs               int
	DroppedHandoffs            int
	HandoffTokens              int
	SkippedOutputPublish       bool
	Duration                   time.Duration
	Error                      string
}

type ExecutionReport struct {
	StartedAt time.Time
	EndedAt   time.Time
	Parallel  bool
	Input     string
	Results   []AgentRunResult
}

type Planner interface {
	Plan(ctx context.Context, p *project.Project, agentNames []string, input string) (ExecutionPlan, []Task, error)
}
