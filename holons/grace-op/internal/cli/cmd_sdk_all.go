package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/sdkprebuilts"
)

const sdkAllSentinel = "all"

var sdkAllBuildOrder = []string{
	"go",
	"java",
	"kotlin",
	"dart",
	"swift",
	"python",
	"csharp",
	"js",
	"js-web",
	"rust",
	"ruby",
	"zig",
	"c",
	"cpp",
}

type sdkAllKind string

const (
	sdkAllBuild   sdkAllKind = "build"
	sdkAllInstall sdkAllKind = "install"
)

type sdkAllStatus string

const (
	sdkAllStatusOK      sdkAllStatus = "OK"
	sdkAllStatusFail    sdkAllStatus = "FAIL"
	sdkAllStatusSkipped sdkAllStatus = "SKIPPED"
)

type sdkAllOptions struct {
	kind       sdkAllKind
	target     string
	langs      []string
	stdout     io.Writer
	stderr     io.Writer
	now        func() time.Time
	logBaseDir string
	runSDK     func(context.Context, string, io.Writer) error
	skipSDK    func(string) ([]string, error)
}

type sdkAllResult struct {
	Lang    string
	Status  sdkAllStatus
	Elapsed time.Duration
	LogPath string
	Detail  string
}

type sdkAllFailedError struct {
	failed int
}

func (e sdkAllFailedError) Error() string {
	if e.failed == 1 {
		return "1 SDK failed"
	}
	return fmt.Sprintf("%d SDKs failed", e.failed)
}

func sdkAllOrderedLangs() []string {
	return append([]string(nil), sdkAllBuildOrder...)
}

func runSdkAll(ctx context.Context, opts sdkAllOptions) ([]sdkAllResult, error) {
	if opts.stdout == nil {
		opts.stdout = io.Discard
	}
	if opts.stderr == nil {
		opts.stderr = io.Discard
	}
	if opts.now == nil {
		opts.now = time.Now
	}
	if len(opts.langs) == 0 {
		opts.langs = sdkAllOrderedLangs()
	}
	if opts.logBaseDir == "" {
		opts.logBaseDir = filepath.Join(openv.OPPATH(), "logs", "sdk-"+string(opts.kind))
	}
	if opts.runSDK == nil {
		return nil, errors.New("sdk batch runner is required")
	}

	runID, runDir, err := createSdkAllRunDir(opts.logBaseDir, opts.now())
	if err != nil {
		return nil, err
	}

	results := make([]sdkAllResult, 0, len(opts.langs))
	failures := 0
	for idx, lang := range opts.langs {
		start := opts.now()
		logPath := filepath.Join(runDir, lang+".log")
		fmt.Fprintf(opts.stdout, "%s %s... [%d/%d]\n", opts.kind.progressVerb(), lang, idx+1, len(opts.langs))

		result := sdkAllResult{
			Lang:    lang,
			Status:  sdkAllStatusOK,
			LogPath: logPath,
		}
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return results, fmt.Errorf("create %s: %w", logPath, err)
		}
		fmt.Fprintf(logFile, "op sdk %s all\nrun_id=%s\nsdk=%s\ntarget=%s\nstarted_at=%s\n\n",
			opts.kind, runID, lang, strings.TrimSpace(opts.target), start.UTC().Format(time.RFC3339))

		var runErr error
		if opts.skipSDK != nil {
			blockers, err := opts.skipSDK(lang)
			if err != nil {
				runErr = err
			} else if len(blockers) > 0 {
				result.Status = sdkAllStatusSkipped
				result.Detail = strings.Join(blockers, "; ")
				fmt.Fprintf(logFile, "SKIPPED: %s\n", result.Detail)
			}
		}
		if runErr == nil && result.Status != sdkAllStatusSkipped {
			runErr = opts.runSDK(ctx, lang, logFile)
		}
		if runErr != nil {
			result.Status = sdkAllStatusFail
			result.Detail = oneLine(runErr.Error())
			fmt.Fprintf(logFile, "\nFAIL: %s\n", runErr.Error())
			failures++
		}
		result.Elapsed = elapsedSince(start, opts.now())
		if err := logFile.Close(); err != nil && runErr == nil {
			return results, fmt.Errorf("close %s: %w", logPath, err)
		}

		fmt.Fprintln(opts.stdout, result.summaryLine())
		if result.Status == sdkAllStatusFail {
			printFailedLogTail(opts.stderr, logPath, lang)
		}
		results = append(results, result)
	}

	summary, summaryPath, err := writeSdkAllSummary(runDir, results)
	if err != nil {
		return results, err
	}
	fmt.Fprintln(opts.stdout)
	fmt.Fprintln(opts.stdout, "Summary:")
	fmt.Fprint(opts.stdout, summary)
	fmt.Fprintf(opts.stdout, "Summary log: %s\n", summaryPath)

	if failures > 0 {
		return results, sdkAllFailedError{failed: failures}
	}
	return results, nil
}

func createSdkAllRunDir(baseDir string, now time.Time) (string, string, error) {
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create %s: %w", baseDir, err)
	}
	now = now.UTC().Truncate(time.Second)
	for i := 0; i < 120; i++ {
		runID := now.Add(time.Duration(i) * time.Second).Format("20060102T150405Z")
		runDir := filepath.Join(baseDir, runID)
		if err := os.Mkdir(runDir, 0o755); err == nil {
			return runID, runDir, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", "", fmt.Errorf("create %s: %w", runDir, err)
		}
	}
	return "", "", fmt.Errorf("could not allocate unique SDK %s log run directory under %s", now.Format("20060102T150405Z"), baseDir)
}

func writeSdkAllSummary(runDir string, results []sdkAllResult) (string, string, error) {
	var b strings.Builder
	for _, result := range results {
		b.WriteString(result.summaryLine())
		b.WriteByte('\n')
	}
	summary := b.String()
	path := filepath.Join(runDir, "summary.txt")
	if err := os.WriteFile(path, []byte(summary), 0o644); err != nil {
		return "", "", fmt.Errorf("write %s: %w", path, err)
	}
	return summary, path, nil
}

func (r sdkAllResult) summaryLine() string {
	if strings.TrimSpace(r.Detail) != "" {
		return fmt.Sprintf("%s | %s | %s | %s | %s", r.Status, r.Lang, formatSdkAllDuration(r.Elapsed), r.LogPath, r.Detail)
	}
	return fmt.Sprintf("%s | %s | %s | %s", r.Status, r.Lang, formatSdkAllDuration(r.Elapsed), r.LogPath)
}

func (k sdkAllKind) progressVerb() string {
	switch k {
	case sdkAllInstall:
		return "Installing"
	default:
		return "Building"
	}
}

func elapsedSince(start, end time.Time) time.Duration {
	elapsed := end.Sub(start)
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func formatSdkAllDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	d = d.Round(time.Second)
	hours := int(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	minutes := int(d / time.Minute)
	d -= time.Duration(minutes) * time.Minute
	seconds := int(d / time.Second)
	switch {
	case hours > 0:
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

func printFailedLogTail(w io.Writer, logPath, lang string) {
	tail, err := tailFileLines(logPath, 20)
	if err != nil {
		fmt.Fprintf(w, "FAIL | %s | could not read log tail %s: %v\n", lang, logPath, err)
		return
	}
	if strings.TrimSpace(tail) == "" {
		return
	}
	fmt.Fprintf(w, "Last 20 lines for %s (%s):\n%s\n", lang, logPath, tail)
}

func tailFileLines(path string, maxLines int) (string, error) {
	if maxLines <= 0 {
		return "", nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	const maxTailBytes int64 = 64 * 1024
	start := int64(0)
	if info.Size() > maxTailBytes {
		start = info.Size() - maxTailBytes
	}
	buf := make([]byte, int(info.Size()-start))
	if _, err := f.ReadAt(buf, start); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	text := string(buf)
	if start > 0 {
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			text = text[idx+1:]
		}
	}
	text = strings.TrimRight(text, "\r\n")
	if text == "" {
		return "", nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n"), nil
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func sdkBuildAllBlockers(lang, target string) ([]string, error) {
	blockers, err := sdkprebuilts.CompileBlockers(lang, target)
	if err != nil {
		return nil, err
	}
	if blocker := sdkBuildAllTargetBlocker(lang, target); blocker != "" {
		blockers = append(blockers, blocker)
	}
	return blockers, nil
}

func sdkBuildAllTargetBlocker(lang, target string) string {
	normalizedLang, err := sdkprebuilts.NormalizeLang(lang)
	if err != nil {
		return err.Error()
	}
	normalizedTarget, err := sdkprebuilts.NormalizeTarget(target)
	if err != nil {
		return err.Error()
	}
	host, err := sdkprebuilts.HostTriplet()
	if err != nil {
		return fmt.Sprintf("host target unresolved: %v", err)
	}

	switch normalizedLang {
	case "dart", "rust", "swift":
		if normalizedTarget != host {
			return fmt.Sprintf("%s SDK build is host-native only; target %s does not match host %s", normalizedLang, normalizedTarget, host)
		}
	case "java", "kotlin":
		if normalizedTarget != "aarch64-apple-darwin" {
			return fmt.Sprintf("%s SDK build currently supports target aarch64-apple-darwin only; got %s", normalizedLang, normalizedTarget)
		}
	case "c", "cpp", "csharp", "js", "python", "ruby", "zig":
		if normalizedTarget == "x86_64-pc-windows-msvc" {
			return fmt.Sprintf("%s SDK build currently supports Windows through x86_64-windows-gnu only; got %s", normalizedLang, normalizedTarget)
		}
	}
	return ""
}
