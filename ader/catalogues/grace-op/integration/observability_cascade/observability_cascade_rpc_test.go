//go:build e2e

package observability_cascade_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ocv1 "observability-cascade-go/gen/go/observability_cascade/v1"
)

type cascadeReport struct {
	Ticks int `json:"ticks"`
	Pass  int `json:"pass"`
	Fail  int `json:"fail"`
}

const (
	defaultCascadeTicks  = 30
	cascadeChainHopCount = 3
)

type typedCascadeSample struct {
	Event typedRecordSummary
}

type typedRecordSummary struct {
	Body              string
	BodyBranch        string
	SeverityNumber    holonsv1.SeverityNumber
	SeverityText      string
	AttributeKeys     []string
	AttributeBranches map[string]string
	ChainLen          int
	ChainMatchesSlug  bool
}

func TestObservabilityCascade_RPCMatrix(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)
	if runtime.GOOS != "darwin" {
		t.Skip("observability-cascade composite validation currently targets macOS hosts")
	}

	sb := integration.NewSandbox(t)
	typedSamples := make(map[string]typedCascadeSample)
	for _, lang := range selectedCascadeLanguages(t) {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			t.Cleanup(func() { cleanupResidualCascadeProcesses(t) })
			slug := "observability-cascade-" + lang
			build := sb.RunOPWithOptions(t, integration.RunOptions{Timeout: 30 * time.Minute}, "build", slug, "--install")
			integration.RequireSuccess(t, build)

			assertCascadeReport(t, sb, slug, "RunDefault", defaultCascadeTicks)
			cleanupResidualCascadeProcesses(t)
			assertCascadeReport(t, sb, slug, "RunLiveStream", defaultCascadeTicks)
			cleanupResidualCascadeProcesses(t)
			typedSamples[lang] = collectTypedCascadeSample(t, sb, slug)
			cleanupResidualCascadeProcesses(t)
		})
	}
	if t.Failed() {
		return
	}
	assertTypedCascadeSamplesEquivalent(t, typedSamples)
}

func selectedCascadeLanguages(t *testing.T) []string {
	t.Helper()
	languages := []string{
		"go",
		"rust",
		"dart",
		"python",
		"ruby",
		"node",
		"java",
		"kotlin",
		"csharp",
		"swift",
		"c",
		"cpp",
		"zig",
	}
	filter := strings.TrimSpace(os.Getenv("OBSERVABILITY_CASCADE_LANG"))
	if filter == "" || filter == "all" {
		return languages
	}
	for _, lang := range languages {
		if filter == lang || filter == "observability-cascade-"+lang {
			return []string{lang}
		}
	}
	t.Fatalf("unknown OBSERVABILITY_CASCADE_LANG=%q", filter)
	return nil
}

func assertCascadeReport(t *testing.T, sb *integration.Sandbox, slug, method string, expectedTicks int) {
	t.Helper()

	result := sb.RunOPWithOptions(t, integration.RunOptions{Timeout: 15 * time.Minute}, "invoke", slug, method, "{}", "-f", "json")
	integration.RequireSuccess(t, result)
	report := integration.DecodeJSON[cascadeReport](t, result.Stdout)
	if report.Ticks != expectedTicks || report.Pass != report.Ticks || report.Fail != 0 {
		t.Fatalf("%s %s = %+v, want pass == ticks == %d and fail == 0", slug, method, report, expectedTicks)
	}
}

func collectTypedCascadeSample(t *testing.T, sb *integration.Sandbox, slug string) typedCascadeSample {
	t.Helper()

	process, address := startObservedCascadeServer(t, sb, slug)
	defer process.Stop(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := grpcclient.Dial(ctx, strings.TrimPrefix(address, "tcp://"))
	if err != nil {
		t.Fatalf("%s dial observed cascade: %v", slug, err)
	}
	defer conn.Close()

	obs := holonsv1.NewHolonObservabilityClient(conn)
	events := startEventRecordCollector(t, obs)

	runCtx, runCancel := context.WithTimeout(context.Background(), 15*time.Minute)
	report, err := ocv1.NewObservabilityCascadeServiceClient(conn).RunDefault(runCtx, &ocv1.RunRequest{})
	runCancel()
	if err != nil {
		t.Fatalf("%s RunDefault via typed client: %v\nprocess output:\n%s", slug, err, process.Combined())
	}
	if report.GetPass() <= 0 || report.GetFail() != 0 || report.GetPass() != report.GetTicks() {
		t.Fatalf("%s observed RunDefault = %+v, want all ticks passing", slug, report)
	}
	time.Sleep(500 * time.Millisecond)

	return typedCascadeSample{
		Event: summarizeRecord(t, slug, "event", findRelayedReadyEvent(t, slug, events.stop(t, "events"))),
	}
}

func cleanupResidualCascadeProcesses(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		return
	}
	scope := integration.DefaultWorkspaceDir(t)
	pids := residualCascadePIDs(t, scope)
	if len(pids) == 0 {
		return
	}
	signalProcesses(t, pids, syscall.SIGTERM)
	time.Sleep(500 * time.Millisecond)
	remaining := residualCascadePIDs(t, scope)
	if len(remaining) > 0 {
		signalProcesses(t, remaining, syscall.SIGKILL)
	}
	t.Logf("cleaned %d residual observability-cascade process(es)", len(pids))
}

func residualCascadePIDs(t *testing.T, scope string) []int {
	t.Helper()
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		t.Fatalf("list processes for residual cascade cleanup: %v", err)
	}
	var pids []int
	self := os.Getpid()
	scopes := processScopeAliases(scope)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !containsAny(line, scopes) || !strings.Contains(line, "observability-cascade-") {
			continue
		}
		pidField, _, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(pidField))
		if err != nil || pid == self {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

func processScopeAliases(scope string) []string {
	scope = filepath.Clean(strings.TrimSpace(scope))
	if scope == "." || scope == "" {
		return nil
	}
	aliases := []string{scope}
	if resolved, err := filepath.EvalSymlinks(scope); err == nil && resolved != scope {
		aliases = append(aliases, resolved)
	}
	if strings.HasPrefix(scope, "/var/") {
		aliases = append(aliases, "/private"+scope)
	}
	if strings.HasPrefix(scope, "/private/var/") {
		aliases = append(aliases, strings.TrimPrefix(scope, "/private"))
	}
	return slices.Compact(aliases)
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func signalProcesses(t *testing.T, pids []int, signal syscall.Signal) {
	t.Helper()
	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		_ = process.Signal(signal)
	}
}

func startObservedCascadeServer(t *testing.T, sb *integration.Sandbox, slug string) (*integration.ProcessHandle, string) {
	t.Helper()
	binary := sb.RunOPWithOptions(t, integration.RunOptions{Timeout: 15 * time.Minute}, "--bin", slug)
	integration.RequireSuccess(t, binary)
	path := strings.TrimSpace(binary.Stdout)
	if path == "" {
		t.Fatalf("%s --bin returned an empty path", slug)
	}
	process := sb.StartProcess(
		t,
		integration.RunOptions{
			BinaryPath:       path,
			Env:              []string{"OP_OBS=logs,events,metrics,prom"},
			SkipDiscoverRoot: true,
		},
		"serve",
		"--listen",
		"tcp://127.0.0.1:0",
	)
	return process, process.WaitForListenAddress(t, integration.ProcessStartTimeout)
}

type recordStream interface {
	Recv() (*holonsv1.LogRecord, error)
}

type recordCollector struct {
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.Mutex
	records []*holonsv1.LogRecord
	err     error
}

func startEventRecordCollector(t *testing.T, client holonsv1.HolonObservabilityClient) *recordCollector {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	stream, err := client.Events(ctx, &holonsv1.EventsRequest{EventNames: []string{"instance.ready"}, Follow: true})
	if err != nil {
		t.Fatalf("open events stream: %v", err)
	}
	return startRecordCollector(cancel, stream)
}

func startRecordCollector(cancel context.CancelFunc, stream recordStream) *recordCollector {
	collector := &recordCollector{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(collector.done)
		for {
			record, err := stream.Recv()
			if err == nil {
				collector.mu.Lock()
				collector.records = append(collector.records, record)
				collector.mu.Unlock()
				continue
			}
			if streamClosed(err) {
				return
			}
			collector.mu.Lock()
			collector.err = err
			collector.mu.Unlock()
			return
		}
	}()
	return collector
}

func (c *recordCollector) stop(t *testing.T, name string) []*holonsv1.LogRecord {
	t.Helper()
	c.cancel()
	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s stream did not stop after cancellation", name)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		t.Fatalf("read %s stream: %v", name, c.err)
	}
	return slices.Clone(c.records)
}

func streamClosed(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		status.Code(err) == codes.Canceled ||
		status.Code(err) == codes.DeadlineExceeded
}

func findRelayedReadyEvent(t *testing.T, slug string, records []*holonsv1.LogRecord) *holonsv1.LogRecord {
	t.Helper()
	for _, record := range records {
		if record.GetEventName() != "instance.ready" {
			continue
		}
		if anyValueBranch(record.GetBody()) == "string_value" && record.GetBody().GetStringValue() == "instance.ready" && len(record.GetChain()) == cascadeChainHopCount {
			return record
		}
	}
	t.Fatalf("%s root events did not include a relayed instance.ready LogRecord with a %d-hop chain; saw %s", slug, cascadeChainHopCount, debugRecords(records))
	return nil
}

func debugRecords(records []*holonsv1.LogRecord) string {
	limit := min(len(records), 5)
	parts := make([]string, 0, limit)
	for _, record := range records[:limit] {
		parts = append(parts, fmt.Sprintf("body=%q event=%q chain=%v attrs=%v", record.GetBody().GetStringValue(), record.GetEventName(), record.GetChain(), attributeBranchMap(record)))
	}
	return fmt.Sprintf("%d record(s): %s", len(records), strings.Join(parts, "; "))
}

func summarizeRecord(t *testing.T, slug, kind string, record *holonsv1.LogRecord) typedRecordSummary {
	t.Helper()
	bodyBranch := anyValueBranch(record.GetBody())
	if bodyBranch != "string_value" {
		t.Fatalf("%s %s body branch = %s, want string_value", slug, kind, bodyBranch)
	}
	branches := attributeBranchMap(record)
	keys := make([]string, 0, len(branches))
	for key := range branches {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	nodeSlug := slug + "-node"
	if !chainEntriesMatchSlug(record.GetChain(), nodeSlug) {
		t.Fatalf("%s %s chain = %v, want %d entries for %q", slug, kind, record.GetChain(), cascadeChainHopCount, nodeSlug)
	}
	return typedRecordSummary{
		Body:              record.GetBody().GetStringValue(),
		BodyBranch:        bodyBranch,
		SeverityNumber:    record.GetSeverityNumber(),
		SeverityText:      record.GetSeverityText(),
		AttributeKeys:     keys,
		AttributeBranches: branches,
		ChainLen:          len(record.GetChain()),
		ChainMatchesSlug:  true,
	}
}

func assertTypedCascadeSamplesEquivalent(t *testing.T, samples map[string]typedCascadeSample) {
	t.Helper()
	if len(samples) < 2 {
		t.Skipf("typed LogRecord equivalence needs at least two language samples, got %d", len(samples))
	}
	langs := make([]string, 0, len(samples))
	for lang := range samples {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	baselineLang := langs[0]
	baseline := samples[baselineLang]
	for _, lang := range langs[1:] {
		compareRecordSummary(t, baselineLang, lang, "event", baseline.Event, samples[lang].Event)
	}
}

func compareRecordSummary(t *testing.T, baselineLang, lang, kind string, want, got typedRecordSummary) {
	t.Helper()
	if want.BodyBranch != got.BodyBranch || want.Body != got.Body {
		t.Fatalf("%s %s body = %s(%q), want %s %s(%q)", lang, kind, got.BodyBranch, got.Body, baselineLang, want.BodyBranch, want.Body)
	}
	if want.SeverityNumber != got.SeverityNumber || want.SeverityText != got.SeverityText {
		t.Fatalf("%s %s severity = %s/%q, want %s %s/%q", lang, kind, got.SeverityNumber, got.SeverityText, baselineLang, want.SeverityNumber, want.SeverityText)
	}
	if !slices.Equal(want.AttributeKeys, got.AttributeKeys) {
		t.Fatalf("%s %s attribute keys = %v, want %s %v", lang, kind, got.AttributeKeys, baselineLang, want.AttributeKeys)
	}
	if !reflect.DeepEqual(want.AttributeBranches, got.AttributeBranches) {
		t.Fatalf("%s %s attribute branches = %v, want %s %v", lang, kind, got.AttributeBranches, baselineLang, want.AttributeBranches)
	}
	if want.ChainLen != got.ChainLen || want.ChainMatchesSlug != got.ChainMatchesSlug {
		t.Fatalf("%s %s chain summary = len %d matchesSlug=%v, want %s len %d matchesSlug=%v", lang, kind, got.ChainLen, got.ChainMatchesSlug, baselineLang, want.ChainLen, want.ChainMatchesSlug)
	}
}

func attributeBranchMap(record *holonsv1.LogRecord) map[string]string {
	out := make(map[string]string, len(record.GetAttributes()))
	for _, attr := range record.GetAttributes() {
		if attr == nil {
			continue
		}
		out[attr.GetKey()] = anyValueBranch(attr.GetValue())
	}
	return out
}

func anyValueBranch(value *holonsv1.AnyValue) string {
	if value == nil {
		return "<nil>"
	}
	switch value.GetValue().(type) {
	case *holonsv1.AnyValue_StringValue:
		return "string_value"
	case *holonsv1.AnyValue_BoolValue:
		return "bool_value"
	case *holonsv1.AnyValue_IntValue:
		return "int_value"
	case *holonsv1.AnyValue_DoubleValue:
		return "double_value"
	default:
		return fmt.Sprintf("%T", value.GetValue())
	}
}

func chainEntriesMatchSlug(chain []string, slug string) bool {
	if len(chain) != cascadeChainHopCount {
		return false
	}
	for _, hop := range chain {
		hopSlug, _, _ := strings.Cut(hop, "/")
		if strings.TrimSpace(hopSlug) != slug {
			return false
		}
	}
	return true
}
