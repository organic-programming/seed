package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	goSlug    = "cascade-node-go"
	dartSlug  = "cascade-node-dart"
	runTicks  = 3
	runPhases = 4
)

var roleOrder = []string{"D", "C", "B", "A"}

type roleSpec struct {
	slug       string
	binaryPath string
}

type roleRuntime struct {
	role          string
	uid           string
	slug          string
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
	liveStream := flag.Bool("live-stream", false, "exercise long-lived Follow:true streams across chain respawns")
	multiPattern := flag.Bool("multi-pattern", false, "exercise go/dart relay cascade patterns using live streams")
	flag.Parse()

	var err error
	if *multiPattern {
		err = runMultiPattern()
	} else if *liveStream {
		err = runLiveStream()
	} else {
		err = run()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nFAIL: %v\n", err)
		os.Exit(1)
	}
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

func runLiveStream() error {
	binaryPath, err := findCascadeNodeBinary()
	if err != nil {
		return err
	}
	runRoot := filepath.Join(os.Getenv("HOME"), ".op", "run")
	transports := []string{"tcp", "unix", "tcp", "unix"}

	fmt.Println("=== relay-cascade-go --live-stream ===")
	fmt.Println()
	fmt.Println("Setup: opening long-lived Follow:true streams on A")
	fmt.Println("       (initial transport: tcp, port 9090)")
	fmt.Println()

	totalPass := 0
	totalFail := 0
	var c *cascade
	var streams *liveStreams
	var streamOpenErr error
	defer func() {
		if c != nil {
			c.stop()
		}
		if streams != nil {
			streams.stop()
		}
	}()

	for phaseIdx, transport := range transports {
		phaseNo := phaseIdx + 1
		if phaseNo == 1 {
			fmt.Printf("Phase %d/%d: initial chain (%s)\n", phaseNo, runPhases, transport)
		} else {
			fmt.Printf("Phase %d/%d: respawn on %s\n", phaseNo, runPhases, transport)
			killStart := time.Now()
			if c != nil {
				c.stop()
			}
			if streams != nil {
				streams.stop()
			}
			fmt.Printf("  killed 4 nodes in %s\n", time.Since(killStart).Round(time.Millisecond))
		}

		spawnStart := time.Now()
		c, err = spawnCascade(phaseNo, transport, binaryPath, runRoot)
		if err != nil {
			totalFail += runTicks
			fmt.Printf("  spawn FAIL: %v\n\n", err)
			streams = nil
			streamOpenErr = err
			continue
		}
		fmt.Printf("  spawned 4 nodes in %s\n", time.Since(spawnStart).Round(time.Millisecond))
		if phaseNo > 1 {
			fmt.Println("  re-opening Follow:true streams on new A")
		}
		streams, streamOpenErr = startLiveStreams(c.roles["A"].conn)
		if streamOpenErr != nil {
			fmt.Printf("  stream re-open failed: %v\n", streamOpenErr)
		}

		previousMetric := float64(0)
		for tick := 1; tick <= runTicks; tick++ {
			tickStart := time.Now()
			result := c.runLiveTick(streams, streamOpenErr, tick, previousMetric)
			if result.metric.pass {
				previousMetric = result.metricValue
			}
			overall := result.log.pass && result.event.pass && result.metric.pass
			if overall {
				totalPass++
			} else {
				totalFail++
			}
			fmt.Printf("  Tick %d/%d: log %s, event %s, metric %s (overall %s in %s)\n",
				tick,
				runTicks,
				passText(result.log.pass),
				passText(result.event.pass),
				passText(result.metric.pass),
				passText(overall),
				time.Since(tickStart).Round(time.Millisecond),
			)
			if !overall {
				printFailureEvidence("log", result.log)
				printFailureEvidence("event", result.event)
				printFailureEvidence("metric", result.metric)
			}
		}
		fmt.Println()
	}

	fmt.Printf("Summary: %d PASS / %d FAIL across %d ticks\n", totalPass, totalFail, totalPass+totalFail)
	if totalFail > 0 {
		return fmt.Errorf("%d tick(s) failed", totalFail)
	}
	return nil
}

type cascadePattern struct {
	name  string
	roles map[string]roleSpec
}

func runMultiPattern() error {
	goBinary, err := findHolonBinary(goSlug)
	if err != nil {
		return err
	}
	dartBinary, err := findHolonBinary(dartSlug)
	if err != nil {
		return err
	}
	patterns := []cascadePattern{
		{
			name: "go-go-go-go",
			roles: map[string]roleSpec{
				"A": {slug: goSlug, binaryPath: goBinary},
				"B": {slug: goSlug, binaryPath: goBinary},
				"C": {slug: goSlug, binaryPath: goBinary},
				"D": {slug: goSlug, binaryPath: goBinary},
			},
		},
		{
			name: "go-go-dart-go",
			roles: map[string]roleSpec{
				"A": {slug: goSlug, binaryPath: goBinary},
				"B": {slug: goSlug, binaryPath: goBinary},
				"C": {slug: dartSlug, binaryPath: dartBinary},
				"D": {slug: goSlug, binaryPath: goBinary},
			},
		},
		{
			name: "go-go-dart-dart",
			roles: map[string]roleSpec{
				"A": {slug: goSlug, binaryPath: goBinary},
				"B": {slug: goSlug, binaryPath: goBinary},
				"C": {slug: dartSlug, binaryPath: dartBinary},
				"D": {slug: dartSlug, binaryPath: dartBinary},
			},
		},
	}
	runRoot := filepath.Join(os.Getenv("HOME"), ".op", "run")
	transports := []string{"tcp", "unix", "tcp", "unix"}

	fmt.Println("=== relay-cascade-go (multi-pattern) ===")
	fmt.Println()

	totalPass := 0
	totalFail := 0
	for patternIdx, pattern := range patterns {
		fmt.Printf("Pattern %d/%d: %s\n", patternIdx+1, len(patterns), pattern.name)
		patternPass := 0
		patternFail := 0
		for phaseIdx, transport := range transports {
			phaseNo := phaseIdx + 1
			spawnStart := time.Now()
			c, err := spawnPatternCascade(phaseNo, transport, pattern.roles, runRoot)
			if err != nil {
				patternFail += runTicks
				totalFail += runTicks
				fmt.Printf("  Phase %d/%d (%s): spawn FAIL (%v)\n", phaseNo, runPhases, transport, err)
				continue
			}
			streams, streamOpenErr := startLiveStreams(c.roles["A"].conn)
			if streamOpenErr == nil {
				ready := waitForEvery(5*time.Second, 50*time.Millisecond, func() checkResult {
					return c.checkLiveEvent(streams)
				})
				if !ready.pass {
					streamOpenErr = fmt.Errorf("live relay readiness: %s", ready.evidence)
				}
			}

			previousMetric := float64(0)
			results := make([]string, 0, runTicks)
			evidence := make([]string, 0)
			for tick := 1; tick <= runTicks; tick++ {
				sender := fmt.Sprintf("%s-phase-%d-tick-%d", pattern.name, phaseNo, tick)
				result := c.runLiveTickWithSender(streams, streamOpenErr, sender, previousMetric)
				if result.metric.pass {
					previousMetric = result.metricValue
				}
				overall := result.log.pass && result.event.pass && result.metric.pass
				if overall {
					patternPass++
					totalPass++
					results = append(results, fmt.Sprintf("Tick %d PASS", tick))
				} else {
					patternFail++
					totalFail++
					summary := failureSummary(result)
					results = append(results, fmt.Sprintf("Tick %d FAIL (%s)", tick, summary))
					evidence = append(evidence, fmt.Sprintf("      Tick %d evidence: %s", tick, compactEvidence(result)))
				}
			}
			fmt.Printf("  Phase %d/%d (%s): %s (spawned in %s)\n",
				phaseNo,
				runPhases,
				transport,
				strings.Join(results, ", "),
				time.Since(spawnStart).Round(time.Millisecond),
			)
			for _, line := range evidence {
				fmt.Println(line)
			}
			if streams != nil {
				streams.stop()
			}
			c.stop()
		}
		fmt.Printf("  Subtotal: %d/12 PASS\n\n", patternPass)
	}

	fmt.Printf("Summary: %d PASS / %d FAIL across %d ticks\n", totalPass, totalFail, totalPass+totalFail)
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
	return c.runTickWithSender(sender, previousMetric)
}

func (c *cascade) runTickWithSender(sender string, previousMetric float64) tickOutcome {
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

func (c *cascade) runLiveTick(streams *liveStreams, streamOpenErr error, tick int, previousMetric float64) tickOutcome {
	sender := fmt.Sprintf("phase-%d-tick-%d", c.phase, tick)
	return c.runLiveTickWithSender(streams, streamOpenErr, sender, previousMetric)
}

func (c *cascade) runLiveTickWithSender(streams *liveStreams, streamOpenErr error, sender string, previousMetric float64) tickOutcome {
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

	logResult := checkResult{evidence: "stream re-open failed: " + errorText(streamOpenErr)}
	eventResult := logResult
	if streamOpenErr == nil {
		logResult = waitForEvery(time.Second, 50*time.Millisecond, func() checkResult {
			return c.checkLiveLog(streams, sender)
		})
		eventResult = waitForEvery(time.Second, 50*time.Millisecond, func() checkResult {
			return c.checkLiveEvent(streams)
		})
	}
	metricValue := previousMetric
	metricResult := waitForEvery(time.Second, 50*time.Millisecond, func() checkResult {
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
	specs := map[string]roleSpec{
		"A": {slug: goSlug, binaryPath: binaryPath},
		"B": {slug: goSlug, binaryPath: binaryPath},
		"C": {slug: goSlug, binaryPath: binaryPath},
		"D": {slug: goSlug, binaryPath: binaryPath},
	}
	return spawnPatternCascade(phase, transportName, specs, runRoot)
}

func spawnPatternCascade(phase int, transportName string, specs map[string]roleSpec, runRoot string) (*cascade, error) {
	c := &cascade{
		phase:     phase,
		transport: transportName,
		runRoot:   runRoot,
		roles:     make(map[string]*roleRuntime, len(roleOrder)),
	}
	for _, role := range roleOrder {
		spec, ok := specs[role]
		if !ok || spec.slug == "" || spec.binaryPath == "" {
			return nil, fmt.Errorf("missing role spec for %s", role)
		}
		r := newRoleRuntime(phase, transportName, role, spec)
		c.roles[role] = r
		if err := os.RemoveAll(filepath.Join(runRoot, r.slug, r.uid)); err != nil {
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

func newRoleRuntime(phase int, transportName, role string, spec roleSpec) *roleRuntime {
	lower := strings.ToLower(role)
	r := &roleRuntime{
		role:       role,
		uid:        fmt.Sprintf("relay-p%02d-%s", phase, lower),
		slug:       spec.slug,
		binaryPath: spec.binaryPath,
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
		child := c.roles[childRole(r.role)]
		args = append(args, "--member", child.slug+"="+r.memberAddress)
	}
	cmd := exec.Command(r.binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"OP_OBS=logs,events,metrics,prom",
		"OP_RUN_DIR="+c.runRoot,
		"OP_INSTANCE_UID="+r.uid,
		"OP_ORGANISM_UID="+c.roles["A"].uid,
		"OP_ORGANISM_SLUG="+c.roles["A"].slug,
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

	meta, err := waitMeta(c.runRoot, r.slug, r.uid, 10*time.Second)
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

func childRole(role string) string {
	switch role {
	case "A":
		return "B"
	case "B":
		return "C"
	case "C":
		return "D"
	default:
		return ""
	}
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

func (c *cascade) checkLiveLog(streams *liveStreams, sender string) checkResult {
	if streams == nil {
		return checkResult{evidence: "live streams are not open"}
	}
	entries := streams.logEntries()
	for _, entry := range entries {
		if entry.GetMessage() != "tick received" {
			continue
		}
		if entry.GetFields()["sender"] != sender || entry.GetFields()["responder_uid"] != c.roles["D"].uid {
			continue
		}
		if err := c.checkChain(entry.GetChain()); err != nil {
			return checkResult{evidence: "matching live log has bad chain: " + err.Error() + " entry=" + marshalProto(entry)}
		}
		return checkResult{pass: true, evidence: marshalProto(entry)}
	}
	return checkResult{evidence: fmt.Sprintf("no live log found for sender=%s current_d_uid=%s within 1s (buffer=%d, stream_errors=%v)", sender, c.roles["D"].uid, len(entries), streams.streamErrors())}
}

func (c *cascade) checkLiveEvent(streams *liveStreams) checkResult {
	if streams == nil {
		return checkResult{evidence: "live streams are not open"}
	}
	events := streams.eventEntries()
	for _, event := range events {
		if event.GetType() != holonsv1.EventType_INSTANCE_READY || event.GetInstanceUid() != c.roles["D"].uid {
			continue
		}
		if err := c.checkChain(event.GetChain()); err != nil {
			return checkResult{evidence: "matching live event has bad chain: " + err.Error() + " event=" + marshalProto(event)}
		}
		return checkResult{pass: true, evidence: marshalProto(event)}
	}
	return checkResult{evidence: fmt.Sprintf("no live INSTANCE_READY event found for current_d_uid=%s within 1s (buffer=%d, stream_errors=%v)", c.roles["D"].uid, len(events), streams.streamErrors())}
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
	wantRoles := []string{"D", "C", "B"}
	if len(chain) < len(wantRoles) {
		return fmt.Errorf("chain length %d < %d", len(chain), len(wantRoles))
	}
	for i, role := range wantRoles {
		want := c.roles[role]
		if chain[i].GetSlug() != want.slug || chain[i].GetInstanceUid() != want.uid {
			return fmt.Errorf("hop %d = %s/%s, want %s/%s", i, chain[i].GetSlug(), chain[i].GetInstanceUid(), want.slug, want.uid)
		}
	}
	return nil
}

type liveStreams struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	logs   []*holonsv1.LogEntry
	events []*holonsv1.EventInfo
	errs   []string
}

func startLiveStreams(conn *grpc.ClientConn) (*liveStreams, error) {
	ctx, cancel := context.WithCancel(context.Background())
	client := holonsv1.NewHolonObservabilityClient(conn)
	logStream, err := client.Logs(ctx, &holonsv1.LogsRequest{
		MinLevel: holonsv1.LogLevel_INFO,
		Follow:   true,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	eventStream, err := client.Events(ctx, &holonsv1.EventsRequest{Follow: true})
	if err != nil {
		cancel()
		return nil, err
	}
	streams := &liveStreams{cancel: cancel}
	streams.wg.Add(2)
	go streams.readLogs(ctx, logStream)
	go streams.readEvents(ctx, eventStream)
	return streams, nil
}

func (s *liveStreams) stop() {
	if s == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
}

func (s *liveStreams) readLogs(ctx context.Context, stream holonsv1.HolonObservability_LogsClient) {
	defer s.wg.Done()
	for {
		entry, err := stream.Recv()
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, io.EOF) {
				s.addErr("logs stream ended: " + err.Error())
			}
			return
		}
		s.mu.Lock()
		s.logs = append(s.logs, entry)
		s.mu.Unlock()
	}
}

func (s *liveStreams) readEvents(ctx context.Context, stream holonsv1.HolonObservability_EventsClient) {
	defer s.wg.Done()
	for {
		event, err := stream.Recv()
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, io.EOF) {
				s.addErr("events stream ended: " + err.Error())
			}
			return
		}
		s.mu.Lock()
		s.events = append(s.events, event)
		s.mu.Unlock()
	}
}

func (s *liveStreams) logEntries() []*holonsv1.LogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*holonsv1.LogEntry(nil), s.logs...)
}

func (s *liveStreams) eventEntries() []*holonsv1.EventInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*holonsv1.EventInfo(nil), s.events...)
}

func (s *liveStreams) addErr(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = append(s.errs, message)
}

func (s *liveStreams) streamErrors() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.errs...)
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
	return waitForEvery(timeout, 100*time.Millisecond, fn)
}

func waitForEvery(timeout, interval time.Duration, fn func() checkResult) checkResult {
	deadline := time.Now().Add(timeout)
	var last checkResult
	for {
		last = fn()
		if last.pass || time.Now().After(deadline) {
			return last
		}
		time.Sleep(interval)
	}
}

func waitMeta(runRoot, slug, uid string, timeout time.Duration) (observability.MetaJSON, error) {
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
	return findHolonBinary(goSlug)
}

func findHolonBinary(slug string) (string, error) {
	envName := "CASCADE_NODE_" + strings.ToUpper(strings.TrimPrefix(slug, "cascade-node-")) + "_BIN"
	if override := strings.TrimSpace(os.Getenv(envName)); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, ".op", "bin", slug+".holon", "bin")
	var found string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || d.Name() != slug {
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
		return "", fmt.Errorf("%s binary not found under %s; run op build %s --install", slug, root, slug)
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

func failureSummary(result tickOutcome) string {
	families := make([]string, 0, 3)
	if !result.log.pass {
		families = append(families, "log family")
	}
	if !result.event.pass {
		families = append(families, "event family")
	}
	if !result.metric.pass {
		families = append(families, "metric family")
	}
	if len(families) == 0 {
		return "unknown"
	}
	return strings.Join(families, ", ")
}

func compactEvidence(result tickOutcome) string {
	parts := make([]string, 0, 3)
	if !result.log.pass {
		parts = append(parts, "log="+truncateEvidence(result.log.evidence))
	}
	if !result.event.pass {
		parts = append(parts, "event="+truncateEvidence(result.event.evidence))
	}
	if !result.metric.pass {
		parts = append(parts, "metric="+truncateEvidence(result.metric.evidence))
	}
	return strings.Join(parts, "; ")
}

func truncateEvidence(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "<empty>"
	}
	if len(value) <= 240 {
		return value
	}
	return value[:240] + "..."
}

func errorText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

func marshalProto(msg proto.Message) string {
	b, _ := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(msg)
	return string(b)
}
