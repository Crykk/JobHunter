package parse

import (
	"net/http"
	"strings"
	"testing"
)

func TestParseDetailJSONLD(t *testing.T) {
	d, err := ParseDetail(testdata("detail_python.html"), "https://duunitori.fi/tyopaikat/tyo/northwind-oy-python-developer-sdsuu-10101011")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "10101011" {
		t.Fatalf("id %s", d.ID)
	}
	if d.Title != "Python Developer" || d.Company != "Northwind Oy" || d.Location != "Helsinki" {
		t.Fatalf("fields %+v", d)
	}
	if d.ApplyURL != "https://jobs.northwind.example/apply/10101011" {
		t.Fatalf("apply %s", d.ApplyURL)
	}
	if d.Description == "" || strings.Contains(d.Description, "<p>") {
		t.Fatalf("description should be stripped text, got %q", d.Description)
	}
	if d.Posted != "2026-10-12" {
		t.Fatalf("posted %s", d.Posted)
	}
}

func TestParseDetailPrefersH1OverJSONLDCategory(t *testing.T) {
	d, err := ParseDetail(testdata("detail_category_title.html"), "https://duunitori.fi/tyopaikat/tyo/hsl-data-analyytikko-sdsuu-20638375")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Data-analyytikko, tukianalytiikan lead" {
		t.Fatalf("title %q (JSON-LD category must not win)", d.Title)
	}
	if d.Company != "HSL Helsingin seudun liikenne -kuntayhtymä" {
		t.Fatalf("company %q", d.Company)
	}
	if d.Location != "Helsinki" {
		t.Fatalf("location %q", d.Location)
	}
	if !strings.Contains(d.Description, "analytics") {
		t.Fatalf("description %q", d.Description)
	}
}

func TestLooksLikeCloudflare(t *testing.T) {
	body := testdata("cloudflare.html")
	h := http.Header{}
	h.Set("Server", "cloudflare")
	h.Set("CF-RAY", "abc")
	if !LooksLikeCloudflare(h, body) {
		t.Fatal("expected challenge detection")
	}
	if LooksLikeCloudflare(http.Header{}, []byte(`<html><body>Löysimme 3 työpaikkaa</body></html>`)) {
		t.Fatal("normal list HTML is not a challenge")
	}
}
