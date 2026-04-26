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
		newSdkListCmd(),
		newSdkUninstallCmd(),
		newSdkVerifyCmd(),
		newSdkPathCmd(),
	)
	return cmd
}

func newSdkInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <lang>",
		Short: "Install an SDK prebuilt",
		Args:  cobra.ExactArgs(1),
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
	cmd.Flags().String("target", "", "target triplet (defaults to host)")
	cmd.Flags().String("version", "", "SDK prebuilt version")
	cmd.Flags().String("source", "", "local tarball or URL source")
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
	cmd.Flags().String("lang", "", "filter by SDK language")
	return cmd
}

func newSdkUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <lang>",
		Short: "Uninstall an SDK prebuilt",
		Args:  cobra.ExactArgs(1),
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
	return cmd
}

func newSdkVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify <lang>",
		Short: "Verify an installed SDK prebuilt",
		Args:  cobra.ExactArgs(1),
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
	return cmd
}

func newSdkPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path <lang>",
		Short: "Print an installed SDK prebuilt path",
		Args:  cobra.ExactArgs(1),
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
	lang, _ := cmd.Flags().GetString("lang")
	if !installed && !available {
		installed = true
	}
	return &opv1.ListSdkPrebuiltsRequest{
		Installed: installed,
		Available: available,
		Lang:      strings.TrimSpace(lang),
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
