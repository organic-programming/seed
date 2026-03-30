// Package grpcclient connects to a holon's gRPC server and forwards calls.
// It prefers HolonMeta/Describe for discovery and dynamic protobuf handling,
// and falls back to gRPC reflection when Describe is unavailable.
package grpcclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"nhooyr.io/websocket"
)

// CallResult holds the output of a gRPC call.
type CallResult struct {
	Service string `json:"service"`
	Method  string `json:"method"`
	Output  string `json:"output"`
}

// Dial connects to a gRPC server at the given address and calls a method.
// It resolves methods via Describe first and falls back to reflection when needed.
func Dial(address, methodName string, inputJSON string) (*CallResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", address, err)
	}
	defer conn.Close()

	return InvokeConn(ctx, conn, methodName, inputJSON)
}

// InvokeConn calls a unary RPC over an existing gRPC connection.
// Describe is used first; reflection is a fallback only.
func InvokeConn(ctx context.Context, conn *grpc.ClientConn, methodName string, inputJSON string) (*CallResult, error) {
	if conn == nil {
		return nil, errors.New("gRPC connection is required")
	}

	result, err := invokeViaDescribe(ctx, conn, methodName, inputJSON)
	if err == nil {
		return result, nil
	}
	if !shouldFallbackToReflection(err) {
		return nil, err
	}

	return invokeConnViaReflection(ctx, conn, methodName, inputJSON)
}

func invokeConnViaReflection(ctx context.Context, conn *grpc.ClientConn, methodName string, inputJSON string) (*CallResult, error) {
	targetMethod := canonicalMethodName(methodName)
	refClient := grpc_reflection_v1alpha.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("reflection not available: %w", err)
	}

	if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	}); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	listResp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("list services response: %w", err)
	}

	listResult := listResp.GetListServicesResponse()
	if listResult == nil {
		return nil, fmt.Errorf("no services found")
	}

	for _, svc := range listResult.Service {
		if svc.Name == "grpc.reflection.v1alpha.ServerReflection" ||
			svc.Name == "grpc.reflection.v1.ServerReflection" {
			continue
		}

		desc, err := resolveService(stream, svc.Name)
		if err != nil {
			continue
		}

		methods := desc.Methods()
		for i := 0; i < methods.Len(); i++ {
			method := methods.Get(i)
			if string(method.Name()) == targetMethod {
				return callMethod(ctx, conn, desc, method, inputJSON)
			}
		}
	}

	var available []string
	for _, svc := range listResult.Service {
		if svc.Name == "grpc.reflection.v1alpha.ServerReflection" ||
			svc.Name == "grpc.reflection.v1.ServerReflection" {
			continue
		}
		desc, err := resolveService(stream, svc.Name)
		if err != nil {
			continue
		}
		methods := desc.Methods()
		for i := 0; i < methods.Len(); i++ {
			available = append(available, fmt.Sprintf("%s/%s", svc.Name, methods.Get(i).Name()))
		}
	}

	return nil, fmt.Errorf("method %q not found. Available: %v", methodName, available)
}

// ListMethods returns all available service methods at the given address.
func ListMethods(address string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", address, err)
	}
	defer conn.Close()

	if methods, err := listMethodsViaDescribe(ctx, conn); err == nil {
		return methods, nil
	} else if !shouldFallbackToReflection(err) {
		return nil, err
	}

	return listMethodsViaReflection(ctx, conn)
}

func listMethodsViaReflection(ctx context.Context, conn *grpc.ClientConn) ([]string, error) {
	refClient := grpc_reflection_v1alpha.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("reflection not available: %w", err)
	}

	if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	}); err != nil {
		return nil, err
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	var methods []string
	for _, svc := range resp.GetListServicesResponse().Service {
		if svc.Name == "grpc.reflection.v1alpha.ServerReflection" ||
			svc.Name == "grpc.reflection.v1.ServerReflection" {
			continue
		}
		desc, err := resolveService(stream, svc.Name)
		if err != nil {
			continue
		}
		ms := desc.Methods()
		for i := 0; i < ms.Len(); i++ {
			methods = append(methods, fmt.Sprintf("%s/%s", svc.Name, ms.Get(i).Name()))
		}
	}
	return methods, nil
}

// --- Internal helpers ---

func resolveService(stream grpc_reflection_v1alpha.ServerReflection_ServerReflectionInfoClient, serviceName string) (protoreflect.ServiceDescriptor, error) {
	if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: serviceName,
		},
	}); err != nil {
		return nil, err
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	fdResp := resp.GetFileDescriptorResponse()
	if fdResp == nil {
		return nil, fmt.Errorf("no file descriptor for %s", serviceName)
	}

	// Parse descriptors returned for the symbol first.
	filesByName := make(map[string]*descriptorpb.FileDescriptorProto)
	var queue []string
	for _, b := range fdResp.FileDescriptorProto {
		fd := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(b, fd); err != nil {
			return nil, fmt.Errorf("unmarshal file descriptor: %w", err)
		}
		name := fd.GetName()
		if name == "" {
			continue
		}
		if _, exists := filesByName[name]; exists {
			continue
		}
		filesByName[name] = fd
		queue = append(queue, name)
	}

	// Some transports/reflection stacks do not include transitive imports in the
	// initial response. Resolve any missing dependencies on-demand.
	for i := 0; i < len(queue); i++ {
		fd := filesByName[queue[i]]
		for _, dep := range fd.GetDependency() {
			if _, exists := filesByName[dep]; exists {
				continue
			}

			depFiles, err := resolveFileByName(stream, dep)
			if err != nil && !strings.HasPrefix(dep, "protos/") {
				// Some servers register descriptors with a "protos/" prefix.
				depFiles, err = resolveFileByName(stream, "protos/"+dep)
			}
			if err != nil {
				return nil, err
			}

			aliasSourceName := ""
			for _, depFD := range depFiles {
				name := depFD.GetName()
				if name == "" || name == dep {
					continue
				}
				if strings.HasSuffix(name, dep) {
					aliasSourceName = name
					break
				}
			}

			resolvedDepName := false
			for _, depFD := range depFiles {
				name := depFD.GetName()
				if name == "" {
					continue
				}
				if name == dep {
					resolvedDepName = true
				}
				if aliasSourceName != "" && name == aliasSourceName {
					// Keep only the aliased name to avoid duplicate symbol conflicts.
					continue
				}
				if _, exists := filesByName[name]; exists {
					continue
				}
				filesByName[name] = depFD
				queue = append(queue, name)
			}

			// Some reflection servers return the right descriptor content under a
			// different filename (e.g. prefixed with "protos/"). Alias it to the
			// dependency name expected by the importing file.
			if !resolvedDepName && aliasSourceName != "" {
				for _, depFD := range depFiles {
					name := depFD.GetName()
					if name != aliasSourceName {
						continue
					}
					aliased := proto.Clone(depFD).(*descriptorpb.FileDescriptorProto)
					aliased.Name = proto.String(dep)
					filesByName[dep] = aliased
					queue = append(queue, dep)
					resolvedDepName = true
					break
				}
			}
		}
	}

	// Build a file descriptor set and resolve
	fds := &descriptorpb.FileDescriptorSet{
		File: make([]*descriptorpb.FileDescriptorProto, 0, len(queue)),
	}
	for _, name := range queue {
		fds.File = append(fds.File, filesByName[name])
	}
	fileDescs, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, fmt.Errorf("build file descriptors: %w", err)
	}

	svcDesc, err := fileDescs.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil, fmt.Errorf("find service %s: %w", serviceName, err)
	}

	sd, ok := svcDesc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%s is not a service", serviceName)
	}

	return sd, nil
}

func resolveFileByName(
	stream grpc_reflection_v1alpha.ServerReflection_ServerReflectionInfoClient,
	filename string,
) ([]*descriptorpb.FileDescriptorProto, error) {
	if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileByFilename{
			FileByFilename: filename,
		},
	}); err != nil {
		return nil, fmt.Errorf("request descriptor %s: %w", filename, err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("read descriptor %s: %w", filename, err)
	}

	fdResp := resp.GetFileDescriptorResponse()
	if fdResp == nil {
		return nil, fmt.Errorf("no file descriptor response for %s", filename)
	}

	files := make([]*descriptorpb.FileDescriptorProto, 0, len(fdResp.FileDescriptorProto))
	for _, b := range fdResp.FileDescriptorProto {
		fd := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(b, fd); err != nil {
			return nil, fmt.Errorf("unmarshal descriptor %s: %w", filename, err)
		}
		files = append(files, fd)
	}

	return files, nil
}

func callMethod(ctx context.Context, conn *grpc.ClientConn, svc protoreflect.ServiceDescriptor, method protoreflect.MethodDescriptor, inputJSON string) (*CallResult, error) {
	// Build the full method path: /package.ServiceName/MethodName
	fullMethod := fmt.Sprintf("/%s/%s", svc.FullName(), method.Name())

	// Create dynamic input message
	inputDesc := method.Input()
	inputMsg := dynamicpb.NewMessage(inputDesc)

	if inputJSON != "" && inputJSON != "{}" {
		if err := protojson.Unmarshal([]byte(inputJSON), inputMsg); err != nil {
			return nil, fmt.Errorf("parse input JSON: %w", err)
		}
	}

	// Create dynamic output message
	outputDesc := method.Output()
	outputMsg := dynamicpb.NewMessage(outputDesc)

	// Call the method
	if err := conn.Invoke(ctx, fullMethod, inputMsg, outputMsg); err != nil {
		return nil, fmt.Errorf("call %s: %w", fullMethod, err)
	}

	// Marshal output to JSON
	outputBytes, err := protojson.Marshal(outputMsg)
	if err != nil {
		return nil, fmt.Errorf("marshal output: %w", err)
	}

	return newCallResult(string(svc.FullName()), string(method.Name()), outputBytes), nil
}

// DialStdio launches a holon binary with `serve --listen stdio://` and
// communicates over stdin/stdout pipes. This is the purest form of
// inter-holon gRPC — zero networking, zero port allocation.
func DialStdio(binaryPath, methodName, inputJSON string) (*CallResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "serve", "--listen", "stdio://")

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", binaryPath, err)
	}
	defer func() {
		cmd.Process.Kill() //nolint:errcheck
		cmd.Wait()         //nolint:errcheck
	}()

	pConn := &pipeConn{
		reader: stdoutPipe,
		writer: stdinPipe,
	}

	// The pipe is a single connection — the dialer must return it exactly
	// once. Subsequent calls return an error (gRPC may try to reconnect).
	var dialOnce sync.Once
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		var conn net.Conn
		dialOnce.Do(func() { conn = pConn })
		if conn == nil {
			return nil, fmt.Errorf("stdio pipe already consumed")
		}
		return conn, nil
	}

	// DialContext+WithBlock forces an immediate HTTP/2 handshake over
	// the pipe, which is required for single-connection transports.
	//nolint:staticcheck // DialContext is deprecated but needed for pipes.
	conn, err := grpc.DialContext(ctx,
		"passthrough:///stdio",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
	)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("server startup timeout")
		}
		return nil, fmt.Errorf("create grpc client over stdio: %w", err)
	}
	defer conn.Close()

	return InvokeConn(ctx, conn, methodName, inputJSON)
}

// pipeConn wraps an io.ReadCloser + io.WriteCloser as a net.Conn.
type pipeConn struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (c *pipeConn) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c *pipeConn) Write(p []byte) (int, error) { return c.writer.Write(p) }
func (c *pipeConn) Close() error {
	writeErr := c.writer.Close()
	readErr := c.reader.Close()
	if writeErr != nil {
		return writeErr
	}
	return readErr
}
func (c *pipeConn) LocalAddr() net.Addr                { return pipeAddr{} }
func (c *pipeConn) RemoteAddr() net.Addr               { return pipeAddr{} }
func (c *pipeConn) SetDeadline(_ time.Time) error      { return nil }
func (c *pipeConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(_ time.Time) error { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "stdio://" }

// DialWebSocket connects to a holon's gRPC server via WebSocket and calls
// a method. URI should be "ws://host:port/path" or "wss://...".
func DialWebSocket(wsURI, methodName, inputJSON string) (*CallResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Establish WebSocket connection
	c, _, err := websocket.Dial(ctx, wsURI, &websocket.DialOptions{
		Subprotocols: []string{"grpc"},
	})
	if err != nil {
		return nil, fmt.Errorf("websocket dial %s: %w", wsURI, err)
	}

	// Wrap as net.Conn
	wsConn := websocket.NetConn(ctx, c, websocket.MessageBinary)

	// Single-use dialer
	dialed := false
	dialer := func(_ context.Context, _ string) (net.Conn, error) {
		if dialed {
			return nil, fmt.Errorf("ws connection already consumed")
		}
		dialed = true
		return wsConn, nil
	}

	//nolint:staticcheck // DialContext needed for single-connection transports.
	conn, err := grpc.DialContext(ctx,
		"passthrough:///ws",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
	)
	if err != nil {
		wsConn.Close()
		return nil, fmt.Errorf("grpc handshake over ws: %w", err)
	}
	defer conn.Close()

	return InvokeConn(ctx, conn, methodName, inputJSON)
}

func canonicalMethodName(method string) string {
	trimmed := strings.TrimSpace(method)
	if i := strings.LastIndex(trimmed, "/"); i >= 0 && i+1 < len(trimmed) {
		return trimmed[i+1:]
	}
	return trimmed
}
