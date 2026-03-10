package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGWSBin            = "gws"
	defaultGWSTimeout        = 30 * time.Second
	defaultGWSMaxOutputBytes = 64 * 1024
)

var (
	errGWSHelp = errors.New("gws help requested")

	defaultGWSAllowedRoots = map[string]struct{}{
		"admin":     {},
		"auth":      {},
		"calendar":  {},
		"chat":      {},
		"classroom": {},
		"config":    {},
		"docs":      {},
		"drive":     {},
		"forms":     {},
		"gmail":     {},
		"groups":    {},
		"help":      {},
		"keep":      {},
		"meet":      {},
		"people":    {},
		"sheets":    {},
		"sites":     {},
		"slides":    {},
		"tasks":     {},
		"version":   {},
		"workspace": {},
	}

	gwsMutatingTokens = map[string]struct{}{
		"accept":    {},
		"add":       {},
		"approve":   {},
		"archive":   {},
		"attach":    {},
		"cancel":    {},
		"close":     {},
		"copy":      {},
		"create":    {},
		"delete":    {},
		"disable":   {},
		"edit":      {},
		"enable":    {},
		"import":    {},
		"insert":    {},
		"login":     {},
		"logout":    {},
		"move":      {},
		"patch":     {},
		"remove":    {},
		"rename":    {},
		"replace":   {},
		"restore":   {},
		"revoke":    {},
		"send":      {},
		"set":       {},
		"share":     {},
		"start":     {},
		"stop":      {},
		"trash":     {},
		"unarchive": {},
		"undelete":  {},
		"untrash":   {},
		"update":    {},
		"upload":    {},
		"watch":     {},
		"write":     {},
	}
)

type gwsExecConfig struct {
	Bin            string
	Timeout        time.Duration
	MaxOutputBytes int
	AutoJSON       bool
	AllowMutating  bool
	AllowAnyRoot   bool
	AllowedRoots   map[string]struct{}
}

func cmdGWS(args []string) error {
	cfg, cmdArgs, err := parseGWSExecArgs(args)
	if err != nil {
		if errors.Is(err, errGWSHelp) {
			fmt.Println(gwsUsageText())
			return nil
		}
		return err
	}
	if len(cmdArgs) == 0 {
		return errors.New("gws requires command arguments (tip: deeph gws drive files list --page-size 5)")
	}
	if err := validateGWSBinary(cfg.Bin); err != nil {
		return err
	}

	root := firstGWSRoot(cmdArgs)
	if root == "" {
		return errors.New("gws command is missing a root group (example: drive, gmail, calendar)")
	}
	if !cfg.AllowAnyRoot {
		if _, ok := cfg.AllowedRoots[root]; !ok {
			return fmt.Errorf("blocked gws root %q (allowed roots: %s); use --allow-any-root to bypass", root, strings.Join(sortedMapKeys(cfg.AllowedRoots), ", "))
		}
	}

	if gwsCommandLooksMutating(cmdArgs) && !cfg.AllowMutating {
		return fmt.Errorf("gws command appears mutating; re-run with --allow-mutate (or --yes): deeph gws --allow-mutate %s", renderChatExecArgs(cmdArgs))
	}
	if cfg.AutoJSON && !gwsHasFormatFlag(cmdArgs) {
		cmdArgs = append(cmdArgs, "--format=json")
	}
	return runGWSCommand(cfg, cmdArgs)
}

func parseGWSExecArgs(args []string) (gwsExecConfig, []string, error) {
	cfg := defaultGWSExecConfig()
	if len(args) == 0 {
		return cfg, nil, errors.New("gws requires arguments (run `deeph gws --help`)")
	}

	for i := 0; i < len(args); i++ {
		tok := strings.TrimSpace(args[i])
		if tok == "" {
			continue
		}
		if tok == "--" {
			return cfg, append([]string{}, args[i+1:]...), nil
		}
		if !strings.HasPrefix(tok, "-") {
			return cfg, append([]string{}, args[i:]...), nil
		}

		switch {
		case tok == "-h" || tok == "--help":
			return cfg, nil, errGWSHelp
		case tok == "--yes" || tok == "--allow-mutate":
			cfg.AllowMutating = true
		case tok == "--json":
			cfg.AutoJSON = true
		case tok == "--allow-any-root":
			cfg.AllowAnyRoot = true
		case strings.HasPrefix(tok, "--bin="):
			cfg.Bin = strings.TrimSpace(strings.TrimPrefix(tok, "--bin="))
		case tok == "--bin":
			if i+1 >= len(args) {
				return cfg, nil, errors.New("--bin requires a value")
			}
			i++
			cfg.Bin = strings.TrimSpace(args[i])
		case strings.HasPrefix(tok, "--timeout="):
			raw := strings.TrimSpace(strings.TrimPrefix(tok, "--timeout="))
			d, err := time.ParseDuration(raw)
			if err != nil || d <= 0 {
				return cfg, nil, fmt.Errorf("invalid --timeout value %q", raw)
			}
			cfg.Timeout = d
		case tok == "--timeout":
			if i+1 >= len(args) {
				return cfg, nil, errors.New("--timeout requires a value")
			}
			i++
			raw := strings.TrimSpace(args[i])
			d, err := time.ParseDuration(raw)
			if err != nil || d <= 0 {
				return cfg, nil, fmt.Errorf("invalid --timeout value %q", raw)
			}
			cfg.Timeout = d
		case strings.HasPrefix(tok, "--max-output-bytes="):
			raw := strings.TrimSpace(strings.TrimPrefix(tok, "--max-output-bytes="))
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				return cfg, nil, fmt.Errorf("invalid --max-output-bytes value %q", raw)
			}
			cfg.MaxOutputBytes = n
		case tok == "--max-output-bytes":
			if i+1 >= len(args) {
				return cfg, nil, errors.New("--max-output-bytes requires a value")
			}
			i++
			raw := strings.TrimSpace(args[i])
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				return cfg, nil, fmt.Errorf("invalid --max-output-bytes value %q", raw)
			}
			cfg.MaxOutputBytes = n
		default:
			// Unknown flags are treated as gws args so users can pass gws-native options.
			return cfg, append([]string{}, args[i:]...), nil
		}
	}

	return cfg, nil, nil
}

func defaultGWSExecConfig() gwsExecConfig {
	cfg := gwsExecConfig{
		Bin:            envOrDefault("DEEPH_GWS_BIN", defaultGWSBin),
		Timeout:        defaultGWSTimeout,
		MaxOutputBytes: defaultGWSMaxOutputBytes,
		AllowedRoots:   loadGWSAllowedRoots(),
	}
	if raw := strings.TrimSpace(os.Getenv("DEEPH_GWS_TIMEOUT")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			cfg.Timeout = d
		}
	}
	if raw := strings.TrimSpace(os.Getenv("DEEPH_GWS_MAX_OUTPUT_BYTES")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			cfg.MaxOutputBytes = n
		}
	}
	if parseEnvBool(os.Getenv("DEEPH_GWS_ALLOW_ANY_ROOT")) {
		cfg.AllowAnyRoot = true
	}
	return cfg
}

func loadGWSAllowedRoots() map[string]struct{} {
	out := map[string]struct{}{}
	for k := range defaultGWSAllowedRoots {
		out[k] = struct{}{}
	}
	raw := strings.TrimSpace(os.Getenv("DEEPH_GWS_ALLOWED_ROOTS"))
	if raw == "" {
		return out
	}
	for _, part := range splitCSVLike(raw) {
		token := strings.ToLower(strings.TrimSpace(part))
		if token == "" {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}

func splitCSVLike(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\t', ' ':
			return true
		default:
			return false
		}
	})
}

func parseEnvBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func validateGWSBinary(bin string) error {
	bin = strings.TrimSpace(bin)
	if bin == "" {
		return errors.New("gws binary path is empty; set --bin or DEEPH_GWS_BIN")
	}
	base := strings.ToLower(filepath.Base(bin))
	if base != "gws" && base != "gws.exe" {
		return fmt.Errorf("invalid gws binary %q (expected executable named gws)", bin)
	}
	return nil
}

func firstGWSRoot(args []string) string {
	for _, arg := range args {
		token := strings.ToLower(strings.TrimSpace(arg))
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "-") {
			continue
		}
		return token
	}
	return ""
}

func gwsCommandLooksMutating(args []string) bool {
	for _, arg := range args {
		token := strings.ToLower(strings.TrimSpace(arg))
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "-") {
			continue
		}
		if _, ok := gwsMutatingTokens[token]; ok {
			return true
		}
	}
	return false
}

func gwsHasFormatFlag(args []string) bool {
	for i := 0; i < len(args); i++ {
		token := strings.TrimSpace(args[i])
		switch {
		case strings.HasPrefix(token, "--format="):
			return true
		case token == "--format" && i+1 < len(args):
			return true
		case strings.HasPrefix(token, "--output="):
			return true
		case token == "--output" && i+1 < len(args):
			return true
		}
	}
	return false
}

func runGWSCommand(cfg gwsExecConfig, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cfg.Bin, args...)
	cmd.Env = os.Environ()
	var out cappedBuffer
	out.max = cfg.MaxOutputBytes
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if text := out.String(); text != "" {
		fmt.Print(text)
		if !strings.HasSuffix(text, "\n") {
			fmt.Println("")
		}
	}
	if out.truncated {
		fmt.Printf("[gws] output truncated to %d bytes\n", cfg.MaxOutputBytes)
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("gws timed out after %s", cfg.Timeout)
	}
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("gws binary not found (install gws or set --bin / DEEPH_GWS_BIN)")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("gws failed with exit code %d", exitErr.ExitCode())
	}
	return err
}

func sortedMapKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func gwsUsageText() string {
	lines := []string{
		"Usage:",
		"  deeph gws [--yes|--allow-mutate] [--json] [--timeout 30s] [--max-output-bytes 65536] [--bin gws] [--allow-any-root] <gws args...>",
		"  deeph gws -- <raw gws args...>",
		"",
		"Examples:",
		"  deeph gws drive files list --page-size 5",
		"  deeph gws --json gmail users.messages.list --user me --max-results 10",
		"  deeph gws --allow-mutate drive files delete --file-id 123",
		"",
		"Notes:",
		"  - Runs the external gws CLI without shell expansion (safe exec.Command).",
		"  - Blocks unknown root groups by default; use --allow-any-root to bypass.",
		"  - Commands that look mutating require --allow-mutate (or --yes).",
		"  - Use `--` to pass raw gws flags that conflict with wrapper flags.",
	}
	return strings.Join(lines, "\n")
}

type cappedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.max <= 0 {
		b.truncated = true
		return len(p), nil
	}
	remaining := b.max - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	return b.buf.String()
}
