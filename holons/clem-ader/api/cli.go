package api

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	aderv1 "github.com/organic-programming/clem-ader/gen/go/v1"
	"github.com/organic-programming/clem-ader/internal/engine"
	serverpkg "github.com/organic-programming/clem-ader/internal/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RunCLI executes the CLI facet and returns the process exit code.
func RunCLI(args []string, outputs ...io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), cliTerminationSignals()...)
	defer stop()
	return runCLIContext(ctx, args, outputs...)
}

func runCLIContext(ctx context.Context, args []string, outputs ...io.Writer) int {
	stdout := io.Writer(os.Stdout)
	stderr := io.Writer(os.Stderr)
	if len(outputs) > 0 && outputs[0] != nil {
		stdout = outputs[0]
	}
	if len(outputs) > 1 && outputs[1] != nil {
		stderr = outputs[1]
	}
	cmd := newRootCommand(stdout, stderr)
	cmd.SetContext(ctx)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func newRootCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "ader",
		Short:         "Freeze the repo, run the proof, archive the evidence.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newTestCommand(stdout, stderr))
	root.AddCommand(newTestBouquetCommand(stdout))
	root.AddCommand(newArchiveCommand(stdout))
	root.AddCommand(newArchiveBouquetCommand(stdout))
	root.AddCommand(newCleanupCommand(stdout))
	root.AddCommand(newPromoteCommand(stdout))
	root.AddCommand(newDowngradeCommand(stdout))
	root.AddCommand(newHistoryCommand(stdout))
	root.AddCommand(newHistoryBouquetCommand(stdout))
	root.AddCommand(newShowCommand(stdout))
	root.AddCommand(newShowBouquetCommand(stdout))
	root.AddCommand(newServeCommand())
	root.InitDefaultCompletionCmd()
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version derived from the manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(stdout, "ader %s\n", VersionString())
			return err
		},
	})
	if completion := findCommand(root, "completion"); completion != nil {
		completion.Short = "Generate shell completion scripts for ader"
		completion.AddCommand(newCompletionInstallCommand(stdout))
	}
	return root
}

func newCompletionInstallCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [shell]",
		Short: "Install shell completion into the active shell profile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			override, _ := cmd.Flags().GetString("profile")
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			shell, profile, line, err := completionInstallTarget(args, os.Getenv("SHELL"), home, override)
			if err != nil {
				return err
			}
			status, err := ensureProfileSnippet(profile, "# ader CLI autocompletion", line)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "%s %s completion in %s\n", status, shell, profile)
			return err
		},
	}
	cmd.Flags().String("profile", "", "explicit shell profile path")
	return cmd
}

func newTestCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <catalogue-dir>",
		Short: "Freeze a snapshot and execute one suite profile from a catalogue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := commandViper(cmd, args[0])
			if err != nil {
				return err
			}
			profile := cfg.GetString("test.profile")
			if cfg.GetBool("test.full") {
				profile = "full"
			}
			req := &aderv1.TestRequest{
				ConfigDir:     args[0],
				Suite:         cfg.GetString("test.suite"),
				Profile:       profile,
				Lane:          cfg.GetString("test.lane"),
				StepFilter:    cfg.GetString("test.step-filter"),
				Source:        cfg.GetString("test.source"),
				ArchivePolicy: cfg.GetString("test.archive"),
				KeepReport:    cfg.GetBool("test.keep-report"),
				KeepSnapshot:  cfg.GetBool("test.keep-snapshot"),
			}
			if cfg.GetBool("test.silent") {
				result, err := testContext(commandContext(cmd), req)
				if err != nil {
					return err
				}
				return printRunResult(stdout, result.GetManifest())
			}
			result, err := engine.RunWithProgress(commandContext(cmd), engine.RunOptions{
				ConfigDir:     req.GetConfigDir(),
				Suite:         req.GetSuite(),
				Profile:       req.GetProfile(),
				Lane:          req.GetLane(),
				StepFilter:    req.GetStepFilter(),
				Source:        req.GetSource(),
				ArchivePolicy: req.GetArchivePolicy(),
				KeepReport:    req.GetKeepReport(),
				KeepSnapshot:  req.GetKeepSnapshot(),
			}, stderr)
			if err != nil {
				return err
			}
			return printRunResult(stdout, manifestToProto(result.Manifest))
		},
	}
	cmd.Flags().String("suite", "", "suite name from <config-dir>/suites")
	cmd.Flags().String("profile", "", "suite profile")
	cmd.Flags().Bool("full", false, "shorthand for --profile full")
	cmd.Flags().String("lane", "", "suite lane: regression, progression, or both")
	cmd.Flags().String("step-filter", "", "regex filter applied to step ids")
	cmd.Flags().String("source", "", "snapshot source: committed or workspace")
	cmd.Flags().String("archive", "", "archive policy: auto, always, or never")
	cmd.Flags().Bool("keep-report", false, "keep the extracted report directory after a successful archive")
	cmd.Flags().Bool("keep-snapshot", false, "keep the deterministic snapshot and per-run artifacts")
	cmd.Flags().Bool("silent", false, "suppress live step progress and subprocess output")
	_ = cmd.MarkFlagRequired("suite")
	cmd.ValidArgsFunction = completeConfigDirs
	_ = cmd.RegisterFlagCompletionFunc("suite", completeSuites)
	_ = cmd.RegisterFlagCompletionFunc("profile", completeProfiles)
	_ = cmd.RegisterFlagCompletionFunc("lane", fixedCompletion(
		engine.CompletionItem{Value: "regression", Description: "Committed proof lane"},
		engine.CompletionItem{Value: "progression", Description: "TDD proof lane"},
		engine.CompletionItem{Value: "both", Description: "Run progression then regression"},
	))
	_ = cmd.RegisterFlagCompletionFunc("source", fixedCompletion(
		engine.CompletionItem{Value: "committed", Description: "Freeze HEAD from Git"},
		engine.CompletionItem{Value: "workspace", Description: "Freeze the current working tree"},
	))
	_ = cmd.RegisterFlagCompletionFunc("archive", fixedCompletion(
		engine.CompletionItem{Value: "auto", Description: "Archive only the canonical heavy proof"},
		engine.CompletionItem{Value: "always", Description: "Always archive the report"},
		engine.CompletionItem{Value: "never", Description: "Never archive automatically"},
	))
	return cmd
}

func newTestBouquetCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test-bouquet <ader-root>",
		Short: "Execute a bouquet across multiple catalogues",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("test-bouquet requires --name")
			}
			result, err := testBouquetContext(commandContext(cmd), &aderv1.BouquetRequest{
				AderRoot: args[0],
				Name:     name,
			})
			if err != nil {
				return err
			}
			return printBouquetRunResult(stdout, result.GetManifest())
		},
	}
	cmd.Flags().String("name", "", "bouquet name from <ader-root>/bouquets")
	cmd.ValidArgsFunction = completeAderRoots
	_ = cmd.RegisterFlagCompletionFunc("name", completeBouquets)
	return cmd
}

func newArchiveCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <catalogue-dir>",
		Short: "Archive an extracted run report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := commandViper(cmd, args[0])
			if err != nil {
				return err
			}
			result, err := archiveContext(commandContext(cmd), &aderv1.ArchiveRequest{
				ConfigDir: args[0],
				HistoryId: cfg.GetString("archive.id"),
				Latest:    cfg.GetBool("archive.latest"),
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "archived %s -> %s\n", result.GetManifest().GetHistoryId(), result.GetArchivePath())
			return err
		},
	}
	cmd.Flags().String("id", "", "history id to archive")
	cmd.Flags().Bool("latest", false, "archive the latest extracted run")
	cmd.ValidArgsFunction = completeConfigDirs
	_ = cmd.RegisterFlagCompletionFunc("id", completeHistoryIDs)
	return cmd
}

func newArchiveBouquetCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive-bouquet <ader-root>",
		Short: "Archive an extracted bouquet report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			historyID, _ := cmd.Flags().GetString("id")
			latest, _ := cmd.Flags().GetBool("latest")
			result, err := archiveBouquetContext(commandContext(cmd), &aderv1.ArchiveBouquetRequest{
				AderRoot:  args[0],
				HistoryId: historyID,
				Latest:    latest,
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "archived bouquet %s -> %s\n", result.GetManifest().GetHistoryId(), result.GetArchivePath())
			return err
		},
	}
	cmd.Flags().String("id", "", "bouquet history id to archive")
	cmd.Flags().Bool("latest", false, "archive the latest extracted bouquet run")
	cmd.ValidArgsFunction = completeAderRoots
	_ = cmd.RegisterFlagCompletionFunc("id", completeBouquetHistoryIDs)
	return cmd
}

func newCleanupCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup <catalogue-dir>",
		Short: "Remove deterministic proof residue while preserving archives and caches",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := commandViper(cmd, args[0]); err != nil {
				return err
			}
			result, err := cleanupContext(commandContext(cmd), &aderv1.CleanupRequest{ConfigDir: args[0]})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "cleanup removed local-suite=%d temp-stores=%d temp-aliases=%d\n",
				result.GetRemovedLocalSuiteDirs(),
				result.GetRemovedTempStores(),
				result.GetRemovedTempAliases(),
			)
			return err
		},
	}
	cmd.ValidArgsFunction = completeConfigDirs
	return cmd
}

func newPromoteCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "promote <catalogue-dir>",
		Short: "Move progression steps to regression in the suite YAML",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := commandViper(cmd, args[0])
			if err != nil {
				return err
			}
			result, err := promoteContext(commandContext(cmd), &aderv1.PromoteRequest{
				ConfigDir: args[0],
				Suite:     cfg.GetString("promote.suite"),
				StepIds:   cfg.GetStringSlice("promote.step"),
				All:       cfg.GetBool("promote.all"),
			})
			if err != nil {
				return err
			}
			return printPromoteResult(stdout, result)
		},
	}
	cmd.Flags().String("suite", "", "suite name from <config-dir>/suites")
	cmd.Flags().StringArray("step", nil, "step id to promote; repeat for multiple steps")
	cmd.Flags().Bool("all", false, "promote all progression steps")
	_ = cmd.MarkFlagRequired("suite")
	cmd.ValidArgsFunction = completeConfigDirs
	_ = cmd.RegisterFlagCompletionFunc("suite", completeSuites)
	_ = cmd.RegisterFlagCompletionFunc("step", completePromoteSteps)
	return cmd
}

func newDowngradeCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "downgrade <catalogue-dir>",
		Short: "Move regression steps back to progression in the suite YAML",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := commandViper(cmd, args[0])
			if err != nil {
				return err
			}
			result, err := downgradeContext(commandContext(cmd), &aderv1.DowngradeRequest{
				ConfigDir: args[0],
				Suite:     cfg.GetString("downgrade.suite"),
				StepIds:   cfg.GetStringSlice("downgrade.step"),
				All:       cfg.GetBool("downgrade.all"),
			})
			if err != nil {
				return err
			}
			return printDowngradeResult(stdout, result)
		},
	}
	cmd.Flags().String("suite", "", "suite name from <config-dir>/suites")
	cmd.Flags().StringArray("step", nil, "step id to downgrade; repeat for multiple steps")
	cmd.Flags().Bool("all", false, "downgrade all regression steps")
	_ = cmd.MarkFlagRequired("suite")
	cmd.ValidArgsFunction = completeConfigDirs
	_ = cmd.RegisterFlagCompletionFunc("suite", completeSuites)
	_ = cmd.RegisterFlagCompletionFunc("step", completeDowngradeSteps)
	return cmd
}

func newHistoryCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <catalogue-dir>",
		Short: "List extracted and archived proof history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := commandViper(cmd, args[0]); err != nil {
				return err
			}
			response, err := historyContext(commandContext(cmd), &aderv1.HistoryRequest{ConfigDir: args[0]})
			if err != nil {
				return err
			}
			entries := response.GetEntries()
			sort.Slice(entries, func(i, j int) bool { return entries[i].GetHistoryId() > entries[j].GetHistoryId() })
			for _, entry := range entries {
				location := entry.GetArchivePath()
				if strings.TrimSpace(location) == "" {
					location = entry.GetReportDir()
				}
				if _, err := fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					entry.GetHistoryId(),
					entry.GetFinalStatus(),
					entry.GetSuite(),
					entry.GetProfile(),
					entry.GetLane(),
					entry.GetSource(),
					location,
				); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.ValidArgsFunction = completeConfigDirs
	return cmd
}

func newHistoryBouquetCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history-bouquet <ader-root>",
		Short: "List extracted and archived bouquet history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			response, err := bouquetHistoryContext(commandContext(cmd), &aderv1.BouquetHistoryRequest{AderRoot: args[0]})
			if err != nil {
				return err
			}
			entries := response.GetEntries()
			sort.Slice(entries, func(i, j int) bool { return entries[i].GetHistoryId() > entries[j].GetHistoryId() })
			for _, entry := range entries {
				location := entry.GetArchivePath()
				if strings.TrimSpace(location) == "" {
					location = entry.GetReportDir()
				}
				if _, err := fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n",
					entry.GetHistoryId(),
					entry.GetFinalStatus(),
					entry.GetBouquet(),
					location,
				); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.ValidArgsFunction = completeAderRoots
	return cmd
}

func newShowCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <catalogue-dir>",
		Short: "Show the stored report for one history entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := commandViper(cmd, args[0])
			if err != nil {
				return err
			}
			historyID := strings.TrimSpace(cfg.GetString("show.id"))
			if historyID == "" {
				return fmt.Errorf("show requires --id")
			}
			response, err := showHistoryContext(commandContext(cmd), &aderv1.ShowHistoryRequest{
				ConfigDir: args[0],
				HistoryId: historyID,
			})
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, response.GetSummaryMarkdown())
			return err
		},
	}
	cmd.Flags().String("id", "", "history id to show")
	cmd.ValidArgsFunction = completeConfigDirs
	_ = cmd.RegisterFlagCompletionFunc("id", completeHistoryIDs)
	return cmd
}

func newShowBouquetCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-bouquet <ader-root>",
		Short: "Show the stored report for one bouquet history entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			historyID, _ := cmd.Flags().GetString("id")
			if strings.TrimSpace(historyID) == "" {
				return fmt.Errorf("show-bouquet requires --id")
			}
			response, err := showBouquetHistoryContext(commandContext(cmd), &aderv1.ShowBouquetHistoryRequest{
				AderRoot:  args[0],
				HistoryId: historyID,
			})
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, response.GetSummaryMarkdown())
			return err
		},
	}
	cmd.Flags().String("id", "", "bouquet history id to show")
	cmd.ValidArgsFunction = completeAderRoots
	_ = cmd.RegisterFlagCompletionFunc("id", completeBouquetHistoryIDs)
	return cmd
}

func newServeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the RPC facet",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := viper.New()
			mustBind(cfg, "serve.listen", cmd, "listen")
			mustBind(cfg, "serve.reflect", cmd, "reflect")
			listen := cfg.GetStringSlice("serve.listen")
			reflect := cfg.GetBool("serve.reflect")
			return serverpkg.ListenAndServe(listen, reflect, RPCHandler{})
		},
	}
	cmd.Flags().StringSlice("listen", nil, "listen URI(s)")
	cmd.Flags().Bool("reflect", false, "enable gRPC reflection")
	return cmd
}

func commandViper(cmd *cobra.Command, configDir string) (*viper.Viper, error) {
	cfg := viper.New()
	cfg.SetConfigName("ader")
	cfg.SetConfigType("yaml")
	cfg.AddConfigPath(configDir)
	cfg.SetEnvPrefix("ADER")
	cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	cfg.AutomaticEnv()
	if err := cfg.ReadInConfig(); err != nil {
		return nil, err
	}
	for _, flag := range []string{"suite", "profile", "full", "lane", "step-filter", "source", "archive", "keep-report", "keep-snapshot", "silent", "id", "latest", "step", "all"} {
		if cmd.Flags().Lookup(flag) == nil {
			continue
		}
		key := flag
		switch cmd.Name() {
		case "test":
			key = "test." + flag
		case "archive":
			key = "archive." + flag
		case "promote":
			key = "promote." + flag
		case "downgrade":
			key = "downgrade." + flag
		case "show":
			key = "show." + flag
		}
		mustBind(cfg, key, cmd, flag)
	}
	return cfg, nil
}

func mustBind(cfg *viper.Viper, key string, cmd *cobra.Command, flag string) {
	_ = cfg.BindPFlag(key, cmd.Flags().Lookup(flag))
}

func printRunResult(w io.Writer, manifest *aderv1.HistoryRecord) error {
	_, err := fmt.Fprintf(w, "summary: pass=%d fail=%d skip=%d\nreport: %s\n",
		manifest.GetPassCount(),
		manifest.GetFailCount(),
		manifest.GetSkipCount(),
		manifest.GetReportDir(),
	)
	if err != nil {
		return err
	}
	if strings.TrimSpace(manifest.GetArchivePath()) != "" {
		_, err = fmt.Fprintf(w, "archive: %s\n", manifest.GetArchivePath())
		if err != nil {
			return err
		}
	}
	return nil
}

func printBouquetRunResult(w io.Writer, manifest *aderv1.BouquetRecord) error {
	_, err := fmt.Fprintf(w, "summary: pass=%d fail=%d skip=%d\nreport: %s\n",
		manifest.GetPassCount(),
		manifest.GetFailCount(),
		manifest.GetSkipCount(),
		manifest.GetReportDir(),
	)
	if err != nil {
		return err
	}
	if strings.TrimSpace(manifest.GetArchivePath()) != "" {
		_, err = fmt.Fprintf(w, "archive: %s\n", manifest.GetArchivePath())
		if err != nil {
			return err
		}
	}
	return nil
}

func printPromoteResult(w io.Writer, response *aderv1.PromoteResponse) error {
	if _, err := fmt.Fprintf(w, "suite: %s\nsuite-file: %s\n", response.GetSuite(), response.GetSuiteFile()); err != nil {
		return err
	}
	if len(response.GetPromotedSteps()) == 0 {
		if _, err := io.WriteString(w, "promoted: none\n"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "promoted: %s\n", strings.Join(response.GetPromotedSteps(), ", ")); err != nil {
			return err
		}
	}
	if len(response.GetIgnoredSteps()) > 0 {
		if _, err := fmt.Fprintf(w, "ignored: %s\n", strings.Join(response.GetIgnoredSteps(), ", ")); err != nil {
			return err
		}
	}
	return nil
}

func printDowngradeResult(w io.Writer, response *aderv1.DowngradeResponse) error {
	if _, err := fmt.Fprintf(w, "suite: %s\nsuite-file: %s\n", response.GetSuite(), response.GetSuiteFile()); err != nil {
		return err
	}
	if len(response.GetDowngradedSteps()) == 0 {
		if _, err := io.WriteString(w, "downgraded: none\n"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "downgraded: %s\n", strings.Join(response.GetDowngradedSteps(), ", ")); err != nil {
			return err
		}
	}
	if len(response.GetIgnoredSteps()) > 0 {
		if _, err := fmt.Fprintf(w, "ignored: %s\n", strings.Join(response.GetIgnoredSteps(), ", ")); err != nil {
			return err
		}
	}
	return nil
}

func findCommand(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func commandContext(cmd *cobra.Command) context.Context {
	if cmd != nil && cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}

func completeConfigDirs(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	start, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items, err := engine.DiscoverConfigDirs(start)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeAderRoots(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	start, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items, err := discoverAderRoots(start)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeSuites(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	configDir, ok := positionalConfigDir(args)
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items, err := engine.ListSuites(configDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeProfiles(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	configDir, ok := positionalConfigDir(args)
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	suite, _ := cmd.Flags().GetString("suite")
	if strings.TrimSpace(suite) == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items, err := engine.ListProfiles(configDir, suite)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeHistoryIDs(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	configDir, ok := positionalConfigDir(args)
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, err := engine.History(context.Background(), configDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items := make([]engine.CompletionItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, engine.CompletionItem{
			Value:       entry.HistoryID,
			Description: compactHistoryDescription(entry),
		})
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeDowngradeSteps(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	configDir, ok := positionalConfigDir(args)
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	suite, _ := cmd.Flags().GetString("suite")
	if strings.TrimSpace(suite) == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items, err := engine.ListRegressionSteps(configDir, suite)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completePromoteSteps(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	configDir, ok := positionalConfigDir(args)
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	suite, _ := cmd.Flags().GetString("suite")
	if strings.TrimSpace(suite) == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items, err := engine.ListProgressionSteps(configDir, suite)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeBouquets(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	root, ok := positionalConfigDir(args)
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items, err := listBouquets(root)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeBouquetHistoryIDs(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	root, ok := positionalConfigDir(args)
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	response, err := BouquetHistory(&aderv1.BouquetHistoryRequest{AderRoot: root})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items := make([]engine.CompletionItem, 0, len(response.GetEntries()))
	for _, entry := range response.GetEntries() {
		items = append(items, engine.CompletionItem{
			Value:       entry.GetHistoryId(),
			Description: strings.Join([]string{entry.GetFinalStatus(), entry.GetBouquet()}, " | "),
		})
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func positionalConfigDir(args []string) (string, bool) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return "", false
	}
	return args[0], true
}

func fixedCompletion(items ...engine.CompletionItem) func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completionValues(items []engine.CompletionItem, toComplete string) []cobra.Completion {
	out := make([]cobra.Completion, 0, len(items))
	for _, item := range items {
		if !strings.HasPrefix(item.Value, toComplete) {
			continue
		}
		out = append(out, completionItem(item.Value, item.Description))
	}
	return out
}

func completionItem(value string, description string) cobra.Completion {
	if strings.TrimSpace(description) == "" {
		return cobra.Completion(value)
	}
	return cobra.CompletionWithDesc(value, description)
}

func compactHistoryDescription(entry engine.HistoryEntry) string {
	when := strings.TrimSpace(entry.StartedAt)
	if parsed, err := time.Parse(time.RFC3339, when); err == nil {
		when = parsed.Format("2006-01-02 15:04")
	}
	if when == "" {
		when = "unknown-date"
	}
	return strings.Join([]string{
		entry.FinalStatus,
		entry.Suite,
		entry.Profile,
		when,
	}, " | ")
}

func discoverAderRoots(start string) ([]engine.CompletionItem, error) {
	items := make([]engine.CompletionItem, 0)
	seen := map[string]struct{}{}
	configs, err := engine.DiscoverConfigDirs(start)
	if err != nil {
		return nil, err
	}
	for _, item := range configs {
		dir := item.Value
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		parent := filepath.Dir(absDir)
		if filepath.Base(parent) != "catalogues" {
			continue
		}
		root := filepath.Dir(parent)
		rel, err := filepath.Rel(start, root)
		if err != nil {
			rel = root
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = root
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		items = append(items, engine.CompletionItem{Value: rel})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	return items, nil
}

func listBouquets(root string) ([]engine.CompletionItem, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(absRoot, "bouquets"))
	if err != nil {
		return nil, err
	}
	items := make([]engine.CompletionItem, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		items = append(items, engine.CompletionItem{Value: name})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	return items, nil
}

func completionInstallTarget(args []string, shellPath string, home string, overrideProfile string) (shell string, profile string, line string, err error) {
	if len(args) > 0 {
		shell = strings.ToLower(strings.TrimSpace(args[0]))
	} else {
		shell = strings.ToLower(strings.TrimSpace(filepath.Base(shellPath)))
	}
	if shell == "" {
		return "", "", "", fmt.Errorf("completion install requires a shell name or SHELL to be set")
	}
	switch shell {
	case "zsh":
		line = `eval "$(ader completion zsh)"`
		if strings.TrimSpace(overrideProfile) != "" {
			profile = overrideProfile
		} else {
			profile = filepath.Join(home, ".zshrc")
		}
	case "bash":
		line = `source <(ader completion bash)`
		if strings.TrimSpace(overrideProfile) != "" {
			profile = overrideProfile
		} else {
			profile = filepath.Join(home, ".bashrc")
		}
	default:
		return "", "", "", fmt.Errorf("unsupported shell %q; supported shells: zsh, bash", shell)
	}
	return shell, profile, line, nil
}

func ensureProfileSnippet(profile string, comment string, line string) (string, error) {
	content := ""
	if data, err := os.ReadFile(profile); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if strings.Contains(content, line) {
		return "already configured", nil
	}
	var b strings.Builder
	if strings.TrimSpace(content) != "" {
		b.WriteString(strings.TrimRight(content, "\n"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(comment)
	b.WriteString("\n")
	b.WriteString(line)
	b.WriteString("\n")
	if err := os.WriteFile(profile, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return "installed", nil
}
