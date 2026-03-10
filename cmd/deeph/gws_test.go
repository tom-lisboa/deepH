package main

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"
)

func TestParseGWSExecArgsPreservesUnknownFlagsForGWS(t *testing.T) {
	cfg, cmdArgs, err := parseGWSExecArgs([]string{"--format=json", "drive", "files", "list"})
	if err != nil {
		t.Fatalf("parse gws args: %v", err)
	}
	if cfg.AutoJSON {
		t.Fatalf("auto json should remain false when wrapper flag is not used")
	}
	if got := strings.Join(cmdArgs, "|"); got != "--format=json|drive|files|list" {
		t.Fatalf("cmd args mismatch: %q", got)
	}
}

func TestParseGWSExecArgsHandlesWrapperFlagsAndSeparator(t *testing.T) {
	cfg, cmdArgs, err := parseGWSExecArgs([]string{
		"--json",
		"--timeout", "45s",
		"--max-output-bytes", "2048",
		"--bin", "/tmp/gws",
		"--",
		"--format=yaml", "drive", "files", "list",
	})
	if err != nil {
		t.Fatalf("parse gws args: %v", err)
	}
	if !cfg.AutoJSON {
		t.Fatalf("expected --json to set autoJSON")
	}
	if cfg.Timeout != 45*time.Second {
		t.Fatalf("timeout mismatch: %s", cfg.Timeout)
	}
	if cfg.MaxOutputBytes != 2048 {
		t.Fatalf("max bytes mismatch: %d", cfg.MaxOutputBytes)
	}
	if cfg.Bin != "/tmp/gws" {
		t.Fatalf("bin mismatch: %q", cfg.Bin)
	}
	if got := strings.Join(cmdArgs, "|"); got != "--format=yaml|drive|files|list" {
		t.Fatalf("cmd args mismatch: %q", got)
	}
}

func TestGWSCommandLooksMutating(t *testing.T) {
	if !gwsCommandLooksMutating([]string{"drive", "files", "delete", "--file-id", "123"}) {
		t.Fatalf("expected delete command to be classified as mutating")
	}
	if gwsCommandLooksMutating([]string{"drive", "files", "list", "--page-size", "5"}) {
		t.Fatalf("expected list command to be classified as read-only")
	}
}

func TestCmdGWSBlocksMutatingWithoutAllowMutate(t *testing.T) {
	err := cmdGWS([]string{"drive", "files", "delete", "--file-id", "123"})
	if err == nil {
		t.Fatalf("expected mutating command to be blocked without --allow-mutate")
	}
	if !strings.Contains(err.Error(), "appears mutating") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGWSBlocksUnknownRootByDefault(t *testing.T) {
	err := cmdGWS([]string{"unknownroot", "list"})
	if err == nil {
		t.Fatalf("expected unknown root to be blocked")
	}
	if !strings.Contains(err.Error(), "blocked gws root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGWSExecutesScriptBinary(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("scripted fake gws binary test is unix-only")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "gws")
	script := "#!/bin/sh\necho \"gws-ok:$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gws: %v", err)
	}

	var runErr error
	out := captureStdout(t, func() {
		runErr = cmdGWS([]string{"--bin", bin, "drive", "files", "list"})
	})
	if runErr != nil {
		t.Fatalf("cmd gws: %v", runErr)
	}
	if !strings.Contains(out, "gws-ok:drive files list") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCmdGWSRejectsNonGWSBinaryName(t *testing.T) {
	err := cmdGWS([]string{"--bin", "/tmp/not-gws", "drive", "files", "list"})
	if err == nil {
		t.Fatalf("expected --bin validation failure")
	}
	if !strings.Contains(err.Error(), "expected executable named gws") {
		t.Fatalf("unexpected error: %v", err)
	}
}
