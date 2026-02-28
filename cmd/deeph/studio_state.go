package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"deeph/internal/project"
)

type studioState struct {
	LastWorkspace string    `json:"last_workspace"`
	LastAgentSpec string    `json:"last_agent_spec,omitempty"`
	LastSessionID string    `json:"last_session_id,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

type studioStatus struct {
	Workspace            string
	RootPath             string
	Initialized          bool
	LoadError            string
	ValidationIssues     int
	ValidationErrors     int
	DefaultProvider      string
	DefaultProviderType  string
	DefaultProviderModel string
	APIKeyEnv            string
	APIKeySet            bool
	Providers            int
	Agents               int
	Skills               int
	Sessions             int
	LatestSession        string
	LatestAgentSpec      string
	Executable           string
	BinaryDir            string
	BinaryDirOnPath      bool
	OS                   string
	Arch                 string
}

func loadStudioState() *studioState {
	path, err := studioStatePath()
	if err != nil {
		return &studioState{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return &studioState{}
	}
	var state studioState
	if err := json.Unmarshal(b, &state); err != nil {
		return &studioState{}
	}
	return &state
}

func saveStudioState(state *studioState) error {
	if state == nil {
		return nil
	}
	path, err := studioStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create studio config dir: %w", err)
	}
	state.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal studio state: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write studio state: %w", err)
	}
	return nil
}

func saveStudioRecent(workspace, agentSpec, sessionID string) {
	state := loadStudioState()
	if ws := strings.TrimSpace(workspace); ws != "" {
		if abs, err := filepath.Abs(ws); err == nil {
			state.LastWorkspace = abs
		} else {
			state.LastWorkspace = ws
		}
	}
	if spec := strings.TrimSpace(agentSpec); spec != "" {
		state.LastAgentSpec = spec
	}
	if id := strings.TrimSpace(sessionID); id != "" {
		state.LastSessionID = id
	}
	_ = saveStudioState(state)
}

func studioStatePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "deeph", "studio.json"), nil
}

func workspaceInitialized(workspace string) bool {
	rootPath := filepath.Join(workspace, project.RootConfigFile)
	if _, err := os.Stat(rootPath); err != nil {
		return false
	}
	return true
}

func resolveInitialStudioWorkspace(flagWorkspace string, explicit bool, state *studioState) string {
	abs, err := filepath.Abs(flagWorkspace)
	if err != nil {
		abs = flagWorkspace
	}
	if explicit {
		return abs
	}
	if workspaceInitialized(abs) {
		return abs
	}
	saved := strings.TrimSpace(state.LastWorkspace)
	if saved == "" {
		return abs
	}
	if !workspaceInitialized(saved) {
		return abs
	}
	return saved
}

func collectStudioStatus(workspace string) studioStatus {
	abs, err := filepath.Abs(workspace)
	if err != nil {
		abs = workspace
	}
	status := studioStatus{
		Workspace: abs,
		RootPath:  filepath.Join(abs, project.RootConfigFile),
		OS:        goruntime.GOOS,
		Arch:      goruntime.GOARCH,
	}
	if exe, err := os.Executable(); err == nil {
		status.Executable = exe
		status.BinaryDir = filepath.Dir(exe)
		status.BinaryDirOnPath = pathContainsDir(os.Getenv("PATH"), status.BinaryDir)
	}
	if !workspaceInitialized(abs) {
		return status
	}
	status.Initialized = true

	p, err := project.Load(abs)
	if err != nil {
		status.LoadError = err.Error()
		return status
	}
	status.Providers = len(p.Root.Providers)
	status.Agents = len(p.Agents)
	status.Skills = len(p.Skills)
	status.DefaultProvider = strings.TrimSpace(p.Root.DefaultProvider)

	if status.DefaultProvider != "" {
		for _, cfg := range p.Root.Providers {
			if strings.TrimSpace(cfg.Name) != status.DefaultProvider {
				continue
			}
			status.DefaultProviderType = strings.TrimSpace(cfg.Type)
			status.DefaultProviderModel = strings.TrimSpace(cfg.Model)
			status.APIKeyEnv = strings.TrimSpace(cfg.APIKeyEnv)
			break
		}
	}
	if status.APIKeyEnv != "" && strings.TrimSpace(os.Getenv(status.APIKeyEnv)) != "" {
		status.APIKeySet = true
	}

	verr := project.Validate(p)
	if verr != nil {
		status.ValidationIssues = len(verr.Issues)
		for _, issue := range verr.Issues {
			if issue.Level == project.IssueError {
				status.ValidationErrors++
			}
		}
	}

	if metas, err := listChatSessionMetas(abs); err == nil {
		status.Sessions = len(metas)
		if len(metas) > 0 {
			status.LatestSession = metas[0].ID
			status.LatestAgentSpec = metas[0].AgentSpec
		}
	}

	return status
}

func printStudioDoctor(workspace string) {
	status := collectStudioStatus(workspace)
	fmt.Println("Studio doctor")
	fmt.Println("=============")
	fmt.Printf("workspace: %s\n", status.Workspace)
	fmt.Printf("root config: %s\n", status.RootPath)
	fmt.Printf("initialized: %s\n", yesNo(status.Initialized))
	fmt.Printf("os/arch: %s/%s\n", status.OS, status.Arch)
	if status.Executable != "" {
		fmt.Printf("executable: %s\n", status.Executable)
		fmt.Printf("binary dir in PATH: %s\n", yesNo(status.BinaryDirOnPath))
	}
	if !status.Initialized {
		fmt.Println("")
		fmt.Println("Next steps:")
		fmt.Printf("  1. deeph quickstart --workspace %q --deepseek\n", status.Workspace)
		fmt.Printf("  2. deeph studio --workspace %q\n", status.Workspace)
		return
	}
	if status.LoadError != "" {
		fmt.Printf("load error: %s\n", status.LoadError)
		return
	}

	providerLabel := "(none)"
	if status.DefaultProvider != "" {
		providerLabel = status.DefaultProvider
		if status.DefaultProviderType != "" {
			providerLabel += " [" + status.DefaultProviderType + "]"
		}
		if status.DefaultProviderModel != "" {
			providerLabel += " model=" + status.DefaultProviderModel
		}
	}
	fmt.Printf("default provider: %s\n", providerLabel)
	if status.APIKeyEnv != "" {
		fmt.Printf("api key env: %s (%s in current session)\n", status.APIKeyEnv, yesNo(status.APIKeySet))
	} else {
		fmt.Println("api key env: (not configured on default provider)")
	}
	fmt.Printf("agents: %d\n", status.Agents)
	fmt.Printf("skills: %d\n", status.Skills)
	fmt.Printf("providers: %d\n", status.Providers)
	fmt.Printf("sessions: %d\n", status.Sessions)
	if status.LatestSession != "" {
		fmt.Printf("latest session: %s (%s)\n", status.LatestSession, status.LatestAgentSpec)
	}
	if status.ValidationIssues == 0 {
		fmt.Println("validation: clean")
	} else {
		fmt.Printf("validation: %d issue(s), %d error(s)\n", status.ValidationIssues, status.ValidationErrors)
	}

	fmt.Println("")
	fmt.Println("Recommendations:")
	if !status.APIKeySet && status.APIKeyEnv != "" {
		fmt.Printf("  - Set %s in this shell before running DeepSeek agents.\n", status.APIKeyEnv)
	}
	if status.ValidationErrors > 0 {
		fmt.Printf("  - Run `deeph validate --workspace %q` and fix the errors above.\n", status.Workspace)
	}
	if status.Sessions == 0 {
		fmt.Println("  - Start a first conversation with option 7 or `deeph chat --workspace ... guide`.")
	}
	if !status.BinaryDirOnPath {
		fmt.Println("  - Restart the terminal if the installer just changed PATH.")
	}
}

func pathContainsDir(pathValue, dir string) bool {
	if strings.TrimSpace(pathValue) == "" || strings.TrimSpace(dir) == "" {
		return false
	}
	want := filepath.Clean(dir)
	for _, part := range filepath.SplitList(pathValue) {
		if samePath(part, want) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	a = filepath.Clean(strings.TrimSpace(a))
	b = filepath.Clean(strings.TrimSpace(b))
	if goruntime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func defaultCalculatorWorkspace(currentWorkspace string) string {
	if strings.TrimSpace(currentWorkspace) != "" && currentWorkspace != "." && currentWorkspace != string(filepath.Separator) {
		base := strings.ToLower(filepath.Base(currentWorkspace))
		if strings.Contains(base, "calc") || strings.Contains(base, "calcul") {
			return currentWorkspace
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return currentWorkspace
	}
	return filepath.Join(home, "deeph-calculadora")
}
