package parse

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/cryk/jobhunter/internal/job"
)

const pageSize = 20

var (
	jobPathRe = regexp.MustCompile(`(?i)/tyopaikat/tyo/([^/?#]+)`)
	jobIDRe   = regexp.MustCompile(`(\d{5,})$`)
	totalRe   = regexp.MustCompile(`(?i)löysimme\s+([\d\s]+)\s+työpaikka`)
	shownRe   = regexp.MustCompile(`(?i)näytetään työpaikat\s+(\d+)\s*[-–]\s*(\d+)`)
	postedRe  = regexp.MustCompile(`(?i)julkaistu\s+([^–\n<]+)`)
)

// ListPage is the parsed search listing.
type ListPage struct {
	Cards     []job.Card
	Total     int
	PageSize  int
	LastPage  int
	ShownFrom int
	ShownTo   int
}

// ParseList extracts job cards from a Duunitori search HTML page.
// Prefers list-card markup; falls back to unique /tyopaikat/tyo/{slug}-{id} links.
func ParseList(html []byte, pageURL string) (*ListPage, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(html)))
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(pageURL)

	out := &ListPage{PageSize: pageSize}
	seen := map[string]struct{}{}

	// Classic job-box cards.
	doc.Find(".job-box, article.job-card, .job-card, [data-job-id]").Each(func(_ int, s *goquery.Selection) {
		card, ok := cardFromBox(s, base)
		if !ok {
			return
		}
		if _, dup := seen[card.ID]; dup {
			return
		}
		seen[card.ID] = struct{}{}
		out.Cards = append(out.Cards, card)
	})

	// Fallback: any job detail link (still list-page only — no description).
	if len(out.Cards) == 0 {
		doc.Find(`a[href*="/tyopaikat/tyo/"]`).Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			id, abs := jobIDFromHref(href, base)
			if id == "" {
				return
			}
			if _, dup := seen[id]; dup {
				return
			}
			title := strings.TrimSpace(s.Text())
			if title == "" {
				title = s.AttrOr("title", "")
			}
			if title == "" {
				return
			}
			company := strings.TrimSpace(s.AttrOr("data-company", ""))
			loc := nearbyText(s, ".job-box__job-location, .job-card__location, [class*='location']")
			posted := nearbyPosted(s)
			seen[id] = struct{}{}
			out.Cards = append(out.Cards, job.Card{
				ID:       id,
				Title:    collapseSpace(title),
				Company:  company,
				Location: loc,
				Posted:   posted,
				URL:      abs,
			})
		})
	}

	text := collapseSpace(doc.Find("body").Text())
	out.Total, out.ShownFrom, out.ShownTo = parseListCounts(text)
	if out.Total > 0 {
		out.LastPage = (out.Total + pageSize - 1) / pageSize
	}
	return out, nil
}

// PageURL builds https://duunitori.fi/tyopaikat?haku=...&alue=...&sivu=N
func PageURL(query, area string, page int) string {
	u, _ := url.Parse("https://duunitori.fi/tyopaikat")
	q := u.Query()
	q.Set("haku", query)
	if strings.TrimSpace(area) != "" {
		q.Set("alue", area)
	}
	if page > 1 {
		q.Set("sivu", strconv.Itoa(page))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func cardFromBox(s *goquery.Selection, base *url.URL) (job.Card, bool) {
	link := s.Find(`a[href*="/tyopaikat/tyo/"]`).First()
	if link.Length() == 0 {
		if href, ok := s.Attr("href"); ok && strings.Contains(href, "/tyopaikat/tyo/") {
			link = s
		} else {
			return job.Card{}, false
		}
	}
	href, _ := link.Attr("href")
	id, abs := jobIDFromHref(href, base)
	if id == "" {
		return job.Card{}, false
	}

	title := strings.TrimSpace(s.Find(".job-box__title, .job-card__title, h2, h3").First().Text())
	if title == "" {
		title = strings.TrimSpace(link.Text())
	}
	company := strings.TrimSpace(s.Find(".job-box__company, .job-card__company, [class*='company']").First().Text())
	if company == "" {
		company = strings.TrimSpace(s.AttrOr("data-company", ""))
	}
	location := strings.TrimSpace(s.Find(".job-box__job-location, .job-card__location, [class*='location']").First().Text())
	posted := strings.TrimSpace(s.Find(".job-box__job-posted, time, [class*='posted']").First().Text())
	if posted == "" {
		if m := postedRe.FindStringSubmatch(s.Text()); len(m) == 2 {
			posted = collapseSpace(m[1])
		}
	}
	title = collapseSpace(title)
	if title == "" {
		return job.Card{}, false
	}
	return job.Card{
		ID:       id,
		Title:    title,
		Company:  collapseSpace(company),
		Location: collapseSpace(location),
		Posted:   collapseSpace(posted),
		URL:      abs,
	}, true
}

func jobIDFromHref(href string, base *url.URL) (id, abs string) {
	href = strings.TrimSpace(href)
	if href == "" {
		return "", ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return "", ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	path := u.Path
	m := jobPathRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return "", ""
	}
	slug := m[1]
	idm := jobIDRe.FindStringSubmatch(slug)
	if len(idm) < 2 {
		return "", ""
	}
	return idm[1], u.String()
}

func parseListCounts(text string) (total, from, to int) {
	if m := totalRe.FindStringSubmatch(text); len(m) == 2 {
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, m[1])
		total, _ = strconv.Atoi(digits)
	}
	if m := shownRe.FindStringSubmatch(text); len(m) == 3 {
		from, _ = strconv.Atoi(m[1])
		to, _ = strconv.Atoi(m[2])
	}
	return total, from, to
}

func nearbyText(s *goquery.Selection, sel string) string {
	t := strings.TrimSpace(s.Find(sel).First().Text())
	if t != "" {
		return collapseSpace(t)
	}
	parent := s.ParentsFiltered(".job-box, .job-card, article, li").First()
	if parent.Length() > 0 {
		t = strings.TrimSpace(parent.Find(sel).First().Text())
	}
	return collapseSpace(t)
}

func nearbyPosted(s *goquery.Selection) string {
	scope := s.ParentsFiltered(".job-box, .job-card, article, li").First()
	if scope.Length() == 0 {
		scope = s.Parent()
	}
	if m := postedRe.FindStringSubmatch(scope.Text()); len(m) == 2 {
		return collapseSpace(m[1])
	}
	return ""
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// LastPageFromTotal is ceil(total/20).
func LastPageFromTotal(total int) int {
	if total <= 0 {
		return 0
	}
	return (total + pageSize - 1) / pageSize
}

// FormatPage reports list progress for stderr.
func FormatPage(n, last int) string {
	if last > 0 {
		return fmt.Sprintf("%d/%d", n, last)
	}
	return strconv.Itoa(n)
}
