package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"deeph/internal/runtime"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	daemonServiceName   = "deeph.daemon.v1.DaemonService"
	daemonMethodPing    = "/" + daemonServiceName + "/Ping"
	daemonMethodRun     = "/" + daemonServiceName + "/Run"
	daemonMethodTrace   = "/" + daemonServiceName + "/Trace"
	daemonMethodStop    = "/" + daemonServiceName + "/Shutdown"
	daemonTargetEnvVar  = "DEEPH_DAEMON_TARGET"
	daemonDebugEnvVar   = "DEEPH_DAEMON_DEBUG"
	defaultDaemonTarget = "127.0.0.1:7788"
	defaultDaemonDialTO = 1500 * time.Millisecond
)

var deephDaemonConnCache = struct {
	mu    sync.Mutex
	conns map[string]*grpc.ClientConn
}{
	conns: map[string]*grpc.ClientConn{},
}

var deephDaemonConnStats = struct {
	hits   atomic.Uint64
	misses atomic.Uint64
	dials  atomic.Uint64
	drops  atomic.Uint64
}{}

type daemonConnStatsSnapshot struct {
	Hits   uint64
	Misses uint64
	Dials  uint64
	Drops  uint64
}

type daemonRunRequest struct {
	Workspace           string `json:"workspace"`
	AgentSpecArg        string `json:"agent_spec_arg"`
	Input               string `json:"input"`
	Multiverse          int    `json:"multiverse"`
	JudgeAgent          string `json:"judge_agent,omitempty"`
	JudgeMaxOutputChars int    `json:"judge_max_output_chars,omitempty"`
}

type daemonTraceRequest struct {
	Workspace    string `json:"workspace"`
	AgentSpecArg string `json:"agent_spec_arg"`
	Input        string `json:"input"`
	Multiverse   int    `json:"multiverse"`
}

type daemonTraceResponse struct {
	Workspace        string                      `json:"workspace"`
	Source           string                      `json:"source"`
	ResolvedSpec     string                      `json:"resolved_spec,omitempty"`
	Scheduler        string                      `json:"scheduler"`
	Multiverse       bool                        `json:"multiverse"`
	Plan             runtime.ExecutionPlan       `json:"plan,omitempty"`
	UniverseHandoffs []multiverseUniverseHandoff `json:"universe_handoffs,omitempty"`
	Branches         []multiverseTraceBranch     `json:"branches,omitempty"`
}

type daemonRunResponse struct {
	Workspace        string                      `json:"workspace"`
	Source           string                      `json:"source"`
	ResolvedSpec     string                      `json:"resolved_spec,omitempty"`
	Scheduler        string                      `json:"scheduler"`
	Multiverse       bool                        `json:"multiverse"`
	Plan             runtime.ExecutionPlan       `json:"plan,omitempty"`
	Report           runtime.ExecutionReport     `json:"report,omitempty"`
	UniverseHandoffs []multiverseUniverseHandoff `json:"universe_handoffs,omitempty"`
	Branches         []multiverseRunBranch       `json:"branches,omitempty"`
	Judge            multiverseJudgeRun          `json:"judge,omitempty"`
}

func cmdDaemon(args []string) error {
	if len(args) == 0 {
		return errors.New("daemon requires a subcommand: serve, start, status or stop")
	}
	switch args[0] {
	case "serve":
		return cmdDaemonServe(args[1:])
	case "start":
		return cmdDaemonStart(args[1:])
	case "status":
		return cmdDaemonStatus(args[1:])
	case "stop":
		return cmdDaemonStop(args[1:])
	default:
		return fmt.Errorf("unknown daemon subcommand %q", args[0])
	}
}

func cmdDaemonServe(args []string) error {
	fs := flag.NewFlagSet("daemon serve", flag.ContinueOnError)
	target := fs.String("target", deephDaemonDefaultTarget(), "daemon listen target (host:port)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("daemon serve does not accept positional arguments")
	}
	listener, err := net.Listen("tcp", strings.TrimSpace(*target))
	if err != nil {
		return fmt.Errorf("daemon listen %q: %w", *target, err)
	}
	defer listener.Close()

	grpcServer := grpc.NewServer()
	svc := &deephDaemonServiceServer{grpcServer: grpcServer}
	grpcServer.RegisterService(&deephDaemonServiceDesc, svc)

	fmt.Printf("deephd listening on %s\n", listener.Addr().String())
	if err := grpcServer.Serve(listener); err != nil {
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	}
	return nil
}

func cmdDaemonStart(args []string) error {
	fs := flag.NewFlagSet("daemon start", flag.ContinueOnError)
	target := fs.String("target", deephDaemonDefaultTarget(), "daemon target (host:port)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("daemon start does not accept positional arguments")
	}
	targetVal := strings.TrimSpace(*target)
	logPath, alreadyRunning, err := startDaemonBackground(targetVal)
	if err != nil {
		return err
	}
	if alreadyRunning {
		fmt.Printf("deephd already running at %s\n", targetVal)
		return nil
	}
	fmt.Printf("deephd started at %s\n", targetVal)
	if strings.TrimSpace(logPath) != "" {
		fmt.Printf("log: %s\n", logPath)
	}
	return nil
}

func startDaemonBackground(target string) (logPath string, alreadyRunning bool, err error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", false, errors.New("--target cannot be empty")
	}
	if err := deephDaemonPing(target, 1200*time.Millisecond); err == nil {
		return "", true, nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	logPath = filepath.Join(os.TempDir(), "deephd.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", false, err
	}
	defer logFile.Close()

	cmd := exec.Command(exePath, "daemon", "serve", "--target", target)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return "", false, fmt.Errorf("start deephd: %w", err)
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := deephDaemonPing(target, 1200*time.Millisecond); err == nil {
			return logPath, false, nil
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	return logPath, false, fmt.Errorf("deephd did not become reachable at %s (check %s)", target, logPath)
}

func cmdDaemonStatus(args []string) error {
	fs := flag.NewFlagSet("daemon status", flag.ContinueOnError)
	target := fs.String("target", deephDaemonDefaultTarget(), "daemon target (host:port)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("daemon status does not accept positional arguments")
	}
	targetVal := strings.TrimSpace(*target)
	if targetVal == "" {
		return errors.New("--target cannot be empty")
	}
	if err := deephDaemonPing(targetVal, 1200*time.Millisecond); err != nil {
		fmt.Printf("deephd is not reachable at %s\n", targetVal)
		return err
	}
	fmt.Printf("deephd is running at %s\n", targetVal)
	return nil
}

func cmdDaemonStop(args []string) error {
	fs := flag.NewFlagSet("daemon stop", flag.ContinueOnError)
	target := fs.String("target", deephDaemonDefaultTarget(), "daemon target (host:port)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("daemon stop does not accept positional arguments")
	}
	targetVal := strings.TrimSpace(*target)
	if targetVal == "" {
		return errors.New("--target cannot be empty")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var resp map[string]any
	if err := deephDaemonInvoke(ctx, targetVal, daemonMethodStop, map[string]any{}, &resp); err != nil {
		return err
	}
	fmt.Printf("deephd stop requested at %s\n", targetVal)
	return nil
}

func deephDaemonDefaultTarget() string {
	if raw := strings.TrimSpace(os.Getenv(daemonTargetEnvVar)); raw != "" {
		return raw
	}
	return defaultDaemonTarget
}

func deephDaemonPing(target string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var resp map[string]any
	return deephDaemonInvoke(ctx, target, daemonMethodPing, map[string]any{"ping": true}, &resp)
}

func deephDaemonTrace(target string, req daemonTraceRequest) (daemonTraceResponse, error) {
	ctx := context.Background()
	resp := daemonTraceResponse{}
	if err := deephDaemonInvoke(ctx, target, daemonMethodTrace, req, &resp); err != nil {
		return daemonTraceResponse{}, err
	}
	return resp, nil
}

func deephDaemonRun(target string, req daemonRunRequest) (daemonRunResponse, error) {
	ctx := context.Background()
	resp := daemonRunResponse{}
	if err := deephDaemonInvoke(ctx, target, daemonMethodRun, req, &resp); err != nil {
		return daemonRunResponse{}, err
	}
	return resp, nil
}

func cmdTraceViaDaemon(target string, req daemonTraceRequest, jsonOut bool) error {
	resp, err := deephDaemonTrace(target, req)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(resp.Workspace, "trace", resp.ResolvedSpec)
	if jsonOut {
		if resp.Multiverse {
			payload := struct {
				Workspace        string                      `json:"workspace"`
				Scheduler        string                      `json:"scheduler"`
				Source           string                      `json:"source"`
				UniverseHandoffs []multiverseUniverseHandoff `json:"universe_handoffs,omitempty"`
				Branches         []multiverseTraceBranch     `json:"branches"`
			}{
				Workspace:        resp.Workspace,
				Scheduler:        resp.Scheduler,
				Source:           resp.Source,
				UniverseHandoffs: append([]multiverseUniverseHandoff(nil), resp.UniverseHandoffs...),
				Branches:         append([]multiverseTraceBranch(nil), resp.Branches...),
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}
		payload := struct {
			Workspace string                `json:"workspace"`
			Scheduler string                `json:"scheduler"`
			Plan      runtime.ExecutionPlan `json:"plan"`
		}{
			Workspace: resp.Workspace,
			Scheduler: resp.Scheduler,
			Plan:      resp.Plan,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	if resp.Multiverse {
		mvPlan := &multiverseOrchestrationPlan{
			Scheduler: resp.Scheduler,
			Handoffs:  append([]multiverseUniverseHandoff(nil), resp.UniverseHandoffs...),
		}
		printMultiverseTraceText(resp.Workspace, req.AgentSpecArg, mvPlan, resp.Branches)
		return nil
	}
	printTracePlanText(resp.Workspace, resp.Plan)
	return nil
}

func cmdRunViaDaemon(target string, req daemonRunRequest, showTrace bool) error {
	resp, err := deephDaemonRun(target, req)
	if err != nil {
		return err
	}
	recordCoachCommandTransition(resp.Workspace, "run", resp.ResolvedSpec)
	if resp.Multiverse {
		mvPlan := &multiverseOrchestrationPlan{
			Scheduler: resp.Scheduler,
			Handoffs:  append([]multiverseUniverseHandoff(nil), resp.UniverseHandoffs...),
		}
		printMultiverseRunText(resp.Workspace, req.AgentSpecArg, mvPlan, resp.Branches)
		if strings.TrimSpace(resp.Judge.Spec) != "" {
			printMultiverseJudgeText(resp.Judge)
		}
		if showTrace {
			fmt.Printf("Trace summary: multiverse_branches=%d source=%q scheduler=%s\n", len(resp.Branches), req.AgentSpecArg, resp.Scheduler)
		}
		saveStudioRecent(resp.Workspace, req.AgentSpecArg, "")
		return nil
	}
	printRunReportText(resp.Plan, resp.Report)
	if showTrace {
		fmt.Printf("Trace summary: tasks=%d stages=%d handoffs=%d parallel=%v\n", len(resp.Plan.Tasks), len(resp.Plan.Stages), len(resp.Plan.Handoffs), resp.Plan.Parallel)
	}
	saveStudioRecent(resp.Workspace, req.AgentSpecArg, "")
	return nil
}

func deephDaemonInvoke(ctx context.Context, target, method string, req any, out any) error {
	target = strings.TrimSpace(target)
	if target == "" {
		target = deephDaemonDefaultTarget()
	}
	in, err := deephStructFromAny(req)
	if err != nil {
		return err
	}
	dialCtx := ctx
	cancelDial := func() {}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, defaultDaemonDialTO)
		cancelDial = cancel
	}
	defer cancelDial()
	conn, err := deephDaemonGetConn(dialCtx, target)
	if err != nil {
		return fmt.Errorf("connect deephd %s: %w", target, err)
	}

	var resp structpb.Struct
	if err := conn.Invoke(ctx, method, in, &resp); err != nil {
		if isDaemonUnavailableError(err) {
			deephDaemonDropConn(target, conn)
		}
		return err
	}
	if out == nil {
		return nil
	}
	return deephStructToAny(&resp, out)
}

func deephDaemonGetConn(ctx context.Context, target string) (*grpc.ClientConn, error) {
	miss := true
	deephDaemonConnCache.mu.Lock()
	if conn := deephDaemonConnCache.conns[target]; conn != nil {
		if conn.GetState() != connectivity.Shutdown {
			miss = false
			deephDaemonConnStats.hits.Add(1)
			deephDaemonConnCache.mu.Unlock()
			return conn, nil
		}
		delete(deephDaemonConnCache.conns, target)
	}
	deephDaemonConnCache.mu.Unlock()
	if miss {
		deephDaemonConnStats.misses.Add(1)
	}

	conn, err := grpc.DialContext(ctx, target, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	deephDaemonConnStats.dials.Add(1)

	deephDaemonConnCache.mu.Lock()
	if existing := deephDaemonConnCache.conns[target]; existing != nil && existing.GetState() != connectivity.Shutdown {
		deephDaemonConnStats.hits.Add(1)
		deephDaemonConnCache.mu.Unlock()
		_ = conn.Close()
		return existing, nil
	}
	deephDaemonConnCache.conns[target] = conn
	deephDaemonConnCache.mu.Unlock()
	return conn, nil
}

func deephDaemonDropConn(target string, conn *grpc.ClientConn) {
	if conn == nil {
		return
	}
	dropped := false
	deephDaemonConnCache.mu.Lock()
	if existing := deephDaemonConnCache.conns[target]; existing == conn {
		delete(deephDaemonConnCache.conns, target)
		dropped = true
	}
	deephDaemonConnCache.mu.Unlock()
	if dropped {
		deephDaemonConnStats.drops.Add(1)
	}
	_ = conn.Close()
}

func daemonConnStatsSnapshotValue() daemonConnStatsSnapshot {
	return daemonConnStatsSnapshot{
		Hits:   deephDaemonConnStats.hits.Load(),
		Misses: deephDaemonConnStats.misses.Load(),
		Dials:  deephDaemonConnStats.dials.Load(),
		Drops:  deephDaemonConnStats.drops.Load(),
	}
}

func daemonConnDebugEnabled() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(daemonDebugEnvVar)))
	switch raw {
	case "1", "true", "yes", "on", "debug":
		return true
	default:
		return false
	}
}

func maybePrintDaemonConnStats(target string) {
	if !daemonConnDebugEnabled() {
		return
	}
	s := daemonConnStatsSnapshotValue()
	fmt.Fprintf(os.Stderr, "daemon_conn_pool target=%s hits=%d misses=%d dials=%d drops=%d\n", strings.TrimSpace(target), s.Hits, s.Misses, s.Dials, s.Drops)
}

func deephStructFromAny(v any) (*structpb.Struct, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if len(bytesTrimSpace(b)) > 0 {
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, err
		}
	}
	return structpb.NewStruct(out)
}

func deephStructToAny(in *structpb.Struct, out any) error {
	if out == nil {
		return nil
	}
	payload := map[string]any{}
	if in != nil {
		payload = in.AsMap()
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if len(bytesTrimSpace(b)) == 0 {
		return nil
	}
	return json.Unmarshal(b, out)
}

func isDaemonUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable, codes.DeadlineExceeded:
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connect deephd"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "no such host"),
		strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "transport is closing"),
		strings.Contains(msg, "connection error"):
		return true
	}
	return false
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

type deephDaemonService interface {
	Ping(context.Context, *structpb.Struct) (*structpb.Struct, error)
	Run(context.Context, *structpb.Struct) (*structpb.Struct, error)
	Trace(context.Context, *structpb.Struct) (*structpb.Struct, error)
	Shutdown(context.Context, *structpb.Struct) (*structpb.Struct, error)
}

type deephDaemonServiceServer struct {
	grpcServer *grpc.Server
}

func (s *deephDaemonServiceServer) Ping(_ context.Context, _ *structpb.Struct) (*structpb.Struct, error) {
	return deephStructFromAny(map[string]any{
		"ok":          true,
		"service":     "deephd",
		"server_time": time.Now().Format(time.RFC3339),
	})
}

func (s *deephDaemonServiceServer) Shutdown(_ context.Context, _ *structpb.Struct) (*structpb.Struct, error) {
	go func() {
		if s.grpcServer != nil {
			s.grpcServer.GracefulStop()
		}
	}()
	return deephStructFromAny(map[string]any{"ok": true})
}

func (s *deephDaemonServiceServer) Trace(ctx context.Context, in *structpb.Struct) (*structpb.Struct, error) {
	req := daemonTraceRequest{
		Workspace:  ".",
		Multiverse: 1,
	}
	if err := deephStructToAny(in, &req); err != nil {
		return nil, fmt.Errorf("decode trace request: %w", err)
	}
	if strings.TrimSpace(req.AgentSpecArg) == "" {
		return nil, errors.New("trace request requires agent_spec_arg")
	}
	if req.Multiverse < 0 {
		return nil, errors.New("trace request multiverse must be >= 0")
	}
	resp, err := executeDaemonTrace(ctx, req)
	if err != nil {
		return nil, err
	}
	return deephStructFromAny(resp)
}

func (s *deephDaemonServiceServer) Run(ctx context.Context, in *structpb.Struct) (*structpb.Struct, error) {
	req := daemonRunRequest{
		Workspace:           ".",
		Multiverse:          1,
		JudgeMaxOutputChars: 700,
	}
	if err := deephStructToAny(in, &req); err != nil {
		return nil, fmt.Errorf("decode run request: %w", err)
	}
	if strings.TrimSpace(req.AgentSpecArg) == "" {
		return nil, errors.New("run request requires agent_spec_arg")
	}
	if req.Multiverse < 0 {
		return nil, errors.New("run request multiverse must be >= 0")
	}
	resp, err := executeDaemonRun(ctx, req)
	if err != nil {
		return nil, err
	}
	return deephStructFromAny(resp)
}

func executeDaemonTrace(ctx context.Context, req daemonTraceRequest) (daemonTraceResponse, error) {
	p, abs, verr, err := loadAndValidate(req.Workspace)
	if err != nil {
		return daemonTraceResponse{}, err
	}
	if verr != nil && verr.HasErrors() {
		return daemonTraceResponse{}, verr
	}
	eng, err := runtime.New(abs, p)
	if err != nil {
		return daemonTraceResponse{}, err
	}
	resolvedSpec, crew, err := resolveAgentSpecOrCrew(abs, req.AgentSpecArg)
	if err != nil {
		return daemonTraceResponse{}, err
	}
	universes, err := buildMultiverseUniverses(abs, req.AgentSpecArg, resolvedSpec, req.Input, req.Multiverse, crew)
	if err != nil {
		return daemonTraceResponse{}, err
	}

	resp := daemonTraceResponse{
		Workspace:    abs,
		Source:       req.AgentSpecArg,
		ResolvedSpec: resolvedSpec,
		Scheduler:    "dag_channels",
	}
	if len(universes) > 1 {
		branches, mvPlan, err := traceMultiverse(ctx, abs, p, universes)
		if err != nil {
			return daemonTraceResponse{}, err
		}
		resp.Multiverse = true
		resp.Scheduler = mvPlan.Scheduler
		resp.UniverseHandoffs = append([]multiverseUniverseHandoff(nil), mvPlan.Handoffs...)
		resp.Branches = append([]multiverseTraceBranch(nil), branches...)
		return resp, nil
	}

	plan, _, err := eng.PlanSpec(ctx, resolvedSpec, req.Input)
	if err != nil {
		return daemonTraceResponse{}, err
	}
	resp.Plan = plan
	return resp, nil
}

func executeDaemonRun(ctx context.Context, req daemonRunRequest) (daemonRunResponse, error) {
	p, abs, verr, err := loadAndValidate(req.Workspace)
	if err != nil {
		return daemonRunResponse{}, err
	}
	if verr != nil && verr.HasErrors() {
		return daemonRunResponse{}, verr
	}
	eng, err := runtime.New(abs, p)
	if err != nil {
		return daemonRunResponse{}, err
	}
	resolvedSpec, crew, err := resolveAgentSpecOrCrew(abs, req.AgentSpecArg)
	if err != nil {
		return daemonRunResponse{}, err
	}
	universes, err := buildMultiverseUniverses(abs, req.AgentSpecArg, resolvedSpec, req.Input, req.Multiverse, crew)
	if err != nil {
		return daemonRunResponse{}, err
	}

	resp := daemonRunResponse{
		Workspace:    abs,
		Source:       req.AgentSpecArg,
		ResolvedSpec: resolvedSpec,
		Scheduler:    "dag_channels",
	}
	if len(universes) > 1 {
		branches, mvPlan, err := runMultiverse(ctx, abs, p, universes)
		if err != nil {
			return daemonRunResponse{}, err
		}
		resp.Multiverse = true
		resp.Scheduler = mvPlan.Scheduler
		resp.UniverseHandoffs = append([]multiverseUniverseHandoff(nil), mvPlan.Handoffs...)
		resp.Branches = append([]multiverseRunBranch(nil), branches...)
		if strings.TrimSpace(req.JudgeAgent) != "" {
			judgeSpec, _, jerr := resolveAgentSpecOrCrew(abs, strings.TrimSpace(req.JudgeAgent))
			if jerr != nil {
				resp.Judge = multiverseJudgeRun{Spec: strings.TrimSpace(req.JudgeAgent), Error: jerr.Error()}
			} else {
				resp.Judge = runMultiverseJudge(ctx, abs, p, judgeSpec, req.AgentSpecArg, req.Input, branches, req.JudgeMaxOutputChars)
			}
		}
		return resp, nil
	}

	plan, _, err := eng.PlanSpec(ctx, resolvedSpec, req.Input)
	if err != nil {
		return daemonRunResponse{}, err
	}
	report, err := eng.RunSpec(ctx, resolvedSpec, req.Input)
	if err != nil {
		return daemonRunResponse{}, err
	}
	resp.Plan = plan
	resp.Report = report
	return resp, nil
}

var deephDaemonServiceDesc = grpc.ServiceDesc{
	ServiceName: daemonServiceName,
	HandlerType: (*deephDaemonService)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler:    deephDaemonPingHandler,
		},
		{
			MethodName: "Run",
			Handler:    deephDaemonRunHandler,
		},
		{
			MethodName: "Trace",
			Handler:    deephDaemonTraceHandler,
		},
		{
			MethodName: "Shutdown",
			Handler:    deephDaemonShutdownHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/deeph/daemon/v1/daemon.proto",
}

func deephDaemonPingHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(deephDaemonService).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: daemonMethodPing,
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(deephDaemonService).Ping(ctx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func deephDaemonRunHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(deephDaemonService).Run(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: daemonMethodRun,
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(deephDaemonService).Run(ctx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func deephDaemonTraceHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(deephDaemonService).Trace(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: daemonMethodTrace,
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(deephDaemonService).Trace(ctx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func deephDaemonShutdownHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(deephDaemonService).Shutdown(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: daemonMethodStop,
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(deephDaemonService).Shutdown(ctx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}
