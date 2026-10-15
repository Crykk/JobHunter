package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	ResponseFormat *respFormat   `json:"response_format,omitempty"`
	Temperature    float64       `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type respFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ChatJSON calls OpenRouter chat completions and returns the assistant text.
func (c *Client) ChatJSON(ctx context.Context, model, system, user string) (string, error) {
	if model == "" {
		model = ArgonModel
	}
	payload := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		ResponseFormat: &respFormat{Type: "json_object"},
		Temperature:    0.2,
	}
	raw, status, err := c.post(ctx, ChatCompletionsPath, payload)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", decodeError(status, raw)
	}
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("decode chat response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openrouter chat: empty choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
