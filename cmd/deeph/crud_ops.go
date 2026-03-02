package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type crudComposeRuntime struct {
	Program string
	Prefix  []string
	Label   string
}

type crudComposeDoc struct {
	Services map[string]crudComposeService `yaml:"services"`
}

type crudComposeService struct {
	Ports []any `yaml:"ports"`
}

type crudFileCandidate struct {
	Path  string
	Score int
}

func cmdCRUDUp(args []string) error {
	fs := flag.NewFlagSet("crud up", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	composeFile := fs.String("compose-file", "", "path to docker compose file")
	build := fs.Bool("build", true, "build images before starting containers")
	detach := fs.Bool("detach", true, "run services in background")
	waitFor := fs.Duration("wait", 45*time.Second, "time to wait for the API to become reachable")
	baseURL := fs.String("base-url", "", "explicit base URL for the API health probe")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("crud up does not accept positional arguments")
	}

	abs, err := filepath.Abs(strings.TrimSpace(*workspace))
	if err != nil {
		return err
	}
	resolvedCompose, err := resolveCRUDComposeFile(abs, *composeFile)
	if err != nil {
		if cfg, ok, _ := loadCRUDWorkspaceConfig(abs); ok && !cfg.Containers {
			return fmt.Errorf("%w. this CRUD profile currently disables containers; rerun `deeph crud init --workspace %q --containers=true` if you want container startup", err, abs)
		}
		return err
	}
	runtime, err := resolveCRUDComposeRuntime()
	if err != nil {
		return err
	}

	recordCoachCommandTransition(abs, "crud up")
	cmdArgs := append([]string{}, runtime.Prefix...)
	cmdArgs = append(cmdArgs, "-f", resolvedCompose, "up")
	if *build {
		cmdArgs = append(cmdArgs, "--build")
	}
	if *detach {
		cmdArgs = append(cmdArgs, "-d")
	}
	if err := runStreamingCommand(filepath.Dir(resolvedCompose), nil, runtime.Program, cmdArgs...); err != nil {
		return err
	}
	fmt.Printf("Started CRUD environment with %s\n", runtime.Label)
	fmt.Printf("Compose file: %s\n", resolvedCompose)

	psArgs := append([]string{}, runtime.Prefix...)
	psArgs = append(psArgs, "-f", resolvedCompose, "ps")
	if err := runStreamingCommand(filepath.Dir(resolvedCompose), nil, runtime.Program, psArgs...); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not print compose status: %v\n", err)
	}

	if *waitFor > 0 {
		probeURL := coalesce(strings.TrimSpace(*baseURL), detectCRUDBaseURLFromComposeFile(resolvedCompose))
		if probeURL != "" {
			if err := waitForCRUDHTTPReady(probeURL, defaultCRUDRouteBaseFromEntity(loadCRUDEntity(abs)), *waitFor); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			} else {
				fmt.Printf("API is reachable at %s\n", probeURL)
			}
		} else {
			fmt.Fprintln(os.Stderr, "warning: could not auto-detect the API base URL; use `deeph crud smoke --base-url http://127.0.0.1:8080` if needed")
		}
	}

	fmt.Println("Next steps:")
	fmt.Printf("  1. deeph crud smoke --workspace %q\n", abs)
	fmt.Printf("  2. deeph crud down --workspace %q\n", abs)
	return nil
}

func cmdCRUDDown(args []string) error {
	fs := flag.NewFlagSet("crud down", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	composeFile := fs.String("compose-file", "", "path to docker compose file")
	volumes := fs.Bool("volumes", false, "remove named volumes when stopping the stack")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("crud down does not accept positional arguments")
	}

	abs, err := filepath.Abs(strings.TrimSpace(*workspace))
	if err != nil {
		return err
	}
	resolvedCompose, err := resolveCRUDComposeFile(abs, *composeFile)
	if err != nil {
		return err
	}
	runtime, err := resolveCRUDComposeRuntime()
	if err != nil {
		return err
	}

	recordCoachCommandTransition(abs, "crud down")
	cmdArgs := append([]string{}, runtime.Prefix...)
	cmdArgs = append(cmdArgs, "-f", resolvedCompose, "down", "--remove-orphans")
	if *volumes {
		cmdArgs = append(cmdArgs, "--volumes")
	}
	if err := runStreamingCommand(filepath.Dir(resolvedCompose), nil, runtime.Program, cmdArgs...); err != nil {
		return err
	}
	fmt.Printf("Stopped CRUD environment defined by %s\n", resolvedCompose)
	return nil
}

func cmdCRUDSmoke(args []string) error {
	fs := flag.NewFlagSet("crud smoke", flag.ContinueOnError)
	workspace := fs.String("workspace", ".", "workspace path")
	composeFile := fs.String("compose-file", "", "path to docker compose file")
	baseURL := fs.String("base-url", "", "explicit base URL for the API under test")
	routeBase := fs.String("route-base", "", "explicit CRUD route base, ex.: /people")
	entity := fs.String("entity", "", "entity/table name for the CRUD")
	fields := fs.String("fields", "", "comma-separated fields, ex.: nome:text,cidade:text")
	noScript := fs.Bool("no-script", false, "skip generated smoke scripts and use the built-in HTTP smoke test")
	timeout := fs.Duration("timeout", 45*time.Second, "time to wait for the API to become reachable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("crud smoke does not accept positional arguments")
	}

	abs, err := filepath.Abs(strings.TrimSpace(*workspace))
	if err != nil {
		return err
	}
	cfg, cfgOK, err := loadCRUDWorkspaceConfig(abs)
	if err != nil {
		return err
	}
	if !cfgOK {
		cfg = crudWorkspaceConfig{}
	}
	if flagProvided(args, "entity") {
		cfg.Entity = strings.TrimSpace(*entity)
	}
	if flagProvided(args, "fields") {
		parsedFields, err := parseCRUDFields(*fields)
		if err != nil {
			return err
		}
		cfg.Fields = parsedFields
	}

	resolvedCompose, _ := resolveCRUDComposeFile(abs, *composeFile)
	if !*noScript {
		if scriptPath, ok := findCRUDSmokeScript(abs, resolvedCompose, goruntime.GOOS); ok {
			recordCoachCommandTransition(abs, "crud smoke")
			base := strings.TrimSpace(*baseURL)
			if base == "" && resolvedCompose != "" {
				base = detectCRUDBaseURLFromComposeFile(resolvedCompose)
			}
			route := strings.TrimSpace(*routeBase)
			if route == "" {
				route = defaultCRUDRouteBaseFromEntity(cfg.Entity)
			}
			if err := runCRUDSmokeScript(scriptPath, base, route, cfg.Entity); err != nil {
				return err
			}
			fmt.Printf("Smoke script passed: %s\n", scriptPath)
			return nil
		}
	}

	if strings.TrimSpace(cfg.Entity) == "" {
		return errors.New("crud smoke needs an entity; run `deeph crud init` first or pass --entity")
	}
	if len(cfg.Fields) == 0 {
		return errors.New("crud smoke needs fields; run `deeph crud init` first or pass --fields")
	}

	base := strings.TrimSpace(*baseURL)
	if base == "" && resolvedCompose != "" {
		base = detectCRUDBaseURLFromComposeFile(resolvedCompose)
	}
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	route := strings.TrimSpace(*routeBase)
	if route == "" {
		route = defaultCRUDRouteBaseFromEntity(cfg.Entity)
	}

	recordCoachCommandTransition(abs, "crud smoke")
	if err := waitForCRUDHTTPReady(base, route, *timeout); err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	if err := runBuiltInCRUDSmoke(client, base, route, cfg.Fields); err != nil {
		return err
	}
	fmt.Printf("CRUD smoke passed against %s%s\n", strings.TrimRight(base, "/"), route)
	return nil
}

func resolveCRUDComposeRuntime() (crudComposeRuntime, error) {
	if dockerPath, err := exec.LookPath("docker"); err == nil {
		cmd := exec.Command(dockerPath, "compose", "version")
		if err := cmd.Run(); err == nil {
			return crudComposeRuntime{
				Program: dockerPath,
				Prefix:  []string{"compose"},
				Label:   "docker compose",
			}, nil
		}
	}
	if composePath, err := exec.LookPath("docker-compose"); err == nil {
		return crudComposeRuntime{
			Program: composePath,
			Label:   "docker-compose",
		}, nil
	}
	return crudComposeRuntime{}, errors.New("docker compose is not available. install Docker Desktop or Docker Engine with Compose support and try again")
}

func resolveCRUDComposeFile(workspace, override string) (string, error) {
	raw := strings.TrimSpace(override)
	if raw != "" {
		path := raw
		if !filepath.IsAbs(path) {
			path = filepath.Join(workspace, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", fmt.Errorf("compose file %q is a directory", abs)
		}
		return abs, nil
	}
	return findCRUDComposeFile(workspace)
}

func findCRUDComposeFile(workspace string) (string, error) {
	candidates := make([]crudFileCandidate, 0, 4)
	err := filepath.WalkDir(workspace, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != workspace && shouldSkipCRUDSearchDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		base := strings.ToLower(d.Name())
		if !isCRUDComposeFileName(base) {
			return nil
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return nil
		}
		candidates = append(candidates, crudFileCandidate{
			Path:  path,
			Score: scoreCRUDComposeCandidate(rel, base),
		})
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("could not find a docker compose file in %s; run `deeph crud run` first or pass --compose-file", workspace)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Path < candidates[j].Path
		}
		return candidates[i].Score < candidates[j].Score
	})
	return candidates[0].Path, nil
}

func shouldSkipCRUDSearchDir(name string) bool {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return false
	}
	switch name {
	case ".git", ".deeph", "node_modules", ".next", ".turbo", "dist", "build", "coverage", "vendor", ".venv", "venv":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func isCRUDComposeFileName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		return true
	default:
		return false
	}
}

func scoreCRUDComposeCandidate(rel, base string) int {
	rel = filepath.Clean(rel)
	score := strings.Count(rel, string(os.PathSeparator)) * 10
	if filepath.Dir(rel) == "." {
		score -= 20
	}
	switch base {
	case "docker-compose.yml":
		score -= 5
	case "docker-compose.yaml":
		score -= 4
	case "compose.yml":
		score -= 3
	case "compose.yaml":
		score -= 2
	}
	return score
}

func findCRUDSmokeScript(workspace, composeFile, goos string) (string, bool) {
	dirs := orderedUniqueStrings(workspace, filepath.Dir(strings.TrimSpace(composeFile)))
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		for _, rel := range preferredCRUDSmokeScriptPaths(goos) {
			path := filepath.Join(dir, filepath.FromSlash(rel))
			info, err := os.Stat(path)
			if err == nil && !info.IsDir() {
				return path, true
			}
		}
	}
	return "", false
}

func preferredCRUDSmokeScriptPaths(goos string) []string {
	if strings.EqualFold(goos, "windows") {
		return []string{
			"scripts/smoke.ps1",
			"smoke.ps1",
			"scripts/smoke.cmd",
			"smoke.cmd",
			"scripts/smoke.bat",
			"smoke.bat",
			"scripts/smoke.sh",
			"smoke.sh",
		}
	}
	return []string{
		"scripts/smoke.sh",
		"smoke.sh",
		"scripts/smoke.ps1",
		"smoke.ps1",
	}
}

func orderedUniqueStrings(items ...string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func runCRUDSmokeScript(scriptPath, baseURL, routeBase, entity string) error {
	ext := strings.ToLower(filepath.Ext(scriptPath))
	env := []string{
		"CRUD_BASE_URL=" + strings.TrimSpace(baseURL),
		"CRUD_ROUTE_BASE=" + strings.TrimSpace(routeBase),
		"CRUD_ENTITY=" + strings.TrimSpace(entity),
	}
	switch ext {
	case ".ps1":
		powerShell, err := resolvePowerShellExecutable()
		if err != nil {
			return err
		}
		return runStreamingCommand(filepath.Dir(scriptPath), env, powerShell, "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	case ".cmd", ".bat":
		return runStreamingCommand(filepath.Dir(scriptPath), env, "cmd", "/c", scriptPath)
	case ".sh":
		return runStreamingCommand(filepath.Dir(scriptPath), env, "sh", scriptPath)
	default:
		return runStreamingCommand(filepath.Dir(scriptPath), env, scriptPath)
	}
}

func resolvePowerShellExecutable() (string, error) {
	for _, name := range []string{"pwsh", "powershell"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("could not find `pwsh` or `powershell` to run the generated smoke script")
}

func runStreamingCommand(dir string, extraEnv []string, program string, args ...string) error {
	cmd := exec.Command(program, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", filepath.Base(program), strings.Join(args, " "), err)
	}
	return nil
}

func detectCRUDBaseURLFromComposeFile(composeFile string) string {
	b, err := os.ReadFile(composeFile)
	if err != nil {
		return ""
	}
	var doc crudComposeDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return ""
	}
	type portCandidate struct {
		BaseURL string
		Score   int
	}
	candidates := make([]portCandidate, 0, 4)
	for serviceName, service := range doc.Services {
		for _, port := range service.Ports {
			published, ok := extractCRUDPublishedPort(port)
			if !ok || published <= 0 {
				continue
			}
			score := 100
			lowerName := strings.ToLower(strings.TrimSpace(serviceName))
			switch {
			case strings.Contains(lowerName, "api"), strings.Contains(lowerName, "backend"), strings.Contains(lowerName, "server"):
				score -= 30
			case strings.Contains(lowerName, "frontend"), strings.Contains(lowerName, "web"), strings.Contains(lowerName, "next"):
				score += 10
			}
			switch published {
			case 8080:
				score -= 20
			case 8000:
				score -= 15
			case 3000:
				score += 5
			}
			candidates = append(candidates, portCandidate{
				BaseURL: fmt.Sprintf("http://127.0.0.1:%d", published),
				Score:   score,
			})
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].BaseURL < candidates[j].BaseURL
		}
		return candidates[i].Score < candidates[j].Score
	})
	return candidates[0].BaseURL
}

func extractCRUDPublishedPort(port any) (int, bool) {
	switch v := port.(type) {
	case string:
		return parseCRUDPublishedPortString(v)
	case map[string]any:
		if raw, ok := v["published"]; ok {
			return parseCRUDPortValue(raw)
		}
	case map[any]any:
		if raw, ok := v["published"]; ok {
			return parseCRUDPortValue(raw)
		}
	}
	return 0, false
}

func parseCRUDPortValue(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		port, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			return 0, false
		}
		return port, true
	default:
		return 0, false
	}
}

func parseCRUDPublishedPortString(raw string) (int, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, false
	}
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 0, false
	}
	host := strings.TrimSpace(parts[len(parts)-2])
	port, err := strconv.Atoi(host)
	if err != nil {
		return 0, false
	}
	return port, true
}

func waitForCRUDHTTPReady(baseURL, routeBase string, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}
	client := &http.Client{Timeout: 3 * time.Second}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	routeBase = normalizeCRUDRouteBase(routeBase)
	deadline := time.Now().Add(timeout)
	for {
		if probeCRUDURL(client, joinCRUDURL(baseURL, "/health")) || probeCRUDURL(client, joinCRUDURL(baseURL, "/healthz")) || probeCRUDURL(client, joinCRUDURL(baseURL, routeBase)) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("API did not become reachable at %s within %s", baseURL, timeout)
		}
		time.Sleep(time.Second)
	}
}

func probeCRUDURL(client *http.Client, rawURL string) bool {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode > 0 && resp.StatusCode < 500
}

func runBuiltInCRUDSmoke(client *http.Client, baseURL, routeBase string, fields []crudField) error {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	routeBase = normalizeCRUDRouteBase(routeBase)
	createPayload := buildCRUDSamplePayload(fields, false)
	updatePayload := buildCRUDSamplePayload(fields, true)

	createStatus, createBody, createHeaders, err := doCRUDJSONRequest(client, http.MethodPost, joinCRUDURL(baseURL, routeBase), createPayload)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	if createStatus < 200 || createStatus >= 300 {
		return fmt.Errorf("create request returned HTTP %d", createStatus)
	}

	id, ok := extractCRUDID(createBody)
	if !ok {
		if location := strings.TrimSpace(createHeaders.Get("Location")); location != "" {
			id = crudIDFromLocation(location)
			ok = id != ""
		}
	}

	listStatus, listBody, _, err := doCRUDJSONRequest(client, http.MethodGet, joinCRUDURL(baseURL, routeBase), nil)
	if err != nil {
		return fmt.Errorf("list request failed: %w", err)
	}
	if listStatus < 200 || listStatus >= 300 {
		return fmt.Errorf("list request returned HTTP %d", listStatus)
	}
	if !ok {
		id, ok = extractCRUDID(listBody)
	}
	if !ok || strings.TrimSpace(id) == "" {
		return errors.New("could not extract the created resource id from create/list responses")
	}

	itemURL := joinCRUDURL(baseURL, routeBase+"/"+url.PathEscape(id))
	readStatus, _, _, err := doCRUDJSONRequest(client, http.MethodGet, itemURL, nil)
	if err != nil {
		return fmt.Errorf("read request failed: %w", err)
	}
	if readStatus < 200 || readStatus >= 300 {
		return fmt.Errorf("read request returned HTTP %d", readStatus)
	}

	updateStatus, _, _, err := doCRUDJSONRequest(client, http.MethodPut, itemURL, updatePayload)
	if err != nil {
		return fmt.Errorf("update request failed: %w", err)
	}
	if updateStatus == http.StatusMethodNotAllowed {
		updateStatus, _, _, err = doCRUDJSONRequest(client, http.MethodPatch, itemURL, updatePayload)
		if err != nil {
			return fmt.Errorf("patch request failed: %w", err)
		}
	}
	if updateStatus < 200 || updateStatus >= 300 {
		return fmt.Errorf("update request returned HTTP %d", updateStatus)
	}

	deleteStatus, _, _, err := doCRUDJSONRequest(client, http.MethodDelete, itemURL, nil)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	if deleteStatus != http.StatusNoContent && (deleteStatus < 200 || deleteStatus >= 300) {
		return fmt.Errorf("delete request returned HTTP %d", deleteStatus)
	}
	return nil
}

func doCRUDJSONRequest(client *http.Client, method, rawURL string, payload any) (int, []byte, http.Header, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return 0, nil, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, err
	}
	return resp.StatusCode, respBody, resp.Header.Clone(), nil
}

func buildCRUDSamplePayload(fields []crudField, updated bool) map[string]any {
	payload := make(map[string]any, len(fields))
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" || strings.EqualFold(name, "id") || strings.EqualFold(name, "_id") {
			continue
		}
		payload[name] = sampleCRUDValue(field, updated)
	}
	return payload
}

func sampleCRUDValue(field crudField, updated bool) any {
	typ := strings.ToLower(strings.TrimSpace(field.Type))
	name := strings.ToLower(strings.TrimSpace(field.Name))
	switch typ {
	case "int", "integer", "serial", "bigint", "smallint":
		if updated {
			return 2
		}
		return 1
	case "float", "double", "decimal", "numeric", "number":
		if updated {
			return 2.5
		}
		return 1.5
	case "bool", "boolean":
		return updated
	case "uuid":
		if updated {
			return "00000000-0000-0000-0000-000000000002"
		}
		return "00000000-0000-0000-0000-000000000001"
	case "date":
		if updated {
			return "2026-03-03"
		}
		return "2026-03-02"
	case "datetime", "timestamp", "timestamptz":
		if updated {
			return "2026-03-03T10:00:00Z"
		}
		return "2026-03-02T10:00:00Z"
	}
	switch {
	case strings.Contains(name, "email"):
		if updated {
			return "updated@example.com"
		}
		return "teste@example.com"
	case strings.Contains(name, "cidade"):
		if updated {
			return "Varzea Grande"
		}
		return "Cuiaba"
	case strings.Contains(name, "nome"), strings.Contains(name, "name"):
		if updated {
			return "Registro atualizado"
		}
		return "Registro teste"
	default:
		if updated {
			return "valor-atualizado"
		}
		return "valor-teste"
	}
}

func extractCRUDID(body []byte) (string, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return "", false
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", false
	}
	return extractCRUDIDValue(decoded)
}

func extractCRUDIDValue(v any) (string, bool) {
	switch item := v.(type) {
	case map[string]any:
		for _, key := range []string{"id", "_id"} {
			if raw, ok := item[key]; ok {
				if id := stringifyCRUDID(raw); id != "" {
					return id, true
				}
			}
		}
		for _, key := range []string{"data", "item", "record", "result"} {
			if raw, ok := item[key]; ok {
				if id, ok := extractCRUDIDValue(raw); ok {
					return id, true
				}
			}
		}
	case []any:
		for _, raw := range item {
			if id, ok := extractCRUDIDValue(raw); ok {
				return id, true
			}
		}
	}
	return "", false
}

func stringifyCRUDID(v any) string {
	switch item := v.(type) {
	case string:
		return strings.TrimSpace(item)
	case float64:
		if item == float64(int64(item)) {
			return strconv.FormatInt(int64(item), 10)
		}
		return strconv.FormatFloat(item, 'f', -1, 64)
	case int:
		return strconv.Itoa(item)
	case int64:
		return strconv.FormatInt(item, 10)
	case json.Number:
		return item.String()
	default:
		return fmt.Sprint(item)
	}
}

func crudIDFromLocation(location string) string {
	location = strings.TrimSpace(location)
	if location == "" {
		return ""
	}
	if parsed, err := url.Parse(location); err == nil {
		location = parsed.Path
	}
	location = strings.TrimRight(location, "/")
	if location == "" {
		return ""
	}
	parts := strings.Split(location, "/")
	return strings.TrimSpace(parts[len(parts)-1])
}

func joinCRUDURL(baseURL, path string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path = normalizeCRUDRouteBase(path)
	if baseURL == "" {
		return path
	}
	return baseURL + path
}

func normalizeCRUDRouteBase(routeBase string) string {
	routeBase = strings.TrimSpace(routeBase)
	if routeBase == "" {
		return "/people"
	}
	if !strings.HasPrefix(routeBase, "/") {
		routeBase = "/" + routeBase
	}
	return strings.TrimRight(routeBase, "/")
}

func defaultCRUDRouteBaseFromEntity(entity string) string {
	entity = strings.Trim(strings.TrimSpace(entity), "/")
	if entity == "" {
		return "/people"
	}
	return "/" + entity
}

func loadCRUDEntity(workspace string) string {
	cfg, ok, err := loadCRUDWorkspaceConfig(workspace)
	if err != nil || !ok {
		return "people"
	}
	return cfg.Entity
}
