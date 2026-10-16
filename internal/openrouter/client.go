package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultBaseURL      = "https://openrouter.ai"
	DecisionsPath       = "/api/alpha/decisions"
	ChatCompletionsPath = "/api/v1/chat/completions"
	LunaModel           = "openai/gpt-6-luna-decisions"
	JevModel            = "typesafe/jev-1.14"
	ArgonModel          = "google/gemini-4-argon"
)

// Client talks to OpenRouter Decisions and Chat Completions.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	APIKey  string
	Referer string
	Title   string
}

func New(apiKey string) *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 120 * time.Second},
		BaseURL: DefaultBaseURL,
		APIKey:  apiKey,
		Referer: "https://github.com/Crykk/JobHunter",
		Title:   "JobHunter",
	}
}

func (c *Client) post(ctx context.Context, path string, payload any) ([]byte, int, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, 0, fmt.Errorf("OPENROUTER_API_KEY is not set")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	url := strings.TrimRight(c.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", c.Referer)
	req.Header.Set("X-Title", c.Title)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, res.StatusCode, err
	}
	return raw, res.StatusCode, nil
}

// APIError is a non-2xx OpenRouter response.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e == nil {
		return "openrouter error"
	}
	return fmt.Sprintf("openrouter HTTP %d: %s", e.Status, e.Message)
}

func decodeError(status int, raw []byte) error {
	var wrap struct {
		Error struct {
			Message string `json:"message"`
			Code    any    `json:"code"`
		} `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &wrap)
	msg := wrap.Error.Message
	if msg == "" {
		msg = wrap.Message
	}
	if msg == "" {
		msg = strings.TrimSpace(string(raw))
	}
	if len(msg) > 400 {
		msg = msg[:400]
	}
	return &APIError{Status: status, Message: msg}
}

var suitableKey = regexp.MustCompile(`suitable_[A-Za-z0-9-]+`)

// RefusedQuestionKeys pulls suitable_{id} keys out of a Decisions refusal / 502.
func RefusedQuestionKeys(err error) []string {
	if err == nil {
		return nil
	}
	msg := err.Error()
	var api *APIError
	if errors.As(err, &api) {
		msg = api.Message
	}
	found := suitableKey.FindAllString(msg, -1)
	if len(found) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, k := range found {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// Retryable is a transient OpenRouter / provider failure (502, 503, 429, 408).
func Retryable(err error) bool {
	var api *APIError
	if errors.As(err, &api) {
		switch api.Status {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusRequestTimeout:
			return true
		}
		return false
	}
	s := err.Error()
	return strings.Contains(s, "HTTP 502") || strings.Contains(s, "HTTP 503") || strings.Contains(s, "HTTP 429")
}
