package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFindCRUDComposeFilePrefersRootCompose(t *testing.T) {
	ws := t.TempDir()
	rootCompose := filepath.Join(ws, "docker-compose.yml")
	nestedDir := filepath.Join(ws, "app", "deploy")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(rootCompose, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write root compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write nested compose: %v", err)
	}

	got, err := findCRUDComposeFile(ws)
	if err != nil {
		t.Fatalf("find compose: %v", err)
	}
	if got != rootCompose {
		t.Fatalf("compose=%q want %q", got, rootCompose)
	}
}

func TestDetectCRUDBaseURLFromComposeFilePrefersBackendPort(t *testing.T) {
	ws := t.TempDir()
	composePath := filepath.Join(ws, "docker-compose.yml")
	content := `
services:
  frontend:
    ports:
      - "3000:3000"
  api:
    ports:
      - "8080:8080"
`
	if err := os.WriteFile(composePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	got := detectCRUDBaseURLFromComposeFile(composePath)
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("baseURL=%q", got)
	}
}

func TestRunBuiltInCRUDSmoke(t *testing.T) {
	store := map[string]map[string]any{}
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := ""
			status := http.StatusNotFound
			headers := make(http.Header)
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/players":
				store["1"] = map[string]any{"id": 1, "nome": "Registro teste", "cidade": "Cuiaba"}
				status = http.StatusCreated
				headers.Set("Content-Type", "application/json")
				body = `{"id":1,"nome":"Registro teste","cidade":"Cuiaba"}`
			case r.Method == http.MethodGet && r.URL.Path == "/players":
				status = http.StatusOK
				headers.Set("Content-Type", "application/json")
				body = `[{"id":1,"nome":"Registro teste","cidade":"Cuiaba"}]`
			case r.Method == http.MethodGet && r.URL.Path == "/players/1":
				status = http.StatusOK
				headers.Set("Content-Type", "application/json")
				body = `{"id":1,"nome":"Registro teste","cidade":"Cuiaba"}`
			case r.Method == http.MethodPut && r.URL.Path == "/players/1":
				status = http.StatusOK
				headers.Set("Content-Type", "application/json")
				body = `{"id":1}`
			case r.Method == http.MethodDelete && r.URL.Path == "/players/1":
				delete(store, "1")
				status = http.StatusNoContent
			}
			return &http.Response{
				StatusCode: status,
				Header:     headers,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
				Request:    r,
			}, nil
		}),
	}
	if err := runBuiltInCRUDSmoke(client, "http://127.0.0.1:8080", "/players", []crudField{
		{Name: "nome", Type: "text"},
		{Name: "cidade", Type: "text"},
	}); err != nil {
		t.Fatalf("built-in smoke failed: %v", err)
	}
	if len(store) != 0 {
		t.Fatalf("expected delete to clean the record, store=%v", store)
	}
}

func TestFindCRUDSmokeScriptPrefersPlatformScript(t *testing.T) {
	ws := t.TempDir()
	scriptsDir := filepath.Join(ws, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ps1Path := filepath.Join(scriptsDir, "smoke.ps1")
	shPath := filepath.Join(scriptsDir, "smoke.sh")
	if err := os.WriteFile(ps1Path, []byte("Write-Host ok\n"), 0o644); err != nil {
		t.Fatalf("write ps1: %v", err)
	}
	if err := os.WriteFile(shPath, []byte("echo ok\n"), 0o644); err != nil {
		t.Fatalf("write sh: %v", err)
	}

	got, ok := findCRUDSmokeScript(ws, "", "windows")
	if !ok {
		t.Fatalf("expected smoke script")
	}
	if !strings.HasSuffix(strings.ToLower(got), "smoke.ps1") {
		t.Fatalf("script=%q", got)
	}
}
