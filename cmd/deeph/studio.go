package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdStudio(args []string) error {
	fs := flag.NewFlagSet("studio", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "default workspace path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("studio does not accept positional arguments")
	}

	abs, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	state := loadStudioState()
	currentWorkspace := resolveInitialStudioWorkspace(abs, hasExplicitWorkspaceArg(args), state)
	saveStudioRecent(currentWorkspace, "", "")
	reader := bufio.NewReader(os.Stdin)

	for {
		printStudioScreen(currentWorkspace)
		choice, err := promptLine(reader, "Select option", "")
		if err != nil {
			return err
		}
		choice = strings.ToLower(strings.TrimSpace(choice))

		var runErr error
		switch choice {
		case "1":
			currentWorkspace, runErr = studioQuickstart(reader, state, currentWorkspace, true)
		case "2":
			currentWorkspace, runErr = studioQuickstart(reader, state, currentWorkspace, false)
		case "3":
			currentWorkspace, runErr = studioProviderAdd(reader, currentWorkspace)
		case "4":
			currentWorkspace, runErr = studioAgentCreate(reader, state, currentWorkspace)
		case "5":
			currentWorkspace, runErr = studioValidate(reader, currentWorkspace)
		case "6":
			currentWorkspace, runErr = studioRun(reader, state, currentWorkspace)
		case "7":
			currentWorkspace, runErr = studioChat(reader, state, currentWorkspace)
		case "8":
			runErr = studioCommandList(reader)
		case "9":
			runErr = studioUpdate(reader)
		case "10":
			printUsage()
		case "11":
			printStudioDoctor(currentWorkspace)
		case "12":
			currentWorkspace, runErr = studioCalculatorStarter(reader, state, currentWorkspace)
		case "0", "q", "quit", "exit":
			fmt.Println("Bye.")
			return nil
		default:
			runErr = fmt.Errorf("unknown option %q", choice)
		}

		if strings.TrimSpace(currentWorkspace) != "" {
			state.LastWorkspace = currentWorkspace
			if err := saveStudioState(state); err != nil {
				fmt.Printf("warning: failed to save studio state: %v\n", err)
			}
		}
		if runErr != nil {
			fmt.Printf("error: %v\n", runErr)
		}
		if err := waitEnter(reader); err != nil {
			return err
		}
	}
}

func printStudioScreen(workspace string) {
	status := collectStudioStatus(workspace)
	fmt.Print("\033[2J\033[H")
	fmt.Println("deepH STUDIO")
	fmt.Println("==============")
	fmt.Printf("workspace: %s\n", status.Workspace)
	if !status.Initialized {
		fmt.Printf("status: setup needed | binary in PATH: %s | sessions: %d\n", yesNo(status.BinaryDirOnPath), status.Sessions)
	} else if status.LoadError != "" {
		fmt.Printf("status: load error | binary in PATH: %s\n", yesNo(status.BinaryDirOnPath))
	} else {
		keyStatus := "n/a"
		providerLabel := status.DefaultProvider
		if strings.TrimSpace(providerLabel) == "" {
			providerLabel = "(none)"
		}
		if status.APIKeyEnv != "" {
			keyStatus = status.APIKeyEnv + "=" + yesNo(status.APIKeySet)
		}
		fmt.Printf("status: ready | provider: %s | key: %s\n", providerLabel, keyStatus)
		fmt.Printf("agents: %d | skills: %d | sessions: %d | validation issues: %d\n", status.Agents, status.Skills, status.Sessions, status.ValidationIssues)
	}
	if status.LatestSession != "" {
		fmt.Printf("latest session: %s (%s)\n", status.LatestSession, status.LatestAgentSpec)
	}
	fmt.Println("")
	fmt.Println("1) Quickstart (DeepSeek)")
	fmt.Println("2) Quickstart (local mock)")
	fmt.Println("3) Provider add (DeepSeek)")
	fmt.Println("4) Agent create")
	fmt.Println("5) Validate workspace")
	fmt.Println("6) Run once")
	fmt.Println("7) Chat")
	fmt.Println("8) Command dictionary")
	fmt.Println("9) Update deeph")
	fmt.Println("10) Help")
	fmt.Println("11) Studio doctor")
	fmt.Println("12) Calculator workspace")
	fmt.Println("0) Exit")
	fmt.Println("")
}

func studioQuickstart(reader *bufio.Reader, state *studioState, defaultWorkspace string, deepseek bool) (string, error) {
	ws, err := promptWorkspace(reader, defaultWorkspace)
	if err != nil {
		return defaultWorkspace, err
	}
	agentDefault := "guide"
	if strings.TrimSpace(state.LastAgentSpec) != "" {
		agentDefault = state.LastAgentSpec
	}
	agent, err := promptLine(reader, "Starter agent", agentDefault)
	if err != nil {
		return ws, err
	}
	args := []string{"--workspace", ws, "--agent", agent}
	if deepseek {
		args = append(args, "--deepseek")
	} else {
		args = append(args, "--provider", "local_mock", "--model", "mock-small")
	}
	if err := cmdQuickstart(args); err != nil {
		return ws, err
	}
	state.LastAgentSpec = agent
	return ws, nil
}

func studioProviderAdd(reader *bufio.Reader, defaultWorkspace string) (string, error) {
	ws, err := promptWorkspace(reader, defaultWorkspace)
	if err != nil {
		return defaultWorkspace, err
	}
	name, err := promptLine(reader, "Provider name", "deepseek")
	if err != nil {
		return ws, err
	}
	model, err := promptLine(reader, "Model", "deepseek-chat")
	if err != nil {
		return ws, err
	}
	return ws, cmdProviderAdd([]string{
		"--workspace", ws,
		"--name", name,
		"--model", model,
		"--set-default",
		"deepseek",
	})
}

func studioAgentCreate(reader *bufio.Reader, state *studioState, defaultWorkspace string) (string, error) {
	ws, err := promptWorkspace(reader, defaultWorkspace)
	if err != nil {
		return defaultWorkspace, err
	}
	nameDefault := "planner"
	if strings.TrimSpace(state.LastAgentSpec) != "" {
		nameDefault = state.LastAgentSpec
	}
	name, err := promptLine(reader, "Agent name", nameDefault)
	if err != nil {
		return ws, err
	}
	provider, err := promptLine(reader, "Provider (optional)", "")
	if err != nil {
		return ws, err
	}
	model, err := promptLine(reader, "Model", "deepseek-chat")
	if err != nil {
		return ws, err
	}
	args := []string{"--workspace", ws}
	if strings.TrimSpace(provider) != "" {
		args = append(args, "--provider", provider)
	}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}
	args = append(args, name)
	if err := cmdAgentCreate(args); err != nil {
		return ws, err
	}
	state.LastAgentSpec = name
	return ws, nil
}

func studioValidate(reader *bufio.Reader, defaultWorkspace string) (string, error) {
	ws, err := promptWorkspace(reader, defaultWorkspace)
	if err != nil {
		return defaultWorkspace, err
	}
	return ws, cmdValidate([]string{"--workspace", ws})
}

func studioRun(reader *bufio.Reader, state *studioState, defaultWorkspace string) (string, error) {
	ws, err := promptWorkspace(reader, defaultWorkspace)
	if err != nil {
		return defaultWorkspace, err
	}
	specDefault := "guide"
	wsStatus := collectStudioStatus(ws)
	if strings.TrimSpace(wsStatus.LatestAgentSpec) != "" {
		specDefault = wsStatus.LatestAgentSpec
	}
	if strings.TrimSpace(state.LastAgentSpec) != "" {
		specDefault = state.LastAgentSpec
	}
	spec, err := promptLine(reader, "Agent spec", specDefault)
	if err != nil {
		return ws, err
	}
	input, err := promptLine(reader, "Input", "hello")
	if err != nil {
		return ws, err
	}
	args := []string{"--workspace", ws, spec}
	if strings.TrimSpace(input) != "" {
		args = append(args, input)
	}
	if err := cmdRun(args); err != nil {
		return ws, err
	}
	state.LastAgentSpec = spec
	return ws, nil
}

func studioChat(reader *bufio.Reader, state *studioState, defaultWorkspace string) (string, error) {
	ws, err := promptWorkspace(reader, defaultWorkspace)
	if err != nil {
		return defaultWorkspace, err
	}
	specDefault := "guide"
	wsStatus := collectStudioStatus(ws)
	if strings.TrimSpace(wsStatus.LatestAgentSpec) != "" {
		specDefault = wsStatus.LatestAgentSpec
	}
	if strings.TrimSpace(state.LastAgentSpec) != "" {
		specDefault = state.LastAgentSpec
	}
	spec, err := promptLine(reader, "Agent spec", specDefault)
	if err != nil {
		return ws, err
	}
	sessionDefault := wsStatus.LatestSession
	if strings.TrimSpace(sessionDefault) == "" {
		sessionDefault = state.LastSessionID
	}
	sessionID, err := promptLine(reader, "Session id (optional)", sessionDefault)
	if err != nil {
		return ws, err
	}
	args := []string{"--workspace", ws}
	if strings.TrimSpace(sessionID) != "" {
		args = append(args, "--session", sessionID)
	}
	args = append(args, spec)
	if err := cmdChat(args); err != nil {
		return ws, err
	}
	state.LastAgentSpec = spec
	if strings.TrimSpace(sessionID) != "" {
		state.LastSessionID = sessionID
	}
	return ws, nil
}

func studioCommandList(reader *bufio.Reader) error {
	category, err := promptLine(reader, "Category (optional)", "")
	if err != nil {
		return err
	}
	args := []string{"list"}
	if strings.TrimSpace(category) != "" {
		args = append(args, "--category", category)
	}
	return cmdCommand(args)
}

func studioUpdate(reader *bufio.Reader) error {
	tag, err := promptLine(reader, "Release tag (latest or vX.Y.Z)", "latest")
	if err != nil {
		return err
	}
	checkOnlyText, err := promptLine(reader, "Check only? (y/N)", "N")
	if err != nil {
		return err
	}
	args := []string{"--tag", strings.TrimSpace(tag)}
	if strings.EqualFold(strings.TrimSpace(checkOnlyText), "y") || strings.EqualFold(strings.TrimSpace(checkOnlyText), "yes") {
		args = append(args, "--check")
	}
	return cmdUpdate(args)
}

func studioCalculatorStarter(reader *bufio.Reader, state *studioState, defaultWorkspace string) (string, error) {
	ws, err := promptWorkspace(reader, defaultCalculatorWorkspace(defaultWorkspace))
	if err != nil {
		return defaultWorkspace, err
	}
	if !workspaceInitialized(ws) {
		fmt.Println("Bootstrapping calculator workspace...")
		if err := cmdQuickstart([]string{"--workspace", ws, "--agent", "guide", "--deepseek"}); err != nil {
			return ws, err
		}
	}
	fmt.Println("Configuring DeepSeek provider...")
	if err := cmdProviderAdd([]string{
		"--workspace", ws,
		"--name", "deepseek",
		"--model", "deepseek-chat",
		"--timeout-ms", "120000",
		"--set-default",
		"--force",
		"deepseek",
	}); err != nil {
		return ws, err
	}
	fmt.Println("Installing calculator kit...")
	if err := cmdKitAdd([]string{
		"--workspace", ws,
		"--provider-name", "deepseek",
		"--model", "deepseek-chat",
		"--set-default-provider",
		"crud-next-multiverse",
	}); err != nil {
		return ws, err
	}
	runNow, err := promptYesNo(reader, "Generate calculator now?", false)
	if err != nil {
		return ws, err
	}
	state.LastAgentSpec = "@crud_fullstack_multiverse"
	if !runNow {
		fmt.Println("Workspace ready.")
		fmt.Printf("Next: deeph run --workspace %q --multiverse 0 @crud_fullstack_multiverse \"Crie uma calculadora fullstack com frontend Next.js, rotas API, controller/service, operacoes soma/subtracao/multiplicacao/divisao e testes basicos\"\n", ws)
		return ws, nil
	}
	prompt := "Crie uma calculadora fullstack com frontend Next.js, rotas API, controller/service, operacoes soma/subtracao/multiplicacao/divisao e testes basicos"
	if err := cmdRun([]string{"--workspace", ws, "--multiverse", "0", "@crud_fullstack_multiverse", prompt}); err != nil {
		return ws, err
	}
	return ws, nil
}

func promptLine(reader *bufio.Reader, label, def string) (string, error) {
	if strings.TrimSpace(def) != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	raw, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(raw)
	if v == "" {
		return def, nil
	}
	return v, nil
}

func promptWorkspace(reader *bufio.Reader, defaultWorkspace string) (string, error) {
	ws, err := promptLine(reader, "Workspace", defaultWorkspace)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(ws)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func promptYesNo(reader *bufio.Reader, label string, def bool) (bool, error) {
	defaultText := "N"
	if def {
		defaultText = "Y"
	}
	answer, err := promptLine(reader, label+" (y/N)", defaultText)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func hasExplicitWorkspaceArg(args []string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == "--workspace" {
			return true
		}
		if strings.HasPrefix(args[i], "--workspace=") {
			return true
		}
	}
	return false
}

func waitEnter(reader *bufio.Reader) error {
	fmt.Print("\nPress Enter to continue...")
	_, err := reader.ReadString('\n')
	return err
}
