package filter

import (
	"context"
	"fmt"
	"io"

	"github.com/Crykk/JobHunter/internal/job"
	"github.com/Crykk/JobHunter/internal/openrouter"
)

// MaxQuestions is kept under the Decisions cap of 200 (plan: ~100 per call).
const MaxQuestions = 100

const lunaInstructions = "Is this job (matched by question id suitable_{id}) a reasonable match for the candidate in state.profile? Use only title, company, and location. Answer yes if skills, seniority, location, or industry could fit; no if the role is clearly unrelated or hits a deal-breaker."

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

// Apply runs OpenRouter Luna Decisions in batches and returns surviving cards.
func (f *Filter) Apply(ctx context.Context, profile string, cards []job.Card) ([]job.Card, map[string]float64, error) {
	thr := f.Threshold
	if thr <= 0 {
		thr = 0.55
	}
	scores := map[string]float64{}
	if len(cards) == 0 {
		return nil, scores, nil
	}
	var kept []job.Card
	for start := 0; start < len(cards); start += MaxQuestions {
		end := start + MaxQuestions
		if end > len(cards) {
			end = len(cards)
		}
		batch := cards[start:end]
		model := f.Model
		if model == "" {
			model = openrouter.LunaModel
		}
		f.logf("%s: scoring jobs %d–%d of %d (threshold %.2f)", logName(model), start+1, end, len(cards), thr)

		state := lunaState{Profile: profile}
		questions := make(map[string]openrouter.NoulQuestion, len(batch))
		for _, c := range batch {
			state.Jobs = append(state.Jobs, lunaJob{
				ID: c.ID, Title: c.Title, Company: c.Company, Location: c.Location,
			})
			questions["suitable_"+c.ID] = openrouter.NoulQuestion{
				Type:         "noul",
				Instructions: lunaInstructions,
				Criteria: map[string]string{
					"true":  "The listing could fit the profile's skills, seniority, location, or industry.",
					"false": "The listing is clearly unrelated or a deal-breaker given the profile.",
				},
			}
		}

		resp, err := f.OR.Decide(ctx, openrouter.DecisionsRequest{
			Model:     model,
			State:     state,
			Questions: questions,
		})
		if err != nil {
			return kept, scores, err
		}
		for _, c := range batch {
			ans, ok := resp.Answers["suitable_"+c.ID]
			if !ok {
				f.logf("%s: no answer for %s; dropping", logName(model), c.ID)
				continue
			}
			scores[c.ID] = ans.Noul
			if ans.Noul >= thr {
				kept = append(kept, c)
			}
		}
	}
	f.logf("%s: kept %d of %d", logName(f.Model), len(kept), len(cards))
	return kept, scores, nil
}

func logName(model string) string {
	if model == openrouter.JevModel {
		return "jev"
	}
	return "luna"
}
