package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	opmod "github.com/organic-programming/grace-op/internal/mod"
	"github.com/organic-programming/grace-op/internal/suggest"
)

func cmdMod(format Format, globalQuiet bool, args []string) int {
	if len(args) == 0 {
		printModUsage()
		return 1
	}

	switch args[0] {
	case "init":
		return cmdModInit(format, globalQuiet, args[1:])
	case "add":
		return cmdModAdd(format, globalQuiet, args[1:])
	case "remove":
		return cmdModRemove(format, globalQuiet, args[1:])
	case "tidy":
		return cmdModTidy(format, globalQuiet, args[1:])
	case "pull":
		return cmdModPull(format, globalQuiet, args[1:])
	case "update":
		return cmdModUpdate(format, globalQuiet, args[1:])
	case "list":
		return cmdModList(format, globalQuiet, args[1:])
	case "graph":
		return cmdModGraph(format, globalQuiet, args[1:])
	case "help", "--help", "-h":
		printModUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "op mod: unknown command %q\n", args[0])
		printModUsage()
		return 1
	}
}

func cmdModInit(format Format, globalQuiet bool, args []string) int {
	ui, args, _ := extractQuietFlag(args)
	quiet := globalQuiet || ui.Quiet
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "usage: op mod init [holon-path]")
		return 1
	}
	var holonPath string
	if len(args) == 1 {
		holonPath = args[0]
	}

	result, err := opmod.Init(".", holonPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op mod init: %v\n", err)
		return 1
	}

	if format == FormatJSON {
		printJSON(result)
		return 0
	}
	fmt.Printf("created %s\n", result.ModFile)
	emitSuggestions(os.Stderr, format, quiet, suggest.Context{Command: "mod init"})
	return 0
}

func cmdModAdd(format Format, globalQuiet bool, args []string) int {
	ui, args, _ := extractQuietFlag(args)
	quiet := globalQuiet || ui.Quiet
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: op mod add <module> [version]")
		return 1
	}

	version := ""
	if len(args) == 2 {
		version = args[1]
	}

	result, err := opmod.Add(".", args[0], version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op mod add: %v\n", err)
		return 1
	}

	if format == FormatJSON {
		printJSON(result)
		return 0
	}

	dep := result.Dependency
	switch {
	case dep.CachePath != "":
		fmt.Printf("added %s@%s -> %s\n", dep.Path, dep.Version, dep.CachePath)
	case result.Deferred:
		fmt.Printf("added %s@%s (fetch deferred)\n", dep.Path, dep.Version)
	default:
		fmt.Printf("added %s@%s\n", dep.Path, dep.Version)
	}
	emitSuggestions(os.Stderr, format, quiet, suggest.Context{Command: "mod add"})
	return 0
}

func cmdModRemove(format Format, globalQuiet bool, args []string) int {
	_, args, _ = extractQuietFlag(args)
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: op mod remove <module>")
		return 1
	}

	result, err := opmod.Remove(".", args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "op mod remove: %v\n", err)
		return 1
	}

	if format == FormatJSON {
		printJSON(result)
		return 0
	}
	fmt.Printf("removed %s\n", result.Path)
	return 0
}

func cmdModTidy(format Format, globalQuiet bool, args []string) int {
	ui, args, _ := extractQuietFlag(args)
	quiet := globalQuiet || ui.Quiet
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: op mod tidy")
		return 1
	}

	printer := commandProgress(format, quiet)
	defer printer.Close()
	result, err := opmod.Tidy(".", opmod.Options{Progress: printer})
	if err != nil {
		printer.Done("mod tidy failed", err)
		fmt.Fprintf(os.Stderr, "op mod tidy: %v\n", err)
		return 1
	}

	printer.Done(fmt.Sprintf("tidied dependencies in %s", humanElapsed(printer)), nil)
	if format == FormatJSON {
		printJSON(result)
		return 0
	}

	fmt.Printf("updated %s\n", result.SumFile)
	if len(result.Pruned) > 0 {
		fmt.Println("pruned:")
		for _, entry := range result.Pruned {
			fmt.Printf("  %s\n", entry)
		}
	}
	emitSuggestions(os.Stderr, format, quiet, suggest.Context{Command: "mod tidy"})
	return 0
}

func cmdModPull(format Format, globalQuiet bool, args []string) int {
	ui, args, _ := extractQuietFlag(args)
	quiet := globalQuiet || ui.Quiet
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: op mod pull")
		return 1
	}

	printer := commandProgress(format, quiet)
	defer printer.Close()
	result, err := opmod.Pull(".", opmod.Options{Progress: printer})
	if err != nil {
		printer.Done("mod pull failed", err)
		fmt.Fprintf(os.Stderr, "op mod pull: %v\n", err)
		return 1
	}

	printer.Done(fmt.Sprintf("pulled %d dependencies in %s", len(result.Fetched), humanElapsed(printer)), nil)
	if format == FormatJSON {
		printJSON(result)
		return 0
	}

	if len(result.Fetched) == 0 {
		fmt.Println("all dependencies up to date")
		return 0
	}
	for _, dep := range result.Fetched {
		fmt.Printf("  %s@%s -> %s\n", dep.Path, dep.Version, dep.CachePath)
	}
	emitSuggestions(os.Stderr, format, quiet, suggest.Context{Command: "mod pull"})
	return 0
}

func cmdModUpdate(format Format, globalQuiet bool, args []string) int {
	ui, args, _ := extractQuietFlag(args)
	quiet := globalQuiet || ui.Quiet
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "usage: op mod update [module]")
		return 1
	}
	target := ""
	if len(args) == 1 {
		target = args[0]
	}

	printer := commandProgress(format, quiet)
	defer printer.Close()
	result, err := opmod.Update(".", target, opmod.Options{Progress: printer})
	if err != nil {
		printer.Done("mod update failed", err)
		fmt.Fprintf(os.Stderr, "op mod update: %v\n", err)
		return 1
	}

	printer.Done(fmt.Sprintf("updated %d dependencies in %s", len(result.Updated), humanElapsed(printer)), nil)
	if format == FormatJSON {
		printJSON(result)
		return 0
	}

	if len(result.Updated) == 0 {
		fmt.Println("all dependencies at latest compatible version")
		return 0
	}
	for _, updated := range result.Updated {
		fmt.Printf("  %s: %s -> %s\n", updated.Path, updated.OldVersion, updated.NewVersion)
	}
	return 0
}

func cmdModList(format Format, globalQuiet bool, args []string) int {
	_, args, _ = extractQuietFlag(args)
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: op mod list")
		return 1
	}

	result, err := opmod.List(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "op mod list: %v\n", err)
		return 1
	}

	if format == FormatJSON {
		printJSON(result)
		return 0
	}

	if len(result.Dependencies) == 0 {
		fmt.Printf("%s\n(no dependencies)\n", result.HolonPath)
		return 0
	}

	fmt.Printf("%s\n", result.HolonPath)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODULE\tVERSION\tCACHE")
	for _, dep := range result.Dependencies {
		cache := dep.CachePath
		if _, err := os.Stat(cache); err != nil {
			cache = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", dep.Path, dep.Version, cache)
	}
	_ = w.Flush()
	return 0
}

func cmdModGraph(format Format, globalQuiet bool, args []string) int {
	_, args, _ = extractQuietFlag(args)
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: op mod graph")
		return 1
	}

	result, err := opmod.Graph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "op mod graph: %v\n", err)
		return 1
	}

	if format == FormatJSON {
		printJSON(result)
		return 0
	}

	lines := []string{result.Root}
	for _, edge := range result.Edges {
		lines = append(lines, fmt.Sprintf("  %s -> %s@%s", edge.From, edge.To, edge.Version))
	}
	fmt.Println(strings.Join(lines, "\n"))
	return 0
}

func printModUsage() {
	fmt.Fprintln(os.Stderr, `usage: op mod <command>

Commands:
  op mod init [holon-path]
  op mod add <module> [version]
  op mod remove <module>
  op mod tidy
  op mod pull
  op mod update [module]
  op mod list
  op mod graph`)
}

func printJSON(v any) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println("{}")
		return
	}
	fmt.Println(string(out))
}
