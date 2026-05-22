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
	var all, jsonOut, stale bool
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
			renderInstancesTable(items)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Scan every configured run-root candidate instead of only the first readable root")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON objects instead of a table")
	cmd.Flags().BoolVar(&stale, "stale", false, "Include entries whose PID is no longer alive")
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
			renderInstancesTable(items)
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
	var follow, jsonOut bool
	var chainOrigin string
	cmd := &cobra.Command{
		Use:   "logs <slug>",
		Short: "Tail structured logs from a running holon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd.Context(), args[0], logsOptions{
				Since: since, Level: level, Session: session, Method: method,
				Follow: follow, JSON: jsonOut,
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
	return cmd
}

type logsOptions struct {
	Since, Level, Session, Method, ChainOrigin string
	Follow, JSON                               bool
}

func runLogs(ctx context.Context, slug string, opts logsOptions) error {
	instances, err := discoverInstances(slug, false, false)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		return fmt.Errorf("op logs: no running instance for %q (run `op ps` to list)", slug)
	}
	inst := instances[0]
	conn, err := dialInstance(ctx, inst)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := v1.NewHolonObservabilityClient(conn)

	req := &v1.LogsRequest{
		MinSeverityNumber: parseSeverityNumber(opts.Level),
		Follow:            opts.Follow,
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
		if opts.ChainOrigin != "" && observability.StringAttribute(entry.GetAttributes(), observability.AttrHolonsSlug) != opts.ChainOrigin {
			continue
		}
		renderLogRecord(entry, opts.JSON)
	}
}

func parseSeverityNumber(s string) v1.SeverityNumber {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE":
		return v1.SeverityNumber_SEVERITY_NUMBER_TRACE
	case "DEBUG":
		return v1.SeverityNumber_SEVERITY_NUMBER_DEBUG
	case "INFO":
		return v1.SeverityNumber_SEVERITY_NUMBER_INFO
	case "WARN", "WARNING":
		return v1.SeverityNumber_SEVERITY_NUMBER_WARN
	case "ERROR":
		return v1.SeverityNumber_SEVERITY_NUMBER_ERROR
	case "FATAL":
		return v1.SeverityNumber_SEVERITY_NUMBER_FATAL
	default:
		return v1.SeverityNumber_SEVERITY_NUMBER_INFO
	}
}

func renderLogRecord(e *v1.LogRecord, jsonOut bool) {
	if jsonOut {
		b, _ := json.Marshal(logRecordJSON(e))
		fmt.Println(string(b))
		return
	}
	ts := ""
	if e.GetTimeUnixNano() != 0 {
		ts = time.Unix(0, int64(e.GetTimeUnixNano())).UTC().Format(time.RFC3339Nano)
	}
	attrs := e.GetAttributes()
	fmt.Printf("%s %-5s slug=%s instance_uid=%s", ts, severityNumberLabel(e.GetSeverityNumber()),
		observability.StringAttribute(attrs, observability.AttrHolonsSlug),
		observability.StringAttribute(attrs, observability.AttrHolonsInstanceUID))
	if sessionID := observability.StringAttribute(attrs, observability.AttrHolonsSessionID); sessionID != "" {
		fmt.Printf(" session_id=%s", sessionID)
	}
	if rpcMethod := observability.StringAttribute(attrs, observability.AttrRPCMethod); rpcMethod != "" {
		fmt.Printf(" method=%s", rpcMethod)
	}
	if chain := formatChain(e.GetChain()); chain != "" {
		fmt.Printf(" chain=%s", chain)
	}
	fmt.Printf(" msg=%q", anyValueText(e.GetBody()))
	fields := userAttributes(attrs)
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf(" %s=%q", k, fields[k])
	}
	fmt.Println()
}

func logRecordJSON(e *v1.LogRecord) map[string]any {
	attrs := e.GetAttributes()
	out := map[string]any{
		"kind":            "log",
		"severity_number": e.GetSeverityNumber().String(),
		"severity_text":   e.GetSeverityText(),
		"body":            anyValuePlain(e.GetBody()),
		"attributes":      attributesJSON(attrs),
	}
	if e.GetTimeUnixNano() != 0 {
		out["time_unix_nano"] = e.GetTimeUnixNano()
		out["ts"] = time.Unix(0, int64(e.GetTimeUnixNano())).UTC().Format(time.RFC3339Nano)
	}
	if e.GetObservedTimeUnixNano() != 0 {
		out["observed_time_unix_nano"] = e.GetObservedTimeUnixNano()
	}
	if e.GetEventName() != "" {
		out["event_name"] = e.GetEventName()
	}
	if len(e.GetChain()) > 0 {
		out["chain"] = e.GetChain()
	}
	return out
}

func attributesJSON(attrs []*v1.KeyValue) []map[string]any {
	out := make([]map[string]any, 0, len(attrs))
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		out = append(out, map[string]any{"key": attr.Key, "value": anyValuePlain(attr.Value)})
	}
	return out
}

func userAttributes(attrs []*v1.KeyValue) map[string]any {
	out := map[string]any{}
	for _, attr := range attrs {
		if attr == nil || isSystemAttribute(attr.Key) {
			continue
		}
		out[attr.Key] = anyValuePlain(attr.Value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isSystemAttribute(key string) bool {
	switch key {
	case observability.AttrHolonsSlug, observability.AttrHolonsInstanceUID, observability.AttrHolonsSessionID,
		observability.AttrHolonsTransport, observability.AttrServiceName, observability.AttrServiceInstanceID,
		observability.AttrRPCMethod, observability.AttrLoggerName, observability.AttrCodeCaller:
		return true
	default:
		return false
	}
}

func formatChain(chain []string) string {
	if len(chain) == 0 {
		return ""
	}
	parts := make([]string, 0, len(chain))
	for _, slug := range chain {
		if slug == "" {
			continue
		}
		parts = append(parts, slug)
	}
	return strings.Join(parts, ">")
}

func severityNumberLabel(l v1.SeverityNumber) string {
	switch l {
	case v1.SeverityNumber_SEVERITY_NUMBER_TRACE:
		return "TRACE"
	case v1.SeverityNumber_SEVERITY_NUMBER_DEBUG:
		return "DEBUG"
	case v1.SeverityNumber_SEVERITY_NUMBER_INFO:
		return "INFO"
	case v1.SeverityNumber_SEVERITY_NUMBER_WARN:
		return "WARN"
	case v1.SeverityNumber_SEVERITY_NUMBER_ERROR:
		return "ERROR"
	case v1.SeverityNumber_SEVERITY_NUMBER_FATAL:
		return "FATAL"
	default:
		return "UNSPECIFIED"
	}
}

func anyValueText(v *v1.AnyValue) string {
	switch x := v.GetValue().(type) {
	case *v1.AnyValue_StringValue:
		return x.StringValue
	case *v1.AnyValue_BoolValue:
		return fmt.Sprintf("%t", x.BoolValue)
	case *v1.AnyValue_IntValue:
		return fmt.Sprintf("%d", x.IntValue)
	case *v1.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", x.DoubleValue)
	default:
		return ""
	}
}

func anyValuePlain(v *v1.AnyValue) any {
	switch x := v.GetValue().(type) {
	case *v1.AnyValue_StringValue:
		return x.StringValue
	case *v1.AnyValue_BoolValue:
		return x.BoolValue
	case *v1.AnyValue_IntValue:
		return x.IntValue
	case *v1.AnyValue_DoubleValue:
		return x.DoubleValue
	default:
		return nil
	}
}

// --- op metrics --------------------------------------------------------------

func newMetricsCmd() *cobra.Command {
	var promOut, jsonOut bool
	var prefix string
	cmd := &cobra.Command{
		Use:   "metrics <slug>",
		Short: "Fetch OTLP-shaped metrics from a running holon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMetrics(cmd.Context(), args[0], metricsOptions{
				Prom: promOut, JSON: jsonOut, Prefix: prefix,
			})
		},
	}
	cmd.Flags().BoolVar(&promOut, "prom", false, "Emit Prometheus text exposition format")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit metrics as JSON")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Filter metric names by prefix")
	return cmd
}

type metricsOptions struct {
	Prom, JSON bool
	Prefix     string
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
	req := &v1.MetricsRequest{}
	if opts.Prefix != "" {
		req.NamePrefixes = []string{opts.Prefix}
	}
	stream, err := client.Metrics(ctx, req)
	if err != nil {
		return fmt.Errorf("op metrics: %w", err)
	}
	var metrics []*v1.Metric
	for {
		metric, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		metrics = append(metrics, metric)
	}
	if opts.JSON {
		items := make([]map[string]any, 0, len(metrics))
		for _, metric := range metrics {
			items = append(items, metricJSON(metric))
		}
		b, _ := json.MarshalIndent(items, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	if opts.Prom {
		renderMetricsProm(metrics)
		return nil
	}
	renderMetricsTable(metrics)
	return nil
}

func renderMetricsTable(metrics []*v1.Metric) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "METRIC\tTYPE\tVALUE")
	for _, metric := range metrics {
		switch data := metric.GetData().(type) {
		case *v1.Metric_Sum:
			for _, dp := range data.Sum.GetDataPoints() {
				fmt.Fprintf(w, "%s%s\tsum\t%d\n", metric.Name, renderLabels(dp.GetAttributes()), dp.GetAsInt())
			}
		case *v1.Metric_Gauge:
			for _, dp := range data.Gauge.GetDataPoints() {
				fmt.Fprintf(w, "%s%s\tgauge\t%g\n", metric.Name, renderLabels(dp.GetAttributes()), dp.GetAsDouble())
			}
		case *v1.Metric_Histogram:
			for _, dp := range data.Histogram.GetDataPoints() {
				fmt.Fprintf(w, "%s%s\thistogram\tcount=%d sum=%g\n", metric.Name, renderLabels(dp.GetAttributes()), dp.GetCount(), dp.GetSum())
			}
		}
	}
	_ = w.Flush()
}

func renderMetricsProm(metrics []*v1.Metric) {
	for _, metric := range metrics {
		switch data := metric.GetData().(type) {
		case *v1.Metric_Sum:
			fmt.Printf("# TYPE %s counter\n", metric.Name)
			for _, dp := range data.Sum.GetDataPoints() {
				fmt.Printf("%s%s %d\n", metric.Name, renderLabels(dp.GetAttributes()), dp.GetAsInt())
			}
		case *v1.Metric_Gauge:
			fmt.Printf("# TYPE %s gauge\n", metric.Name)
			for _, dp := range data.Gauge.GetDataPoints() {
				fmt.Printf("%s%s %g\n", metric.Name, renderLabels(dp.GetAttributes()), dp.GetAsDouble())
			}
		case *v1.Metric_Histogram:
			fmt.Printf("# TYPE %s histogram\n", metric.Name)
			for _, dp := range data.Histogram.GetDataPoints() {
				var cumulative uint64
				for i, count := range dp.GetBucketCounts() {
					cumulative += count
					le := "+Inf"
					if i < len(dp.GetExplicitBounds()) {
						le = fmt.Sprintf("%g", dp.GetExplicitBounds()[i])
					}
					labels := append(cloneKeyValues(dp.GetAttributes()), &v1.KeyValue{Key: "le", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: le}}})
					fmt.Printf("%s_bucket%s %d\n", metric.Name, renderLabels(labels), cumulative)
				}
				fmt.Printf("%s_sum%s %g\n", metric.Name, renderLabels(dp.GetAttributes()), dp.GetSum())
				fmt.Printf("%s_count%s %d\n", metric.Name, renderLabels(dp.GetAttributes()), dp.GetCount())
			}
		}
	}
}

func renderLabels(attrs []*v1.KeyValue) string {
	labels := userAttributes(attrs)
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
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
		b.WriteString(fmt.Sprintf("%v", labels[k]))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

func cloneKeyValues(attrs []*v1.KeyValue) []*v1.KeyValue {
	out := make([]*v1.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		out = append(out, &v1.KeyValue{Key: attr.Key, Value: attr.Value})
	}
	return out
}

func metricJSON(metric *v1.Metric) map[string]any {
	out := map[string]any{
		"name":        metric.GetName(),
		"description": metric.GetDescription(),
		"unit":        metric.GetUnit(),
	}
	switch data := metric.GetData().(type) {
	case *v1.Metric_Sum:
		out["type"] = "sum"
		out["is_monotonic"] = data.Sum.GetIsMonotonic()
		out["aggregation_temporality"] = data.Sum.GetAggregationTemporality().String()
		out["data_points"] = numberDataPointsJSON(data.Sum.GetDataPoints())
	case *v1.Metric_Gauge:
		out["type"] = "gauge"
		out["data_points"] = numberDataPointsJSON(data.Gauge.GetDataPoints())
	case *v1.Metric_Histogram:
		out["type"] = "histogram"
		out["aggregation_temporality"] = data.Histogram.GetAggregationTemporality().String()
		points := make([]map[string]any, 0, len(data.Histogram.GetDataPoints()))
		for _, dp := range data.Histogram.GetDataPoints() {
			points = append(points, map[string]any{
				"start_time_unix_nano": dp.GetStartTimeUnixNano(),
				"time_unix_nano":       dp.GetTimeUnixNano(),
				"attributes":           attributesJSON(dp.GetAttributes()),
				"count":                dp.GetCount(),
				"sum":                  dp.GetSum(),
				"bucket_counts":        dp.GetBucketCounts(),
				"explicit_bounds":      dp.GetExplicitBounds(),
				"min":                  dp.GetMin(),
				"max":                  dp.GetMax(),
			})
		}
		out["data_points"] = points
	}
	return out
}

func numberDataPointsJSON(points []*v1.NumberDataPoint) []map[string]any {
	out := make([]map[string]any, 0, len(points))
	for _, dp := range points {
		item := map[string]any{
			"start_time_unix_nano": dp.GetStartTimeUnixNano(),
			"time_unix_nano":       dp.GetTimeUnixNano(),
			"attributes":           attributesJSON(dp.GetAttributes()),
		}
		switch v := dp.GetValue().(type) {
		case *v1.NumberDataPoint_AsDouble:
			item["as_double"] = v.AsDouble
		case *v1.NumberDataPoint_AsInt:
			item["as_int"] = v.AsInt
		}
		out = append(out, item)
	}
	return out
}

// --- op events ---------------------------------------------------------------

func newEventsCmd() *cobra.Command {
	var follow, jsonOut bool
	var since string
	var eventNames []string
	cmd := &cobra.Command{
		Use:   "events <slug>",
		Short: "Stream lifecycle events from a running holon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEvents(cmd.Context(), args[0], eventsOptions{
				Follow: follow, JSON: jsonOut, Since: since, EventNames: eventNames,
			})
		},
	}
	cmd.Flags().BoolVar(&follow, "follow", true, "Stream new events")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit one JSON object per line")
	cmd.Flags().StringVar(&since, "since", "", "Replay events newer than this duration")
	cmd.Flags().StringSliceVar(&eventNames, "event-name", nil, "Filter to these event_name values (repeatable)")
	return cmd
}

type eventsOptions struct {
	Follow, JSON bool
	Since        string
	EventNames   []string
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
	req.EventNames = append(req.EventNames, opts.EventNames...)
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

func renderEvent(e *v1.LogRecord, jsonOut bool) {
	if jsonOut {
		out := logRecordJSON(e)
		out["kind"] = "event"
		b, _ := json.Marshal(out)
		fmt.Println(string(b))
		return
	}
	ts := ""
	if e.GetTimeUnixNano() != 0 {
		ts = time.Unix(0, int64(e.GetTimeUnixNano())).UTC().Format(time.RFC3339Nano)
	}
	attrs := e.GetAttributes()
	fmt.Printf("%s %-20s slug=%s instance_uid=%s", ts, e.GetEventName(),
		observability.StringAttribute(attrs, observability.AttrHolonsSlug),
		observability.StringAttribute(attrs, observability.AttrHolonsInstanceUID))
	if chain := formatChain(e.GetChain()); chain != "" {
		fmt.Printf(" chain=%s", chain)
	}
	payload := userAttributes(attrs)
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf(" %s=%q", k, payload[k])
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
	roots := candidateRunRootChain()
	if all {
		return roots
	}
	for _, root := range roots {
		if isReadableDir(root) {
			return []string{root}
		}
	}
	return nil
}

func candidateRunRootChain() []string {
	var roots []string
	if v := os.Getenv("OP_RUN_DIR"); v != "" {
		roots = append(roots, v)
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

func isReadableDir(path string) bool {
	if path == "" {
		return false
	}
	if entries, err := os.ReadDir(path); err != nil {
		return false
	} else {
		_ = entries
		return true
	}
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

func renderInstancesTable(items []instanceRow) {
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
