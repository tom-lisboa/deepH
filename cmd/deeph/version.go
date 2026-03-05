package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	goruntime "runtime"
	"runtime/debug"
	"strings"
)

var (
	buildVersion = "dev"
	buildCommit  = ""
	buildDate    = ""
)

type versionPayload struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"date,omitempty"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func cmdVersion(args []string) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print version info as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("version does not accept positional arguments")
	}

	payload := versionPayload{
		Name:    "deepH",
		Version: effectiveBuildVersion(),
		Commit:  strings.TrimSpace(buildCommit),
		Date:    strings.TrimSpace(buildDate),
		Go:      goruntime.Version(),
		OS:      goruntime.GOOS,
		Arch:    goruntime.GOARCH,
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	fmt.Printf("%s %s\n", payload.Name, payload.Version)
	if payload.Commit != "" {
		fmt.Printf("commit: %s\n", payload.Commit)
	}
	if payload.Date != "" {
		fmt.Printf("built:  %s\n", payload.Date)
	}
	fmt.Printf("go:     %s\n", payload.Go)
	fmt.Printf("target: %s/%s\n", payload.OS, payload.Arch)
	return nil
}

func effectiveBuildVersion() string {
	version := strings.TrimSpace(buildVersion)
	if version != "" && version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		mainVersion := strings.TrimSpace(info.Main.Version)
		if mainVersion != "" && mainVersion != "(devel)" {
			return mainVersion
		}
	}
	if version == "" {
		return "dev"
	}
	return version
}
