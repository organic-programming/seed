package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/progress"
	"github.com/organic-programming/grace-op/internal/suggest"
)

type uiOptions struct {
	Quiet bool
}

func commandProgress(format Format, quiet bool) *progress.Writer {
	if quiet || format == FormatJSON {
		return progress.Silence()
	}
	return progress.New(os.Stderr)
}

func extractQuietFlag(args []string) (uiOptions, []string, error) {
	var (
		opts      uiOptions
		remaining []string
	)

	for _, arg := range args {
		switch arg {
		case "--quiet", "-q":
			opts.Quiet = true
		default:
			remaining = append(remaining, arg)
		}
	}
	return opts, remaining, nil
}

func emitSuggestions(w io.Writer, format Format, quiet bool, ctx suggest.Context) {
	if quiet || format == FormatJSON {
		return
	}
	suggest.Print(w, ctx)
}

func humanElapsed(p *progress.Writer) string {
	if p == nil {
		return "0s"
	}
	return progress.FormatElapsed(p.Elapsed())
}

func runCleanWithProgress(printer *progress.Writer, target string) (holons.Report, error) {
	cleanStart := time.Now()
	report, err := holons.ExecuteLifecycle(holons.OperationClean, target, holons.BuildOptions{Progress: printer})
	if err != nil {
		if printer != nil {
			printer.Keep()
		}
		return report, err
	}
	if printer != nil {
		printer.FreezeAt(cleanSuccessMessage(report.Holon, cleanStart), cleanStart)
	}
	return report, nil
}

func cleanSuccessMessage(holon string, started time.Time) string {
	return fmt.Sprintf("✓ cleaned %s in %s", holon, progress.FormatElapsed(time.Since(started)))
}

func manifestForSuggestions(ref string) (*holons.LoadedManifest, string) {
	target, err := holons.ResolveTarget(strings.TrimSpace(ref))
	if err != nil || target == nil || target.Manifest == nil {
		return nil, ""
	}
	holon := target.Manifest.BinaryName()
	if holon == "" {
		holon = filepath.Base(target.Dir)
	}
	return target.Manifest, holon
}
