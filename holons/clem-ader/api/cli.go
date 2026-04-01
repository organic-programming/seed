package api

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	aderv1 "github.com/organic-programming/clem-ader/gen/go/v1"
	"github.com/organic-programming/clem-ader/internal/engine"
	serverpkg "github.com/organic-programming/clem-ader/internal/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RunCLI executes the CLI facet and returns the process exit code.
func RunCLI(args []string, outputs ...io.Writer) int {
	stdout := io.Writer(os.Stdout)
	stderr := io.Writer(os.Stderr)
	if len(outputs) > 0 && outputs[0] != nil {
		stdout = outputs[0]
	}
	if len(outputs) > 1 && outputs[1] != nil {
		stderr = outputs[1]
	}
	cmd := newRootCommand(stdout, stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
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
	root.AddCommand(newArchiveCommand(stdout))
	root.AddCommand(newCleanupCommand(stdout))
	root.AddCommand(newHistoryCommand(stdout))
	root.AddCommand(newShowCommand(stdout))
	root.AddCommand(newServeCommand())
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version derived from the manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(stdout, "ader %s\n", VersionString())
			return err
		},
	})
	return root
}

func newTestCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <config-dir>",
		Short: "Freeze a snapshot and execute a verification suite profile",
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
				result, err := Test(req)
				if err != nil {
					return err
				}
				return printRunResult(stdout, result.GetManifest())
			}
			result, err := engine.RunWithProgress(context.Background(), engine.RunOptions{
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
	cmd.Flags().String("profile", "", "verification profile")
	cmd.Flags().Bool("full", false, "shorthand for --profile full")
	cmd.Flags().String("lane", "", "suite lane: regression, progression, or both")
	cmd.Flags().String("step-filter", "", "regex filter applied to step ids")
	cmd.Flags().String("source", "", "snapshot source: committed or workspace")
	cmd.Flags().String("archive", "", "archive policy: auto, always, or never")
	cmd.Flags().Bool("keep-report", false, "keep the extracted report directory after a successful archive")
	cmd.Flags().Bool("keep-snapshot", false, "keep the deterministic snapshot and per-run artifacts")
	cmd.Flags().Bool("silent", false, "suppress live step progress and subprocess output")
	return cmd
}

func newArchiveCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <config-dir>",
		Short: "Archive an extracted run report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := commandViper(cmd, args[0])
			if err != nil {
				return err
			}
			result, err := Archive(&aderv1.ArchiveRequest{
				ConfigDir: args[0],
				RunId:     cfg.GetString("archive.run"),
				Latest:    cfg.GetBool("archive.latest"),
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "archived %s -> %s\n", result.GetManifest().GetRunId(), result.GetArchivePath())
			return err
		},
	}
	cmd.Flags().String("run", "", "run id to archive")
	cmd.Flags().Bool("latest", false, "archive the latest extracted run")
	return cmd
}

func newCleanupCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup <config-dir>",
		Short: "Remove deterministic verification residue while preserving archives and caches",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := commandViper(cmd, args[0]); err != nil {
				return err
			}
			result, err := Cleanup(&aderv1.CleanupRequest{ConfigDir: args[0]})
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
	return cmd
}

func newHistoryCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "history <config-dir>",
		Short: "List extracted and archived verification history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := commandViper(cmd, args[0]); err != nil {
				return err
			}
			response, err := ListRuns(&aderv1.ListRunsRequest{ConfigDir: args[0]})
			if err != nil {
				return err
			}
			runs := response.GetRuns()
			sort.Slice(runs, func(i, j int) bool { return runs[i].GetRunId() > runs[j].GetRunId() })
			for _, run := range runs {
				location := run.GetArchivePath()
				if strings.TrimSpace(location) == "" {
					location = run.GetReportDir()
				}
				if _, err := fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					run.GetRunId(),
					run.GetFinalStatus(),
					run.GetSuite(),
					run.GetProfile(),
					run.GetLane(),
					run.GetSource(),
					location,
				); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newShowCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <config-dir>",
		Short: "Show the stored report for one run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := commandViper(cmd, args[0])
			if err != nil {
				return err
			}
			runID := strings.TrimSpace(cfg.GetString("show.run"))
			if runID == "" {
				return fmt.Errorf("show requires --run")
			}
			response, err := ShowRun(&aderv1.ShowRunRequest{
				ConfigDir: args[0],
				RunId:     runID,
			})
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, response.GetSummaryMarkdown())
			return err
		},
	}
	cmd.Flags().String("run", "", "run id to show")
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
	for _, flag := range []string{"suite", "profile", "full", "lane", "step-filter", "source", "archive", "keep-report", "keep-snapshot", "silent", "run", "latest"} {
		if cmd.Flags().Lookup(flag) == nil {
			continue
		}
		key := flag
		switch cmd.Name() {
		case "test":
			key = "test." + flag
		case "archive":
			key = "archive." + flag
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

func printRunResult(w io.Writer, manifest *aderv1.RunManifest) error {
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
