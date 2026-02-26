package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deeph/internal/project"
)

func TestFileWriteSafeSkillCreatesFile(t *testing.T) {
	ws := t.TempDir()
	s := &FileWriteSafeSkill{
		cfg: project.SkillConfig{
			Name: "file_write_safe",
			Type: "file_write_safe",
			Params: map[string]any{
				"max_bytes": 4096,
			},
		},
		workspace: ws,
	}
	out, err := s.Execute(context.Background(), SkillExecution{
		AgentName: "writer",
		Args: map[string]any{
			"path":    "app/api/calc/route.ts",
			"content": "export const runtime = 'nodejs';\n",
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	full := filepath.Join(ws, "app", "api", "calc", "route.ts")
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(b) != "export const runtime = 'nodejs';\n" {
		t.Fatalf("unexpected file content: %q", string(b))
	}
	if got, _ := out["created"].(bool); !got {
		t.Fatalf("expected created=true, got %#v", out["created"])
	}
	if got, _ := out["wrote"].(bool); !got {
		t.Fatalf("expected wrote=true, got %#v", out["wrote"])
	}
	if got, _ := out["detected_kind"].(string); got != "code/ts" {
		t.Fatalf("expected detected_kind=code/ts, got %q", got)
	}
}

func TestFileWriteSafeSkillRejectsOverwriteByDefault(t *testing.T) {
	ws := t.TempDir()
	full := filepath.Join(ws, "notes.txt")
	if err := os.WriteFile(full, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	s := &FileWriteSafeSkill{
		cfg:       project.SkillConfig{Name: "file_write_safe", Type: "file_write_safe"},
		workspace: ws,
	}
	_, err := s.Execute(context.Background(), SkillExecution{
		Args: map[string]any{
			"path":    "notes.txt",
			"content": "new",
		},
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "overwrite") {
		t.Fatalf("expected overwrite safety error, got %v", err)
	}
	b, _ := os.ReadFile(full)
	if string(b) != "old" {
		t.Fatalf("file should remain unchanged, got %q", string(b))
	}
}

func TestFileWriteSafeSkillOverwriteWithExpectedSHA(t *testing.T) {
	ws := t.TempDir()
	full := filepath.Join(ws, "notes.txt")
	if err := os.WriteFile(full, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	prev, err := sha1HexFile(full)
	if err != nil {
		t.Fatalf("sha1HexFile: %v", err)
	}
	s := &FileWriteSafeSkill{
		cfg:       project.SkillConfig{Name: "file_write_safe", Type: "file_write_safe"},
		workspace: ws,
	}
	_, err = s.Execute(context.Background(), SkillExecution{
		Args: map[string]any{
			"path":                   "notes.txt",
			"content":                "new",
			"overwrite":              true,
			"expected_existing_sha1": prev,
		},
	})
	if err != nil {
		t.Fatalf("overwrite with expected sha should succeed, got %v", err)
	}
	b, _ := os.ReadFile(full)
	if string(b) != "new" {
		t.Fatalf("expected overwritten content, got %q", string(b))
	}
	_, err = s.Execute(context.Background(), SkillExecution{
		Args: map[string]any{
			"path":                   "notes.txt",
			"content":                "again",
			"overwrite":              true,
			"expected_existing_sha1": "deadbeef",
		},
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "sha1 mismatch") {
		t.Fatalf("expected sha1 mismatch error, got %v", err)
	}
}

func TestFileWriteSafeSkillRejectsPathEscape(t *testing.T) {
	ws := t.TempDir()
	s := &FileWriteSafeSkill{
		cfg:       project.SkillConfig{Name: "file_write_safe", Type: "file_write_safe"},
		workspace: ws,
	}
	_, err := s.Execute(context.Background(), SkillExecution{
		Args: map[string]any{
			"path":    "../escape.txt",
			"content": "x",
		},
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "workspace") {
		t.Fatalf("expected workspace path error, got %v", err)
	}
}
