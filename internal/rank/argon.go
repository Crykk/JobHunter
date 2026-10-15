package rank

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cryk/jobhunter/internal/job"
	"github.com/cryk/jobhunter/internal/openrouter"
)

// Chatter is the chat-completions subset used by Argon ranking.
type Chatter interface {
	ChatJSON(ctx context.Context, model, system, user string) (string, error)
}

// Ranker scores surviving job details with Gemini 4 Argon.
type Ranker struct {
	OR  Chatter
	Log io.Writer
}

const systemPrompt = `You rank Finnish job ads against a candidate profile.
Ads may be in Finnish or English. Score skills, location, seniority, and deal-breakers from the profile.
Return a JSON object only, of the form:
{"matches":[{"id":"...","title":"...","company":"...","location":"...","url":"...","score":0,"why":"short reason"}]}
score is an integer 0–100. Sort matches by score descending. Omit jobs that are clearly a poor fit (score below 40) unless the list would be empty — then return the best few anyway.
why must be one or two short sentences.`

func (r *Ranker) logf(format string, args ...any) {
	if r.Log == nil {
		return
	}
	fmt.Fprintf(r.Log, format+"\n", args...)
}

type argonFile struct {
	Matches []job.Match `json:"matches"`
}

// Rank calls google/gemini-4-argon via OpenRouter chat completions.
func (r *Ranker) Rank(ctx context.Context, profile string, details []job.Detail) ([]job.Match, error) {
	if len(details) == 0 {
		return nil, nil
	}
	r.logf("argon: ranking %d jobs", len(details))
	user, err := json.Marshal(map[string]any{
		"profile": profile,
		"jobs":    details,
	})
	if err != nil {
		return nil, err
	}
	raw, err := r.OR.ChatJSON(ctx, openrouter.ArgonModel, systemPrompt, string(user))
	if err != nil {
		return nil, err
	}
	parsed, err := parseMatches(raw)
	if err != nil {
		return nil, err
	}
	fillGaps(parsed, details)
	sortByScore(parsed)
	return parsed, nil
}

func parseMatches(raw string) ([]job.Match, error) {
	raw = stripFence(raw)
	var file argonFile
	if err := json.Unmarshal([]byte(raw), &file); err == nil && file.Matches != nil {
		return file.Matches, nil
	}
	var arr []job.Match
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		return arr, nil
	}
	return nil, fmt.Errorf("argon: could not parse JSON ranking")
}

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func fillGaps(matches []job.Match, details []job.Detail) {
	byID := map[string]job.Detail{}
	for _, d := range details {
		byID[d.ID] = d
	}
	for i := range matches {
		d, ok := byID[matches[i].ID]
		if !ok {
			continue
		}
		if matches[i].Title == "" {
			matches[i].Title = d.Title
		}
		if matches[i].Company == "" {
			matches[i].Company = d.Company
		}
		if matches[i].Location == "" {
			matches[i].Location = d.Location
		}
		if matches[i].URL == "" {
			matches[i].URL = d.URL
		}
		if matches[i].Score < 0 {
			matches[i].Score = 0
		}
		if matches[i].Score > 100 {
			matches[i].Score = 100
		}
	}
}

func sortByScore(matches []job.Match) {
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
}
