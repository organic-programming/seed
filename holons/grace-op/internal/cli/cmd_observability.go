package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	sdkgrpcclient "github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/observability"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

// addObservabilityCommands wires `op ps`, `op logs`, `op metrics`, and
// `op events` into the root command.
func addObservabilityCommands(root *cobra.Command) {
	root.AddCommand(newPsCmd(), newLogsCmd(), newMetricsCmd(), newEventsCmd(), newInstancesCmd())
}

// --- op ps -------------------------------------------------------------------

func newPsCmd() *cobra.Command {
	var all, jsonOut, stale, flat bool
	var slugFilter string
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List running holon instances",
		Long: `Lists every holon instance that wrote a meta.json under .op/run/.
See INSTANCES.md §CLI. Columns: SLUG UID PID STARTED MODE TRANSPORT ADDRESS METRICS_ADDR.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := discoverInstances(slugFilter, all, stale)
			if err != nil {
				return err
			}
			if jsonOut {
				return renderInstancesJSON(items)
			}
			renderInstancesTable(items, flat)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Scan every known .op/run/ candidate (not just OP_RUN_DIR)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON objects instead of a table")
	cmd.Flags().BoolVar(&stale, "stale", false, "Include entries whose PID is no longer alive")
	cmd.Flags().BoolVar(&flat, "flat", false, "Render a flat list (default is tree-aware)")
	cmd.Flags().StringVar(&slugFilter, "slug", "", "Only list instances of this slug")
	return cmd
}

func newInstancesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instances [slug]",
		Short: "List instances of a specific holon (shortcut for op ps --slug <slug>)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := discoverInstances(args[0], false, false)
			if err != nil {
				return err
			}
			renderInstancesTable(items, false)
			return nil
		},
	}
	return cmd
}

type instanceRow struct {
	meta   observability.MetaJSON
	runDir string
	alive  bool
}

// --- op logs -----------------------------------------------------------------

func newLogsCmd() *cobra.Command {
	var since, level, session, method string
	var follow, jsonOut, all, localOnly bool
	var chainOrigin string
	cmd := &cobra.Command{
		Use:   "logs <slug>",
		Short: "Tail structured logs from a running holon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd.Context(), args[0], logsOptions{
				Since: since, Level: level, Session: session, Method: method,
				Follow: follow, JSON: jsonOut, All: all, Local: localOnly,
				ChainOrigin: chainOrigin,
			})
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Replay entries newer than this duration (e.g. 5m)")
	cmd.Flags().StringVar(&level, "level", "info", "Minimum level (trace|debug|info|warn|error|fatal)")
	cmd.Flags().StringVar(&session, "session", "", "Filter to one session id")
	cmd.Flags().StringVar(&method, "method", "", "Filter to one RPC method")
	cmd.Flags().StringVar(&chainOrigin, "chain-origin", "", "Filter to signals originated in this slug")
	cmd.Flags().BoolVar(&follow, "follow", true, "Stream live entries (default true); --follow=false drains and exits")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit one JSON object per line")
	cmd.Flags().BoolVar(&all, "all", false, "Aggregate across every running instance of the slug")
	cmd.Flags().BoolVar(&localOnly, "local", false, "Bypass the multilog (organism roots only): show only the root's own entries")
	return cmd
}

type logsOptions struct {
	Since, Level, Session, Method, ChainOrigin string
	Follow, JSON, All, Local                   bool
}

func runLogs(ctx context.Context, slug string, opts logsOptions) error {
	instances, err := discoverInstances(slug, opts.All, false)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		return fmt.Errorf("op logs: no running instance for %q (run `op ps` to list)", slug)
	}
	// For simplicity in v1, tail the first instance. --all aggregates
	// across streams in a follow-up; ader-suite-driven.
	inst := instances[0]
	conn, err := dialInstance(ctx, inst)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := v1.NewHolonObservabilityClient(conn)

	req := &v1.LogsRequest{
		MinLevel: parseLogLevel(opts.Level),
		Follow:   opts.Follow,
	}
	if opts.Session != "" {
		req.SessionIds = []string{opts.Session}
	}
	if opts.Method != "" {
		req.RpcMethods = []string{opts.Method}
	}
	if opts.Since != "" {
		if d, err := time.ParseDuration(opts.Since); err == nil {
			req.Since = durationpb.New(d)
		}
	}
	stream, err := client.Logs(ctx, req)
	if err != nil {
		return fmt.Errorf("op logs: %w", err)
	}
	for {
		entry, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if opts.ChainOrigin != "" && entry.Slug != opts.ChainOrigin {
			continue
		}
		renderLogEntry(entry, opts.JSON)
	}
}

func parseLogLevel(s string) v1.LogLevel {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE":
		return v1.LogLevel_TRACE
	case "DEBUG":
		return v1.LogLevel_DEBUG
	case "INFO":
		return v1.LogLevel_INFO
	case "WARN", "WARNING":
		return v1.LogLevel_WARN
	case "ERROR":
		return v1.LogLevel_ERROR
	case "FATAL":
		return v1.LogLevel_FATAL
	default:
		return v1.LogLevel_INFO
	}
}

func renderLogEntry(e *v1.LogEntry, jsonOut bool) {
	if jsonOut {
		b, _ := json.Marshal(logEntryJSON(e))
		fmt.Println(string(b))
		return
	}
	ts := ""
	if e.Ts != nil {
		ts = e.Ts.AsTime().UTC().Format(time.RFC3339Nano)
	}
	fmt.Printf("%s %-5s slug=%s instance_uid=%s", ts, logLevelLabel(e.Level), e.Slug, e.InstanceUid)
	if e.SessionId != "" {
		fmt.Printf(" session_id=%s", e.SessionId)
	}
	if e.RpcMethod != "" {
		fmt.Printf(" method=%s", e.RpcMethod)
	}
	if chain := formatChain(e.Chain); chain != "" {
		fmt.Printf(" chain=%s", chain)
	}
	fmt.Printf(" msg=%q", e.Message)
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf(" %s=%q", k, e.Fields[k])
	}
	fmt.Println()
}

func logEntryJSON(e *v1.LogEntry) map[string]any {
	out := map[string]any{
		"kind":         "log",
		"level":        logLevelLabel(e.Level),
		"slug":         e.Slug,
		"instance_uid": e.InstanceUid,
		"message":      e.Message,
	}
	if e.Ts != nil {
		out["ts"] = e.Ts.AsTime().UTC().Format(time.RFC3339Nano)
	}
	if e.SessionId != "" {
		out["session_id"] = e.SessionId
	}
	if e.RpcMethod != "" {
		out["rpc_method"] = e.RpcMethod
	}
	if len(e.Fields) > 0 {
		out["fields"] = e.Fields
	}
	if e.Caller != "" {
		out["caller"] = e.Caller
	}
	if len(e.Chain) > 0 {
		out["chain"] = chainHopsJSON(e.Chain)
	}
	return out
}

func chainHopsJSON(chain []*v1.ChainHop) []map[string]string {
	hops := make([]map[string]string, len(chain))
	for i, h := range chain {
		if h == nil {
			hops[i] = map[string]string{}
			continue
		}
		hops[i] = map[string]string{"slug": h.Slug, "instance_uid": h.InstanceUid}
	}
	return hops
}

func formatChain(chain []*v1.ChainHop) string {
	if len(chain) == 0 {
		return ""
	}
	parts := make([]string, 0, len(chain))
	for _, h := range chain {
		if h == nil || h.Slug == "" {
			continue
		}
		if h.InstanceUid == "" {
			parts = append(parts, h.Slug)
			continue
		}
		parts = append(parts, h.Slug+"/"+h.InstanceUid)
	}
	return strings.Join(parts, ">")
}

func logLevelLabel(l v1.LogLevel) string {
	switch l {
	case v1.LogLevel_TRACE:
		return "TRACE"
	case v1.LogLevel_DEBUG:
		return "DEBUG"
	case v1.LogLevel_INFO:
		return "INFO"
	case v1.LogLevel_WARN:
		return "WARN"
	case v1.LogLevel_ERROR:
		return "ERROR"
	case v1.LogLevel_FATAL:
		return "FATAL"
	default:
		return "UNSPECIFIED"
	}
}

// --- op metrics --------------------------------------------------------------

func newMetricsCmd() *cobra.Command {
	var promOut, jsonOut, sessionRollup bool
	var prefix string
	cmd := &cobra.Command{
		Use:   "metrics <slug>",
		Short: "Fetch a metrics snapshot from a running holon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMetrics(cmd.Context(), args[0], metricsOptions{
				Prom: promOut, JSON: jsonOut, Prefix: prefix, Rollup: sessionRollup,
			})
		},
	}
	cmd.Flags().BoolVar(&promOut, "prom", false, "Emit Prometheus text exposition format")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit the snapshot as JSON")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Filter metric names by prefix")
	cmd.Flags().BoolVar(&sessionRollup, "include-session-rollup", false, "Include session metrics rollup")
	return cmd
}

type metricsOptions struct {
	Prom, JSON, Rollup bool
	Prefix             string
}

func runMetrics(ctx context.Context, slug string, opts metricsOptions) error {
	instances, err := discoverInstances(slug, false, false)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		return fmt.Errorf("op metrics: no running instance for %q", slug)
	}
	inst := instances[0]
	conn, err := dialInstance(ctx, inst)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := v1.NewHolonObservabilityClient(conn)
	req := &v1.MetricsRequest{IncludeSessionRollup: opts.Rollup}
	if opts.Prefix != "" {
		req.NamePrefixes = []string{opts.Prefix}
	}
	snap, err := client.Metrics(ctx, req)
	if err != nil {
		return fmt.Errorf("op metrics: %w", err)
	}
	if opts.JSON {
		b, _ := json.MarshalIndent(snap, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	if opts.Prom {
		renderMetricsProm(snap)
		return nil
	}
	renderMetricsTable(snap)
	return nil
}

func renderMetricsTable(snap *v1.MetricsSnapshot) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "METRIC\tTYPE\tVALUE")
	for _, s := range snap.Samples {
		name := s.Name + renderLabels(s.Labels)
		switch v := s.Value.(type) {
		case *v1.MetricSample_Counter:
			fmt.Fprintf(w, "%s\tcounter\t%d\n", name, v.Counter)
		case *v1.MetricSample_Gauge:
			fmt.Fprintf(w, "%s\tgauge\t%g\n", name, v.Gauge)
		case *v1.MetricSample_Histogram:
			if v.Histogram != nil {
				fmt.Fprintf(w, "%s\thistogram\tcount=%d sum=%g\n", name, v.Histogram.Count, v.Histogram.Sum)
			}
		}
	}
	_ = w.Flush()
}

func renderMetricsProm(snap *v1.MetricsSnapshot) {
	for _, s := range snap.Samples {
		switch v := s.Value.(type) {
		case *v1.MetricSample_Counter:
			fmt.Printf("# TYPE %s counter\n", s.Name)
			fmt.Printf("%s%s %d\n", s.Name, renderLabels(s.Labels), v.Counter)
		case *v1.MetricSample_Gauge:
			fmt.Printf("# TYPE %s gauge\n", s.Name)
			fmt.Printf("%s%s %g\n", s.Name, renderLabels(s.Labels), v.Gauge)
		case *v1.MetricSample_Histogram:
			if v.Histogram == nil {
				continue
			}
			fmt.Printf("# TYPE %s histogram\n", s.Name)
			for _, b := range v.Histogram.Buckets {
				labels := mergeMap(s.Labels, map[string]string{"le": fmt.Sprintf("%g", b.UpperBound)})
				fmt.Printf("%s_bucket%s %d\n", s.Name, renderLabels(labels), b.Count)
			}
			inf := mergeMap(s.Labels, map[string]string{"le": "+Inf"})
			fmt.Printf("%s_bucket%s %d\n", s.Name, renderLabels(inf), v.Histogram.Count)
			fmt.Printf("%s_sum%s %g\n", s.Name, renderLabels(s.Labels), v.Histogram.Sum)
			fmt.Printf("%s_count%s %d\n", s.Name, renderLabels(s.Labels), v.Histogram.Count)
		}
	}
}

func renderLabels(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(m[k])
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

func mergeMap(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// --- op events ---------------------------------------------------------------

func newEventsCmd() *cobra.Command {
	var follow, jsonOut bool
	var since string
	var types []string
	cmd := &cobra.Command{
		Use:   "events <slug>",
		Short: "Stream lifecycle events from a running holon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEvents(cmd.Context(), args[0], eventsOptions{
				Follow: follow, JSON: jsonOut, Since: since, Types: types,
			})
		},
	}
	cmd.Flags().BoolVar(&follow, "follow", true, "Stream new events")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit one JSON object per line")
	cmd.Flags().StringVar(&since, "since", "", "Replay events newer than this duration")
	cmd.Flags().StringSliceVar(&types, "type", nil, "Filter to these event types (repeatable)")
	return cmd
}

type eventsOptions struct {
	Follow, JSON bool
	Since        string
	Types        []string
}

func runEvents(ctx context.Context, slug string, opts eventsOptions) error {
	instances, err := discoverInstances(slug, false, false)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		return fmt.Errorf("op events: no running instance for %q", slug)
	}
	inst := instances[0]
	conn, err := dialInstance(ctx, inst)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := v1.NewHolonObservabilityClient(conn)
	req := &v1.EventsRequest{Follow: opts.Follow}
	if opts.Since != "" {
		if d, err := time.ParseDuration(opts.Since); err == nil {
			req.Since = durationpb.New(d)
		}
	}
	for _, t := range opts.Types {
		if parsed, ok := parseEventType(t); ok {
			req.Types = append(req.Types, parsed)
		}
	}
	stream, err := client.Events(ctx, req)
	if err != nil {
		return fmt.Errorf("op events: %w", err)
	}
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		renderEvent(ev, opts.JSON)
	}
}

func parseEventType(s string) (v1.EventType, bool) {
	switch strings.ToUpper(strings.ReplaceAll(s, "-", "_")) {
	case "INSTANCE_SPAWNED":
		return v1.EventType_INSTANCE_SPAWNED, true
	case "INSTANCE_READY":
		return v1.EventType_INSTANCE_READY, true
	case "INSTANCE_EXITED":
		return v1.EventType_INSTANCE_EXITED, true
	case "INSTANCE_CRASHED":
		return v1.EventType_INSTANCE_CRASHED, true
	case "SESSION_STARTED":
		return v1.EventType_SESSION_STARTED, true
	case "SESSION_ENDED":
		return v1.EventType_SESSION_ENDED, true
	case "HANDLER_PANIC":
		return v1.EventType_HANDLER_PANIC, true
	case "CONFIG_RELOADED":
		return v1.EventType_CONFIG_RELOADED, true
	}
	return v1.EventType_EVENT_TYPE_UNSPECIFIED, false
}

func renderEvent(e *v1.EventInfo, jsonOut bool) {
	if jsonOut {
		out := map[string]any{
			"kind":         "event",
			"ts":           e.Ts.AsTime().UTC().Format(time.RFC3339Nano),
			"type":         e.Type.String(),
			"slug":         e.Slug,
			"instance_uid": e.InstanceUid,
			"session_id":   e.SessionId,
			"payload":      e.Payload,
		}
		if len(e.Chain) > 0 {
			out["chain"] = chainHopsJSON(e.Chain)
		}
		b, _ := json.Marshal(out)
		fmt.Println(string(b))
		return
	}
	fmt.Printf("%s %-20s slug=%s instance_uid=%s", e.Ts.AsTime().UTC().Format(time.RFC3339Nano),
		e.Type.String(), e.Slug, e.InstanceUid)
	if chain := formatChain(e.Chain); chain != "" {
		fmt.Printf(" chain=%s", chain)
	}
	for k, v := range e.Payload {
		fmt.Printf(" %s=%q", k, v)
	}
	fmt.Println()
}

// --- instance discovery and dialing -----------------------------------------

func discoverInstances(slugFilter string, all bool, stale bool) ([]instanceRow, error) {
	roots := candidateRunRoots(all)
	var items []instanceRow
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		// Scan: <root>/<slug>/<uid>/meta.json
		slugEntries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, se := range slugEntries {
			if !se.IsDir() {
				continue
			}
			if slugFilter != "" && se.Name() != slugFilter {
				continue
			}
			slugDir := filepath.Join(root, se.Name())
			uidEntries, err := os.ReadDir(slugDir)
			if err != nil {
				continue
			}
			for _, ue := range uidEntries {
				if !ue.IsDir() {
					continue
				}
				runDir := filepath.Join(slugDir, ue.Name())
				meta, err := observability.ReadMetaJSON(runDir)
				if err != nil {
					continue
				}
				alive := isPidAlive(meta.PID)
				if !alive && !stale {
					continue
				}
				items = append(items, instanceRow{meta: meta, runDir: runDir, alive: alive})
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].meta.Slug < items[j].meta.Slug
	})
	return items, nil
}

func candidateRunRoots(all bool) []string {
	var roots []string
	if v := os.Getenv("OP_RUN_DIR"); v != "" {
		roots = append(roots, v)
	}
	if !all {
		return roots
	}
	if v := os.Getenv("OPPATH"); v != "" {
		roots = append(roots, filepath.Join(v, "run"))
	}
	if v := os.Getenv("OPROOT"); v != "" {
		roots = append(roots, filepath.Join(v, ".op", "run"))
	}
	cwd, err := os.Getwd()
	if err == nil {
		roots = append(roots, filepath.Join(cwd, ".op", "run"))
	}
	return roots
}

func isPidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, Signal(0) checks liveness without side effects.
	return proc.Signal(syscall.Signal(0)) == nil
}

func dialInstance(ctx context.Context, inst instanceRow) (*grpc.ClientConn, error) {
	addr := inst.meta.Address
	if addr == "" {
		return nil, fmt.Errorf("instance %s has no address in meta.json", inst.meta.UID)
	}
	// Strip scheme for grpc dial.
	if strings.HasPrefix(addr, "tcp://") {
		addr = strings.TrimPrefix(addr, "tcp://")
	}
	return sdkgrpcclient.Dial(ctx, addr)
}

// --- rendering for ps --------------------------------------------------------

func renderInstancesTable(items []instanceRow, flat bool) {
	_ = flat // reserved for the tree renderer; falls through to flat view for now.
	if len(items) == 0 {
		fmt.Println("No running instances.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tUID\tPID\tSTARTED\tMODE\tTRANSPORT\tADDRESS\tMETRICS_ADDR")
	for _, it := range items {
		short := it.meta.UID
		if len(short) > 8 {
			short = short[:8]
		}
		started := it.meta.StartedAt.Local().Format("2006-01-02 15:04")
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			it.meta.Slug, short, it.meta.PID, started, it.meta.Mode, it.meta.Transport,
			it.meta.Address, it.meta.MetricsAddr)
	}
	_ = w.Flush()
}

func renderInstancesJSON(items []instanceRow) error {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"slug":          it.meta.Slug,
			"uid":           it.meta.UID,
			"pid":           it.meta.PID,
			"started_at":    it.meta.StartedAt,
			"mode":          it.meta.Mode,
			"transport":     it.meta.Transport,
			"address":       it.meta.Address,
			"metrics_addr":  it.meta.MetricsAddr,
			"log_path":      it.meta.LogPath,
			"organism_uid":  it.meta.OrganismUID,
			"organism_slug": it.meta.OrganismSlug,
			"alive":         it.alive,
		})
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
