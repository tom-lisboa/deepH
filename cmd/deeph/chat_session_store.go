package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type chatSessionMeta struct {
	ID        string    `json:"id"`
	AgentSpec string    `json:"agent_spec"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Turns     int       `json:"turns"`
}

type chatSessionEntry struct {
	Turn      int       `json:"turn"`
	Role      string    `json:"role"`
	Agent     string    `json:"agent,omitempty"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func openOrCreateChatSession(workspace, requestedID, requestedSpec string) (*chatSessionMeta, []chatSessionEntry, bool, error) {
	if err := os.MkdirAll(chatSessionsDir(workspace), 0o755); err != nil {
		return nil, nil, false, fmt.Errorf("create sessions dir: %w", err)
	}
	id := strings.TrimSpace(requestedID)
	if id == "" {
		id = generateChatSessionID(requestedSpec)
	}
	if !validSessionID(id) {
		return nil, nil, false, fmt.Errorf("invalid session id %q (use letters, numbers, -, _)", id)
	}

	meta, err := loadChatSessionMeta(workspace, id)
	if err == nil {
		if strings.TrimSpace(requestedSpec) != "" && strings.TrimSpace(meta.AgentSpec) != "" && strings.TrimSpace(requestedSpec) != strings.TrimSpace(meta.AgentSpec) {
			return nil, nil, false, fmt.Errorf("session %q already uses agent spec %q (requested %q)", id, meta.AgentSpec, requestedSpec)
		}
		if strings.TrimSpace(meta.AgentSpec) == "" && strings.TrimSpace(requestedSpec) != "" {
			meta.AgentSpec = strings.TrimSpace(requestedSpec)
			meta.UpdatedAt = time.Now()
			_ = saveChatSessionMeta(workspace, meta)
		}
		entries, lerr := loadChatSessionEntries(workspace, id)
		if lerr != nil {
			return nil, nil, false, lerr
		}
		return meta, entries, false, nil
	}
	if !os.IsNotExist(err) {
		return nil, nil, false, err
	}

	spec := strings.TrimSpace(requestedSpec)
	meta = &chatSessionMeta{
		ID:        id,
		AgentSpec: spec,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Turns:     0,
	}
	if err := saveChatSessionMeta(workspace, meta); err != nil {
		return nil, nil, false, err
	}
	return meta, nil, true, nil
}

func chatSessionsDir(workspace string) string {
	return filepath.Join(workspace, "sessions")
}

func chatSessionMetaPath(workspace, id string) string {
	return filepath.Join(chatSessionsDir(workspace), id+".meta.json")
}

func chatSessionLogPath(workspace, id string) string {
	return filepath.Join(chatSessionsDir(workspace), id+".jsonl")
}

func loadChatSessionMeta(workspace, id string) (*chatSessionMeta, error) {
	path := chatSessionMetaPath(workspace, id)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta chatSessionMeta
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if strings.TrimSpace(meta.ID) == "" {
		meta.ID = id
	}
	return &meta, nil
}

func saveChatSessionMeta(workspace string, meta *chatSessionMeta) error {
	if meta == nil {
		return fmt.Errorf("session meta is nil")
	}
	if err := os.MkdirAll(chatSessionsDir(workspace), 0o755); err != nil {
		return err
	}
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = time.Now()
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session meta: %w", err)
	}
	path := chatSessionMetaPath(workspace, meta.ID)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func appendChatSessionEntries(workspace, id string, entries []chatSessionEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(chatSessionsDir(workspace), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(chatSessionLogPath(workspace, id), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open session log: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if e.CreatedAt.IsZero() {
			e.CreatedAt = time.Now()
		}
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("append session entry: %w", err)
		}
	}
	return nil
}

func loadChatSessionEntries(workspace, id string) ([]chatSessionEntry, error) {
	path := chatSessionLogPath(workspace, id)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	out := make([]chatSessionEntry, 0, 64)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e chatSessionEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

func listChatSessionMetas(workspace string) ([]chatSessionMeta, error) {
	dir := chatSessionsDir(workspace)
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	out := make([]chatSessionMeta, 0, len(ents))
	for _, ent := range ents {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".meta.json") {
			continue
		}
		id := strings.TrimSuffix(ent.Name(), ".meta.json")
		meta, err := loadChatSessionMeta(workspace, id)
		if err != nil {
			continue
		}
		out = append(out, *meta)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func validSessionID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func generateChatSessionID(agentSpec string) string {
	base := "chat"
	spec := strings.TrimSpace(agentSpec)
	if spec != "" {
		spec = strings.ToLower(spec)
		spec = strings.NewReplacer("+", "-", ">", "-", "\"", "", "'", "", " ", "-").Replace(spec)
		spec = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				return r
			}
			return -1
		}, spec)
		spec = strings.Trim(spec, "-_")
		if spec != "" {
			base = spec
		}
	}
	ts := time.Now().Format("20060102-150405")
	id := base + "-" + ts
	if len(id) > 64 {
		id = id[:64]
	}
	return id
}
