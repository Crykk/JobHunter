package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Crykk/JobHunter/internal/filter"
	"github.com/Crykk/JobHunter/internal/httpx"
	"github.com/Crykk/JobHunter/internal/job"
	"github.com/Crykk/JobHunter/internal/openrouter"
	"github.com/Crykk/JobHunter/internal/output"
	"github.com/Crykk/JobHunter/internal/profile"
	"github.com/Crykk/JobHunter/internal/rank"
	"github.com/Crykk/JobHunter/internal/scraper"
	"github.com/Crykk/JobHunter/internal/settings"
)

// Options is a Duunitori → Luna → Argon search.
type Options struct {
	Query       string
	Area        string
	Pages       int
	Threshold   float64
	Profile     string
	Out         string
	FilterModel string
	RankModel   string
	APIKey      string
}

// Result is the ranked matches plus the JSON payload written to Out.
type Result struct {
	File    job.ResultFile
	Matches []job.Match
}

func logf(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, format+"\n", args...)
}

func wrapNet(err error) error {
	var blocked *httpx.ErrBlocked
	if errors.As(err, &blocked) {
		return blocked
	}
	return err
}

func emptyFile(opts Options, kept int) job.ResultFile {
	return job.ResultFile{
		Query:   opts.Query,
		Area:    opts.Area,
		Pages:   opts.Pages,
		Kept:    kept,
		Matches: []job.Match{},
		Profile: opts.Profile,
		Source:  "duunitori.fi",
	}
}

// Run fetches list cards, filters with Luna, ranks with Argon, and writes JSON to opts.Out.
func Run(ctx context.Context, opts Options, log io.Writer) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	opts.Query = strings.TrimSpace(opts.Query)
	if opts.Query == "" {
		return nil, errors.New("query is required")
	}
	if strings.TrimSpace(opts.Profile) == "" || opts.Profile == "profile.txt" {
		p, err := profile.Ensure()
		if err != nil {
			return nil, fmt.Errorf("create profile: %w", err)
		}
		opts.Profile = p
	}
	if strings.TrimSpace(opts.Out) == "" {
		opts.Out = output.DefaultJSON
	}
	if opts.Pages <= 0 {
		opts.Pages = 3
	}
	cfg := settings.Normalize(settings.File{FilterModel: opts.FilterModel, RankModel: opts.RankModel})
	if strings.TrimSpace(opts.FilterModel) == "" && strings.TrimSpace(opts.RankModel) == "" {
		cfg = settings.Load()
	}
	opts.FilterModel = cfg.FilterModel
	opts.RankModel = cfg.RankModel

	profile, err := os.ReadFile(opts.Profile)
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %w (copy profile.example.txt to profile.txt)", opts.Profile, err)
	}
	apiKey := strings.TrimSpace(opts.APIKey)
	if apiKey == "" {
		apiKey = settings.ResolveAPIKey()
	}
	if apiKey == "" {
		return nil, errors.New("OpenRouter API key is not set (paste it in the JobHunter UI, or set OPENROUTER_API_KEY)")
	}

	httpClient := httpx.New()
	sc := scraper.New(httpClient)
	sc.Log = log
	or := openrouter.New(apiKey)

	logf(log, "searching Duunitori query=%q area=%q pages<=%d", opts.Query, opts.Area, opts.Pages)
	cards, err := sc.ListPages(ctx, opts.Query, opts.Area, opts.Pages)
	if err != nil {
		return nil, wrapNet(err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		logf(log, "no job cards parsed from list pages")
		file := emptyFile(opts, 0)
		if err := output.JSONFile(opts.Out, file); err != nil {
			return nil, err
		}
		return &Result{File: file, Matches: file.Matches}, nil
	}

	fl := &filter.Filter{OR: or, Model: opts.FilterModel, Threshold: opts.Threshold, Log: log}
	kept, _, err := fl.Apply(ctx, string(profile), cards)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(kept) == 0 {
		logf(log, "%s kept 0 jobs", settings.FilterLogName(opts.FilterModel))
		file := emptyFile(opts, 0)
		if err := output.JSONFile(opts.Out, file); err != nil {
			return nil, err
		}
		return &Result{File: file, Matches: file.Matches}, nil
	}

	details, err := sc.Details(ctx, kept)
	if err != nil {
		return nil, wrapNet(err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rk := &rank.Ranker{OR: or, Model: opts.RankModel, Log: log}
	matches, err := rk.Rank(ctx, string(profile), details)
	if err != nil {
		return nil, err
	}
	if matches == nil {
		matches = []job.Match{}
	}

	file := job.ResultFile{
		Query:   opts.Query,
		Area:    opts.Area,
		Pages:   opts.Pages,
		Kept:    len(kept),
		Matches: matches,
		Profile: opts.Profile,
		Source:  "duunitori.fi",
	}
	if err := output.JSONFile(opts.Out, file); err != nil {
		return nil, err
	}
	logf(log, "wrote %s (%d matches)", opts.Out, len(matches))
	return &Result{File: file, Matches: matches}, nil
}
