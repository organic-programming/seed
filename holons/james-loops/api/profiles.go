package api

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/organic-programming/james-loops/internal/profile"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newProfileCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Inspect bundled and local james-loops profiles",
	}
	cmd.AddCommand(newProfileListCommand(stdout))
	cmd.AddCommand(newProfileShowCommand(stdout))
	cmd.AddCommand(newProfileValidateCommand(stdout))
	return cmd
}

func newProfileListCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available AI profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, err := profile.LoadAll()
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(stdout, "NAME\tDRIVER\tMODEL"); err != nil {
				return err
			}
			for _, item := range profiles {
				if _, err := fmt.Fprintf(stdout, "%s\t%s\t%s\n", item.Name, item.Driver, item.Model); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newProfileShowCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show the winning YAML definition for a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := profile.Load(args[0])
			if err != nil {
				return err
			}
			data, err := yaml.Marshal(item)
			if err != nil {
				return err
			}
			_, err = stdout.Write(data)
			return err
		},
	}
}

func newProfileValidateCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <name>",
		Short: "Validate a profile and confirm the backing CLI exists in PATH",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := profile.Load(args[0])
			if err != nil {
				return err
			}
			if err := item.Validate(); err != nil {
				return err
			}
			if _, err := exec.LookPath(item.DriverBinary()); err != nil {
				return fmt.Errorf("driver %q not found in PATH: %w", item.DriverBinary(), err)
			}
			_, err = fmt.Fprintf(stdout, "valid %s\t%s\t%s\n", item.Name, item.Driver, item.Model)
			return err
		},
	}
}
