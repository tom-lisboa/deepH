package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCRUDFieldsDefaultsTypeToText(t *testing.T) {
	got, err := parseCRUDFields("nome,cidade:text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("fields=%d want=2", len(got))
	}
	if got[0].Name != "nome" || got[0].Type != "text" {
		t.Fatalf("first field=%+v", got[0])
	}
	if got[1].Name != "cidade" || got[1].Type != "text" {
		t.Fatalf("second field=%+v", got[1])
	}
}

func TestBuildCRUDPromptOpinionatedDefaults(t *testing.T) {
	got := buildCRUDPrompt(crudPromptOptions{
		Entity:     "people",
		Fields:     []crudField{{Name: "nome", Type: "text"}, {Name: "cidade", Type: "text"}},
		DB:         "postgres",
		Backend:    "go",
		Frontend:   "next",
		Containers: true,
	})
	for _, want := range []string{
		"Backend obrigatorio em Go.",
		"Frontend obrigatorio em Next.js.",
		"Banco obrigatorio: Postgres.",
		"Docker Compose",
		"- nome (text)",
		"- cidade (text)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, got)
		}
	}
}

func TestBuildCRUDPromptBackendOnly(t *testing.T) {
	got := buildCRUDPrompt(crudPromptOptions{
		Entity:      "players",
		Fields:      []crudField{{Name: "nome", Type: "text"}},
		DB:          "postgres",
		Backend:     "go",
		BackendOnly: true,
	})
	if strings.Contains(got, "Frontend obrigatorio") {
		t.Fatalf("did not expect frontend requirement in backend-only prompt:\n%s", got)
	}
	if !strings.Contains(got, "Gere apenas backend e infra local. Nao gere frontend.") {
		t.Fatalf("expected backend-only instruction, got:\n%s", got)
	}
}

func TestChooseCRUDCrew(t *testing.T) {
	cases := []struct {
		name string
		opts crudPromptOptions
		want string
	}{
		{
			name: "relational backend",
			opts: crudPromptOptions{DBKind: "relational", BackendOnly: true},
			want: "crud_backend_relational",
		},
		{
			name: "relational fullstack",
			opts: crudPromptOptions{DBKind: "relational", BackendOnly: false},
			want: "crud_fullstack_relational",
		},
		{
			name: "document backend",
			opts: crudPromptOptions{DBKind: "document", BackendOnly: true},
			want: "crud_backend_document",
		},
		{
			name: "document fullstack",
			opts: crudPromptOptions{DBKind: "document", BackendOnly: false},
			want: "crud_fullstack_document",
		},
	}
	for _, tc := range cases {
		if got := chooseCRUDCrew(tc.opts); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestCRUDWorkspaceConfigRoundTrip(t *testing.T) {
	ws := t.TempDir()
	cfg := crudWorkspaceConfig{
		Version:     1,
		Entity:      "players",
		Fields:      []crudField{{Name: "nome", Type: "text"}, {Name: "cidade", Type: "text"}},
		DBKind:      "document",
		DB:          "mongodb",
		Backend:     "go",
		Frontend:    "next",
		BackendOnly: false,
		Containers:  true,
	}
	if err := saveCRUDWorkspaceConfig(ws, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if gotPath := crudConfigPath(ws); gotPath != filepath.Join(ws, ".deeph", "crud.json") {
		t.Fatalf("config path=%q", gotPath)
	}
	got, ok, err := loadCRUDWorkspaceConfig(ws)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !ok {
		t.Fatalf("expected saved config to exist")
	}
	if got.Entity != "players" || got.DBKind != "document" || got.DB != "mongodb" {
		t.Fatalf("unexpected loaded config: %+v", got)
	}
	if len(got.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %+v", got.Fields)
	}
}
