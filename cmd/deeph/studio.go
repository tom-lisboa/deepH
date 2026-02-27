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
	reader := bufio.NewReader(os.Stdin)

	for {
		printStudioScreen(abs)
		choice, err := promptLine(reader, "Select option", "")
		if err != nil {
			return err
		}
		choice = strings.ToLower(strings.TrimSpace(choice))

		var runErr error
		switch choice {
		case "1":
			runErr = studioQuickstart(reader, abs, true)
		case "2":
			runErr = studioQuickstart(reader, abs, false)
		case "3":
			runErr = studioProviderAdd(reader, abs)
		case "4":
			runErr = studioAgentCreate(reader, abs)
		case "5":
			runErr = studioValidate(reader, abs)
		case "6":
			runErr = studioRun(reader, abs)
		case "7":
			runErr = studioChat(reader, abs)
		case "8":
			runErr = studioCommandList(reader)
		case "9":
			printUsage()
		case "0", "q", "quit", "exit":
			fmt.Println("Bye.")
			return nil
		default:
			runErr = fmt.Errorf("unknown option %q", choice)
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
	fmt.Print("\033[2J\033[H")
	fmt.Println("deepH STUDIO")
	fmt.Println("==============")
	fmt.Printf("workspace: %s\n", workspace)
	fmt.Println("")
	fmt.Println("1) Quickstart (DeepSeek)")
	fmt.Println("2) Quickstart (local mock)")
	fmt.Println("3) Provider add (DeepSeek)")
	fmt.Println("4) Agent create")
	fmt.Println("5) Validate workspace")
	fmt.Println("6) Run once")
	fmt.Println("7) Chat")
	fmt.Println("8) Command dictionary")
	fmt.Println("9) Help")
	fmt.Println("0) Exit")
	fmt.Println("")
}

func studioQuickstart(reader *bufio.Reader, defaultWorkspace string, deepseek bool) error {
	ws, err := promptLine(reader, "Workspace", defaultWorkspace)
	if err != nil {
		return err
	}
	agent, err := promptLine(reader, "Starter agent", "guide")
	if err != nil {
		return err
	}
	args := []string{"--workspace", ws, "--agent", agent}
	if deepseek {
		args = append(args, "--deepseek")
	} else {
		args = append(args, "--provider", "local_mock", "--model", "mock-small")
	}
	return cmdQuickstart(args)
}

func studioProviderAdd(reader *bufio.Reader, defaultWorkspace string) error {
	ws, err := promptLine(reader, "Workspace", defaultWorkspace)
	if err != nil {
		return err
	}
	name, err := promptLine(reader, "Provider name", "deepseek")
	if err != nil {
		return err
	}
	model, err := promptLine(reader, "Model", "deepseek-chat")
	if err != nil {
		return err
	}
	return cmdProviderAdd([]string{
		"--workspace", ws,
		"--name", name,
		"--model", model,
		"--set-default",
		"deepseek",
	})
}

func studioAgentCreate(reader *bufio.Reader, defaultWorkspace string) error {
	ws, err := promptLine(reader, "Workspace", defaultWorkspace)
	if err != nil {
		return err
	}
	name, err := promptLine(reader, "Agent name", "planner")
	if err != nil {
		return err
	}
	provider, err := promptLine(reader, "Provider (optional)", "")
	if err != nil {
		return err
	}
	model, err := promptLine(reader, "Model", "deepseek-chat")
	if err != nil {
		return err
	}
	args := []string{"--workspace", ws}
	if strings.TrimSpace(provider) != "" {
		args = append(args, "--provider", provider)
	}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}
	args = append(args, name)
	return cmdAgentCreate(args)
}

func studioValidate(reader *bufio.Reader, defaultWorkspace string) error {
	ws, err := promptLine(reader, "Workspace", defaultWorkspace)
	if err != nil {
		return err
	}
	return cmdValidate([]string{"--workspace", ws})
}

func studioRun(reader *bufio.Reader, defaultWorkspace string) error {
	ws, err := promptLine(reader, "Workspace", defaultWorkspace)
	if err != nil {
		return err
	}
	spec, err := promptLine(reader, "Agent spec", "guide")
	if err != nil {
		return err
	}
	input, err := promptLine(reader, "Input", "hello")
	if err != nil {
		return err
	}
	args := []string{"--workspace", ws, spec}
	if strings.TrimSpace(input) != "" {
		args = append(args, input)
	}
	return cmdRun(args)
}

func studioChat(reader *bufio.Reader, defaultWorkspace string) error {
	ws, err := promptLine(reader, "Workspace", defaultWorkspace)
	if err != nil {
		return err
	}
	spec, err := promptLine(reader, "Agent spec", "guide")
	if err != nil {
		return err
	}
	sessionID, err := promptLine(reader, "Session id (optional)", "")
	if err != nil {
		return err
	}
	args := []string{"--workspace", ws}
	if strings.TrimSpace(sessionID) != "" {
		args = append(args, "--session", sessionID)
	}
	args = append(args, spec)
	return cmdChat(args)
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

func waitEnter(reader *bufio.Reader) error {
	fmt.Print("\nPress Enter to continue...")
	_, err := reader.ReadString('\n')
	return err
}
