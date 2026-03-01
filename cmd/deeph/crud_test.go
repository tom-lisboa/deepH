package main

import (
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
