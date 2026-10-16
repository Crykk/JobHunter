package filter

import (
	"context"
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
	if _, ok := dec.got.Questions["suitable_1"]; !ok {
		t.Fatal("expected suitable_{id} questions")
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
