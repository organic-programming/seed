package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/go-holons/pkg/transport"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type config struct {
	listenURI      string
	backend        string
	protoDir       string
	manifest       string
	describeStatic string
	reflect        bool
}

type bridgeServer struct {
	backendBase string
	methods     map[string]protoreflect.MethodDescriptor
	client      *http.Client
}

type backendProcess struct {
	cmd          *exec.Cmd
	uri          string
	stdoutTail   string
	stderrTail   string
	stdoutDoneCh chan struct{}
	stderrDoneCh chan struct{}
	logs         *deferredLogger
}

type lineCollector struct {
	mu    sync.Mutex
	lines []string
}

type deferredLogger struct {
	mu     sync.Mutex
	ready  bool
	buffer []deferredLogLine
}

type deferredLogLine struct {
	label string
	line  string
}

type parentPIDProvider func() int

func (c *lineCollector) append(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, line)
	if len(c.lines) > 32 {
		c.lines = c.lines[len(c.lines)-32:]
	}
}

func (c *lineCollector) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.lines, "\n")
}

func (l *deferredLogger) print(label string, line string) {
	l.mu.Lock()
	if l.ready {
		l.mu.Unlock()
		log.Printf("%s: %s", label, line)
		return
	}
	l.buffer = append(l.buffer, deferredLogLine{label: label, line: line})
	l.mu.Unlock()
}

func (l *deferredLogger) flush() {
	l.mu.Lock()
	buffered := append([]deferredLogLine(nil), l.buffer...)
	l.buffer = nil
	l.ready = true
	l.mu.Unlock()

	for _, entry := range buffered {
		log.Printf("%s: %s", entry.label, entry.line)
	}
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		log.Fatalf("grpc-bridge: %v", err)
	}

	methods, err := loadMethodRegistry(cfg.protoDir, cfg.describeStatic)
	if err != nil {
		log.Fatalf("grpc-bridge: load methods: %v", err)
	}

	backend, err := startBackend(cfg.backend)
	if err != nil {
		log.Fatalf("grpc-bridge: start backend: %v", err)
	}
	defer func() {
		if stopErr := backend.stop(); stopErr != nil {
			log.Printf("grpc-bridge: stop backend: %v", stopErr)
		}
	}()

	server := grpc.NewServer(grpc.UnknownServiceHandler((&bridgeServer{
		backendBase: toHTTPBase(backend.uri),
		methods:     methods,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}).handle))
	defer server.Stop()

	if err := registerDescribe(server, cfg); err != nil {
		backend.logs.flush()
		log.Fatalf("grpc-bridge: register describe: %v", err)
	}
	reflectionEnabled := false
	if cfg.reflect {
		reflection.Register(server)
		reflectionEnabled = true
	}

	listener, err := transport.Listen(cfg.listenURI)
	if err != nil {
		backend.logs.flush()
		log.Fatalf("grpc-bridge: listen: %v", err)
	}
	defer listener.Close()

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- server.Serve(listener)
	}()

	mode := "Describe ON, reflection OFF"
	if reflectionEnabled {
		mode = "Describe ON, reflection ON"
	}
	log.Printf("gRPC bridge listening on %s (%s, backend %s)", advertisedURI(cfg.listenURI, listener.Addr()), mode, backend.uri)
	backend.logs.flush()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	parentWatchStop := make(chan struct{})
	parentGoneCh := watchParentExit(os.Getppid(), 250*time.Millisecond, os.Getppid, parentWatchStop)
	defer close(parentWatchStop)

	select {
	case sig := <-sigCh:
		log.Printf("grpc-bridge: shutting down on %s", sig)
		stopGracefully(server)
	case <-parentGoneCh:
		log.Printf("grpc-bridge: parent exited, shutting down")
		stopGracefully(server)
	case err := <-serveErrCh:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			log.Fatalf("grpc-bridge: serve failed: %v", err)
		}
	}
}

func watchParentExit(parentPID int, interval time.Duration, currentParent parentPIDProvider, stop <-chan struct{}) <-chan struct{} {
	done := make(chan struct{})
	if parentPID <= 1 || interval <= 0 || currentParent == nil {
		close(done)
		return done
	}

	go func() {
		defer close(done)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if currentParent() <= 1 {
					return
				}
			}
		}
	}()

	return done
}

func parseArgs(args []string) (config, error) {
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}

	fs := flag.NewFlagSet("grpc-bridge", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := config{}
	fs.StringVar(&cfg.listenURI, "listen", transport.DefaultURI, "transport URI")
	fs.StringVar(&cfg.backend, "backend", "", "backend executable path")
	fs.StringVar(&cfg.protoDir, "proto-dir", "", "proto directory for dynamic forwarding")
	fs.StringVar(&cfg.manifest, "manifest", "", "manifest path for Describe (for example api/v1/holon.proto)")
	fs.StringVar(&cfg.manifest, "holon-yaml", "", "deprecated alias for --manifest")
	fs.StringVar(&cfg.describeStatic, "describe-static", "", "path to build-generated DescribeResponse JSON")
	fs.BoolVar(&cfg.reflect, "reflect", false, "enable gRPC reflection for debugging")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if strings.TrimSpace(cfg.backend) == "" {
		return cfg, fmt.Errorf("--backend is required")
	}
	if strings.TrimSpace(cfg.protoDir) == "" && strings.TrimSpace(cfg.describeStatic) == "" {
		return cfg, fmt.Errorf("--proto-dir is required unless --describe-static is set")
	}
	if strings.TrimSpace(cfg.describeStatic) == "" && strings.TrimSpace(cfg.manifest) == "" {
		return cfg, fmt.Errorf("--manifest is required")
	}
	return cfg, nil
}

func registerDescribe(server *grpc.Server, cfg config) error {
	if strings.TrimSpace(cfg.describeStatic) != "" {
		response, err := loadStaticDescribeResponse(cfg.describeStatic)
		if err != nil {
			return err
		}
		describe.UseStaticResponse(response)
		return describe.Register(server)
	}

	response, err := describe.BuildResponse(cfg.protoDir, cfg.manifest)
	if err != nil {
		return err
	}
	describe.UseStaticResponse(response)
	return describe.Register(server)
}

func loadStaticDescribeResponse(path string) (*holonsv1.DescribeResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read static describe %s: %w", path, err)
	}

	response := &holonsv1.DescribeResponse{}
	if err := protojson.Unmarshal(data, response); err != nil {
		return nil, fmt.Errorf("decode static describe %s: %w", path, err)
	}
	return response, nil
}

func loadMethodRegistry(protoDir string, describeStatic string) (map[string]protoreflect.MethodDescriptor, error) {
	if strings.TrimSpace(protoDir) == "" {
		response, err := loadStaticDescribeResponse(describeStatic)
		if err != nil {
			return nil, err
		}
		return loadMethodRegistryFromDescribeResponse(response)
	}

	absDir, err := filepath.Abs(protoDir)
	if err != nil {
		return nil, fmt.Errorf("resolve proto dir %s: %w", protoDir, err)
	}

	protoFiles, err := collectProtoFiles(absDir)
	if err != nil {
		return nil, err
	}
	if len(protoFiles) == 0 {
		return nil, fmt.Errorf("no .proto files found in %s", absDir)
	}

	parser := protoparse.Parser{
		ImportPaths:               []string{absDir},
		InferImportPaths:          true,
		LookupImport:              desc.LoadFileDescriptor,
		AllowExperimentalEditions: true,
	}
	files, err := parser.ParseFiles(protoFiles...)
	if err != nil {
		return nil, fmt.Errorf("parse proto files: %w", err)
	}

	descriptorSet := &descriptorpb.FileDescriptorSet{}
	appendFileDescriptors(files, descriptorSet, map[string]bool{})

	registry, err := protodesc.NewFiles(descriptorSet)
	if err != nil {
		return nil, fmt.Errorf("build proto registry: %w", err)
	}

	methods := map[string]protoreflect.MethodDescriptor{}
	registry.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		services := file.Services()
		for i := 0; i < services.Len(); i++ {
			service := services.Get(i)
			if string(service.FullName()) == "holons.v1.HolonMeta" {
				continue
			}
			serviceMethods := service.Methods()
			for j := 0; j < serviceMethods.Len(); j++ {
				method := serviceMethods.Get(j)
				fullMethod := "/" + string(service.FullName()) + "/" + string(method.Name())
				methods[fullMethod] = method
			}
		}
		return true
	})
	return methods, nil
}

func appendFileDescriptors(files []*desc.FileDescriptor, set *descriptorpb.FileDescriptorSet, seen map[string]bool) {
	for _, file := range files {
		appendFileDescriptor(file, set, seen)
	}
}

func appendFileDescriptor(file *desc.FileDescriptor, set *descriptorpb.FileDescriptorSet, seen map[string]bool) {
	if file == nil {
		return
	}

	name := file.GetName()
	if seen[name] {
		return
	}
	seen[name] = true

	for _, dep := range file.GetDependencies() {
		appendFileDescriptor(dep, set, seen)
	}

	set.File = append(set.File, file.AsFileDescriptorProto())
}

func collectProtoFiles(root string) ([]string, error) {
	files := []string{}
	canonicalSeen := map[string]struct{}{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) != ".proto" {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		canonical := canonicalProtoScanPath(rel)
		if _, ok := canonicalSeen[canonical]; ok {
			return nil
		}
		canonicalSeen[canonical] = struct{}{}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk proto dir %s: %w", root, err)
	}
	sort.Strings(files)
	return files, nil
}

func canonicalProtoScanPath(rel string) string {
	switch filepath.ToSlash(rel) {
	case "xds/xds/data/orca/v3/orca_load_report.proto":
		return "xds/data/orca/v3/orca_load_report.proto"
	default:
		return filepath.ToSlash(rel)
	}
}

func startBackend(binary string) (*backendProcess, error) {
	cmd := exec.Command(binary, "serve", "--listen", "tcp://127.0.0.1:0")
	cmd.Stdin = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", binary, err)
	}

	stdoutCollector := &lineCollector{}
	stderrCollector := &lineCollector{}
	logCollector := &deferredLogger{}
	proc := &backendProcess{
		cmd:          cmd,
		stdoutDoneCh: make(chan struct{}),
		stderrDoneCh: make(chan struct{}),
		logs:         logCollector,
	}

	uriCh := make(chan string, 1)
	readPipe("backend stdout", stdout, stdoutCollector, uriCh, proc.stdoutDoneCh, logCollector)
	readPipe("backend stderr", stderr, stderrCollector, nil, proc.stderrDoneCh, logCollector)

	select {
	case uri := <-uriCh:
		proc.uri = uri
	case <-time.After(3 * time.Second):
		proc.logs.flush()
		_ = proc.stop()
		return nil, fmt.Errorf("timed out waiting for backend startup: %s", startupDiagnostics(stdoutCollector.text(), stderrCollector.text()))
	}

	proc.stdoutTail = stdoutCollector.text()
	proc.stderrTail = stderrCollector.text()
	return proc, nil
}

func startupDiagnostics(stdout string, stderr string) string {
	parts := []string{}
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	if stdout != "" {
		parts = append(parts, fmt.Sprintf("stdout=%q", stdout))
	}
	if stderr != "" {
		parts = append(parts, fmt.Sprintf("stderr=%q", stderr))
	}
	if len(parts) == 0 {
		return "no backend logs"
	}
	return strings.Join(parts, " ")
}

func readPipe(label string, reader io.ReadCloser, collector *lineCollector, uriCh chan<- string, done chan<- struct{}, logger *deferredLogger) {
	go func() {
		defer close(done)
		defer reader.Close()

		scanner := bufio.NewScanner(reader)
		buffer := make([]byte, 0, 64*1024)
		scanner.Buffer(buffer, 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			collector.append(line)
			if logger != nil {
				logger.print(label, line)
			}
			if uriCh != nil {
				if uri := firstTCPURI(line); uri != "" {
					select {
					case uriCh <- uri:
					default:
					}
				}
			}
		}
	}()
}

func firstTCPURI(text string) string {
	fields := strings.Fields(text)
	for _, field := range fields {
		candidate := strings.Trim(field, "\"'()[]{}.,")
		if strings.HasPrefix(candidate, "tcp://") {
			return candidate
		}
	}
	return ""
}

func (p *backendProcess) stop() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	_ = p.cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case err := <-done:
		return ignoreExitAfterSignal(err)
	case <-time.After(2 * time.Second):
		_ = p.cmd.Process.Kill()
		err := <-done
		return ignoreExitAfterSignal(err)
	}
}

func ignoreExitAfterSignal(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil
	}
	return err
}

func toHTTPBase(uri string) string {
	trimmed := strings.TrimSpace(uri)
	if strings.HasPrefix(trimmed, "tcp://") {
		return "http://" + strings.TrimPrefix(trimmed, "tcp://")
	}
	return trimmed
}

func (b *bridgeServer) handle(_ any, stream grpc.ServerStream) error {
	fullMethod, ok := grpc.MethodFromServerStream(stream)
	if !ok {
		return status.Error(codes.Internal, "missing method name")
	}

	method, ok := b.methods[fullMethod]
	if !ok {
		return status.Errorf(codes.Unimplemented, "unknown method %s", fullMethod)
	}
	if method.IsStreamingClient() || method.IsStreamingServer() {
		return status.Errorf(codes.Unimplemented, "streaming method %s is not supported by c-holons grpc bridge", fullMethod)
	}

	request := dynamicpb.NewMessage(method.Input())
	if err := stream.RecvMsg(request); err != nil {
		return status.Errorf(codes.InvalidArgument, "recv request for %s: %v", fullMethod, err)
	}

	response, err := b.invoke(fullMethod, request, method.Output())
	if err != nil {
		return err
	}
	if err := stream.SendMsg(response); err != nil {
		return status.Errorf(codes.Internal, "send response for %s: %v", fullMethod, err)
	}
	return nil
}

func (b *bridgeServer) invoke(fullMethod string, request *dynamicpb.Message, output protoreflect.MessageDescriptor) (*dynamicpb.Message, error) {
	body, err := protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal(request)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal request %s: %v", fullMethod, err)
	}

	httpRequest, err := http.NewRequestWithContext(context.Background(), http.MethodPost, b.backendBase+fullMethod, strings.NewReader(string(body)))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build backend request %s: %v", fullMethod, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := b.client.Do(httpRequest)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "backend request %s failed: %v", fullMethod, err)
	}
	defer httpResponse.Body.Close()

	data, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read backend response %s: %v", fullMethod, err)
	}

	if httpResponse.StatusCode == http.StatusNotFound {
		return nil, status.Errorf(codes.Unimplemented, "backend does not implement %s", fullMethod)
	}
	if httpResponse.StatusCode >= 400 {
		return nil, status.Errorf(codes.Internal, "backend returned %s for %s", httpResponse.Status, fullMethod)
	}

	response := dynamicpb.NewMessage(output)
	if len(strings.TrimSpace(string(data))) == 0 {
		return response, nil
	}
	if err := (protojson.UnmarshalOptions{
		DiscardUnknown: false,
	}).Unmarshal(data, response); err != nil {
		return nil, status.Errorf(codes.Internal, "decode backend response %s: %v", fullMethod, err)
	}
	return response, nil
}

func advertisedURI(listenURI string, addr net.Addr) string {
	if addr == nil {
		return listenURI
	}

	raw := strings.TrimSpace(addr.String())
	if raw == "" {
		return listenURI
	}
	if strings.Contains(raw, "://") {
		return raw
	}

	switch addr.Network() {
	case "tcp", "tcp4", "tcp6":
		return "tcp://" + raw
	case "unix", "unixgram", "unixpacket":
		return "unix://" + raw
	default:
		switch transport.Scheme(listenURI) {
		case "tcp":
			return "tcp://" + raw
		case "unix":
			return "unix://" + raw
		default:
			return raw
		}
	}
}

func stopGracefully(server *grpc.Server) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.GracefulStop()
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		server.Stop()
	}
}
