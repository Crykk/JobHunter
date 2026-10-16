package cli

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/Crykk/JobHunter/internal/output"
	"github.com/Crykk/JobHunter/internal/pipeline"
)

type searchOpts struct {
	query     string
	area      string
	pages     int
	threshold float64
	profile   string
	out       string
}

func newSearchCmd(stdout, stderr io.Writer) *cobra.Command {
	opts := &searchOpts{}
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Duunitori, filter with Luna, rank with Argon",
		Example: `  jobhunter search --query "python" --area "Helsinki" --pages 3
  jobhunter search -q golang -a Tampere --threshold 0.6 --profile profile.txt`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := pipeline.Run(cmd.Context(), pipeline.Options{
				Query:     opts.query,
				Area:      opts.area,
				Pages:     opts.pages,
				Threshold: opts.threshold,
				Profile:   opts.profile,
				Out:       opts.out,
			}, stderr)
			if err != nil {
				return err
			}
			return output.Table(stdout, res.Matches)
		},
	}
	cmd.Flags().StringVarP(&opts.query, "query", "q", "", "Duunitori haku= search text (required)")
	cmd.Flags().StringVarP(&opts.area, "area", "a", "", "Duunitori alue= location")
	cmd.Flags().IntVarP(&opts.pages, "pages", "p", 3, "max list pages (20 jobs per page)")
	cmd.Flags().Float64Var(&opts.threshold, "threshold", 0.55, "minimum Luna noul score to keep a job")
	cmd.Flags().StringVar(&opts.profile, "profile", "profile.txt", "candidate profile path")
	cmd.Flags().StringVar(&opts.out, "out", output.DefaultJSON, "ranked JSON output path")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}
