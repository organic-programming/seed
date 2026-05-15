package composite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/observability"
	"google.golang.org/grpc"
)

type ChildSpec struct {
	Slug   string
	Binary string
}

type SpawnOptions struct {
	Slug            string
	BinaryPath      string
	Transport       string
	InstanceUID     string
	DownstreamChain []ChildSpec
	ExtraEnv        map[string]string
	DialOptions     []DialOption
}

type SpawnedMember struct {
	Slug      string
	UID       string
	ListenURI string
	Conn      *grpc.ClientConn

	cmd    *exec.Cmd
	waitCh chan error
	relay  *observability.Relay
	once   sync.Once
}

func (m *SpawnedMember) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}
	var stopErr error
	m.once.Do(func() {
		if m.relay != nil {
			m.relay.Stop()
		}
		if m.Conn != nil {
			if err := m.Conn.Close(); err != nil && stopErr == nil {
				stopErr = err
			}
		}
		if m.cmd == nil || m.cmd.Process == nil {
			return
		}
		_ = m.cmd.Process.Signal(syscall.SIGTERM)
		waitCtx := ctx
		if waitCtx == nil {
			var cancel context.CancelFunc
			waitCtx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
		}
		select {
		case err := <-m.waitCh:
			if err != nil && stopErr == nil {
				stopErr = err
			}
		case <-waitCtx.Done():
			_ = m.cmd.Process.Kill()
			if err := <-m.waitCh; err != nil && stopErr == nil {
				stopErr = err
			}
			if stopErr == nil {
				stopErr = waitCtx.Err()
			}
		}
	})
	return stopErr
}

func SpawnMember(ctx context.Context, opts SpawnOptions) (*SpawnedMember, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	slug := strings.TrimSpace(optsSlug(opts))
	binary := strings.TrimSpace(opts.BinaryPath)
	if slug == "" {
		return nil, fmt.Errorf("spawn member: slug is required")
	}
	if binary == "" {
		return nil, fmt.Errorf("spawn member %s: binary path is required", slug)
	}
	uid := strings.TrimSpace(opts.InstanceUID)
	if uid == "" {
		uid = newInstanceUID()
	}
	transportName := strings.ToLower(strings.TrimSpace(opts.Transport))
	if transportName == "" {
		transportName = "stdio"
	}
	listenURI, cleanup, err := listenURIForSpawn(transportName, slug, uid)
	if err != nil {
		return nil, err
	}
	if cleanup != "" {
		_ = os.Remove(cleanup)
	}

	args := []string{"serve", "--listen", listenURI, "--transport", transportName}
	for _, child := range opts.DownstreamChain {
		if child.Slug == "" || child.Binary == "" {
			return nil, fmt.Errorf("spawn member %s: downstream child requires slug and binary", slug)
		}
		args = append(args, "--child", child.Slug+"="+child.Binary)
	}

	cmd := exec.Command(binary, args...)
	cmd.Dir = filepath.Dir(binary)
	cmd.Env = spawnEnvironment(uid, opts.ExtraEnv)

	member := &SpawnedMember{
		Slug:   slug,
		UID:    uid,
		cmd:    cmd,
		waitCh: make(chan error, 1),
	}

	if transportName == "stdio" {
		conn, started, err := grpcclient.DialStdioCommand(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("spawn member %s over stdio: %w", slug, err)
		}
		member.cmd = started
		member.Conn = conn
		member.ListenURI = "stdio://"
		go func() { member.waitCh <- started.Wait() }()
	} else {
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("spawn member %s: %w", slug, err)
		}
		go func() { member.waitCh <- cmd.Wait() }()
		meta, err := waitSpawnMeta(ctx, spawnRunRoot(cmd.Env), slug, uid, 10*time.Second)
		if err != nil {
			_ = member.Stop(context.Background())
			return nil, fmt.Errorf("spawn member %s: %w", slug, err)
		}
		member.ListenURI = meta.Address
		conn, err := dialReady(ctx, normalizeDialTarget(meta.Address), 10*time.Second)
		if err != nil {
			_ = member.Stop(context.Background())
			return nil, fmt.Errorf("spawn member %s dial %s: %w", slug, meta.Address, err)
		}
		member.Conn = conn
	}

	dialOpts := applyDialOptions(opts.DialOptions)
	transitive := true
	if dialOpts.transitiveObservability != nil {
		transitive = *dialOpts.transitiveObservability
	}
	if transitive {
		member.relay = observability.NewRelay(slug, uid, member.Conn)
		if err := member.relay.Start(ctx); err != nil {
			_ = member.Stop(context.Background())
			return nil, err
		}
	}
	return member, nil
}

type CascadeOptions struct {
	Transport string
	Members   []ChildSpec
	ExtraEnv  map[string]string
}

type Cascade struct {
	Top *SpawnedMember
}

func (c *Cascade) Stop(ctx context.Context) error {
	if c == nil || c.Top == nil {
		return nil
	}
	return c.Top.Stop(ctx)
}

func BuildCascade(ctx context.Context, opts CascadeOptions) (*Cascade, error) {
	if len(opts.Members) == 0 {
		return nil, fmt.Errorf("build cascade: at least one member is required")
	}
	top := opts.Members[0]
	spawned, err := SpawnMember(ctx, SpawnOptions{
		Slug:            top.Slug,
		BinaryPath:      top.Binary,
		Transport:       opts.Transport,
		DownstreamChain: append([]ChildSpec(nil), opts.Members[1:]...),
		ExtraEnv:        opts.ExtraEnv,
	})
	if err != nil {
		return nil, err
	}
	return &Cascade{Top: spawned}, nil
}

func optsSlug(opts SpawnOptions) string {
	if slug := strings.TrimSpace(opts.Slug); slug != "" {
		return slug
	}
	return filepath.Base(strings.TrimSpace(opts.BinaryPath))
}

func listenURIForSpawn(transportName, _ string, uid string) (uri string, cleanupPath string, err error) {
	switch transportName {
	case "stdio":
		return "stdio://", "", nil
	case "tcp":
		return "tcp://127.0.0.1:0", "", nil
	case "unix":
		name := fmt.Sprintf("op-%s.sock", cleanSocketToken(uid))
		path := filepath.Join(os.TempDir(), name)
		return "unix://" + path, path, nil
	default:
		return "", "", fmt.Errorf("unsupported transport %q", transportName)
	}
}

func spawnEnvironment(uid string, extra map[string]string) []string {
	env := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	env["OP_INSTANCE_UID"] = uid
	env["OP_RUN_DIR"] = runRootFromEnv(env)
	env["HOLONS_PARENT_PID"] = fmt.Sprintf("%d", os.Getpid())
	if families := activeObservabilityFamilies(); families != "" {
		env["OP_OBS"] = families
	}
	for key, value := range extra {
		env[key] = value
	}
	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}

func activeObservabilityFamilies() string {
	obs := observability.Current()
	if obs == nil {
		return ""
	}
	families := []observability.Family{
		observability.FamilyLogs,
		observability.FamilyMetrics,
		observability.FamilyEvents,
		observability.FamilyProm,
	}
	var enabled []string
	for _, family := range families {
		if obs.Enabled(family) {
			enabled = append(enabled, string(family))
		}
	}
	return strings.Join(enabled, ",")
}

func runRootFromEnv(env map[string]string) string {
	if root := strings.TrimSpace(env["OP_RUN_DIR"]); root != "" {
		return root
	}
	if root := strings.TrimSpace(env["OPPATH"]); root != "" {
		return filepath.Join(root, "run")
	}
	if home := strings.TrimSpace(env["HOME"]); home != "" {
		return filepath.Join(home, ".op", "run")
	}
	return filepath.Join(os.TempDir(), ".op", "run")
}

func spawnRunRoot(env []string) string {
	values := map[string]string{}
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[key] = value
		}
	}
	return runRootFromEnv(values)
}

func waitSpawnMeta(ctx context.Context, runRoot, slug, uid string, timeout time.Duration) (observability.MetaJSON, error) {
	deadline := time.Now().Add(timeout)
	dir := filepath.Join(runRoot, slug, uid)
	var lastErr error
	for {
		meta, err := observability.ReadMetaJSON(dir)
		if err == nil && meta.UID == uid && meta.Address != "" {
			return meta, nil
		}
		if err != nil {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return observability.MetaJSON{}, lastErr
			}
			return observability.MetaJSON{}, fmt.Errorf("meta not ready for %s/%s", slug, uid)
		}
		select {
		case <-ctx.Done():
			return observability.MetaJSON{}, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func dialReady(ctx context.Context, target string, timeout time.Duration) (*grpc.ClientConn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, time.Second)
		conn, err := grpcclient.Dial(attemptCtx, target)
		cancel()
		if err == nil {
			if err := describeReady(ctx, conn, time.Second); err == nil {
				return conn, nil
			} else {
				lastErr = err
				_ = conn.Close()
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return nil, lastErr
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func describeReady(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		_, err := holonsv1.NewHolonMetaClient(conn).Describe(attemptCtx, &holonsv1.DescribeRequest{})
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return lastErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func normalizeDialTarget(target string) string {
	trimmed := strings.TrimSpace(target)
	if !strings.Contains(trimmed, "://") {
		return trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	switch parsed.Scheme {
	case "tcp":
		host := parsed.Hostname()
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		return net.JoinHostPort(host, parsed.Port())
	case "unix":
		return trimmed
	default:
		return trimmed
	}
}

func newInstanceUID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func cleanSocketToken(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 24 {
		value = value[:24]
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-")
	return replacer.Replace(value)
}
