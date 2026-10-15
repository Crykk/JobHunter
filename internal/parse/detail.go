package parse

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/Crykk/JobHunter/internal/job"
)

var htmlTagRe = regexp.MustCompile(`(?s)<[^>]+>`)

type jobPostingLD struct {
	Type               any             `json:"@type"`
	Title              string          `json:"title"`
	Description        string          `json:"description"`
	DatePosted         string          `json:"datePosted"`
	URL                string          `json:"url"`
	HiringOrganization json.RawMessage `json:"hiringOrganization"`
	JobLocation        json.RawMessage `json:"jobLocation"`
}

type orgLD struct {
	Name string `json:"name"`
}

type locLD struct {
	Address struct {
		Locality string `json:"addressLocality"`
		Region   string `json:"addressRegion"`
	} `json:"address"`
}

// ParseDetail strips a Duunitori job page to title, company, location, description, apply URL.
func ParseDetail(html []byte, pageURL string) (*job.Detail, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(html)))
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(pageURL)
	id, abs := jobIDFromHref(pageURL, base)

	d := &job.Detail{ID: id, URL: abs}
	if d.URL == "" {
		d.URL = pageURL
	}

	applyJSONLD(doc, d)
	if d.Title == "" {
		d.Title = collapseSpace(doc.Find("h1, .job-header__title, .gtm-job-title").First().Text())
	}
	if d.Company == "" {
		d.Company = collapseSpace(doc.Find(".job-header__company, .gtm-employer-name, a[href*='/yritys/']").First().Text())
	}
	if d.Location == "" {
		d.Location = collapseSpace(doc.Find(".job-header__location, [class*='job-location']").First().Text())
	}
	if d.Description == "" {
		desc := doc.Find(".description-box, .job-description, [itemprop='description'], .gtm-job-description").First()
		if desc.Length() > 0 {
			d.Description = collapseSpace(desc.Text())
		}
	}
	if d.ApplyURL == "" {
		href, ok := doc.Find(`a[href][class*='apply'], a.btn--apply, a[data-apply], a:contains("Hae")`).First().Attr("href")
		if ok {
			d.ApplyURL = resolveURL(href, base)
		}
	}

	d.Title = collapseSpace(d.Title)
	d.Company = collapseSpace(d.Company)
	d.Location = collapseSpace(d.Location)
	d.Description = collapseSpace(stripHTML(d.Description))
	return d, nil
}

func applyJSONLD(doc *goquery.Document, d *job.Detail) {
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Text())
		if raw == "" || !strings.Contains(raw, "JobPosting") {
			return
		}
		var posting jobPostingLD
		if err := json.Unmarshal([]byte(raw), &posting); err != nil {
			// Sometimes a @graph array.
			var wrap struct {
				Graph []jobPostingLD `json:"@graph"`
			}
			if err2 := json.Unmarshal([]byte(raw), &wrap); err2 != nil {
				return
			}
			for _, g := range wrap.Graph {
				if typeIsJobPosting(g.Type) {
					posting = g
					break
				}
			}
		}
		if posting.Title != "" && d.Title == "" {
			d.Title = posting.Title
		}
		if posting.Description != "" && d.Description == "" {
			d.Description = stripHTML(posting.Description)
		}
		if posting.DatePosted != "" && d.Posted == "" {
			d.Posted = posting.DatePosted
		}
		if posting.URL != "" && d.URL == "" {
			d.URL = posting.URL
		}
		if d.Company == "" {
			d.Company = orgName(posting.HiringOrganization)
		}
		if d.Location == "" {
			d.Location = locality(posting.JobLocation)
		}
	})
}

func typeIsJobPosting(t any) bool {
	switch v := t.(type) {
	case string:
		return strings.EqualFold(v, "JobPosting")
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && strings.EqualFold(s, "JobPosting") {
				return true
			}
		}
	}
	return false
}

func orgName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var o orgLD
	if json.Unmarshal(raw, &o) == nil && o.Name != "" {
		return o.Name
	}
	var arr []orgLD
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		return arr[0].Name
	}
	return ""
}

func locality(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var loc locLD
	if json.Unmarshal(raw, &loc) == nil {
		if loc.Address.Locality != "" {
			return loc.Address.Locality
		}
		return loc.Address.Region
	}
	var arr []locLD
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		if arr[0].Address.Locality != "" {
			return arr[0].Address.Locality
		}
		return arr[0].Address.Region
	}
	return ""
}

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	return collapseSpace(s)
}

func resolveURL(href string, base *url.URL) string {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return href
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	return u.String()
}
