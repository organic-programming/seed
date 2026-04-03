package api

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	jamesloopsv1 "github.com/organic-programming/james-loops/gen/go/v1"
	"github.com/organic-programming/james-loops/internal/engine"
	serverpkg "github.com/organic-programming/james-loops/internal/server"
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
		Use:           "james-loops",
		Short:         "Queue the briefs, run the night, read the morning report.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newRunCommand(stdout))
	root.AddCommand(newEnqueueCommand(stdout))
	root.AddCommand(newProfileCommand(stdout))
	root.AddCommand(newListCommand(stdout))
	root.AddCommand(newStatusCommand(stdout))
	root.AddCommand(newDropCommand(stdout))
	root.AddCommand(newResumeCommand(stdout))
	root.AddCommand(newSkipCommand(stdout))
	root.AddCommand(newAbortCommand(stdout))
	root.AddCommand(newReEnqueueCommand(stdout))
	root.AddCommand(newLogCommand(stdout))
	root.AddCommand(newServeCommand())
	root.InitDefaultCompletionCmd()
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version derived from the manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(stdout, "james-loops %s\n", VersionString())
			return err
		},
	})
	if completion := findCommand(root, "completion"); completion != nil {
		completion.Short = "Generate shell completion scripts for james-loops"
		completion.AddCommand(newCompletionInstallCommand(stdout))
	}
	return root
}

func newRunCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Process the queue with the configured AI profiles and ader gates",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := commandViper(cmd)
			req := &jamesloopsv1.RunRequest{
				Root:             cfg.GetString("run.root"),
				DryRun:           cfg.GetBool("run.dry-run"),
				MaxRetries:       uint32(cfg.GetInt("run.max-retries")),
				CoderProfile:     cfg.GetString("run.coder-profile"),
				EvaluatorProfile: cfg.GetString("run.evaluator-profile"),
			}
			if req.GetDryRun() {
				resp, err := Run(req)
				if err != nil {
					return err
				}
				_, err = io.WriteString(stdout, resp.GetReportMarkdown())
				return err
			}
			resp, err := Run(req)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "report: %s\n", resp.GetReportPath())
			return err
		},
	}
	cmd.Flags().String("root", "", "ader/loops root directory (default: auto-detect)")
	cmd.Flags().Bool("dry-run", false, "print the execution plan without running")
	cmd.Flags().Int("max-retries", 3, "override max retries per step")
	cmd.Flags().String("coder-profile", "", "AI profile for the coder (overrides program.yaml)")
	cmd.Flags().String("evaluator-profile", "", "AI profile for the evaluator (overrides program.yaml)")
	return cmd
}

func newEnqueueCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enqueue <program-dir>",
		Short: "Copy a program into the next queue slot",
		Args: func(cmd *cobra.Command, args []string) error {
			fromCookbook, _ := cmd.Flags().GetString("from-cookbook")
			switch {
			case strings.TrimSpace(fromCookbook) != "" && len(args) != 0:
				return fmt.Errorf("--from-cookbook and <program-dir> are mutually exclusive")
			case strings.TrimSpace(fromCookbook) != "" && len(args) == 0:
				return nil
			case strings.TrimSpace(fromCookbook) == "" && len(args) != 1:
				return cobra.ExactArgs(1)(cmd, args)
			default:
				return nil
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := commandViper(cmd)
			programDir := ""
			if len(args) == 1 {
				programDir = args[0]
			}
			resp, err := Enqueue(&jamesloopsv1.EnqueueRequest{
				Root:         cfg.GetString("enqueue.root"),
				ProgramDir:   programDir,
				FromCookbook: cfg.GetString("enqueue.from-cookbook"),
			})
			if err != nil {
				return err
			}
			return printSlotLine(stdout, resp.GetSummary())
		},
	}
	cmd.Flags().String("from-cookbook", "", "instantiate from cookbook instead")
	cmd.Flags().String("root", "", "ader/loops root directory")
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	}
	_ = cmd.RegisterFlagCompletionFunc("from-cookbook", completeCookbookNames)
	return cmd
}

func newListCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show queue, live, deferred, and done slots",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := commandViper(cmd)
			resp, err := List(&jamesloopsv1.ListRequest{Root: cfg.GetString("list.root")})
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(stdout, "SLOT\tSTATE\tSTEPS\tBRANCH\tDESCRIPTION"); err != nil {
				return err
			}
			for _, item := range flattenStatus(resp.GetStatus()) {
				if err := printSlotLine(stdout, item); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().String("root", "", "ader/loops root directory")
	return cmd
}

func newStatusCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Detailed status of the last run (live, done, and deferred programs)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := commandViper(cmd)
			resp, err := Status(&jamesloopsv1.StatusRequest{Root: cfg.GetString("status.root")})
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, resp.GetReportMarkdown())
			return err
		},
	}
	cmd.Flags().String("root", "", "ader/loops root directory")
	return cmd
}

func newDropCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop <slot>",
		Short: "Remove a queued or deferred slot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := commandViper(cmd)
			resp, err := Drop(&jamesloopsv1.DropRequest{
				Root:     cfg.GetString("drop.root"),
				Slot:     args[0],
				Deferred: cfg.GetBool("drop.deferred"),
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "dropped %s\n", resp.GetSlot())
			return err
		},
	}
	cmd.Flags().Bool("deferred", false, "target the deferred/ directory instead of queue/")
	cmd.Flags().String("root", "", "ader/loops root directory")
	cmd.ValidArgsFunction = completeDropSlots
	return cmd
}

func newResumeCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume a paused live program from the current step",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := commandViper(cmd)
			resp, err := Resume(&jamesloopsv1.ResumeRequest{Root: cfg.GetString("resume.root")})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "report: %s\n", resp.GetReportPath())
			return err
		},
	}
	cmd.Flags().String("root", "", "ader/loops root directory")
	return cmd
}

func newSkipCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skip",
		Short: "Skip the current failing step and continue to the next",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := commandViper(cmd)
			resp, err := Skip(&jamesloopsv1.SkipRequest{Root: cfg.GetString("skip.root")})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "skipped %s next=%s\n", resp.GetSkippedStep(), resp.GetNextStep())
			return err
		},
	}
	cmd.Flags().String("root", "", "ader/loops root directory")
	return cmd
}

func newAbortCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "abort",
		Short: "Abort the live program and move it to deferred/",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := commandViper(cmd)
			resp, err := Abort(&jamesloopsv1.AbortRequest{Root: cfg.GetString("abort.root")})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "deferred %s\n", resp.GetDeferredSlot())
			return err
		},
	}
	cmd.Flags().String("root", "", "ader/loops root directory")
	return cmd
}

func newReEnqueueCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "re-enqueue <deferred-slot>",
		Short: "Move a deferred program back into the queue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := commandViper(cmd)
			resp, err := ReEnqueue(&jamesloopsv1.ReEnqueueRequest{
				Root: cfg.GetString("re-enqueue.root"),
				Slot: args[0],
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "%s -> %s\n", resp.GetFromSlot(), resp.GetToSlot())
			return err
		},
	}
	cmd.Flags().String("root", "", "ader/loops root directory")
	cmd.ValidArgsFunction = completeDeferredSlots
	return cmd
}

func newLogCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <step-id>",
		Short: "Show attempt history and gate reports for a step",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := commandViper(cmd)
			resp, err := Log(&jamesloopsv1.LogRequest{
				Root:   cfg.GetString("log.root"),
				StepId: args[0],
			})
			if err != nil {
				return err
			}
			for i, attempt := range resp.GetAttempts() {
				if _, err := fmt.Fprintf(stdout, "attempt %d\t%s\t%s\t%s\t%s\t%s\n",
					i+1,
					attempt.GetStartedAt(),
					attempt.GetGateResult(),
					attempt.GetGateReport(),
					attempt.GetDiffPatch(),
					resp.GetSlot(),
				); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().String("root", "", "ader/loops root directory")
	cmd.ValidArgsFunction = completeLogSteps
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
			return serverpkg.ListenAndServe(cfg.GetStringSlice("serve.listen"), cfg.GetBool("serve.reflect"), RPCHandler{})
		},
	}
	cmd.Flags().StringSlice("listen", nil, "listen URI(s)")
	cmd.Flags().Bool("reflect", false, "enable gRPC reflection")
	return cmd
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
			status, err := ensureProfileSnippet(profile, "# james-loops CLI autocompletion", line)
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

func commandViper(cmd *cobra.Command) *viper.Viper {
	cfg := viper.New()
	cfg.SetEnvPrefix("JAMES_LOOPS")
	cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	cfg.AutomaticEnv()
	for _, flag := range []string{"root", "dry-run", "max-retries", "coder-profile", "evaluator-profile", "from-cookbook", "deferred"} {
		if cmd.Flags().Lookup(flag) == nil {
			continue
		}
		mustBind(cfg, cmd.Name()+"."+flag, cmd, flag)
	}
	return cfg
}

func mustBind(cfg *viper.Viper, key string, cmd *cobra.Command, flag string) {
	_ = cfg.BindPFlag(key, cmd.Flags().Lookup(flag))
}

func findCommand(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func completeCookbookNames(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	items, err := engine.ListCookbookTemplates(completionRoot(cmd))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeDropSlots(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	deferred, _ := cmd.Flags().GetBool("deferred")
	dirName := "queue"
	if deferred {
		dirName = "deferred"
	}
	items, err := engine.ListSlotsForState(completionRoot(cmd), dirName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeDeferredSlots(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	items, err := engine.ListSlotsForState(completionRoot(cmd), "deferred")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeLogSteps(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	items, err := engine.ListLogStepIDs(completionRoot(cmd))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionValues(items, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completionRoot(cmd *cobra.Command) string {
	if cmd == nil || cmd.Flags().Lookup("root") == nil {
		return ""
	}
	root, _ := cmd.Flags().GetString("root")
	return root
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

func flattenStatus(status *jamesloopsv1.StatusSnapshot) []*jamesloopsv1.SlotSummary {
	if status == nil {
		return nil
	}
	var items []*jamesloopsv1.SlotSummary
	items = append(items, status.GetQueueSlots()...)
	if status.GetLiveSlot() != nil {
		items = append(items, status.GetLiveSlot())
	}
	items = append(items, status.GetDeferredSlots()...)
	items = append(items, status.GetDoneSlots()...)
	return items
}

func printSlotLine(stdout io.Writer, summary *jamesloopsv1.SlotSummary) error {
	if summary == nil {
		return nil
	}
	branch := summary.GetBranch()
	if strings.TrimSpace(branch) == "" {
		branch = "—"
	}
	_, err := fmt.Fprintf(stdout, "%s\t%s\t%d/%d\t%s\t%s\n",
		summary.GetSlot(),
		summary.GetState(),
		summary.GetStepsPassed(),
		summary.GetStepsTotal(),
		branch,
		summary.GetDescription(),
	)
	return err
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
		line = `eval "$(james-loops completion zsh)"`
		if strings.TrimSpace(overrideProfile) != "" {
			profile = overrideProfile
		} else {
			profile = filepath.Join(home, ".zshrc")
		}
	case "bash":
		line = `source <(james-loops completion bash)`
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
