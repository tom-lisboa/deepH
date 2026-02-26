package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"deeph/internal/catalog"
	"gopkg.in/yaml.v3"
)

type kitSourceKind string

const (
	kitSourceCatalog kitSourceKind = "catalog"
	kitSourceGit     kitSourceKind = "git"
)

type kitSourceRef struct {
	Kind         kitSourceKind
	Raw          string
	CatalogName  string
	RepoURL      string
	ManifestHint string
}

type gitKitManifest struct {
	Name           string               `yaml:"name"`
	Description    string               `yaml:"description"`
	ProviderType   string               `yaml:"provider_type"`
	RequiredSkills []string             `yaml:"required_skills"`
	Files          []gitKitManifestFile `yaml:"files"`
}

type gitKitManifestFile struct {
	Path    string `yaml:"path"`
	Source  string `yaml:"source,omitempty"`
	Content string `yaml:"content,omitempty"`
}

func resolveKitTemplate(raw string) (catalog.KitTemplate, string, error) {
	ref, err := parseKitSourceRef(raw)
	if err != nil {
		return catalog.KitTemplate{}, "", err
	}
	switch ref.Kind {
	case kitSourceCatalog:
		k, err := catalog.GetKit(ref.CatalogName)
		if err != nil {
			return catalog.KitTemplate{}, "", err
		}
		return k, "catalog:" + ref.CatalogName, nil
	case kitSourceGit:
		k, err := loadKitTemplateFromGit(ref)
		if err != nil {
			return catalog.KitTemplate{}, "", err
		}
		label := "git:" + ref.RepoURL
		if strings.TrimSpace(ref.ManifestHint) != "" {
			label += "#" + ref.ManifestHint
		}
		return k, label, nil
	default:
		return catalog.KitTemplate{}, "", fmt.Errorf("unsupported kit source")
	}
}

func parseKitSourceRef(raw string) (kitSourceRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return kitSourceRef{}, fmt.Errorf("empty kit reference")
	}
	base := raw
	frag := ""
	if i := strings.Index(raw, "#"); i >= 0 {
		base = strings.TrimSpace(raw[:i])
		frag = strings.TrimSpace(raw[i+1:])
	}
	if looksLikeGitURL(base) {
		return kitSourceRef{
			Kind:         kitSourceGit,
			Raw:          raw,
			RepoURL:      base,
			ManifestHint: frag,
		}, nil
	}
	if frag != "" {
		return kitSourceRef{}, fmt.Errorf("manifest fragment is only supported for git URLs")
	}
	return kitSourceRef{
		Kind:        kitSourceCatalog,
		Raw:         raw,
		CatalogName: base,
	}, nil
}

func looksLikeGitURL(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return false
	}
	return strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "ssh://") ||
		strings.HasPrefix(s, "git@") ||
		strings.HasSuffix(s, ".git")
}

func loadKitTemplateFromGit(ref kitSourceRef) (catalog.KitTemplate, error) {
	tmpDir, err := os.MkdirTemp("", "deeph-kit-repo-*")
	if err != nil {
		return catalog.KitTemplate{}, err
	}
	defer os.RemoveAll(tmpDir)

	clone := exec.Command("git", "clone", "--depth", "1", "--quiet", ref.RepoURL, tmpDir)
	out, err := clone.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return catalog.KitTemplate{}, fmt.Errorf("git clone failed for %q: %s", ref.RepoURL, msg)
	}

	manifestPath, err := findGitKitManifest(tmpDir, ref.ManifestHint)
	if err != nil {
		return catalog.KitTemplate{}, err
	}
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return catalog.KitTemplate{}, err
	}
	var m gitKitManifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return catalog.KitTemplate{}, fmt.Errorf("parse %s: %w", manifestPath, err)
	}
	return buildKitTemplateFromManifest(tmpDir, m, fallbackKitNameFromRepo(ref.RepoURL))
}

func findGitKitManifest(repoRoot, hint string) (string, error) {
	candidates := []string{}
	if h := strings.TrimSpace(hint); h != "" {
		h = strings.TrimPrefix(filepath.Clean(h), "/")
		if strings.HasSuffix(strings.ToLower(h), ".yaml") || strings.HasSuffix(strings.ToLower(h), ".yml") {
			candidates = append(candidates, h)
		} else {
			candidates = append(candidates,
				filepath.Join(h, "deeph-kit.yaml"),
				filepath.Join(h, "deeph-kit.yml"),
				filepath.Join(h, "kit.yaml"),
				filepath.Join(h, "kit.yml"),
			)
		}
	} else {
		candidates = append(candidates, "deeph-kit.yaml", "deeph-kit.yml", "kit.yaml", "kit.yml")
	}

	for _, c := range candidates {
		p, err := secureSourcePath(repoRoot, c)
		if err != nil {
			continue
		}
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.IsDir() {
			continue
		}
		return p, nil
	}
	if strings.TrimSpace(hint) != "" {
		return "", fmt.Errorf("kit manifest not found in git repo (%q)", hint)
	}
	return "", fmt.Errorf("kit manifest not found in git repo (expected deeph-kit.yaml or kit.yaml)")
}

func buildKitTemplateFromManifest(repoRoot string, m gitKitManifest, fallbackName string) (catalog.KitTemplate, error) {
	name := strings.TrimSpace(m.Name)
	if name == "" {
		name = fallbackName
	}
	if name == "" {
		return catalog.KitTemplate{}, fmt.Errorf("manifest missing name")
	}
	out := catalog.KitTemplate{
		Name:         name,
		Description:  strings.TrimSpace(m.Description),
		ProviderType: strings.TrimSpace(strings.ToLower(m.ProviderType)),
	}
	if out.Description == "" {
		out.Description = "Kit imported from git repository"
	}
	out.RequiredSkills = dedupeSorted(m.RequiredSkills)

	if len(m.Files) == 0 {
		return catalog.KitTemplate{}, fmt.Errorf("manifest %q defines no files", name)
	}
	for _, f := range m.Files {
		dst := strings.TrimSpace(f.Path)
		if dst == "" {
			return catalog.KitTemplate{}, fmt.Errorf("manifest %q contains file entry without path", name)
		}
		if strings.TrimSpace(f.Source) != "" && strings.TrimSpace(f.Content) != "" {
			return catalog.KitTemplate{}, fmt.Errorf("manifest %q file %q cannot define both source and content", name, dst)
		}
		content := strings.TrimSpace(f.Content)
		if src := strings.TrimSpace(f.Source); src != "" {
			srcPath, err := secureSourcePath(repoRoot, src)
			if err != nil {
				return catalog.KitTemplate{}, fmt.Errorf("manifest %q file %q invalid source %q: %w", name, dst, src, err)
			}
			b, err := os.ReadFile(srcPath)
			if err != nil {
				return catalog.KitTemplate{}, fmt.Errorf("manifest %q file %q read source %q: %w", name, dst, src, err)
			}
			content = string(b)
		}
		if content == "" {
			return catalog.KitTemplate{}, fmt.Errorf("manifest %q file %q has empty content", name, dst)
		}
		out.Files = append(out.Files, catalog.KitFile{
			Path:    dst,
			Content: content,
		})
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	return out, nil
}

func dedupeSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func secureSourcePath(root, rel string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(rel))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("invalid path %q", rel)
	}
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("path must be relative")
	}
	dst := filepath.Join(root, clean)
	relOut, err := filepath.Rel(root, dst)
	if err != nil {
		return "", err
	}
	if relOut == "." || strings.HasPrefix(relOut, "..") {
		return "", fmt.Errorf("path escapes root")
	}
	return dst, nil
}

func fallbackKitNameFromRepo(repoURL string) string {
	s := strings.TrimSpace(repoURL)
	s = strings.TrimSuffix(s, "/")
	base := filepath.Base(s)
	base = strings.TrimSuffix(base, ".git")
	base = strings.TrimSpace(base)
	if base == "" || base == "." {
		return "remote-kit"
	}
	base = strings.ReplaceAll(base, " ", "-")
	return base
}
