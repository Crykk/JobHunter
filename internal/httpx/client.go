package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Crykk/JobHunter/internal/parse"
)

// BrowserUA is a desktop Chrome UA so Duunitori serves the HTML card markup.
const BrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// DefaultDelay is the pause between Duunitori requests (~0.8s).
const DefaultDelay = 800 * time.Millisecond

// ErrBlocked is returned when Cloudflare or a 403/429 blocks the fetch.
type ErrBlocked struct {
	Status int
	Hint   string
}

func (e *ErrBlocked) Error() string {
	if e.Hint != "" {
		return e.Hint
	}
	return fmt.Sprintf("request blocked (HTTP %d)", e.Status)
}

// Client wraps net/http with a browser UA, proxy env support, and sequential delay.
type Client struct {
	HTTP  *http.Client
	Delay time.Duration
	UA    string

	mu   sync.Mutex
	last time.Time
}

// New returns a client that honors HTTP_PROXY/HTTPS_PROXY via the default transport.
func New() *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout:   45 * time.Second,
			Transport: http.DefaultTransport,
		},
		Delay: DefaultDelay,
		UA:    BrowserUA,
	}
}

// Get fetches url after the inter-request delay. Sequential by design.
func (c *Client) Get(ctx context.Context, url string) ([]byte, int, http.Header, error) {
	c.wait()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	c.decorate(req)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, res.StatusCode, res.Header, err
	}

	if blocked, hint := blockedResponse(res.StatusCode, res.Header, body); blocked {
		return body, res.StatusCode, res.Header, &ErrBlocked{Status: res.StatusCode, Hint: hint}
	}
	return body, res.StatusCode, res.Header, nil
}

func (c *Client) decorate(req *http.Request) {
	ua := c.UA
	if ua == "" {
		ua = BrowserUA
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "fi-FI,fi;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
}

func (c *Client) wait() {
	c.mu.Lock()
	defer c.mu.Unlock()
	d := c.Delay
	if d <= 0 {
		return
	}
	if !c.last.IsZero() {
		if wait := d - time.Since(c.last); wait > 0 {
			time.Sleep(wait)
		}
	}
	c.last = time.Now()
}

func blockedResponse(status int, header http.Header, body []byte) (bool, string) {
	msg := "run from a Finnish/Nordic home IP"
	switch status {
	case http.StatusForbidden, http.StatusTooManyRequests:
		return true, fmt.Sprintf("duunitori blocked this request (HTTP %d). %s", status, msg)
	}
	if parse.LooksLikeCloudflare(header, body) {
		return true, fmt.Sprintf("Cloudflare challenge HTML returned (HTTP %d). %s", status, msg)
	}
	// Some challenges still return 200 with a JS interstitial.
	lower := strings.ToLower(string(body))
	if strings.Contains(lower, "just a moment") && strings.Contains(lower, "cloudflare") {
		return true, fmt.Sprintf("Cloudflare challenge HTML returned (HTTP %d). %s", status, msg)
	}
	return false, ""
}
