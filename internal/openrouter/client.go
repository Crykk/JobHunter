package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultBaseURL      = "https://openrouter.ai"
	DecisionsPath       = "/api/alpha/decisions"
	ChatCompletionsPath = "/api/v1/chat/completions"
	LunaModel           = "openai/gpt-6-luna-decisions"
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
	return fmt.Errorf("openrouter HTTP %d: %s", status, msg)
}
