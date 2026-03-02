package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQuickstartInstallsCodeAndReviewStartersForGuide(t *testing.T) {
	ws := t.TempDir()

	if err := cmdQuickstart([]string{"--workspace", ws, "--provider", "local_mock", "--model", "mock-small"}); err != nil {
		t.Fatalf("quickstart: %v", err)
	}

	for _, rel := range []string{
		"skills/echo.yaml",
		"skills/command_doc.yaml",
		"skills/file_read_range.yaml",
		"skills/file_write_safe.yaml",
		"agents/guide.yaml",
		"agents/coder.yaml",
		"agents/diagnoser.yaml",
		"agents/reviewer.yaml",
		"agents/review_synth.yaml",
		"crews/reviewflow.yaml",
	} {
		if _, err := os.Stat(filepath.Join(ws, rel)); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}
}
