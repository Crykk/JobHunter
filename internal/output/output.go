package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/Crykk/JobHunter/internal/job"
)

// Table writes a ranked table to stdout.
func Table(w io.Writer, matches []job.Match) error {
	if len(matches) == 0 {
		fmt.Fprintln(w, "No matching jobs.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tSCORE\tTITLE\tCOMPANY\tLOCATION\tWHY")
	for i, m := range matches {
		fmt.Fprintf(tw, "%d\t%d\t%s\t%s\t%s\t%s\n",
			i+1, m.Score, clip(m.Title, 48), clip(m.Company, 28), clip(m.Location, 24), clip(m.Why, 80))
	}
	return tw.Flush()
}

// JSONFile writes output/matches.json (or --out).
func JSONFile(path string, file job.ResultFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	if n < 4 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
