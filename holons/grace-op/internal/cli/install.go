package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/suggest"
)

func cmdInstall(format Format, globalQuiet bool, args []string) int {
	ui, args, _ := extractQuietFlag(args)
	quiet := globalQuiet || ui.Quiet

	var (
		opts       holons.InstallOptions
		positional []string
	)
	printer := commandProgress(format, quiet)
	defer printer.Close()
	opts.Progress = printer

	for _, arg := range args {
		switch arg {
		case "--build":
			opts.Build = true
		case "--link-applications":
			opts.LinkApplications = true
		default:
			if strings.HasPrefix(arg, "--") {
				fmt.Fprintf(os.Stderr, "op install: unknown flag %q\n", arg)
				return 1
			}
			positional = append(positional, arg)
		}
	}

	if len(positional) > 1 {
		fmt.Fprintln(os.Stderr, "op install: accepts at most one <holon-or-path>")
		return 1
	}

	target := "."
	if len(positional) == 1 {
		target = positional[0]
	}

	report, err := holons.Install(target, opts)
	if err != nil {
		printer.Keep()
		return printInstallResult(format, report, err, "install")
	}
	printer.Keep()
	exitCode := printInstallResult(format, report, nil, "install")
	if manifest, holon := manifestForSuggestions(target); manifest != nil {
		emitSuggestions(os.Stderr, format, quiet, suggest.Context{
			Command:     "install",
			Holon:       holon,
			Manifest:    manifest,
			BuildTarget: report.BuildTarget,
			Installed:   report.Installed,
		})
	}
	return exitCode
}

func cmdUninstall(format Format, globalQuiet bool, args []string) int {
	ui, args, _ := extractQuietFlag(args)
	quiet := globalQuiet || ui.Quiet

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "op uninstall: requires <holon>")
		return 1
	}

	printer := commandProgress(format, quiet)
	defer printer.Close()
	report, err := holons.UninstallWithOptions(args[0], holons.InstallOptions{Progress: printer})
	if err != nil {
		printer.Done("uninstall failed", err)
		return printInstallResult(format, report, err, "uninstall")
	}
	printer.Done(fmt.Sprintf("uninstalled %s in %s", report.Binary, humanElapsed(printer)), nil)
	return printInstallResult(format, report, nil, "uninstall")
}

func printInstallResult(format Format, report holons.InstallReport, err error, prefix string) int {
	if err != nil {
		if format == FormatJSON {
			payload := struct {
				holons.InstallReport
				Error string `json:"error"`
			}{
				InstallReport: report,
				Error:         err.Error(),
			}
			out, marshalErr := json.MarshalIndent(payload, "", "  ")
			if marshalErr == nil {
				fmt.Println(string(out))
			} else {
				fmt.Fprintf(os.Stderr, "op %s: %v\n", prefix, err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "op %s: %v\n", prefix, err)
		}
		return 1
	}

	if format == FormatJSON {
		out, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "op %s: %v\n", prefix, marshalErr)
			return 1
		}
		fmt.Println(string(out))
		return 0
	}

	fmt.Println(formatInstallReport(report))
	return 0
}

func formatInstallReport(report holons.InstallReport) string {
	var b strings.Builder
	writeInstallLine(&b, "Operation: %s", report.Operation)
	writeInstallLine(&b, "Holon: %s", defaultDash(report.Holon))
	writeInstallLine(&b, "Target: %s", defaultDash(report.Target))
	if report.Dir != "" {
		writeInstallLine(&b, "Dir: %s", report.Dir)
	}
	if report.Manifest != "" {
		writeInstallLine(&b, "Manifest: %s", report.Manifest)
	}
	if report.BuildTarget != "" {
		writeInstallLine(&b, "Build Target: %s", report.BuildTarget)
	}
	if report.BuildMode != "" {
		writeInstallLine(&b, "Build Mode: %s", report.BuildMode)
	}
	if report.Binary != "" {
		writeInstallLine(&b, "Binary: %s", report.Binary)
	}
	if report.Artifact != "" {
		writeInstallLine(&b, "Artifact: %s", report.Artifact)
	}
	if report.Installed != "" {
		writeInstallLine(&b, "Installed: %s", report.Installed)
	}
	if len(report.Notes) > 0 {
		writeInstallLine(&b, "Notes:")
		for _, note := range report.Notes {
			writeInstallLine(&b, "- %s", note)
		}
	}
	return strings.TrimSpace(b.String())
}

func writeInstallLine(b *strings.Builder, format string, args ...any) {
	fmt.Fprintf(b, format+"\n", args...)
}
