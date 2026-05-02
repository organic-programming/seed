package cli

import (
	"fmt"
	"strings"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/spf13/cobra"
)

func newSdkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sdk",
		Short: "Manage SDK native prebuilts",
	}
	cmd.AddCommand(
		newSdkInstallCmd(),
		newSdkBuildCmd(),
		newSdkListCmd(),
		newSdkUninstallCmd(),
		newSdkVerifyCmd(),
		newSdkPathCmd(),
	)
	return cmd
}

func newSdkBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build <lang>",
		Short: "Build a SDK prebuilt from local sources and install it",
		Long: "build invokes the per-SDK script under .github/scripts/ to compile\n" +
			"a SDK prebuilt from the gRPC + per-SDK sources in this checkout,\n" +
			"then installs the resulting tarball into $OPPATH/sdk. Long-running\n" +
			"(~30-60 min cold). Use install instead when a published release is\n" +
			"available.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeCompilableSdkLangs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			resp, err := api.BuildSdkPrebuilt(sdkBuildRequestFromCommand(cmd, args[0]))
			if err != nil {
				return fmt.Errorf("op sdk build: %w", err)
			}
			writeFormattedSDKResponse(cmd, format, resp)
			return nil
		},
	}
	cmd.Flags().String("target", "", "target triplet (defaults to host)")
	cmd.Flags().String("version", "", "SDK prebuilt version")
	cmd.Flags().Int("jobs", 0, "compile parallelism (0 = sensible default)")
	cmd.Flags().Bool("force", false, "rebuild even if a cached tarball exists")
	cmd.Flags().Bool("no-install", false, "leave tarball in dist/ instead of installing")
	_ = cmd.RegisterFlagCompletionFunc("target", completeAllowedSdkTargets)
	_ = cmd.RegisterFlagCompletionFunc("version", completeSdkVersionsForLang)
	return cmd
}

func newSdkInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "install <lang>",
		Short:             "Install an SDK prebuilt",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAvailableSdkLangs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			resp, err := api.InstallSdkPrebuilt(sdkInstallRequestFromCommand(cmd, args[0]))
			if err != nil {
				return fmt.Errorf("op sdk install: %w", err)
			}
			writeFormattedSDKResponse(cmd, format, resp)
			return nil
		},
	}
	cmd.Flags().String("target", "", "target triplet (defaults to host, or to --source manifest target)")
	cmd.Flags().String("version", "", "SDK prebuilt version")
	cmd.Flags().String("source", "", "local tarball or URL source")
	_ = cmd.RegisterFlagCompletionFunc("target", completeAllowedSdkTargets)
	_ = cmd.RegisterFlagCompletionFunc("version", completeSdkVersionsForLang)
	return cmd
}

func newSdkListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SDK prebuilts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			resp, err := api.ListSdkPrebuilts(sdkListRequestFromCommand(cmd))
			if err != nil {
				return fmt.Errorf("op sdk list: %w", err)
			}
			writeFormattedSDKResponse(cmd, format, resp)
			return nil
		},
	}
	cmd.Flags().Bool("installed", false, "list installed prebuilts")
	cmd.Flags().Bool("available", false, "list available release prebuilts")
	cmd.Flags().Bool("compilable", false, "list SDKs that op sdk build can build now")
	cmd.Flags().String("lang", "", "filter by SDK language")
	_ = cmd.RegisterFlagCompletionFunc("lang", completeAllSdkLangs)
	return cmd
}

func newSdkUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "uninstall <lang>",
		Short:             "Uninstall an SDK prebuilt",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeInstalledSdkLangs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			resp, err := api.UninstallSdkPrebuilt(sdkUninstallRequestFromCommand(cmd, args[0]))
			if err != nil {
				return fmt.Errorf("op sdk uninstall: %w", err)
			}
			writeFormattedSDKResponse(cmd, format, resp)
			return nil
		},
	}
	cmd.Flags().String("target", "", "target triplet (defaults to host)")
	cmd.Flags().String("version", "", "SDK prebuilt version")
	_ = cmd.RegisterFlagCompletionFunc("target", completeAllowedSdkTargets)
	_ = cmd.RegisterFlagCompletionFunc("version", completeSdkVersionsForLang)
	return cmd
}

func newSdkVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "verify <lang>",
		Short:             "Verify an installed SDK prebuilt",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeInstalledSdkLangs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			resp, err := api.VerifySdkPrebuilt(sdkVerifyRequestFromCommand(cmd, args[0]))
			if err != nil {
				return fmt.Errorf("op sdk verify: %w", err)
			}
			writeFormattedSDKResponse(cmd, format, resp)
			return nil
		},
	}
	cmd.Flags().String("target", "", "target triplet (defaults to host)")
	cmd.Flags().String("version", "", "SDK prebuilt version")
	_ = cmd.RegisterFlagCompletionFunc("target", completeAllowedSdkTargets)
	_ = cmd.RegisterFlagCompletionFunc("version", completeSdkVersionsForLang)
	return cmd
}

func newSdkPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "path <lang>",
		Short:             "Print an installed SDK prebuilt path",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeInstalledSdkLangs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			resp, err := api.LocateSdkPrebuilt(sdkLocateRequestFromCommand(cmd, args[0]))
			if err != nil {
				return fmt.Errorf("op sdk path: %w", err)
			}
			if format == FormatJSON {
				writeFormattedSDKResponse(cmd, format, resp)
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), resp.GetPrebuilt().GetPath())
			return nil
		},
	}
	cmd.Flags().String("target", "", "target triplet (defaults to host)")
	cmd.Flags().String("version", "", "SDK prebuilt version")
	_ = cmd.RegisterFlagCompletionFunc("target", completeAllowedSdkTargets)
	_ = cmd.RegisterFlagCompletionFunc("version", completeSdkVersionsForLang)
	return cmd
}

func sdkInstallRequestFromCommand(cmd *cobra.Command, lang string) *opv1.InstallSdkPrebuiltRequest {
	target, _ := cmd.Flags().GetString("target")
	version, _ := cmd.Flags().GetString("version")
	source, _ := cmd.Flags().GetString("source")
	return &opv1.InstallSdkPrebuiltRequest{
		Lang:    strings.TrimSpace(lang),
		Target:  strings.TrimSpace(target),
		Version: strings.TrimSpace(version),
		Source:  strings.TrimSpace(source),
	}
}

func sdkListRequestFromCommand(cmd *cobra.Command) *opv1.ListSdkPrebuiltsRequest {
	installed, _ := cmd.Flags().GetBool("installed")
	available, _ := cmd.Flags().GetBool("available")
	compilable, _ := cmd.Flags().GetBool("compilable")
	lang, _ := cmd.Flags().GetString("lang")
	if !installed && !available && !compilable {
		installed = true
	}
	return &opv1.ListSdkPrebuiltsRequest{
		Installed:  installed,
		Available:  available,
		Compilable: compilable,
		Lang:       strings.TrimSpace(lang),
	}
}

func sdkBuildRequestFromCommand(cmd *cobra.Command, lang string) *opv1.BuildSdkPrebuiltRequest {
	target, _ := cmd.Flags().GetString("target")
	version, _ := cmd.Flags().GetString("version")
	jobs, _ := cmd.Flags().GetInt("jobs")
	force, _ := cmd.Flags().GetBool("force")
	noInstall, _ := cmd.Flags().GetBool("no-install")
	return &opv1.BuildSdkPrebuiltRequest{
		Lang:              strings.TrimSpace(lang),
		Target:            strings.TrimSpace(target),
		Version:           strings.TrimSpace(version),
		Jobs:              int32(jobs),
		Force:             force,
		InstallAfterBuild: !noInstall,
	}
}

func sdkUninstallRequestFromCommand(cmd *cobra.Command, lang string) *opv1.UninstallSdkPrebuiltRequest {
	target, _ := cmd.Flags().GetString("target")
	version, _ := cmd.Flags().GetString("version")
	return &opv1.UninstallSdkPrebuiltRequest{
		Lang:    strings.TrimSpace(lang),
		Target:  strings.TrimSpace(target),
		Version: strings.TrimSpace(version),
	}
}

func sdkVerifyRequestFromCommand(cmd *cobra.Command, lang string) *opv1.VerifySdkPrebuiltRequest {
	target, _ := cmd.Flags().GetString("target")
	version, _ := cmd.Flags().GetString("version")
	return &opv1.VerifySdkPrebuiltRequest{
		Lang:    strings.TrimSpace(lang),
		Target:  strings.TrimSpace(target),
		Version: strings.TrimSpace(version),
	}
}

func sdkLocateRequestFromCommand(cmd *cobra.Command, lang string) *opv1.LocateSdkPrebuiltRequest {
	target, _ := cmd.Flags().GetString("target")
	version, _ := cmd.Flags().GetString("version")
	return &opv1.LocateSdkPrebuiltRequest{
		Lang:    strings.TrimSpace(lang),
		Target:  strings.TrimSpace(target),
		Version: strings.TrimSpace(version),
	}
}

func writeFormattedSDKResponse(cmd *cobra.Command, format Format, resp any) {
	switch typed := resp.(type) {
	case *opv1.SdkPrebuiltResponse:
		out := FormatResponse(format, typed)
		if strings.TrimSpace(out) != "" {
			fmt.Fprintln(cmd.OutOrStdout(), out)
		}
	case *opv1.ListSdkPrebuiltsResponse:
		out := FormatResponse(format, typed)
		if strings.TrimSpace(out) != "" {
			fmt.Fprintln(cmd.OutOrStdout(), out)
		}
	}
}
