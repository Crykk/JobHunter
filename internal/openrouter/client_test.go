package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecide(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != DecisionsPath {
			t.Errorf("path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth %s", got)
		}
		var req DecisionsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != LunaModel {
			t.Errorf("model %s", req.Model)
		}
		if _, ok := req.Questions["suitable_10101011"]; !ok {
			t.Errorf("missing question: %+v", req.Questions)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{
			"id":"gen-dec-test",
			"model":"openai/gpt-6-luna-decisions",
			"answers":{"suitable_10101011":{"type":"noul","noul":0.81}}
		}`)
	}))
	t.Cleanup(srv.Close)

	c := New("test-key")
	c.BaseURL = srv.URL
	resp, err := c.Decide(context.Background(), DecisionsRequest{
		Model: LunaModel,
		State: map[string]any{"profile": "x"},
		Questions: map[string]NoulQuestion{
			"suitable_10101011": {Type: "noul", Instructions: "fit?"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Answers["suitable_10101011"].Noul != 0.81 {
		t.Fatalf("%+v", resp.Answers)
	}
}

func TestChatJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ChatCompletionsPath {
			t.Errorf("path %s", r.URL.Path)
		}
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != ArgonModel {
			t.Errorf("model %s", req.Model)
		}
		if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
			t.Errorf("response_format %+v", req.ResponseFormat)
		}
		io.WriteString(w, `{
			"choices":[{"message":{"content":"{\"matches\":[{\"id\":\"1\",\"score\":90,\"why\":\"ok\"}]}"}}]
		}`)
	}))
	t.Cleanup(srv.Close)

	c := New("test-key")
	c.BaseURL = srv.URL
	got, err := c.ChatJSON(context.Background(), ArgonModel, "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("empty content")
	}
}

func TestMissingKey(t *testing.T) {
	c := New("")
	_, err := c.Decide(context.Background(), DecisionsRequest{Questions: map[string]NoulQuestion{}})
	if err == nil {
		t.Fatal("expected error")
	}
}
