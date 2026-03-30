package api

import (
	"fmt"
	"os"
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/grace-op/internal/holons"
)

func (c cliState) runLifecycleCommand(format Format, quiet bool, operation string, args []string) int {
	var build opv1.BuildOptions
	var positional []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--target" && i+1 < len(args):
			build.Target = args[i+1]
			i++
		case args[i] == "--mode" && i+1 < len(args):
			build.Mode = args[i+1]
			i++
		case args[i] == "--dry-run":
			build.DryRun = true
		case args[i] == "--no-sign" && operation == "build":
			build.NoSign = true
		case strings.HasPrefix(args[i], "--"):
			fmt.Fprintf(c.stderr, "op %s: unknown flag %q\n", operation, args[i])
			return 1
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) > 1 {
		fmt.Fprintf(c.stderr, "op %s: accepts at most one <holon-or-path>\n", operation)
		return 1
	}
	target := "."
	if len(positional) == 1 {
		target = positional[0]
	}

	printer := commandProgress(format, quiet, c.stderr)
	defer printer.Close()

	buildOpts := buildOptionsFromProto(&build)
	switch operation {
	case "build", "test", "clean":
		if !buildOpts.DryRun {
			buildOpts.Progress = printer
		}
	}

	var op holons.Operation
	switch operation {
	case "check":
		op = holons.OperationCheck
	case "build":
		op = holons.OperationBuild
	case "test":
		op = holons.OperationTest
	case "clean":
		op = holons.OperationClean
	default:
		fmt.Fprintf(c.stderr, "op %s: unsupported operation\n", operation)
		return 1
	}

	report, err := holons.ExecuteLifecycle(op, target, buildOpts)
	resp := &opv1.LifecycleResponse{Report: lifecycleReportToProto(report)}
	if err != nil {
		switch op {
		case holons.OperationBuild, holons.OperationTest, holons.OperationClean:
			printer.Keep()
		}
		fmt.Fprintf(c.stderr, "op %s: %v\n", operation, err)
		if format == FormatJSON {
			c.writeFormatted(format, resp)
		}
		return 1
	}

	if !buildOpts.DryRun {
		switch op {
		case holons.OperationBuild:
			printer.KeepAs(fmt.Sprintf("built %s… ✓", report.Holon))
		case holons.OperationTest:
			printer.Done(fmt.Sprintf("tests passed for %s in %s", report.Holon, humanElapsed(printer)), nil)
		case holons.OperationClean:
			printer.Done(fmt.Sprintf("cleaned %s in %s", report.Holon, humanElapsed(printer)), nil)
		}
	}
	c.writeFormatted(format, resp)
	return 0
}

func (c cliState) runInstallCommand(format Format, quiet bool, args []string) int {
	var (
		req        opv1.InstallRequest
		positional []string
	)
	for _, arg := range args {
		switch arg {
		case "--build":
			req.Build = true
		case "--link-applications":
			req.LinkApplications = true
		default:
			if strings.HasPrefix(arg, "--") {
				fmt.Fprintf(c.stderr, "op install: unknown flag %q\n", arg)
				return 1
			}
			positional = append(positional, arg)
		}
	}
	if len(positional) > 1 {
		fmt.Fprintln(c.stderr, "op install: accepts at most one <holon-or-path>")
		return 1
	}
	req.Target = "."
	if len(positional) == 1 {
		req.Target = positional[0]
	}

	printer := commandProgress(format, quiet, c.stderr)
	defer printer.Close()

	report, err := holons.Install(req.Target, holons.InstallOptions{
		Build:            req.Build,
		LinkApplications: req.LinkApplications,
		Progress:         printer,
	})
	resp := &opv1.InstallResponse{Report: installReportToProto(report)}
	if err != nil {
		printer.Keep()
		fmt.Fprintf(c.stderr, "op install: %v\n", err)
		if format == FormatJSON {
			c.writeFormatted(format, resp)
		}
		return 1
	}
	printer.Keep()
	c.writeFormatted(format, resp)
	return 0
}

func (c cliState) runUninstallCommand(format Format, quiet bool, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(c.stderr, "op uninstall: requires <holon>")
		return 1
	}

	printer := commandProgress(format, quiet, c.stderr)
	defer printer.Close()

	report, err := holons.UninstallWithOptions(args[0], holons.InstallOptions{Progress: printer})
	resp := &opv1.InstallResponse{Report: installReportToProto(report)}
	if err != nil {
		printer.Done("uninstall failed", err)
		fmt.Fprintf(c.stderr, "op uninstall: %v\n", err)
		if format == FormatJSON {
			c.writeFormatted(format, resp)
		}
		return 1
	}
	printer.Done(fmt.Sprintf("uninstalled %s in %s", report.Binary, humanElapsed(printer)), nil)
	c.writeFormatted(format, resp)
	return 0
}

type runOptions struct {
	ListenURI string
	NoBuild   bool
	Target    string
	Mode      string
}

func (c cliState) runRunCommand(format Format, quiet bool, args []string) int {
	holonName, opts, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintf(c.stderr, "op run: %v\n", err)
		return 1
	}

	printer := commandProgress(format, quiet, c.stderr)
	defer printer.Close()

	req := resolveRunRequest(holonName, opts.ListenURI, opts.NoBuild, opts.Target, opts.Mode)
	resp, err := runWithIO(req, runIO{
		stdin:         os.Stdin,
		stdout:        c.stdout,
		stderr:        c.stderr,
		forwardSignal: true,
		progress:      printer,
	})
	if err != nil {
		printer.Done("run failed", err)
		fmt.Fprintf(c.stderr, "op run: %v\n", err)
		return 1
	}
	if resp.GetExitCode() == 0 {
		printer.Done(fmt.Sprintf("%s exited in %s", holonName, humanElapsed(printer)), nil)
	}
	return int(resp.GetExitCode())
}

func parseRunArgs(args []string) (string, runOptions, error) {
	opts := runOptions{ListenURI: "stdio://"}
	var positional []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--listen":
			if i+1 >= len(args) {
				return "", opts, fmt.Errorf("--listen requires a value")
			}
			opts.ListenURI = args[i+1]
			i++
		case args[i] == "--no-build":
			opts.NoBuild = true
		case args[i] == "--target":
			if i+1 >= len(args) {
				return "", opts, fmt.Errorf("--target requires a value")
			}
			opts.Target = args[i+1]
			i++
		case args[i] == "--mode":
			if i+1 >= len(args) {
				return "", opts, fmt.Errorf("--mode requires a value")
			}
			opts.Mode = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--"):
			return "", opts, fmt.Errorf("unknown flag %q", args[i])
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) != 1 {
		return "", opts, fmt.Errorf("accepts exactly one <holon>")
	}
	return positional[0], opts, nil
}
