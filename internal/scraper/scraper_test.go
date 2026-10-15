package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Crykk/JobHunter/internal/httpx"
	"github.com/Crykk/JobHunter/internal/job"
)

func fixture(name string) []byte {
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "testdata", name))
	if err != nil {
		panic(err)
	}
	return b
}

// rewriteFixture points job hrefs at the httptest server so details can be fetched.
func TestListPagesPaginationAndDedup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tyopaikat", func(w http.ResponseWriter, r *http.Request) {
		sivu := r.URL.Query().Get("sivu")
		switch sivu {
		case "", "1":
			w.Write(fixture("list_page1.html"))
		case "2":
			w.Write(fixture("list_page2.html"))
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := New(&rewriteClient{inner: httpx.New(), host: srv.URL})
	s.Fetch.(*rewriteClient).inner.Delay = 0
	cards, err := s.ListPages(context.Background(), "python", "Helsinki", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 4 {
		t.Fatalf("unique cards=%d want 4 (3 page1 + data engineer; python ad deduped)", len(cards))
	}
	ids := map[string]int{}
	for _, c := range cards {
		ids[c.ID]++
	}
	if ids["10101011"] != 1 || ids["20202021"] != 1 {
		t.Fatalf("dedup failed: %+v", ids)
	}
}

func TestDetails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tyopaikat/tyo/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(fixture("detail_python.html"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	hc := httpx.New()
	hc.Delay = 0
	s := New(hc)
	cards := []job.Card{{
		ID:    "10101011",
		Title: "Python Developer",
		URL:   srv.URL + "/tyopaikat/tyo/northwind-oy-python-developer-sdsuu-10101011",
	}}
	details, err := s.Details(context.Background(), cards)
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != 1 || details[0].Company != "Northwind Oy" {
		t.Fatalf("%+v", details)
	}
}

type rewriteClient struct {
	inner *httpx.Client
	host  string
}

func (r *rewriteClient) Get(ctx context.Context, rawURL string) ([]byte, int, http.Header, error) {
	u := strings.Replace(rawURL, "https://duunitori.fi", r.host, 1)
	return r.inner.Get(ctx, u)
}
