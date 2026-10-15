package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// Execute runs the jobhunter cobra command.
func Execute(version, commit, date string) error {
	root := newRoot(version, commit, date, os.Stdout, os.Stderr)
	return root.Execute()
}

func newRoot(version, commit, date string, stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "jobhunter",
		Short:         "Filter Duunitori listings with Luna Decisions, then rank with Gemini 4 Argon",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.PersistentFlags().Bool("version", false, "print version")
	cmd.AddCommand(newSearchCmd(stdout, stderr))
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(c *cobra.Command, _ []string) {
			fmt.Fprintf(c.OutOrStdout(), "jobhunter %s (commit %s, built %s)\n", version, commit, date)
		},
	})
	cmd.RunE = func(c *cobra.Command, args []string) error {
		v, _ := c.Flags().GetBool("version")
		if v {
			fmt.Fprintf(c.OutOrStdout(), "jobhunter %s (commit %s, built %s)\n", version, commit, date)
			return nil
		}
		return c.Help()
	}
	return cmd
}
