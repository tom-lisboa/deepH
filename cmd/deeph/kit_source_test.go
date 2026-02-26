package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseKitSourceRefCatalog(t *testing.T) {
	ref, err := parseKitSourceRef("hello-next-tailwind")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Kind != kitSourceCatalog {
		t.Fatalf("kind=%v want=%v", ref.Kind, kitSourceCatalog)
	}
	if ref.CatalogName != "hello-next-tailwind" {
		t.Fatalf("catalog name=%q", ref.CatalogName)
	}
}

func TestParseKitSourceRefGit(t *testing.T) {
	ref, err := parseKitSourceRef("https://github.com/acme/repo.git#kits/hello/kit.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Kind != kitSourceGit {
		t.Fatalf("kind=%v want=%v", ref.Kind, kitSourceGit)
	}
	if ref.RepoURL != "https://github.com/acme/repo.git" {
		t.Fatalf("repo=%q", ref.RepoURL)
	}
	if ref.ManifestHint != "kits/hello/kit.yaml" {
		t.Fatalf("hint=%q", ref.ManifestHint)
	}
}

func TestBuildKitTemplateFromManifest_SourceFile(t *testing.T) {
	root := t.TempDir()
	srcPath := filepath.Join(root, "templates", "hello_builder.yaml")
	if err := os.MkdirAll(filepath.Dir(srcPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	srcContent := "name: hello_builder\nprovider: deepseek\n"
	if err := os.WriteFile(srcPath, []byte(srcContent), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	m := gitKitManifest{
		Name:           "x",
		ProviderType:   "deepseek",
		RequiredSkills: []string{"echo", "echo", "file_write_safe"},
		Files: []gitKitManifestFile{
			{Path: "agents/hello_builder.yaml", Source: "templates/hello_builder.yaml"},
			{Path: "crews/hello.yaml", Content: "name: hello\nspec: hello_builder\n"},
		},
	}
	k, err := buildKitTemplateFromManifest(root, m, "fallback")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if k.Name != "x" {
		t.Fatalf("name=%q", k.Name)
	}
	if len(k.RequiredSkills) != 2 {
		t.Fatalf("skills=%v", k.RequiredSkills)
	}
	if len(k.Files) != 2 {
		t.Fatalf("files=%d", len(k.Files))
	}
	if k.Files[0].Path != "agents/hello_builder.yaml" {
		t.Fatalf("first file path=%q", k.Files[0].Path)
	}
	if k.Files[0].Content != srcContent {
		t.Fatalf("source content mismatch")
	}
}

func TestBuildKitTemplateFromManifest_InvalidSourceEscape(t *testing.T) {
	root := t.TempDir()
	m := gitKitManifest{
		Name: "x",
		Files: []gitKitManifestFile{
			{Path: "agents/a.yaml", Source: "../escape.yaml"},
		},
	}
	_, err := buildKitTemplateFromManifest(root, m, "fallback")
	if err == nil {
		t.Fatalf("expected error for source escape")
	}
}
