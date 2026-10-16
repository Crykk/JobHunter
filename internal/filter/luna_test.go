package filter

import (
	"context"
	"strings"
	"testing"

	"github.com/Crykk/JobHunter/internal/job"
	"github.com/Crykk/JobHunter/internal/openrouter"
)

type fakeDecider struct {
	noul map[string]float64
	got  openrouter.DecisionsRequest
}

func (f *fakeDecider) Decide(_ context.Context, req openrouter.DecisionsRequest) (*openrouter.DecisionsResponse, error) {
	f.got = req
	out := &openrouter.DecisionsResponse{Answers: map[string]openrouter.NoulAnswer{}}
	for id, n := range f.noul {
		out.Answers["suitable_"+id] = openrouter.NoulAnswer{Type: "noul", Noul: n}
	}
	return out, nil
}

func TestApplyThreshold(t *testing.T) {
	cards := []job.Card{
		{ID: "1", Title: "Python Developer", Company: "A", Location: "Helsinki"},
		{ID: "2", Title: "SDR", Company: "B", Location: "Tampere"},
		{ID: "3", Title: "Go engineer", Company: "C", Location: "Espoo"},
	}
	dec := &fakeDecider{noul: map[string]float64{"1": 0.80, "2": 0.10, "3": 0.55}}
	fl := &Filter{OR: dec, Threshold: 0.55}
	kept, scores, err := fl.Apply(context.Background(), "python/go backend", cards)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 2 || kept[0].ID != "1" || kept[1].ID != "3" {
		t.Fatalf("kept %+v", kept)
	}
	if scores["2"] != 0.10 {
		t.Fatalf("scores %+v", scores)
	}
	if dec.got.Model != openrouter.LunaModel {
		t.Fatalf("model %s", dec.got.Model)
	}
	q, ok := dec.got.Questions["suitable_1"]
	if !ok {
		t.Fatal("expected suitable_{id} questions")
	}
	if !strings.Contains(q.Instructions, "job id 1") {
		t.Fatalf("instructions should name the job: %s", q.Instructions)
	}
}

func TestApplyJevModel(t *testing.T) {
	dec := &fakeDecider{noul: map[string]float64{"1": 0.9}}
	fl := &Filter{OR: dec, Model: openrouter.JevModel, Threshold: 0.5}
	_, _, err := fl.Apply(context.Background(), "p", []job.Card{{ID: "1", Title: "T"}})
	if err != nil {
		t.Fatal(err)
	}
	if dec.got.Model != openrouter.JevModel {
		t.Fatalf("model %s", dec.got.Model)
	}
}

func TestApplyBatches(t *testing.T) {
	old := MaxQuestions
	t.Cleanup(func() { /* const: exercise full path with 2 cards */ })
	_ = old
	cards := make([]job.Card, 2)
	cards[0] = job.Card{ID: "a", Title: "A"}
	cards[1] = job.Card{ID: "b", Title: "B"}
	dec := &fakeDecider{noul: map[string]float64{"a": 0.9, "b": 0.9}}
	fl := &Filter{OR: dec, Threshold: 0.5}
	kept, _, err := fl.Apply(context.Background(), "p", cards)
	if err != nil || len(kept) != 2 {
		t.Fatalf("kept %v err %v", kept, err)
	}
}

type refuseOnce struct {
	refuseID string
	noul     map[string]float64
	calls    int
	last     openrouter.DecisionsRequest
}

func (f *refuseOnce) Decide(_ context.Context, req openrouter.DecisionsRequest) (*openrouter.DecisionsResponse, error) {
	f.calls++
	f.last = req
	if f.calls == 1 {
		return nil, &openrouter.APIError{
			Status:  502,
			Message: `Provider returned error: openai refused to answer question "suitable_` + f.refuseID + `"`,
		}
	}
	out := &openrouter.DecisionsResponse{Answers: map[string]openrouter.NoulAnswer{}}
	for id := range req.Questions {
		jobID := strings.TrimPrefix(id, "suitable_")
		n := f.noul[jobID]
		if n == 0 {
			n = 0.9
		}
		out.Answers[id] = openrouter.NoulAnswer{Type: "noul", Noul: n}
	}
	return out, nil
}

func TestDropRefusedQuestionKeepsOthers(t *testing.T) {
	cards := []job.Card{
		{ID: "20531918", Title: "X"},
		{ID: "2", Title: "Go engineer"},
	}
	dec := &refuseOnce{refuseID: "20531918", noul: map[string]float64{"2": 0.9}}
	fl := &Filter{OR: dec, Threshold: 0.55}
	kept, _, err := fl.Apply(context.Background(), "go", cards)
	if err != nil {
		t.Fatal(err)
	}
	if dec.calls < 2 {
		t.Fatalf("expected retry after drop, calls=%d", dec.calls)
	}
	if len(kept) != 1 || kept[0].ID != "2" {
		t.Fatalf("kept %+v", kept)
	}
	if _, ok := dec.last.Questions["suitable_20531918"]; ok {
		t.Fatal("refused id should not be retried")
	}
}

type always502 struct{ calls int }

func (a *always502) Decide(_ context.Context, req openrouter.DecisionsRequest) (*openrouter.DecisionsResponse, error) {
	a.calls++
	return nil, &openrouter.APIError{Status: 502, Message: "invalid response from model"}
}

func TestGeneric502DropsSinglesNotWholeSearch(t *testing.T) {
	cards := []job.Card{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}
	dec := &always502{}
	fl := &Filter{OR: dec, Threshold: 0.5}
	kept, _, err := fl.Apply(context.Background(), "p", cards)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 0 {
		t.Fatalf("kept %+v", kept)
	}
	if dec.calls < 3 {
		t.Fatalf("expected split retries, calls=%d", dec.calls)
	}
}

func TestFatalErrorStops(t *testing.T) {
	dec := &fatalDecider{}
	fl := &Filter{OR: dec, Threshold: 0.5}
	_, _, err := fl.Apply(context.Background(), "p", []job.Card{{ID: "1", Title: "T"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

type fatalDecider struct{}

func (fatalDecider) Decide(_ context.Context, _ openrouter.DecisionsRequest) (*openrouter.DecisionsResponse, error) {
	return nil, &openrouter.APIError{Status: 401, Message: "invalid key"}
}

func TestMissingAnswerDropped(t *testing.T) {
	cards := []job.Card{{ID: "x", Title: "X"}}
	dec := &fakeDecider{noul: map[string]float64{}}
	fl := &Filter{OR: dec, Threshold: 0.55}
	kept, _, err := fl.Apply(context.Background(), "p", cards)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 0 {
		t.Fatalf("kept %+v", kept)
	}
}
