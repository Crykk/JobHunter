package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Crykk/JobHunter/internal/tui"
)

// Execute runs the jobhunter cobra command.
func Execute(version, commit, date string) error {
	root := newRoot(version, commit, date, os.Stdout, os.Stderr)
	return root.Execute()
}

func writerIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func newRoot(version, commit, date string, stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "jobhunter",
		Short:         "Terminal UI for Duunitori search (Luna + Argon). Use `search` for scripts.",
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
		if len(args) == 0 && writerIsTTY(stdout) {
			return tui.Run(version, commit, date)
		}
		return c.Help()
	}
	return cmd
}
