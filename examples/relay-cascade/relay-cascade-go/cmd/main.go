package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	relayv1 "cascade-node-go/gen/go/relay/v1"
	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	slug      = "cascade-node-go"
	runTicks  = 3
	runPhases = 4
)

var roleOrder = []string{"D", "C", "B", "A"}

type roleRuntime struct {
	role          string
	uid           string
	binaryPath    string
	listenURIs    []string
	relayAddress  string
	memberAddress string
	clientTarget  string
	metricsAddr   string
	cmd           *exec.Cmd
	conn          *grpc.ClientConn
	stdout        bytes.Buffer
	stderr        bytes.Buffer
}

type cascade struct {
	phase     int
	transport string
	runRoot   string
	roles     map[string]*roleRuntime
}

type checkResult struct {
	pass     bool
	evidence string
}

func main() {
	start := time.Now()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nFAIL: %v\n", err)
		os.Exit(1)
	}
	_ = start
}

func run() error {
	binaryPath, err := findCascadeNodeBinary()
	if err != nil {
		return err
	}
	runRoot := filepath.Join(os.Getenv("HOME"), ".op", "run")
	transports := []string{"stdio", "tcp", "unix", "stdio"}

	fmt.Println("=== relay-cascade-go ===")
	fmt.Println()

	totalPass := 0
	totalFail := 0
	var previous string
	for phaseIdx, transport := range transports {
		phaseNo := phaseIdx + 1
		if previous == "" {
			fmt.Printf("Phase %d/%d: transport=%s\n", phaseNo, runPhases, transport)
		} else if phaseNo == runPhases && transport == transports[0] {
			fmt.Printf("Phase %d/%d: transport=%s (cycle wrap)\n", phaseNo, runPhases, transport)
		} else {
			fmt.Printf("Phase %d/%d: transport=%s (switching from %s)\n", phaseNo, runPhases, transport, previous)
		}

		spawnStart := time.Now()
		c, err := spawnCascade(phaseNo, transport, binaryPath, runRoot)
		if err != nil {
			totalFail += runTicks
			fmt.Printf("  spawn FAIL: %v\n\n", err)
			previous = transport
			continue
		}
		fmt.Printf("  spawned 4 nodes in %s\n", time.Since(spawnStart).Round(time.Millisecond))

		previousMetric := float64(0)
		for tick := 1; tick <= runTicks; tick++ {
			tickStart := time.Now()
			result := c.runTick(tick, previousMetric)
			if result.metric.pass {
				previousMetric = result.metricValue
			}
			overall := result.log.pass && result.event.pass && result.metric.pass
			if overall {
				totalPass++
			} else {
				totalFail++
			}
			status := "FAIL"
			if overall {
				status = "PASS"
			}
			fmt.Printf("  Tick %d/%d: log %s, event %s, metric %s (overall %s in %s)\n",
				tick,
				runTicks,
				passText(result.log.pass),
				passText(result.event.pass),
				passText(result.metric.pass),
				status,
				time.Since(tickStart).Round(time.Millisecond),
			)
			if !overall {
				printFailureEvidence("log", result.log)
				printFailureEvidence("event", result.event)
				printFailureEvidence("metric", result.metric)
			}
		}
		c.stop()
		fmt.Println()
		previous = transport
	}

	fmt.Printf("Summary: %d ticks, %d PASS, %d FAIL\n", totalPass+totalFail, totalPass, totalFail)
	if totalFail > 0 {
		return fmt.Errorf("%d tick(s) failed", totalFail)
	}
	return nil
}

type tickOutcome struct {
	log         checkResult
	event       checkResult
	metric      checkResult
	metricValue float64
}

func (c *cascade) runTick(tick int, previousMetric float64) tickOutcome {
	sender := fmt.Sprintf("phase-%d-tick-%d", c.phase, tick)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, tickErr := relayv1.NewRelayServiceClient(c.roles["D"].conn).Tick(ctx, &relayv1.TickRequest{
		Sender: sender,
		Note:   c.transport,
	})
	if tickErr != nil {
		errResult := checkResult{evidence: tickErr.Error()}
		return tickOutcome{log: errResult, event: errResult, metric: errResult}
	}

	logResult := waitFor(3*time.Second, func() checkResult {
		return c.checkLog(sender)
	})
	eventResult := waitFor(3*time.Second, c.checkEvent)
	metricValue := previousMetric
	metricResult := waitFor(3*time.Second, func() checkResult {
		res, value := c.checkMetric(previousMetric)
		if res.pass {
			metricValue = value
		}
		return res
	})
	return tickOutcome{
		log:         logResult,
		event:       eventResult,
		metric:      metricResult,
		metricValue: metricValue,
	}
}

func spawnCascade(phase int, transportName, binaryPath, runRoot string) (*cascade, error) {
	c := &cascade{
		phase:     phase,
		transport: transportName,
		runRoot:   runRoot,
		roles:     make(map[string]*roleRuntime, len(roleOrder)),
	}
	for _, role := range roleOrder {
		r := newRoleRuntime(phase, transportName, role, binaryPath)
		c.roles[role] = r
		if err := os.RemoveAll(filepath.Join(runRoot, slug, r.uid)); err != nil {
			return nil, err
		}
		if transportName == "unix" || transportName == "stdio" {
			for _, uri := range r.listenURIs {
				if strings.HasPrefix(uri, "unix://") {
					_ = os.Remove(strings.TrimPrefix(uri, "unix://"))
				}
			}
		}
	}
	for i, role := range roleOrder {
		r := c.roles[role]
		if i > 0 {
			child := c.roles[roleOrder[i-1]]
			r.memberAddress = child.relayAddress
		}
		if err := c.startRole(r); err != nil {
			c.stop()
			return nil, err
		}
	}
	time.Sleep(150 * time.Millisecond)
	return c, nil
}

func newRoleRuntime(phase int, transportName, role, binaryPath string) *roleRuntime {
	lower := strings.ToLower(role)
	r := &roleRuntime{
		role:       role,
		uid:        fmt.Sprintf("relay-p%02d-%s", phase, lower),
		binaryPath: binaryPath,
	}
	switch transportName {
	case "tcp":
		port := map[string]int{"A": 9090, "B": 9091, "C": 9092, "D": 9093}[role]
		r.listenURIs = []string{fmt.Sprintf("tcp://127.0.0.1:%d", port)}
		r.clientTarget = fmt.Sprintf("127.0.0.1:%d", port)
		r.relayAddress = r.listenURIs[0]
	case "unix":
		path := fmt.Sprintf("/tmp/relay-cascade-%s.sock", lower)
		r.listenURIs = []string{"unix://" + path}
		r.clientTarget = "unix://" + path
		r.relayAddress = r.clientTarget
	case "stdio":
		sidecar := fmt.Sprintf("/tmp/relay-cascade-stdio-%s.sock", lower)
		// The composite owns each process' stdio pipe. Member relays need a
		// dialable endpoint between sibling processes, so stdio phases expose
		// a private Unix sidecar while the composite talks to each node over
		// stdio.
		r.listenURIs = []string{"stdio://", "unix://" + sidecar}
		r.clientTarget = "stdio://"
		r.relayAddress = "unix://" + sidecar
	default:
		panic("unknown transport " + transportName)
	}
	return r
}

func (c *cascade) startRole(r *roleRuntime) error {
	args := []string{"serve"}
	for _, uri := range r.listenURIs {
		args = append(args, "--listen", uri)
	}
	if r.memberAddress != "" {
		args = append(args, "--member", slug+"="+r.memberAddress)
	}
	cmd := exec.Command(r.binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"OP_OBS=logs,events,metrics,prom",
		"OP_RUN_DIR="+c.runRoot,
		"OP_INSTANCE_UID="+r.uid,
		"OP_ORGANISM_UID="+c.roles["A"].uid,
		"OP_ORGANISM_SLUG="+slug,
		"OP_PROM_ADDR=127.0.0.1:0",
	)
	cmd.Stderr = &r.stderr

	if c.transport == "stdio" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, started, err := grpcclient.DialStdioCommand(ctx, cmd)
		if err != nil {
			return fmt.Errorf("start %s over stdio: %w; stderr=%s", r.role, err, r.stderr.String())
		}
		r.cmd = started
		r.conn = conn
	} else {
		cmd.Stdout = &r.stdout
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start %s: %w", r.role, err)
		}
		r.cmd = cmd
	}

	meta, err := waitMeta(c.runRoot, r.uid, 10*time.Second)
	if err != nil {
		return fmt.Errorf("wait %s meta: %w; stderr=%s", r.role, err, r.stderr.String())
	}
	r.metricsAddr = meta.MetricsAddr
	if r.conn == nil {
		conn, err := dialReady(r.clientTarget, 10*time.Second)
		if err != nil {
			return fmt.Errorf("dial %s: %w; stderr=%s", r.role, err, r.stderr.String())
		}
		r.conn = conn
	} else if err := describeReady(r.conn, 5*time.Second); err != nil {
		return fmt.Errorf("describe %s: %w; stderr=%s", r.role, err, r.stderr.String())
	}
	return nil
}

func (c *cascade) stop() {
	for i := len(roleOrder) - 1; i >= 0; i-- {
		r := c.roles[roleOrder[i]]
		if r == nil {
			continue
		}
		if r.conn != nil {
			_ = r.conn.Close()
		}
		if r.cmd != nil && r.cmd.Process != nil {
			_ = r.cmd.Process.Signal(syscall.SIGTERM)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for _, role := range roleOrder {
		r := c.roles[role]
		if r == nil || r.cmd == nil {
			continue
		}
		done := make(chan error, 1)
		go func(cmd *exec.Cmd) { done <- cmd.Wait() }(r.cmd)
		remaining := time.Until(deadline)
		if remaining <= 0 {
			remaining = 10 * time.Millisecond
		}
		select {
		case <-done:
		case <-time.After(remaining):
			if r.cmd.Process != nil {
				_ = r.cmd.Process.Kill()
			}
			<-done
		}
	}
}

func (c *cascade) checkLog(sender string) checkResult {
	entries, err := readLogs(c.roles["A"].conn)
	if err != nil {
		return checkResult{evidence: err.Error()}
	}
	for _, entry := range entries {
		if entry.GetMessage() != "tick received" {
			continue
		}
		if entry.GetFields()["sender"] != sender || entry.GetFields()["responder_uid"] != c.roles["D"].uid {
			continue
		}
		if err := c.checkChain(entry.GetChain()); err != nil {
			return checkResult{evidence: "matching log has bad chain: " + err.Error() + " entry=" + marshalProto(entry)}
		}
		return checkResult{pass: true, evidence: marshalProto(entry)}
	}
	return checkResult{evidence: fmt.Sprintf("no relayed D tick log for sender=%s in %d A log entries", sender, len(entries))}
}

func (c *cascade) checkEvent() checkResult {
	events, err := readEvents(c.roles["A"].conn)
	if err != nil {
		return checkResult{evidence: err.Error()}
	}
	for _, event := range events {
		if event.GetType() != holonsv1.EventType_INSTANCE_READY || event.GetInstanceUid() != c.roles["D"].uid {
			continue
		}
		if err := c.checkChain(event.GetChain()); err != nil {
			return checkResult{evidence: "matching event has bad chain: " + err.Error() + " event=" + marshalProto(event)}
		}
		return checkResult{pass: true, evidence: marshalProto(event)}
	}
	return checkResult{evidence: fmt.Sprintf("no relayed D INSTANCE_READY event in %d A events", len(events))}
}

func (c *cascade) checkMetric(previous float64) (checkResult, float64) {
	body, err := fetchMetrics(c.roles["D"].metricsAddr)
	if err != nil {
		return checkResult{evidence: err.Error()}, previous
	}
	value, ok := parseCascadeTicks(body, c.roles["D"].uid)
	if !ok {
		return checkResult{evidence: body}, previous
	}
	if value <= previous {
		return checkResult{evidence: fmt.Sprintf("cascade_ticks_total=%v did not increase beyond %v\n%s", value, previous, body)}, value
	}
	return checkResult{pass: true, evidence: fmt.Sprintf("cascade_ticks_total=%v", value)}, value
}

func (c *cascade) checkChain(chain []*holonsv1.ChainHop) error {
	want := []string{c.roles["D"].uid, c.roles["C"].uid, c.roles["B"].uid}
	if len(chain) < len(want) {
		return fmt.Errorf("chain length %d < %d", len(chain), len(want))
	}
	for i, uid := range want {
		if chain[i].GetSlug() != slug || chain[i].GetInstanceUid() != uid {
			return fmt.Errorf("hop %d = %s/%s, want %s/%s", i, chain[i].GetSlug(), chain[i].GetInstanceUid(), slug, uid)
		}
	}
	return nil
}

func readLogs(conn *grpc.ClientConn) ([]*holonsv1.LogEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream, err := holonsv1.NewHolonObservabilityClient(conn).Logs(ctx, &holonsv1.LogsRequest{
		MinLevel: holonsv1.LogLevel_INFO,
		Follow:   false,
	})
	if err != nil {
		return nil, err
	}
	var entries []*holonsv1.LogEntry
	for {
		entry, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
}

func readEvents(conn *grpc.ClientConn) ([]*holonsv1.EventInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream, err := holonsv1.NewHolonObservabilityClient(conn).Events(ctx, &holonsv1.EventsRequest{Follow: false})
	if err != nil {
		return nil, err
	}
	var events []*holonsv1.EventInfo
	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return events, nil
		}
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
}

func waitFor(timeout time.Duration, fn func() checkResult) checkResult {
	deadline := time.Now().Add(timeout)
	var last checkResult
	for {
		last = fn()
		if last.pass || time.Now().After(deadline) {
			return last
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitMeta(runRoot, uid string, timeout time.Duration) (observability.MetaJSON, error) {
	deadline := time.Now().Add(timeout)
	dir := filepath.Join(runRoot, slug, uid)
	for {
		meta, err := observability.ReadMetaJSON(dir)
		if err == nil && meta.UID == uid && meta.MetricsAddr != "" {
			return meta, nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return observability.MetaJSON{}, err
			}
			return observability.MetaJSON{}, fmt.Errorf("meta not ready for %s", uid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func dialReady(target string, timeout time.Duration) (*grpc.ClientConn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		conn, err := grpcclient.Dial(ctx, target)
		cancel()
		if err == nil {
			if describeErr := describeReady(conn, time.Second); describeErr == nil {
				return conn, nil
			} else {
				lastErr = describeErr
				_ = conn.Close()
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return nil, lastErr
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func describeReady(conn *grpc.ClientConn, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_, err := holonsv1.NewHolonMetaClient(conn).Describe(ctx, &holonsv1.DescribeRequest{})
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func fetchMetrics(addr string) (string, error) {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(addr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return string(body), fmt.Errorf("metrics HTTP %s", resp.Status)
	}
	return string(body), nil
}

func parseCascadeTicks(body, uid string) (float64, bool) {
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "cascade_ticks_total{") || !strings.Contains(line, `responder_uid="`+uid+`"`) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		if err == nil {
			return value, true
		}
	}
	return 0, false
}

func findCascadeNodeBinary() (string, error) {
	if override := strings.TrimSpace(os.Getenv("CASCADE_NODE_GO_BIN")); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, ".op", "bin", "cascade-node-go.holon", "bin")
	var found string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || d.Name() != "cascade-node-go" {
			return walkErr
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&0o111 == 0 {
			return nil
		}
		if strings.Contains(path, runtime.GOOS) || found == "" {
			found = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("cascade-node-go binary not found under %s; run op build cascade-node-go --install", root)
	}
	return found, nil
}

func passText(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}

func printFailureEvidence(family string, result checkResult) {
	if result.pass {
		return
	}
	evidence := strings.TrimSpace(result.evidence)
	if evidence == "" {
		evidence = "<empty>"
	}
	fmt.Printf("    %s evidence: %s\n", family, evidence)
}

func marshalProto(msg proto.Message) string {
	b, _ := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(msg)
	return string(b)
}
