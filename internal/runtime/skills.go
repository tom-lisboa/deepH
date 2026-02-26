package runtime

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"deeph/internal/project"
	"deeph/internal/typesys"
)

type EchoSkill struct{ cfg project.SkillConfig }

func (s *EchoSkill) Name() string        { return s.cfg.Name }
func (s *EchoSkill) Description() string { return coalesce(s.cfg.Description, "Echoes input and args") }
func (s *EchoSkill) Execute(_ context.Context, exec SkillExecution) (map[string]any, error) {
	return map[string]any{
		"agent": exec.AgentName,
		"input": exec.Input,
		"args":  exec.Args,
	}, nil
}

type FileReadSkill struct {
	cfg       project.SkillConfig
	workspace string
}

func (s *FileReadSkill) Name() string { return s.cfg.Name }
func (s *FileReadSkill) Description() string {
	return coalesce(s.cfg.Description, "Reads a file from the workspace")
}
func (s *FileReadSkill) Execute(_ context.Context, exec SkillExecution) (map[string]any, error) {
	pathVal, ok := exec.Args["path"].(string)
	if !ok || strings.TrimSpace(pathVal) == "" {
		return nil, fmt.Errorf("file_read requires args.path (string)")
	}
	clean, fullClean, err := resolveWorkspacePath(s.workspace, pathVal)
	if err != nil {
		return nil, err
	}
	maxBytes := 32768
	if v, ok := intParam(s.cfg.Params, "max_bytes"); ok && v > 0 {
		maxBytes = v
	}
	f, err := os.Open(fullClean)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, int64(maxBytes)))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"path":          clean,
		"bytes":         len(b),
		"text":          string(b),
		"content_sha1":  sha1HexBytes(b),
		"detected_kind": typesys.InferKindFromPath(clean).String(),
	}, nil
}

type FileReadRangeSkill struct {
	cfg       project.SkillConfig
	workspace string
}

func (s *FileReadRangeSkill) Name() string { return s.cfg.Name }
func (s *FileReadRangeSkill) Description() string {
	return coalesce(s.cfg.Description, "Reads a line range from a file in the workspace")
}
func (s *FileReadRangeSkill) Execute(_ context.Context, exec SkillExecution) (map[string]any, error) {
	pathVal, ok := exec.Args["path"].(string)
	if !ok || strings.TrimSpace(pathVal) == "" {
		return nil, fmt.Errorf("file_read_range requires args.path (string)")
	}
	clean, fullClean, err := resolveWorkspacePath(s.workspace, pathVal)
	if err != nil {
		return nil, err
	}

	startLine := 1
	if v, ok := intArg(exec.Args, "start_line"); ok && v > 0 {
		startLine = v
	}
	endLine := startLine + 199
	if v, ok := intArg(exec.Args, "end_line"); ok && v >= startLine {
		endLine = v
	}
	if endLine < startLine {
		return nil, fmt.Errorf("end_line must be >= start_line")
	}
	if endLine-startLine > 1000 {
		endLine = startLine + 1000
	}

	maxBytes := 32768
	if v, ok := intParam(s.cfg.Params, "max_bytes"); ok && v > 0 {
		maxBytes = v
	}
	if v, ok := intArg(exec.Args, "max_bytes"); ok && v > 0 && v < maxBytes {
		maxBytes = v
	}

	f, err := os.Open(fullClean)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	var (
		lineNo     int
		lines      []string
		bytesUsed  int
		truncated  bool
		lastLineNo int
	)
	for scanner.Scan() {
		lineNo++
		if lineNo < startLine {
			continue
		}
		if lineNo > endLine {
			break
		}
		txt := scanner.Text()
		needed := len(txt)
		if len(lines) > 0 {
			needed++ // newline
		}
		if bytesUsed+needed > maxBytes {
			truncated = true
			break
		}
		lines = append(lines, txt)
		bytesUsed += needed
		lastLineNo = lineNo
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if lastLineNo == 0 && len(lines) == 0 {
		lastLineNo = startLine - 1
	}
	text := strings.Join(lines, "\n")
	return map[string]any{
		"path":          clean,
		"start_line":    startLine,
		"end_line":      endLine,
		"actual_end":    lastLineNo,
		"lines_read":    len(lines),
		"bytes":         len(text),
		"truncated":     truncated,
		"text":          text,
		"content_sha1":  sha1HexBytes([]byte(text)),
		"detected_kind": typesys.InferKindFromPath(clean).String(),
	}, nil
}

type FileWriteSafeSkill struct {
	cfg       project.SkillConfig
	workspace string
}

func (s *FileWriteSafeSkill) Name() string { return s.cfg.Name }
func (s *FileWriteSafeSkill) Description() string {
	return coalesce(s.cfg.Description, "Writes a text file inside the workspace with safe defaults")
}
func (s *FileWriteSafeSkill) Execute(_ context.Context, exec SkillExecution) (map[string]any, error) {
	pathVal, ok := exec.Args["path"].(string)
	if !ok || strings.TrimSpace(pathVal) == "" {
		return nil, fmt.Errorf("file_write_safe requires args.path (string)")
	}
	content := coalesce(anyString(exec.Args["content"]), anyString(exec.Args["text"]))
	if strings.TrimSpace(content) == "" && !hasStringKey(exec.Args, "content") && !hasStringKey(exec.Args, "text") {
		return nil, fmt.Errorf("file_write_safe requires args.content (string)")
	}

	clean, fullClean, err := resolveWorkspacePath(s.workspace, pathVal)
	if err != nil {
		return nil, err
	}
	createDirs := true
	if v, ok := boolParam(s.cfg.Params, "create_dirs"); ok {
		createDirs = v
	}
	if v, ok := boolArg(exec.Args, "create_dirs"); ok {
		createDirs = v
	}
	createIfMissing := true
	if v, ok := boolParam(s.cfg.Params, "create_if_missing"); ok {
		createIfMissing = v
	}
	if v, ok := boolArg(exec.Args, "create_if_missing"); ok {
		createIfMissing = v
	}
	overwrite := false
	if v, ok := boolParam(s.cfg.Params, "overwrite_default"); ok {
		overwrite = v
	}
	if v, ok := boolArg(exec.Args, "overwrite"); ok {
		overwrite = v
	}
	maxBytes := 128 * 1024
	if v, ok := intParam(s.cfg.Params, "max_bytes"); ok && v > 0 {
		maxBytes = v
	}
	if v, ok := intArg(exec.Args, "max_bytes"); ok && v > 0 && v < maxBytes {
		maxBytes = v
	}
	if len(content) > maxBytes {
		return nil, fmt.Errorf("content exceeds max_bytes (%d)", maxBytes)
	}

	parent := filepath.Dir(fullClean)
	if createDirs {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return nil, err
		}
	}

	var (
		exists   bool
		created  bool
		changed  = true
		prevSHA1 string
	)
	if st, err := os.Stat(fullClean); err == nil {
		if st.IsDir() {
			return nil, fmt.Errorf("target path is a directory")
		}
		exists = true
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if !exists && !createIfMissing {
		return nil, fmt.Errorf("target file does not exist and create_if_missing=false")
	}
	if exists && !overwrite {
		return nil, fmt.Errorf("target file exists; set overwrite=true to replace it")
	}
	if exists {
		if sha, err := sha1HexFile(fullClean); err == nil {
			prevSHA1 = sha
		} else {
			return nil, err
		}
		if expected := strings.TrimSpace(coalesce(anyString(exec.Args["expected_sha1"]), anyString(exec.Args["expected_existing_sha1"]))); expected != "" && !strings.EqualFold(expected, prevSHA1) {
			return nil, fmt.Errorf("existing file sha1 mismatch (expected %s got %s)", expected, prevSHA1)
		}
		if b, err := os.ReadFile(fullClean); err == nil && bytes.Equal(b, []byte(content)) {
			changed = false
		}
	} else {
		created = true
	}
	if changed {
		if err := writeFileAtomic(fullClean, []byte(content), 0o644); err != nil {
			return nil, err
		}
	}

	result := map[string]any{
		"path":          clean,
		"bytes":         len(content),
		"wrote":         changed,
		"changed":       changed,
		"created":       created,
		"overwrote":     exists && changed,
		"content_sha1":  sha1HexBytes([]byte(content)),
		"detected_kind": typesys.InferKindFromPath(clean).String(),
	}
	if prevSHA1 != "" {
		result["previous_sha1"] = prevSHA1
	}
	if !changed && exists {
		result["note"] = "content unchanged; write skipped"
	}
	return result, nil
}

type HTTPSkill struct {
	cfg    project.SkillConfig
	client *http.Client
}

func (s *HTTPSkill) Name() string        { return s.cfg.Name }
func (s *HTTPSkill) Description() string { return coalesce(s.cfg.Description, "Makes an HTTP request") }
func (s *HTTPSkill) Execute(ctx context.Context, exec SkillExecution) (map[string]any, error) {
	method := strings.ToUpper(coalesce(anyString(exec.Args["method"]), s.cfg.Method, http.MethodGet))
	rawURL := coalesce(anyString(exec.Args["url"]), s.cfg.URL)
	if strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("http skill requires url in config or args")
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	bodyStr := anyString(exec.Args["body"])
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewBufferString(bodyStr))
	if err != nil {
		return nil, err
	}
	for k, v := range s.cfg.Headers {
		req.Header.Set(k, v)
	}
	if headers, ok := exec.Args["headers"].(map[string]any); ok {
		for k, v := range headers {
			req.Header.Set(k, anyString(v))
		}
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	maxBytes := 65536
	if v, ok := intParam(s.cfg.Params, "max_bytes"); ok && v > 0 {
		maxBytes = v
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)))
	if err != nil {
		return nil, err
	}
	contentType := resp.Header.Get("Content-Type")
	detectedKind := detectHTTPBodyKind(contentType, b)
	return map[string]any{
		"status":        resp.StatusCode,
		"body":          string(b),
		"content_type":  contentType,
		"detected_kind": detectedKind.String(),
	}, nil
}

func newSkill(workspace string, sc project.SkillConfig) Skill {
	timeout := time.Duration(sc.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	switch sc.Type {
	case "echo":
		return &EchoSkill{cfg: sc}
	case "file_read":
		return &FileReadSkill{cfg: sc, workspace: workspace}
	case "file_read_range":
		return &FileReadRangeSkill{cfg: sc, workspace: workspace}
	case "file_write_safe":
		return &FileWriteSafeSkill{cfg: sc, workspace: workspace}
	case "http":
		return &HTTPSkill{cfg: sc, client: &http.Client{Timeout: timeout}}
	default:
		return &EchoSkill{cfg: project.SkillConfig{Name: sc.Name, Description: "fallback for unsupported skill type"}}
	}
}

func resolveWorkspacePath(workspace, pathVal string) (clean string, fullClean string, err error) {
	clean = filepath.Clean(pathVal)
	if filepath.IsAbs(clean) {
		return "", "", fmt.Errorf("absolute paths are not allowed")
	}
	full := filepath.Join(workspace, clean)
	workspaceClean := filepath.Clean(workspace)
	fullClean = filepath.Clean(full)
	if !strings.HasPrefix(fullClean, workspaceClean+string(os.PathSeparator)) && fullClean != workspaceClean {
		return "", "", fmt.Errorf("path escapes workspace")
	}
	return clean, fullClean, nil
}

func anyString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}

func intParam(params map[string]any, key string) (int, bool) {
	if params == nil {
		return 0, false
	}
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func intArg(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	return intParam(args, key)
}

func boolParam(params map[string]any, key string) (bool, bool) {
	if params == nil {
		return false, false
	}
	v, ok := params[key]
	if !ok {
		return false, false
	}
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		switch strings.ToLower(strings.TrimSpace(b)) {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		}
	case int:
		return b != 0, true
	case int64:
		return b != 0, true
	case float64:
		return b != 0, true
	}
	return false, false
}

func boolArg(args map[string]any, key string) (bool, bool) {
	if args == nil {
		return false, false
	}
	return boolParam(args, key)
}

func hasStringKey(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	_, ok := args[key]
	return ok
}

func sha1HexBytes(b []byte) string {
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:])
}

func sha1HexFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeFileAtomic(path string, b []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".deeph-write-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func detectHTTPBodyKind(contentType string, body []byte) typesys.Kind {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.Contains(ct, "application/json"):
		return typesys.KindJSONObject
	case strings.Contains(ct, "text/markdown"):
		return typesys.KindTextMarkdown
	case strings.Contains(ct, "text/html"):
		return typesys.KindTextPlain
	case strings.Contains(ct, "text/"):
		return typesys.KindTextPlain
	}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "{") {
		return typesys.KindJSONObject
	}
	if strings.HasPrefix(trimmed, "[") {
		return typesys.KindJSONArray
	}
	return typesys.KindTextPlain
}
