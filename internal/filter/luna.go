package filter

import (
	"context"
	"fmt"
	"io"

	"github.com/Crykk/JobHunter/internal/job"
	"github.com/Crykk/JobHunter/internal/openrouter"
)

// MaxQuestions is well under the Decisions cap of 200. Smaller batches
// reduce Luna 502s when one listing in a large set is refused.
const MaxQuestions = 25

func questionKey(id string) string { return "suitable_" + id }

func instructionsFor(id string) string {
	return fmt.Sprintf("Is job id %s in state.jobs a reasonable match for the candidate in state.profile? Use only that job's title, company, and location. Answer yes if skills, seniority, location, or industry could fit; no if the role is clearly unrelated or hits a deal-breaker.", id)
}

var noulCriteria = map[string]string{
	"true":  "The listing could fit the profile's skills, seniority, location, or industry.",
	"false": "The listing is clearly unrelated or a deal-breaker given the profile.",
}

// Client is the Decisions subset used by the Luna filter.
type Client interface {
	Decide(ctx context.Context, req openrouter.DecisionsRequest) (*openrouter.DecisionsResponse, error)
}

// Filter keeps cards whose noul score is >= threshold.
type Filter struct {
	OR        Client
	Model     string
	Threshold float64
	Log       io.Writer
}

func (f *Filter) logf(format string, args ...any) {
	if f.Log == nil {
		return
	}
	fmt.Fprintf(f.Log, format+"\n", args...)
}

type lunaJob struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Company  string `json:"company"`
	Location string `json:"location"`
}

type lunaState struct {
	Profile string    `json:"profile"`
	Jobs    []lunaJob `json:"jobs"`
}

// Apply runs OpenRouter Decisions in batches and returns surviving cards.
func (f *Filter) Apply(ctx context.Context, profile string, cards []job.Card) ([]job.Card, map[string]float64, error) {
	thr := f.Threshold
	if thr <= 0 {
		thr = 0.55
	}
	scores := map[string]float64{}
	if len(cards) == 0 {
		return nil, scores, nil
	}
	model := f.Model
	if model == "" {
		model = openrouter.LunaModel
	}
	var kept []job.Card
	for start := 0; start < len(cards); start += MaxQuestions {
		if err := ctx.Err(); err != nil {
			return kept, scores, err
		}
		end := start + MaxQuestions
		if end > len(cards) {
			end = len(cards)
		}
		f.logf("%s: scoring jobs %d–%d of %d (threshold %.2f)", logName(model), start+1, end, len(cards), thr)
		k, s, err := f.scoreCards(ctx, model, profile, cards[start:end], thr)
		for id, n := range s {
			scores[id] = n
		}
		if err != nil {
			return kept, scores, err
		}
		kept = append(kept, k...)
	}
	f.logf("%s: kept %d of %d", logName(f.Model), len(kept), len(cards))
	return kept, scores, nil
}

func (f *Filter) scoreCards(ctx context.Context, model, profile string, cards []job.Card, thr float64) ([]job.Card, map[string]float64, error) {
	scores := map[string]float64{}
	if len(cards) == 0 {
		return nil, scores, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, scores, err
	}

	resp, err := f.decide(ctx, model, profile, cards)
	if err == nil {
		return collect(cards, resp, thr, f), scoresFrom(cards, resp, scores), nil
	}

	if keys := openrouter.RefusedQuestionKeys(err); len(keys) > 0 {
		drop := map[string]struct{}{}
		for _, k := range keys {
			drop[k] = struct{}{}
		}
		var left []job.Card
		for _, c := range cards {
			if _, ok := drop[questionKey(c.ID)]; ok {
				f.logf("%s: refused %s; dropping", logName(model), c.ID)
				continue
			}
			left = append(left, c)
		}
		if len(left) == len(cards) {
			// Parsed a key that is not in this batch — treat as a generic failure.
			return f.recover(ctx, model, profile, cards, thr, err)
		}
		return f.scoreCards(ctx, model, profile, left, thr)
	}

	return f.recover(ctx, model, profile, cards, thr, err)
}

func (f *Filter) recover(ctx context.Context, model, profile string, cards []job.Card, thr float64, err error) ([]job.Card, map[string]float64, error) {
	if !openrouter.Retryable(err) {
		return nil, nil, err
	}
	if len(cards) == 1 {
		f.logf("%s: dropping %s after %s", logName(model), cards[0].ID, err)
		return nil, map[string]float64{}, nil
	}
	mid := len(cards) / 2
	f.logf("%s: splitting batch of %d after %s", logName(model), len(cards), err)
	k1, s1, e1 := f.scoreCards(ctx, model, profile, cards[:mid], thr)
	k2, s2, e2 := f.scoreCards(ctx, model, profile, cards[mid:], thr)
	scores := map[string]float64{}
	for id, n := range s1 {
		scores[id] = n
	}
	for id, n := range s2 {
		scores[id] = n
	}
	if e1 != nil && !openrouter.Retryable(e1) {
		return append(k1, k2...), scores, e1
	}
	if e2 != nil && !openrouter.Retryable(e2) {
		return append(k1, k2...), scores, e2
	}
	if e1 != nil {
		f.logf("%s: %v", logName(model), e1)
	}
	if e2 != nil {
		f.logf("%s: %v", logName(model), e2)
	}
	return append(k1, k2...), scores, nil
}

func (f *Filter) decide(ctx context.Context, model, profile string, batch []job.Card) (*openrouter.DecisionsResponse, error) {
	state := lunaState{Profile: profile}
	questions := make(map[string]openrouter.NoulQuestion, len(batch))
	for _, c := range batch {
		state.Jobs = append(state.Jobs, lunaJob{
			ID: c.ID, Title: c.Title, Company: c.Company, Location: c.Location,
		})
		questions[questionKey(c.ID)] = openrouter.NoulQuestion{
			Type:         "noul",
			Instructions: instructionsFor(c.ID),
			Criteria:     noulCriteria,
		}
	}
	return f.OR.Decide(ctx, openrouter.DecisionsRequest{
		Model:     model,
		State:     state,
		Questions: questions,
	})
}

func scoresFrom(batch []job.Card, resp *openrouter.DecisionsResponse, scores map[string]float64) map[string]float64 {
	if resp == nil {
		return scores
	}
	for _, c := range batch {
		if ans, ok := resp.Answers[questionKey(c.ID)]; ok {
			scores[c.ID] = ans.Noul
		}
	}
	return scores
}

func collect(batch []job.Card, resp *openrouter.DecisionsResponse, thr float64, f *Filter) []job.Card {
	var kept []job.Card
	if resp == nil {
		return kept
	}
	for _, c := range batch {
		ans, ok := resp.Answers[questionKey(c.ID)]
		if !ok {
			f.logf("%s: no answer for %s; dropping", logName(f.Model), c.ID)
			continue
		}
		if ans.Noul >= thr {
			kept = append(kept, c)
		}
	}
	return kept
}

func logName(model string) string {
	if model == openrouter.JevModel {
		return "jev"
	}
	return "luna"
}
