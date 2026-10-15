package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/Crykk/JobHunter/internal/httpx"
	"github.com/Crykk/JobHunter/internal/job"
	"github.com/Crykk/JobHunter/internal/parse"
)

// Fetcher is the HTTP surface used by the scraper (real client or httptest).
type Fetcher interface {
	Get(ctx context.Context, url string) (body []byte, status int, header http.Header, err error)
}

// Scraper fetches Duunitori list and detail pages over plain HTTP.
type Scraper struct {
	Fetch Fetcher
	Log   io.Writer
}

func New(f Fetcher) *Scraper {
	if f == nil {
		f = httpx.New()
	}
	return &Scraper{Fetch: f}
}

func (s *Scraper) logf(format string, args ...any) {
	if s.Log == nil {
		return
	}
	fmt.Fprintf(s.Log, format+"\n", args...)
}

// ListPages walks sivu pagination (20/page) until last page, --pages cap, or HTTP 404.
func (s *Scraper) ListPages(ctx context.Context, query, area string, maxPages int) ([]job.Card, error) {
	if maxPages <= 0 {
		maxPages = 1
	}
	var (
		all      []job.Card
		seen     = map[string]struct{}{}
		lastPage = 0
	)
	for page := 1; page <= maxPages; page++ {
		if lastPage > 0 && page > lastPage {
			break
		}
		u := parse.PageURL(query, area, page)
		s.logf("fetching list page %s", parse.FormatPage(page, lastPage))
		body, status, _, err := s.Fetch.Get(ctx, u)
		if err != nil {
			return all, err
		}
		if status == http.StatusNotFound {
			s.logf("list page %d returned 404; stopping", page)
			break
		}
		if status < 200 || status >= 300 {
			return all, fmt.Errorf("list fetch HTTP %d for %s", status, u)
		}
		parsed, err := parse.ParseList(body, u)
		if err != nil {
			return all, err
		}
		if parsed.LastPage > 0 {
			lastPage = parsed.LastPage
		}
		added := 0
		for _, c := range parsed.Cards {
			if _, dup := seen[c.ID]; dup {
				continue
			}
			seen[c.ID] = struct{}{}
			all = append(all, c)
			added++
		}
		s.logf("page %d: %d cards (%d new, %d unique so far)", page, len(parsed.Cards), added, len(all))
		if lastPage == 0 && len(parsed.Cards) == 0 {
			break
		}
		if lastPage > 0 && page >= lastPage {
			break
		}
		if len(parsed.Cards) == 0 {
			break
		}
	}
	return all, nil
}

// Details fetches surviving job pages and strips HTML to text.
func (s *Scraper) Details(ctx context.Context, cards []job.Card) ([]job.Detail, error) {
	out := make([]job.Detail, 0, len(cards))
	for i, c := range cards {
		s.logf("fetching detail %d/%d %s", i+1, len(cards), c.ID)
		body, status, _, err := s.Fetch.Get(ctx, c.URL)
		if err != nil {
			return out, err
		}
		if status == http.StatusNotFound {
			s.logf("skip missing job %s (404)", c.ID)
			continue
		}
		if status < 200 || status >= 300 {
			return out, fmt.Errorf("detail fetch HTTP %d for %s", status, c.URL)
		}
		d, err := parse.ParseDetail(body, c.URL)
		if err != nil {
			return out, err
		}
		if d.ID == "" {
			d.ID = c.ID
		}
		if d.Title == "" {
			d.Title = c.Title
		}
		if d.Company == "" {
			d.Company = c.Company
		}
		if d.Location == "" {
			d.Location = c.Location
		}
		if d.Posted == "" {
			d.Posted = c.Posted
		}
		if d.URL == "" {
			d.URL = c.URL
		}
		out = append(out, *d)
	}
	return out, nil
}
