package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetBlockedCloudflare(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "cloudflare.html"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "cloudflare")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	c := New()
	c.Delay = 0
	_, _, _, err = c.Get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected block")
	}
	if _, ok := err.(*ErrBlocked); !ok {
		t.Fatalf("want ErrBlocked, got %T %v", err, err)
	}
}

func TestGetForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("blocked"))
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.Delay = 0
	_, _, _, err := c.Get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected block")
	}
}
