package parse

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testdata(name string) []byte {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
	b, err := os.ReadFile(root)
	if err != nil {
		panic(err)
	}
	return b
}

func TestParseListPage1(t *testing.T) {
	page, err := ParseList(testdata("list_page1.html"), "https://duunitori.fi/tyopaikat?haku=python&alue=Helsinki")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 45 {
		t.Fatalf("total=%d want 45", page.Total)
	}
	if page.LastPage != 3 {
		t.Fatalf("last page=%d want ceil(45/20)=3", page.LastPage)
	}
	if page.ShownFrom != 1 || page.ShownTo != 20 {
		t.Fatalf("shown %d-%d", page.ShownFrom, page.ShownTo)
	}
	if len(page.Cards) != 3 {
		t.Fatalf("cards=%d want 3 unique (ad dup dropped)", len(page.Cards))
	}
	if page.Cards[0].ID != "10101011" || page.Cards[0].Title != "Python Developer" {
		t.Fatalf("first card %+v", page.Cards[0])
	}
	if page.Cards[0].Company != "Northwind Oy" || page.Cards[0].Location != "Helsinki" {
		t.Fatalf("company/location %+v", page.Cards[0])
	}
	if page.Cards[0].URL != "https://duunitori.fi/tyopaikat/tyo/northwind-oy-python-developer-sdsuu-10101011" {
		t.Fatalf("url %s", page.Cards[0].URL)
	}
	if page.Cards[1].ID != "10101012" {
		t.Fatalf("second id %s", page.Cards[1].ID)
	}
}

func TestParseListPage2DedupWithinPage(t *testing.T) {
	page, err := ParseList(testdata("list_page2.html"), "https://duunitori.fi/tyopaikat?haku=python&sivu=2")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Cards) != 2 {
		t.Fatalf("cards=%d want 2", len(page.Cards))
	}
	if page.Cards[0].ID != "20202021" {
		t.Fatalf("id %s", page.Cards[0].ID)
	}
}

func TestParseListCurrentMarkupCompanyAndLocation(t *testing.T) {
	page, err := ParseList(testdata("list_page_current.html"), "https://duunitori.fi/tyopaikat?haku=python&alue=Helsinki")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Cards) != 2 {
		t.Fatalf("cards=%d want 2", len(page.Cards))
	}
	c := page.Cards[0]
	if c.ID != "20638375" {
		t.Fatalf("id %s", c.ID)
	}
	if c.Company != "HSL Helsingin seudun liikenne -kuntayhtymä" {
		t.Fatalf("company %q", c.Company)
	}
	if c.Location != "Helsinki" {
		t.Fatalf("location %q", c.Location)
	}
	if page.Cards[1].Company != "Kultakiertue Oy" {
		t.Fatalf("company2 %q", page.Cards[1].Company)
	}
	if page.Cards[1].Location != "Helsinki ja 1 muu" {
		t.Fatalf("location2 %q", page.Cards[1].Location)
	}
}

func TestPageURL(t *testing.T) {
	u := PageURL("python", "Helsinki", 3)
	if u != "https://duunitori.fi/tyopaikat?alue=Helsinki&haku=python&sivu=3" {
		t.Fatalf("url %s", u)
	}
	u1 := PageURL("go", "", 1)
	if u1 != "https://duunitori.fi/tyopaikat?haku=go" {
		t.Fatalf("page1 should omit sivu: %s", u1)
	}
}

func TestLastPageFromTotal(t *testing.T) {
	if LastPageFromTotal(20) != 1 || LastPageFromTotal(21) != 2 || LastPageFromTotal(45) != 3 {
		t.Fatalf("unexpected last-page math")
	}
}
