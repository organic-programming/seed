package api

import (
	"fmt"
	"os"
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

func (c cliState) runModCommand(format Format, quiet bool, args []string) int {
	_ = quiet
	if len(args) == 0 {
		c.printModUsage()
		return 1
	}
	switch args[0] {
	case "init":
		req := &opv1.ModInitRequest{}
		if len(args) > 2 {
			fmt.Fprintln(c.stderr, "usage: op mod init [holon-path]")
			return 1
		}
		if len(args) == 2 {
			req.HolonPath = args[1]
		}
		resp, err := ModInit(req)
		if err != nil {
			fmt.Fprintf(c.stderr, "op mod init: %v\n", err)
			return 1
		}
		if format == FormatJSON {
			c.writeFormatted(format, resp)
			return 0
		}
		fmt.Fprintf(c.stdout, "created %s\n", resp.GetModFile())
		return 0
	case "add":
		if len(args) < 2 || len(args) > 3 {
			fmt.Fprintln(c.stderr, "usage: op mod add <module> [version]")
			return 1
		}
		req := &opv1.ModAddRequest{Module: args[1]}
		if len(args) == 3 {
			req.Version = args[2]
		}
		resp, err := ModAdd(req)
		if err != nil {
			fmt.Fprintf(c.stderr, "op mod add: %v\n", err)
			return 1
		}
		if format == FormatJSON {
			c.writeFormatted(format, resp)
			return 0
		}
		dep := resp.GetDependency()
		switch {
		case dep.GetCachePath() != "":
			fmt.Fprintf(c.stdout, "added %s@%s -> %s\n", dep.GetPath(), dep.GetVersion(), dep.GetCachePath())
		case resp.GetDeferred():
			fmt.Fprintf(c.stdout, "added %s@%s (fetch deferred)\n", dep.GetPath(), dep.GetVersion())
		default:
			fmt.Fprintf(c.stdout, "added %s@%s\n", dep.GetPath(), dep.GetVersion())
		}
		return 0
	case "remove":
		if len(args) != 2 {
			fmt.Fprintln(c.stderr, "usage: op mod remove <module>")
			return 1
		}
		resp, err := ModRemove(&opv1.ModRemoveRequest{Module: args[1]})
		if err != nil {
			fmt.Fprintf(c.stderr, "op mod remove: %v\n", err)
			return 1
		}
		if format == FormatJSON {
			c.writeFormatted(format, resp)
			return 0
		}
		fmt.Fprintf(c.stdout, "removed %s\n", resp.GetPath())
		return 0
	case "tidy":
		resp, err := ModTidy(&opv1.ModTidyRequest{})
		if err != nil {
			fmt.Fprintf(c.stderr, "op mod tidy: %v\n", err)
			return 1
		}
		if format == FormatJSON {
			c.writeFormatted(format, resp)
			return 0
		}
		fmt.Fprintf(c.stdout, "updated %s\n", resp.GetSumFile())
		if len(resp.GetPruned()) > 0 {
			fmt.Fprintln(c.stdout, "pruned:")
			for _, entry := range resp.GetPruned() {
				fmt.Fprintf(c.stdout, "  %s\n", entry)
			}
		}
		return 0
	case "pull":
		resp, err := ModPull(&opv1.ModPullRequest{})
		if err != nil {
			fmt.Fprintf(c.stderr, "op mod pull: %v\n", err)
			return 1
		}
		if format == FormatJSON {
			c.writeFormatted(format, resp)
			return 0
		}
		if len(resp.GetFetched()) == 0 {
			fmt.Fprintln(c.stdout, "all dependencies up to date")
			return 0
		}
		for _, dep := range resp.GetFetched() {
			fmt.Fprintf(c.stdout, "  %s@%s -> %s\n", dep.GetPath(), dep.GetVersion(), dep.GetCachePath())
		}
		return 0
	case "update":
		req := &opv1.ModUpdateRequest{}
		if len(args) > 2 {
			fmt.Fprintln(c.stderr, "usage: op mod update [module]")
			return 1
		}
		if len(args) == 2 {
			req.Module = args[1]
		}
		resp, err := ModUpdate(req)
		if err != nil {
			fmt.Fprintf(c.stderr, "op mod update: %v\n", err)
			return 1
		}
		if format == FormatJSON {
			c.writeFormatted(format, resp)
			return 0
		}
		if len(resp.GetUpdated()) == 0 {
			fmt.Fprintln(c.stdout, "all dependencies at latest compatible version")
			return 0
		}
		for _, dep := range resp.GetUpdated() {
			fmt.Fprintf(c.stdout, "  %s: %s -> %s\n", dep.GetPath(), dep.GetOldVersion(), dep.GetNewVersion())
		}
		return 0
	case "list":
		resp, err := ModList(&opv1.ModListRequest{})
		if err != nil {
			fmt.Fprintf(c.stderr, "op mod list: %v\n", err)
			return 1
		}
		if format == FormatJSON {
			c.writeFormatted(format, resp)
			return 0
		}
		if len(resp.GetDependencies()) == 0 {
			fmt.Fprintf(c.stdout, "%s\n(no dependencies)\n", resp.GetHolonPath())
			return 0
		}
		fmt.Fprintf(c.stdout, "%s\n", resp.GetHolonPath())
		for _, dep := range resp.GetDependencies() {
			cache := dep.GetCachePath()
			if cache == "" {
				cache = "-"
			} else if _, err := os.Stat(cache); err != nil {
				cache = "-"
			}
			fmt.Fprintf(c.stdout, "%s\t%s\t%s\n", dep.GetPath(), dep.GetVersion(), cache)
		}
		return 0
	case "graph":
		resp, err := ModGraph(&opv1.ModGraphRequest{})
		if err != nil {
			fmt.Fprintf(c.stderr, "op mod graph: %v\n", err)
			return 1
		}
		if format == FormatJSON {
			c.writeFormatted(format, resp)
			return 0
		}
		lines := []string{resp.GetRoot()}
		for _, edge := range resp.GetEdges() {
			lines = append(lines, fmt.Sprintf("  %s -> %s@%s", edge.GetFrom(), edge.GetTo(), edge.GetVersion()))
		}
		fmt.Fprintln(c.stdout, strings.Join(lines, "\n"))
		return 0
	case "help", "--help", "-h":
		c.printModUsage()
		return 0
	default:
		fmt.Fprintf(c.stderr, "op mod: unknown command %q\n", args[0])
		c.printModUsage()
		return 1
	}
}

func (c cliState) printModUsage() {
	fmt.Fprintln(c.stderr, `usage: op mod <command>

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
