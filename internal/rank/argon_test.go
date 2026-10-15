package rank

import (
	"context"
	"strings"
	"testing"

	"github.com/Crykk/JobHunter/internal/job"
	"github.com/Crykk/JobHunter/internal/openrouter"
)

type fakeChat struct {
	content string
	model   string
	err     error
}

func (f *fakeChat) ChatJSON(_ context.Context, model, _, _ string) (string, error) {
	f.model = model
	return f.content, f.err
}

func TestRankParsesFencedJSON(t *testing.T) {
	fc := &fakeChat{content: "```json\n{\"matches\":[{\"id\":\"10101011\",\"score\":91,\"why\":\"Python APIs, Helsinki hybrid.\"},{\"id\":\"20202021\",\"score\":70,\"why\":\"Data role, remote FI.\"}]}\n```"}
	r := &Ranker{OR: fc}
	details := []job.Detail{
		{ID: "10101011", Title: "Python Developer", Company: "Northwind Oy", Location: "Helsinki", URL: "https://duunitori.fi/x"},
		{ID: "20202021", Title: "Data Engineer", Company: "Aurora Labs", Location: "Remote", URL: "https://duunitori.fi/y"},
	}
	got, err := r.Rank(context.Background(), "profile", details)
	if err != nil {
		t.Fatal(err)
	}
	if fc.model != openrouter.ArgonModel {
		t.Fatalf("model %s", fc.model)
	}
	if len(got) != 2 || got[0].ID != "10101011" || got[0].Score != 91 {
		t.Fatalf("%+v", got)
	}
	if got[0].Company != "Northwind Oy" {
		t.Fatalf("gap fill %+v", got[0])
	}
}

func TestSortByScore(t *testing.T) {
	fc := &fakeChat{content: `{"matches":[{"id":"a","score":40,"why":"x"},{"id":"b","score":80,"why":"y"}]}`}
	got, err := (&Ranker{OR: fc}).Rank(context.Background(), "p", []job.Detail{{ID: "a"}, {ID: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "b" {
		t.Fatalf("not sorted: %+v", got)
	}
}

func TestParseError(t *testing.T) {
	fc := &fakeChat{content: "not json"}
	_, err := (&Ranker{OR: fc}).Rank(context.Background(), "p", []job.Detail{{ID: "a"}})
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("err %v", err)
	}
}
