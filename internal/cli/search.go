package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Crykk/JobHunter/internal/filter"
	"github.com/Crykk/JobHunter/internal/httpx"
	"github.com/Crykk/JobHunter/internal/job"
	"github.com/Crykk/JobHunter/internal/openrouter"
	"github.com/Crykk/JobHunter/internal/output"
	"github.com/Crykk/JobHunter/internal/rank"
	"github.com/Crykk/JobHunter/internal/scraper"
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
			return runSearch(cmd.Context(), opts, stdout, stderr)
		},
	}
	cmd.Flags().StringVarP(&opts.query, "query", "q", "", "Duunitori haku= search text (required)")
	cmd.Flags().StringVarP(&opts.area, "area", "a", "", "Duunitori alue= location")
	cmd.Flags().IntVarP(&opts.pages, "pages", "p", 3, "max list pages (20 jobs per page)")
	cmd.Flags().Float64Var(&opts.threshold, "threshold", 0.55, "minimum Luna noul score to keep a job")
	cmd.Flags().StringVar(&opts.profile, "profile", "profile.txt", "candidate profile path")
	cmd.Flags().StringVar(&opts.out, "out", "output/matches.json", "ranked JSON output path")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}

func runSearch(ctx context.Context, opts *searchOpts, stdout, stderr io.Writer) error {
	if strings.TrimSpace(opts.query) == "" {
		return errors.New("--query is required")
	}
	profile, err := os.ReadFile(opts.profile)
	if err != nil {
		return fmt.Errorf("read profile %s: %w (copy profile.example.txt to profile.txt)", opts.profile, err)
	}
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		return errors.New("OPENROUTER_API_KEY is not set")
	}

	httpClient := httpx.New()
	sc := scraper.New(httpClient)
	sc.Log = stderr
	or := openrouter.New(apiKey)

	fmt.Fprintf(stderr, "searching Duunitori query=%q area=%q pages<=%d\n", opts.query, opts.area, opts.pages)
	cards, err := sc.ListPages(ctx, opts.query, opts.area, opts.pages)
	if err != nil {
		return wrapNet(err)
	}
	if len(cards) == 0 {
		fmt.Fprintln(stderr, "no job cards parsed from list pages")
		fmt.Fprintln(stdout, "No matching jobs.")
		return output.JSONFile(opts.out, job.ResultFile{
			Query: opts.query, Area: opts.area, Pages: opts.pages,
			Profile: opts.profile, Source: "duunitori.fi", Matches: []job.Match{},
		})
	}

	fl := &filter.Filter{OR: or, Threshold: opts.threshold, Log: stderr}
	kept, _, err := fl.Apply(ctx, string(profile), cards)
	if err != nil {
		return err
	}
	if len(kept) == 0 {
		fmt.Fprintln(stderr, "luna kept 0 jobs")
		fmt.Fprintln(stdout, "No matching jobs.")
		return output.JSONFile(opts.out, job.ResultFile{
			Query: opts.query, Area: opts.area, Pages: opts.pages, Kept: 0,
			Profile: opts.profile, Source: "duunitori.fi", Matches: []job.Match{},
		})
	}

	details, err := sc.Details(ctx, kept)
	if err != nil {
		return wrapNet(err)
	}

	rk := &rank.Ranker{OR: or, Log: stderr}
	matches, err := rk.Rank(ctx, string(profile), details)
	if err != nil {
		return err
	}

	if err := output.Table(stdout, matches); err != nil {
		return err
	}
	file := job.ResultFile{
		Query:   opts.query,
		Area:    opts.area,
		Pages:   opts.pages,
		Kept:    len(kept),
		Matches: matches,
		Profile: opts.profile,
		Source:  "duunitori.fi",
	}
	if err := output.JSONFile(opts.out, file); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "wrote %s (%d matches)\n", opts.out, len(matches))
	return nil
}

func wrapNet(err error) error {
	var blocked *httpx.ErrBlocked
	if errors.As(err, &blocked) {
		return blocked
	}
	return err
}
