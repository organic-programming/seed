package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/suggest"
)

func cmdLifecycle(format Format, globalQuiet bool, operation holons.Operation, args []string) int {
	ui, args, _ := extractQuietFlag(args)
	quiet := globalQuiet || ui.Quiet

	// Parse build-specific flags before the positional argument.
	var opts holons.BuildOptions
	cleanFirst := false
	var positional []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--target" && i+1 < len(args):
			opts.Target = args[i+1]
			i++
		case args[i] == "--mode" && i+1 < len(args):
			opts.Mode = args[i+1]
			i++
		case args[i] == "--dry-run":
			opts.DryRun = true
		case args[i] == "--clean" && operation == holons.OperationBuild:
			cleanFirst = true
		case args[i] == "--no-sign" && operation == holons.OperationBuild:
			opts.NoSign = true
		case strings.HasPrefix(args[i], "--"):
			fmt.Fprintf(os.Stderr, "op %s: unknown flag %q\n", operation, args[i])
			return 1
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) > 1 {
		fmt.Fprintf(os.Stderr, "op %s: accepts at most one <holon-or-path>\n", operation)
		return 1
	}

	target := "."
	if len(positional) == 1 {
		target = positional[0]
	}
	if cleanFirst && opts.DryRun {
		fmt.Fprintln(os.Stderr, "op build: --clean cannot be combined with --dry-run")
		return 1
	}

	printer := commandProgress(format, quiet)
	defer printer.Close()
	if operation == holons.OperationBuild || operation == holons.OperationTest || operation == holons.OperationClean {
		if !opts.DryRun {
			opts.Progress = printer
		}
	}
	if operation == holons.OperationBuild && cleanFirst {
		if _, err := runCleanWithProgress(printer, target); err != nil {
			fmt.Fprintf(os.Stderr, "op build: %v\n", err)
			return 1
		}
	}

	report, err := holons.ExecuteLifecycle(operation, target, opts)
	if err != nil {
		if operation == holons.OperationBuild || operation == holons.OperationTest || operation == holons.OperationClean {
			printer.Keep()
		}
		if format == FormatJSON {
			type errorReport struct {
				holons.Report
				Error string `json:"error"`
			}
			payload := errorReport{
				Report: report,
				Error:  err.Error(),
			}
			out, marshalErr := json.MarshalIndent(payload, "", "  ")
			if marshalErr == nil {
				fmt.Println(string(out))
			} else {
				fmt.Fprintf(os.Stderr, "op %s: %v\n", operation, err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "op %s: %v\n", operation, err)
		}
		return 1
	}

	if opts.DryRun && format != FormatJSON && !quiet {
		printDryRunLifecyclePlan(os.Stderr, report, "")
	} else {
		switch operation {
		case holons.OperationBuild:
			printer.KeepAs(fmt.Sprintf("built %s… ✓", report.Holon))
		case holons.OperationTest:
			printer.Done(fmt.Sprintf("tests passed for %s in %s", report.Holon, humanElapsed(printer)), nil)
		case holons.OperationClean:
			printer.Done(fmt.Sprintf("cleaned %s in %s", report.Holon, humanElapsed(printer)), nil)
		}
	}

	fmt.Println(formatLifecycleReport(format, report))
	if manifest, holon := manifestForSuggestions(target); manifest != nil {
		switch operation {
		case holons.OperationBuild:
			emitSuggestions(os.Stderr, format, quiet, suggest.Context{
				Command:     "build",
				Holon:       holon,
				Manifest:    manifest,
				BuildTarget: report.BuildTarget,
				Artifact:    report.Artifact,
			})
		case holons.OperationTest:
			emitSuggestions(os.Stderr, format, quiet, suggest.Context{
				Command:     "test",
				Holon:       holon,
				Manifest:    manifest,
				BuildTarget: report.BuildTarget,
				Artifact:    report.Artifact,
			})
		case holons.OperationClean:
			emitSuggestions(os.Stderr, format, quiet, suggest.Context{
				Command:  "clean",
				Holon:    holon,
				Manifest: manifest,
			})
		}
	}
	return 0
}

func printDryRunLifecyclePlan(w *os.File, report holons.Report, indent string) {
	if indent == "" {
		fmt.Fprintln(w, "checking manifest...")
		fmt.Fprintln(w, "validating prerequisites...")
	}
	for _, command := range report.Commands {
		line := command
		if strings.HasPrefix(command, "build_member ") {
			line = "building member: " + strings.TrimPrefix(command, "build_member ")
		}
		fmt.Fprintf(w, "%s%s\n", indent, line)
	}
	for _, child := range report.Children {
		printDryRunLifecyclePlan(w, child, indent+"  ")
	}
	if indent == "" && report.Artifact != "" {
		fmt.Fprintln(w, "verifying artifact...")
	}
}

func formatLifecycleReport(format Format, report holons.Report) string {
	if format == FormatJSON {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "{}"
		}
		return string(out)
	}

	var b strings.Builder
	writeLifecycleText(&b, report, "")
	return strings.TrimSpace(b.String())
}

func writeLifecycleText(b *strings.Builder, report holons.Report, indent string) {
	writeLifecycleLine(b, indent, "Operation: %s", report.Operation)
	writeLifecycleLine(b, indent, "Holon: %s", defaultDash(report.Holon))
	writeLifecycleLine(b, indent, "Dir: %s", defaultDash(report.Dir))
	if report.Manifest != "" {
		writeLifecycleLine(b, indent, "Manifest: %s", report.Manifest)
	}
	if report.Runner != "" {
		writeLifecycleLine(b, indent, "Runner: %s", report.Runner)
	}
	if report.Kind != "" {
		writeLifecycleLine(b, indent, "Kind: %s", report.Kind)
	}
	if report.Binary != "" {
		writeLifecycleLine(b, indent, "Binary: %s", report.Binary)
	}
	if report.BuildTarget != "" {
		writeLifecycleLine(b, indent, "Target: %s", report.BuildTarget)
	}
	if report.BuildMode != "" {
		writeLifecycleLine(b, indent, "Mode: %s", report.BuildMode)
	}
	if report.Artifact != "" {
		writeLifecycleLine(b, indent, "Artifact: %s", report.Artifact)
	}
	if len(report.Commands) > 0 {
		writeLifecycleLine(b, indent, "Commands:")
		for _, command := range report.Commands {
			writeLifecycleLine(b, indent, "- %s", command)
		}
	}
	if len(report.Notes) > 0 {
		writeLifecycleLine(b, indent, "Notes:")
		for _, note := range report.Notes {
			writeLifecycleLine(b, indent, "- %s", note)
		}
	}
	if len(report.Children) > 0 {
		writeLifecycleLine(b, indent, "Children:")
		for i, child := range report.Children {
			writeLifecycleText(b, child, indent+"  ")
			if i < len(report.Children)-1 {
				b.WriteString("\n")
			}
		}
	}
}

func writeLifecycleLine(b *strings.Builder, indent, format string, args ...any) {
	fmt.Fprintf(b, indent+format+"\n", args...)
}
