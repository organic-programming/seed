package connect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/discover"
	holonsgrpcclient "github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/transport"
	"google.golang.org/grpc"
	testgrpc "google.golang.org/grpc/interop/grpc_testing"
)

type connectTestServer struct {
	testgrpc.UnimplementedTestServiceServer
}

func (s *connectTestServer) EmptyCall(context.Context, *testgrpc.Empty) (*testgrpc.Empty, error) {
	return &testgrpc.Empty{}, nil
}

func (s *connectTestServer) UnaryCall(_ context.Context, in *testgrpc.SimpleRequest) (*testgrpc.SimpleResponse, error) {
	payload := in.GetPayload()
	if payload == nil {
		payload = &testgrpc.Payload{
			Type: testgrpc.PayloadType_COMPRESSABLE,
			Body: []byte("connect-echo"),
		}
	}

	return &testgrpc.SimpleResponse{
		Payload: &testgrpc.Payload{
			Type: payload.GetType(),
			Body: append([]byte(nil), payload.GetBody()...),
		},
	}, nil
}

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		os.Exit(runServeProcess(os.Args[2:]))
	}

	os.Exit(m.Run())
}

func TestConnectDirectTCPRoundTrip(t *testing.T) {
	lis, err := transport.Listen("tcp://127.0.0.1:0")
	if err != nil {
		skipIfLocalBindDenied(t, err)
		t.Fatalf("listen tcp: %v", err)
	}
	t.Cleanup(func() { _ = lis.Close() })

	server := grpc.NewServer()
	testgrpc.RegisterTestServiceServer(server, &connectTestServer{})
	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- server.Serve(lis)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = <-serveErrCh
	})

	conn, err := Connect(lis.Addr().String())
	if err != nil {
		t.Fatalf("Connect direct target: %v", err)
	}
	t.Cleanup(func() { _ = Disconnect(conn) })

	requireUnaryEcho(t, conn, "direct-connect")
}

func TestConnectStartsSlugEphemerally(t *testing.T) {
	root, slug := writeHolonFixture(t, "Connect", "Ephemeral")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	conn, err := Connect(slug)
	if err != nil {
		t.Fatalf("Connect slug: %v", err)
	}

	handle := lookupHandle(t, conn)
	pid := handle.process.cmd.Process.Pid
	requireUnaryEcho(t, conn, "ephemeral-connect")

	if err := Disconnect(conn); err != nil {
		t.Fatalf("Disconnect slug: %v", err)
	}

	waitForProcessExit(t, pid)

	portFile := filepath.Join(root, ".op", "run", slug+".port")
	if _, err := os.Stat(portFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ephemeral connect wrote unexpected port file %q", portFile)
	}
}

func TestConnectStartsSlugViaStdioByDefault(t *testing.T) {
	root, slug := writeHolonFixture(t, "Connect", "Stdio")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	conn, err := Connect(slug)
	if err != nil {
		t.Fatalf("Connect slug: %v", err)
	}

	handle := lookupHandle(t, conn)
	if got, want := handle.process.cmd.Args[1:], []string{"serve", "--listen", "stdio://"}; !reflect.DeepEqual(got, want) {
		_ = Disconnect(conn)
		t.Fatalf("child args = %v, want %v", got, want)
	}

	requireUnaryEcho(t, conn, "stdio-default-connect")

	portFile := filepath.Join(root, ".op", "run", slug+".port")
	if _, err := os.Stat(portFile); !errors.Is(err, os.ErrNotExist) {
		_ = Disconnect(conn)
		t.Fatalf("stdio default should not write port file %q, got err=%v", portFile, err)
	}

	pid := handle.process.cmd.Process.Pid
	if err := Disconnect(conn); err != nil {
		t.Fatalf("Disconnect stdio slug: %v", err)
	}

	waitForProcessExit(t, pid)

	if _, err := os.Stat(portFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stdio default should leave no port file %q, got err=%v", portFile, err)
	}
}

func TestConnectStartsProtoSlugViaStdioByDefault(t *testing.T) {
	root, slug := writeProtoHolonFixture(t, "Proto", "Connect")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	conn, err := Connect(slug)
	if err != nil {
		t.Fatalf("Connect proto slug: %v", err)
	}

	handle := lookupHandle(t, conn)
	if got, want := handle.process.cmd.Args[1:], []string{"serve", "--listen", "stdio://"}; !reflect.DeepEqual(got, want) {
		_ = Disconnect(conn)
		t.Fatalf("child args = %v, want %v", got, want)
	}

	requireUnaryEcho(t, conn, "proto-stdio-connect")

	portFile := filepath.Join(root, ".op", "run", slug+".port")
	if _, err := os.Stat(portFile); !errors.Is(err, os.ErrNotExist) {
		_ = Disconnect(conn)
		t.Fatalf("proto stdio should not write port file %q, got err=%v", portFile, err)
	}

	pid := handle.process.cmd.Process.Pid
	if err := Disconnect(conn); err != nil {
		t.Fatalf("Disconnect proto stdio slug: %v", err)
	}

	waitForProcessExit(t, pid)
}

func TestConnectStartsRealProtoSourceHolonViaStdio(t *testing.T) {
	repoRoot := connectRepoRoot(t)
	t.Chdir(repoRoot)
	t.Setenv("OPPATH", filepath.Join(t.TempDir(), ".op-home"))
	t.Setenv("OPBIN", filepath.Join(t.TempDir(), ".op-bin"))
	_ = os.RemoveAll(filepath.Join(repoRoot, "examples", "hello-world", "gabriel-greeting-go", ".op", "build"))

	conn, err := ConnectWithOpts("gabriel-greeting-go", ConnectOptions{
		Timeout:   20 * time.Second,
		Transport: TransportStdio,
		Lifecycle: LifecycleEphemeral,
		Start:     true,
	})
	if err != nil {
		t.Fatalf("Connect proto source slug: %v", err)
	}

	handle := lookupHandle(t, conn)
	if got, want := handle.process.cmd.Dir, filepath.Join(repoRoot, "examples", "hello-world", "gabriel-greeting-go"); got != want {
		_ = Disconnect(conn)
		t.Fatalf("child dir = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(handle.process.cmd.Args, []string{"go", "run", "./cmd", "serve", "--listen", "stdio://"}) &&
		!reflect.DeepEqual(handle.process.cmd.Args[1:], []string{"serve", "--listen", "stdio://"}) {
		_ = Disconnect(conn)
		t.Fatalf("unexpected child args = %v", handle.process.cmd.Args)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := holonsv1.NewHolonMetaClient(conn).Describe(ctx, &holonsv1.DescribeRequest{})
	if err != nil {
		_ = Disconnect(conn)
		t.Fatalf("Describe: %v", err)
	}
	if got := strings.ToLower(strings.Trim(strings.ReplaceAll(response.GetManifest().GetIdentity().GetGivenName()+"-"+strings.TrimSuffix(response.GetManifest().GetIdentity().GetFamilyName(), "?"), " ", "-"), "-")); got != "gabriel-greeting-go" {
		_ = Disconnect(conn)
		t.Fatalf("slug = %q, want %q", got, "gabriel-greeting-go")
	}

	pid := handle.process.cmd.Process.Pid
	if err := Disconnect(conn); err != nil {
		t.Fatalf("Disconnect proto source slug: %v", err)
	}
	waitForProcessExit(t, pid)
}

func TestBuiltGabrielPackageBinaryDescribeWithoutProtoFiles(t *testing.T) {
	repoRoot := connectRepoRoot(t)
	exampleSrc := filepath.Join(repoRoot, "examples", "hello-world", "gabriel-greeting-go")
	exampleDir := filepath.Join(t.TempDir(), "gabriel-greeting-go")
	if err := copyDirTree(exampleSrc, exampleDir); err != nil {
		t.Fatalf("copy example tree: %v", err)
	}
	sharedProto, err := os.ReadFile(filepath.Join(exampleDir, ".op", "protos", "v1", "greeting.proto"))
	if err != nil {
		t.Fatalf("read copied shared proto: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(exampleDir, "v1"), 0o755); err != nil {
		t.Fatalf("mkdir copied v1 proto dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(exampleDir, "v1", "greeting.proto"), sharedProto, 0o644); err != nil {
		t.Fatalf("write copied shared proto: %v", err)
	}
	goModPath := filepath.Join(exampleDir, "go.mod")
	goModData, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read copied go.mod: %v", err)
	}
	updatedGoMod := strings.ReplaceAll(string(goModData), "../../../sdk/go-holons", filepath.Join(repoRoot, "sdk", "go-holons"))
	if err := os.WriteFile(goModPath, []byte(updatedGoMod), 0o644); err != nil {
		t.Fatalf("write copied go.mod: %v", err)
	}

	opBinary := filepath.Join(t.TempDir(), "op")
	buildOp := exec.Command("go", "build", "-o", opBinary, "./cmd/op")
	buildOp.Dir = filepath.Join(repoRoot, "holons", "grace-op")
	buildOp.Env = os.Environ()
	output, err := buildOp.CombinedOutput()
	if err != nil {
		t.Fatalf("build local op: %v\n%s", err, string(output))
	}

	build := exec.Command(opBinary, "build", exampleDir)
	build.Dir = repoRoot
	build.Env = append(os.Environ(),
		"OPPATH="+filepath.Join(t.TempDir(), ".op-home"),
		"OPBIN="+filepath.Join(t.TempDir(), ".op-bin"),
	)
	output, err = build.CombinedOutput()
	if err != nil {
		t.Fatalf("op build example: %v\n%s", err, string(output))
	}

	binaryPath := filepath.Join(exampleDir, ".op", "build", "gabriel-greeting-go.holon", "bin", packageArchDir(), "gabriel-greeting-go")
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("stat built binary: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.Command(binaryPath, "serve", "--listen", "stdio://")
	cmd.Dir = t.TempDir()
	conn, startedCmd, err := holonsgrpcclient.DialStdioCommand(ctx, cmd)
	if err != nil {
		t.Fatalf("DialStdioCommand built package: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		if startedCmd.Process != nil {
			_ = startedCmd.Process.Kill()
		}
		_ = startedCmd.Wait()
	})

	describeCtx, describeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer describeCancel()

	response, err := holonsv1.NewHolonMetaClient(conn).Describe(describeCtx, &holonsv1.DescribeRequest{})
	if err != nil {
		t.Fatalf("Describe built package: %v", err)
	}
	if got := strings.ToLower(strings.Trim(strings.ReplaceAll(response.GetManifest().GetIdentity().GetGivenName()+"-"+strings.TrimSuffix(response.GetManifest().GetIdentity().GetFamilyName(), "?"), " ", "-"), "-")); got != "gabriel-greeting-go" {
		t.Fatalf("slug = %q, want %q", got, "gabriel-greeting-go")
	}
	if len(response.GetServices()) == 0 {
		t.Fatal("Describe built package returned no services")
	}
	if response.GetServices()[0].GetName() != "greeting.v1.GreetingService" {
		t.Fatalf("first service = %q, want %q", response.GetServices()[0].GetName(), "greeting.v1.GreetingService")
	}
}

func TestConnectStartsPackageBinaryViaStdio(t *testing.T) {
	root, slug, packageRoot := writePackageBinaryFixture(t, "Package", "Binary")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	conn, err := ConnectWithOpts(slug, ConnectOptions{
		Timeout:   5 * time.Second,
		Transport: TransportStdio,
		Lifecycle: LifecycleEphemeral,
		Start:     true,
	})
	if err != nil {
		t.Fatalf("Connect package slug: %v", err)
	}

	handle := lookupHandle(t, conn)
	if got, want := handle.process.cmd.Dir, packageRoot; got != want {
		_ = Disconnect(conn)
		t.Fatalf("child dir = %q, want %q", got, want)
	}
	if got, want := handle.process.cmd.Args[1:], []string{"serve", "--listen", "stdio://"}; !reflect.DeepEqual(got, want) {
		_ = Disconnect(conn)
		t.Fatalf("child args = %v, want %v", got, want)
	}

	requireUnaryEcho(t, conn, "package-binary-connect")

	pid := handle.process.cmd.Process.Pid
	if err := Disconnect(conn); err != nil {
		t.Fatalf("Disconnect package slug: %v", err)
	}
	waitForProcessExit(t, pid)
}

func TestResolveLaunchTargetUsesPackageDistInterpreter(t *testing.T) {
	root, slug := writePackageDistFixture(t, "Package", "Python", "python", "main.py")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	entry, err := discover.FindBySlug(slug)
	if err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if entry == nil {
		t.Fatal("expected package entry")
	}

	target, err := resolveLaunchTarget(*entry)
	if err != nil {
		t.Fatalf("resolveLaunchTarget: %v", err)
	}

	wantRoot := filepath.Join(root, ".op", "build", slug+".holon")
	if target.commandPath != "python3" {
		t.Fatalf("commandPath = %q, want %q", target.commandPath, "python3")
	}
	if !reflect.DeepEqual(target.args, []string{filepath.Join(wantRoot, "dist", "main.py")}) {
		t.Fatalf("args = %v", target.args)
	}
	if target.workingDirectory != wantRoot {
		t.Fatalf("workingDirectory = %q, want %q", target.workingDirectory, wantRoot)
	}
}

func TestResolveLaunchTargetFallsBackToPackageGitSource(t *testing.T) {
	root, slug := writePackageGitFixture(t, "Package", "Git")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	entry, err := discover.FindBySlug(slug)
	if err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if entry == nil {
		t.Fatal("expected package entry")
	}

	target, err := resolveLaunchTarget(*entry)
	if err != nil {
		t.Fatalf("resolveLaunchTarget: %v", err)
	}

	wantRoot := filepath.Join(root, ".op", "build", slug+".holon", "git")
	if target.commandPath != "go" {
		t.Fatalf("commandPath = %q, want %q", target.commandPath, "go")
	}
	if !reflect.DeepEqual(target.args, []string{"run", "./cmd/daemon"}) {
		t.Fatalf("args = %v, want %v", target.args, []string{"run", "./cmd/daemon"})
	}
	if target.workingDirectory != wantRoot {
		t.Fatalf("workingDirectory = %q, want %q", target.workingDirectory, wantRoot)
	}
}

func TestResolveSourceLaunchTargetUsesInterpreterForSupportedRunners(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name     string
		runner   string
		main     string
		wantCmd  string
		wantArgs []string
	}{
		{
			name:     "node",
			runner:   "node",
			main:     "server.js",
			wantCmd:  "node",
			wantArgs: []string{"server.js"},
		},
		{
			name:     "typescript",
			runner:   "typescript",
			main:     "src/main.ts",
			wantCmd:  "node",
			wantArgs: []string{"src/main.ts"},
		},
		{
			name:     "ruby",
			runner:   "ruby",
			main:     "app.rb",
			wantCmd:  "ruby",
			wantArgs: []string{"app.rb"},
		},
		{
			name:     "dart",
			runner:   "dart",
			main:     "bin/main.dart",
			wantCmd:  "dart",
			wantArgs: []string{"run", "bin/main.dart"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			target, err := resolveSourceLaunchTarget(discover.HolonEntry{
				Slug: "fixture",
				Dir:  root,
				Manifest: &discover.Manifest{
					Build: discover.Build{
						Runner: tc.runner,
						Main:   tc.main,
					},
					Artifacts: discover.Artifacts{Binary: "fixture"},
				},
			})
			if err != nil {
				t.Fatalf("resolveSourceLaunchTarget: %v", err)
			}
			if target.commandPath != tc.wantCmd {
				t.Fatalf("commandPath = %q, want %q", target.commandPath, tc.wantCmd)
			}
			if !reflect.DeepEqual(target.args, tc.wantArgs) {
				t.Fatalf("args = %v, want %v", target.args, tc.wantArgs)
			}
			if target.workingDirectory != root {
				t.Fatalf("workingDirectory = %q, want %q", target.workingDirectory, root)
			}
		})
	}
}

func TestResolveSourceLaunchTargetReturnsErrBinaryNotFoundForCompiledRunner(t *testing.T) {
	target, err := resolveSourceLaunchTarget(discover.HolonEntry{
		Slug: "compiled-fixture",
		Dir:  t.TempDir(),
		Manifest: &discover.Manifest{
			Build: discover.Build{Runner: "cmake"},
			Artifacts: discover.Artifacts{
				Binary: "compiled-fixture",
			},
		},
	})
	if err == nil {
		t.Fatalf("resolveSourceLaunchTarget = %#v, want error", target)
	}
	if !errors.Is(err, ErrBinaryNotFound) {
		t.Fatalf("error = %v, want ErrBinaryNotFound", err)
	}
}

func TestResolveLaunchTargetPackageMissingCurrentArch(t *testing.T) {
	root, slug := writePackageManifestOnlyFixture(t, "Package", "MissingArch", "go-module", "package-missingarch", []string{"linux_amd64"})
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	entry, err := discover.FindBySlug(slug)
	if err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if entry == nil {
		t.Fatal("expected package entry")
	}

	_, err = resolveLaunchTarget(*entry)
	if err == nil {
		t.Fatal("expected resolveLaunchTarget to fail")
	}
	if !strings.Contains(err.Error(), packageArchDir()) {
		t.Fatalf("error = %v, want mention of %q", err, packageArchDir())
	}
}

func TestConnectWithOptsPreservesErrBinaryNotFoundAcrossTransportAttempts(t *testing.T) {
	root, slug := writeProtoSourceManifestOnlyFixture(t, "Compiled", "MissingBinary", "cmake")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	_, err := ConnectWithOpts(slug, ConnectOptions{
		Timeout:   time.Second,
		Transport: TransportAuto,
		Lifecycle: LifecycleEphemeral,
		Start:     true,
	})
	if err == nil {
		t.Fatal("expected ConnectWithOpts to fail")
	}
	if !errors.Is(err, ErrBinaryNotFound) {
		t.Fatalf("error = %v, want ErrBinaryNotFound", err)
	}
}

func TestConnectWithOptsTCPEphemeralStopsProcessWithoutPortFile(t *testing.T) {
	root, slug := writeHolonFixture(t, "Connect", "TCPEphemeral")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	conn, err := ConnectWithOpts(slug, ConnectOptions{
		Timeout:   5 * time.Second,
		Transport: TransportTCP,
		Lifecycle: LifecycleEphemeral,
		Start:     true,
	})
	if err != nil {
		t.Fatalf("ConnectWithOpts tcp ephemeral: %v", err)
	}

	handle := lookupHandle(t, conn)
	pid := handle.process.cmd.Process.Pid
	requireUnaryEcho(t, conn, "tcp-ephemeral-connect")

	portFile := filepath.Join(root, ".op", "run", slug+".port")
	if _, err := os.Stat(portFile); !errors.Is(err, os.ErrNotExist) {
		_ = Disconnect(conn)
		t.Fatalf("ephemeral tcp connect wrote unexpected port file %q, err=%v", portFile, err)
	}

	if err := Disconnect(conn); err != nil {
		t.Fatalf("Disconnect tcp ephemeral conn: %v", err)
	}

	waitForProcessExit(t, pid)
	if _, err := os.Stat(portFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ephemeral tcp connect left unexpected port file %q, err=%v", portFile, err)
	}
}

func TestConnectWithOptsWritesPortFileAndLeavesProcessRunning(t *testing.T) {
	root, slug := writeHolonFixture(t, "Connect", "Persistent")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	conn, err := ConnectWithOpts(slug, ConnectOptions{Timeout: 5 * time.Second, Start: true})
	if err != nil {
		t.Fatalf("ConnectWithOpts slug: %v", err)
	}

	handle := lookupHandle(t, conn)
	pid := handle.process.cmd.Process.Pid
	requireUnaryEcho(t, conn, "persistent-connect")

	portFile := filepath.Join(root, ".op", "run", slug+".port")
	data, err := os.ReadFile(portFile)
	if err != nil {
		t.Fatalf("read port file: %v", err)
	}
	target := strings.TrimSpace(string(data))
	if !strings.HasPrefix(target, "tcp://127.0.0.1:") {
		t.Fatalf("port file should contain a tcp URI, got %q", target)
	}

	if err := Disconnect(conn); err != nil {
		t.Fatalf("Disconnect persistent conn: %v", err)
	}

	if !pidExists(pid) {
		t.Fatalf("persistent process %d exited after Disconnect", pid)
	}

	reused, err := Connect(slug)
	if err != nil {
		_ = stopProcess(handle.process)
		t.Fatalf("Connect via port file: %v", err)
	}
	defer func() { _ = Disconnect(reused) }()

	if remembered(reused) {
		_ = stopProcess(handle.process)
		t.Fatal("reused persistent port file should not register a started process")
	}
	requireUnaryEcho(t, reused, "persistent-reuse")

	if err := stopProcess(handle.process); err != nil {
		t.Fatalf("stop persistent child: %v", err)
	}
	waitForProcessExit(t, pid)
}

func TestConnectWithUnixOptionsWritesPortFileAndLeavesProcessRunning(t *testing.T) {
	root, slug := writeHolonFixture(t, "Connect", "Unix")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	conn, err := ConnectWithOpts(slug, ConnectOptions{
		Timeout:   5 * time.Second,
		Transport: "unix",
		Start:     true,
	})
	if err != nil {
		t.Fatalf("ConnectWithOpts unix slug: %v", err)
	}

	handle := lookupHandle(t, conn)
	pid := handle.process.cmd.Process.Pid
	requireUnaryEcho(t, conn, "unix-connect")

	portFile := filepath.Join(root, ".op", "run", slug+".port")
	data, err := os.ReadFile(portFile)
	if err != nil {
		t.Fatalf("read unix port file: %v", err)
	}
	target := strings.TrimSpace(string(data))
	if !strings.HasPrefix(target, "unix:///tmp/holons-") {
		t.Fatalf("port file should contain a unix URI, got %q", target)
	}

	if err := Disconnect(conn); err != nil {
		t.Fatalf("Disconnect unix conn: %v", err)
	}

	if !pidExists(pid) {
		t.Fatalf("unix process %d exited after Disconnect", pid)
	}

	reused, err := Connect(slug)
	if err != nil {
		_ = stopProcess(handle.process)
		t.Fatalf("Connect via unix port file: %v", err)
	}
	defer func() { _ = Disconnect(reused) }()

	if remembered(reused) {
		_ = stopProcess(handle.process)
		t.Fatal("reused unix port file should not register a started process")
	}
	requireUnaryEcho(t, reused, "unix-reuse")

	if err := stopProcess(handle.process); err != nil {
		t.Fatalf("stop unix child: %v", err)
	}
	waitForProcessExit(t, pid)
}

func TestConnectReusesExistingPortFile(t *testing.T) {
	root, slug := writeHolonFixture(t, "Connect", "Reuse")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	binaryPath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	startCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.Command(binaryPath, "serve", "--listen", "tcp://127.0.0.1:0")
	uri, proc, err := startTCPHolon(startCtx, cmd)
	if err != nil {
		skipIfLocalBindDenied(t, err)
		t.Fatalf("startTCPHolon: %v", err)
	}
	t.Cleanup(func() { _ = stopProcess(proc) })

	portFile := filepath.Join(root, ".op", "run", slug+".port")
	if err := writePortFile(portFile, uri); err != nil {
		t.Fatalf("write port file: %v", err)
	}

	conn, err := Connect(slug)
	if err != nil {
		t.Fatalf("Connect via existing port file: %v", err)
	}
	defer func() { _ = Disconnect(conn) }()

	if remembered(conn) {
		t.Fatal("existing port file should not register a started process")
	}
	requireUnaryEcho(t, conn, "reuse-connect")

	if !pidExists(proc.cmd.Process.Pid) {
		t.Fatal("existing process should remain alive after reused connection")
	}
}

func TestConnectRemovesStalePortFileAndStartsFresh(t *testing.T) {
	root, slug := writeHolonFixture(t, "Connect", "Stale")
	t.Chdir(root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-bin"))

	stalePort, err := reserveLoopbackPort()
	if err != nil {
		t.Fatalf("reserve loopback port: %v", err)
	}

	portFile := filepath.Join(root, ".op", "run", slug+".port")
	if err := writePortFile(portFile, fmt.Sprintf("tcp://127.0.0.1:%d", stalePort)); err != nil {
		t.Fatalf("write stale port file: %v", err)
	}

	conn, err := Connect(slug)
	if err != nil {
		t.Fatalf("Connect with stale port file: %v", err)
	}

	handle := lookupHandle(t, conn)
	pid := handle.process.cmd.Process.Pid
	requireUnaryEcho(t, conn, "stale-restart")

	if _, err := os.Stat(portFile); !errors.Is(err, os.ErrNotExist) {
		_ = Disconnect(conn)
		t.Fatalf("stale port file should be removed, got err=%v", err)
	}

	if err := Disconnect(conn); err != nil {
		t.Fatalf("Disconnect restarted slug: %v", err)
	}
	waitForProcessExit(t, pid)
}

func TestUsableStartupURI(t *testing.T) {
	tests := []struct {
		uri  string
		want bool
	}{
		{uri: "", want: false},
		{uri: "tcp://127.0.0.1:0", want: false},
		{uri: "tcp://:0", want: false},
		{uri: "tcp://127.0.0.1:9090", want: true},
		{uri: "tcp://:9090", want: true},
		{uri: "unix:///tmp/demo.sock", want: true},
	}

	for _, tt := range tests {
		if got := usableStartupURI(tt.uri); got != tt.want {
			t.Fatalf("usableStartupURI(%q) = %v, want %v", tt.uri, got, tt.want)
		}
	}
}

func runServeProcess(args []string) int {
	listenURI := "tcp://127.0.0.1:0"
	for i := 0; i < len(args); i++ {
		if args[i] == "--listen" && i+1 < len(args) {
			listenURI = args[i+1]
			break
		}
	}

	lis, err := transport.Listen(listenURI)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer lis.Close()

	server := grpc.NewServer()
	testgrpc.RegisterTestServiceServer(server, &connectTestServer{})

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- server.Serve(lis)
	}()

	if strings.HasPrefix(listenURI, "tcp://") {
		fmt.Fprintln(os.Stderr, publicURI(listenURI, lis.Addr()))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	var serveErr error
	select {
	case <-sigCh:
		server.GracefulStop()
		serveErr = <-serveErrCh
	case serveErr = <-serveErrCh:
	}

	if isBenignServeError(serveErr) {
		return 0
	}

	fmt.Fprintln(os.Stderr, serveErr)
	return 1
}

func publicURI(listenURI string, addr net.Addr) string {
	host := "127.0.0.1"
	if strings.HasPrefix(listenURI, "tcp://") {
		rawHost := strings.TrimPrefix(listenURI, "tcp://")
		parsedHost, _, err := net.SplitHostPort(rawHost)
		if err == nil && parsedHost != "" && parsedHost != "0.0.0.0" && parsedHost != "::" && parsedHost != "[::]" {
			host = parsedHost
		}
	}

	_, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "tcp://" + addr.String()
	}
	return fmt.Sprintf("tcp://%s:%s", host, port)
}

func isBenignServeError(err error) bool {
	if err == nil ||
		errors.Is(err, grpc.ErrServerStopped) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}

func requireUnaryEcho(t *testing.T, conn *grpc.ClientConn, message string) {
	t.Helper()

	client := testgrpc.NewTestServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.UnaryCall(ctx, &testgrpc.SimpleRequest{
		Payload: &testgrpc.Payload{
			Type: testgrpc.PayloadType_COMPRESSABLE,
			Body: []byte(message),
		},
	})
	if err != nil {
		t.Fatalf("UnaryCall: %v", err)
	}

	if got := string(resp.GetPayload().GetBody()); got != message {
		t.Fatalf("UnaryCall body = %q, want %q", got, message)
	}
}

func writeHolonFixture(t *testing.T, given, family string) (string, string) {
	return writeProtoHolonFixture(t, given, family)
}

func writePackageBinaryFixture(t *testing.T, given, family string) (string, string, string) {
	t.Helper()

	slug := strings.ToLower(given + "-" + family)
	root, _ := writePackageManifestOnlyFixture(t, given, family, "go-module", slug, []string{packageArchDir()})
	packageRoot := filepath.Join(root, ".op", "build", slug+".holon")
	binaryPath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	builtBinary := filepath.Join(packageRoot, "bin", packageArchDir(), slug)
	if err := os.MkdirAll(filepath.Dir(builtBinary), 0o755); err != nil {
		t.Fatalf("mkdir package bin dir: %v", err)
	}
	if err := copyExecutable(binaryPath, builtBinary); err != nil {
		t.Fatalf("copy package binary: %v", err)
	}
	return root, slug, packageRoot
}

func writePackageDistFixture(t *testing.T, given, family, runner, entrypoint string) (string, string) {
	t.Helper()

	root, slug := writePackageManifestOnlyFixture(t, given, family, runner, entrypoint, nil)
	packageRoot := filepath.Join(root, ".op", "build", slug+".holon")
	updatePackageFixtureFlags(t, packageRoot, true, false)
	distPath := filepath.Join(packageRoot, "dist", filepath.FromSlash(entrypoint))
	if err := os.MkdirAll(filepath.Dir(distPath), 0o755); err != nil {
		t.Fatalf("mkdir dist dir: %v", err)
	}
	if err := os.WriteFile(distPath, []byte("print('hello')\n"), 0o644); err != nil {
		t.Fatalf("write dist entrypoint: %v", err)
	}
	return root, slug
}

func writePackageGitFixture(t *testing.T, given, family string) (string, string) {
	t.Helper()

	slug := strings.ToLower(given + "-" + family)
	root, _ := writePackageManifestOnlyFixture(t, given, family, "go-module", slug, nil)
	gitRoot := filepath.Join(root, ".op", "build", slug+".holon", "git")
	updatePackageFixtureFlags(t, filepath.Join(root, ".op", "build", slug+".holon"), false, true)
	writeGitSourceFixture(t, gitRoot, slug, given, family)
	return root, slug
}

func writePackageManifestOnlyFixture(t *testing.T, given, family, runner, entrypoint string, architectures []string) (string, string) {
	t.Helper()

	root := t.TempDir()
	slug := strings.ToLower(given + "-" + family)
	packageRoot := filepath.Join(root, ".op", "build", slug+".holon")
	if err := os.MkdirAll(packageRoot, 0o755); err != nil {
		t.Fatalf("mkdir package root: %v", err)
	}

	if len(architectures) == 0 {
		architectures = []string{}
	}
	architecturesJSON := "[" + strings.Join(quoteStrings(architectures), ", ") + "]"
	data := fmt.Sprintf(`{
  "schema": "holon-package/v1",
  "slug": %q,
  "uuid": %q,
  "identity": {
    "given_name": %q,
    "family_name": %q
  },
  "lang": "go",
  "runner": %q,
  "status": "draft",
  "kind": "native",
  "entrypoint": %q,
  "architectures": %s,
  "has_dist": false,
  "has_source": false
}
`, slug, slug+"-uuid", given, family, runner, entrypoint, architecturesJSON)
	if err := os.WriteFile(filepath.Join(packageRoot, ".holon.json"), []byte(data), 0o644); err != nil {
		t.Fatalf("write .holon.json: %v", err)
	}
	return root, slug
}

func updatePackageFixtureFlags(t *testing.T, packageRoot string, hasDist, hasSource bool) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(packageRoot, ".holon.json"))
	if err != nil {
		t.Fatalf("read .holon.json: %v", err)
	}
	updated := strings.Replace(string(data), `"has_dist": false`, fmt.Sprintf(`"has_dist": %t`, hasDist), 1)
	updated = strings.Replace(updated, `"has_source": false`, fmt.Sprintf(`"has_source": %t`, hasSource), 1)
	if err := os.WriteFile(filepath.Join(packageRoot, ".holon.json"), []byte(updated), 0o644); err != nil {
		t.Fatalf("write .holon.json: %v", err)
	}
}

func writeGitSourceFixture(t *testing.T, root, slug, given, family string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, "api", "v1"), 0o755); err != nil {
		t.Fatalf("mkdir api dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cmd", "daemon"), 0o755); err != nil {
		t.Fatalf("mkdir daemon dir: %v", err)
	}
	writeConnectManifestProto(t, root)

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/package-git\n\ngo 1.25.1\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "daemon", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	proto := fmt.Sprintf(`syntax = "proto3";

package connect.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    uuid: %q
    given_name: %q
    family_name: %q
    motto: "Package git fixture."
    composer: "connect-test"
    clade: "deterministic/pure"
    status: "draft"
    born: "2026-03-15"
  }
  lineage: {
    reproduction: "manual"
    generated_by: "connect-test"
  }
  kind: "native"
  lang: "go"
  build: {
    runner: "go-module"
    main: "./cmd/daemon"
  }
  requires: {
    files: ["go.mod"]
  }
  artifacts: {
    binary: %q
  }
};
`, slug+"-uuid", given, family, slug)
	if err := os.WriteFile(filepath.Join(root, "api", "v1", "holon.proto"), []byte(proto), 0o644); err != nil {
		t.Fatalf("write holon.proto: %v", err)
	}
}

func quoteStrings(values []string) []string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return quoted
}

func writeProtoHolonFixture(t *testing.T, given, family string) (string, string) {
	t.Helper()

	root := t.TempDir()
	binaryPath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	slug := strings.ToLower(given + "-" + family)
	holonDir := filepath.Join(root, "holons", slug)
	if err := os.MkdirAll(filepath.Join(holonDir, "v1"), 0o755); err != nil {
		t.Fatalf("mkdir proto dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(holonDir, "cmd", "daemon"), 0o755); err != nil {
		t.Fatalf("mkdir daemon dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(holonDir, ".op", "build", "bin"), 0o755); err != nil {
		t.Fatalf("mkdir build dir: %v", err)
	}

	writeConnectManifestProto(t, root)

	if err := os.WriteFile(filepath.Join(holonDir, "go.mod"), []byte("module example.com/proto-connect\n\ngo 1.25.1\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(holonDir, "cmd", "daemon", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	builtBinary := filepath.Join(holonDir, ".op", "build", "bin", slug)
	if err := copyExecutable(binaryPath, builtBinary); err != nil {
		t.Fatalf("copy test binary: %v", err)
	}

	proto := fmt.Sprintf(`syntax = "proto3";

package connect.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    uuid: %q
    given_name: %q
    family_name: %q
    motto: "Proto connect fixture."
    composer: "connect-test"
    clade: "deterministic/pure"
    status: "draft"
    born: "2026-03-15"
  }
  lineage: {
    reproduction: "manual"
    generated_by: "connect-test"
  }
  kind: "native"
  lang: "go"
  build: {
    runner: "go-module"
    main: "./cmd/daemon"
  }
  requires: {
    files: ["go.mod"]
  }
  artifacts: {
    binary: %q
  }
};
`, slug+"-uuid", given, family, slug)

	if err := os.WriteFile(filepath.Join(holonDir, "v1", "holon.proto"), []byte(proto), 0o644); err != nil {
		t.Fatalf("write holon.proto: %v", err)
	}

	return root, slug
}

func writeProtoSourceManifestOnlyFixture(t *testing.T, given, family, runner string) (string, string) {
	t.Helper()

	root := t.TempDir()
	slug := strings.ToLower(given + "-" + family)
	holonDir := filepath.Join(root, "holons", slug)
	if err := os.MkdirAll(filepath.Join(holonDir, "v1"), 0o755); err != nil {
		t.Fatalf("mkdir proto dir: %v", err)
	}

	writeConnectManifestProto(t, root)

	proto := fmt.Sprintf(`syntax = "proto3";

package connect.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    uuid: %q
    given_name: %q
    family_name: %q
    motto: "Proto source-only connect fixture."
    composer: "connect-test"
    clade: "deterministic/pure"
    status: "draft"
    born: "2026-03-24"
  }
  lineage: {
    reproduction: "manual"
    generated_by: "connect-test"
  }
  kind: "native"
  lang: "go"
  build: {
    runner: %q
  }
  artifacts: {
    binary: %q
  }
};
`, slug+"-uuid", given, family, runner, slug)

	if err := os.WriteFile(filepath.Join(holonDir, "v1", "holon.proto"), []byte(proto), 0o644); err != nil {
		t.Fatalf("write holon.proto: %v", err)
	}

	return root, slug
}

func lookupHandle(t *testing.T, conn *grpc.ClientConn) connHandle {
	t.Helper()

	mu.Lock()
	handle, ok := started[conn]
	mu.Unlock()
	if !ok || handle.process == nil || handle.process.cmd == nil || handle.process.cmd.Process == nil {
		t.Fatal("expected started process handle")
	}
	return handle
}

func remembered(conn *grpc.ClientConn) bool {
	mu.Lock()
	_, ok := started[conn]
	mu.Unlock()
	return ok
}

func waitForProcessExit(t *testing.T, pid int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !pidExists(pid) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("process %d still running after deadline", pid)
}

func pidExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func reserveLoopbackPort() (int, error) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer lis.Close()

	addr, ok := lis.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected address type %T", lis.Addr())
	}
	return addr.Port, nil
}

func skipIfLocalBindDenied(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}
	msg := strings.ToLower(err.Error())
	if errors.Is(err, syscall.EPERM) || strings.Contains(msg, "operation not permitted") {
		t.Skipf("local bind denied in this environment: %v", err)
	}
}

func writeConnectManifestProto(t *testing.T, root string) {
	t.Helper()

	source := filepath.Join(connectIdentityTestdataRoot(t), "_protos", "holons", "v1", "manifest.proto")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}

	target := filepath.Join(root, "_protos", "holons", "v1")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", target, err)
	}
	if err := os.WriteFile(filepath.Join(target, "manifest.proto"), data, 0o644); err != nil {
		t.Fatalf("write manifest.proto: %v", err)
	}
}

func connectIdentityTestdataRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "identity", "testdata")
}

func connectRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

func copyExecutable(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode()|fs.FileMode(0o111))
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDirTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}
