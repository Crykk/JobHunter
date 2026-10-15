package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
)

// NoulQuestion is a yes/no Decisions question.
type NoulQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

// DecisionsRequest is POST /api/alpha/decisions.
type DecisionsRequest struct {
	Model     string                  `json:"model"`
	State     any                     `json:"state"`
	Questions map[string]NoulQuestion `json:"questions"`
}

// NoulAnswer is a typed noul result.
type NoulAnswer struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"`
}

// DecisionsResponse is the OpenRouter Decisions payload.
type DecisionsResponse struct {
	ID      string                `json:"id"`
	Model   string                `json:"model"`
	Answers map[string]NoulAnswer `json:"answers"`
}

// Decide sends a Decisions request (Luna stage-1).
func (c *Client) Decide(ctx context.Context, req DecisionsRequest) (*DecisionsResponse, error) {
	if req.Model == "" {
		req.Model = LunaModel
	}
	raw, status, err := c.post(ctx, DecisionsPath, req)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, decodeError(status, raw)
	}
	var out DecisionsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode decisions response: %w", err)
	}
	return &out, nil
}
