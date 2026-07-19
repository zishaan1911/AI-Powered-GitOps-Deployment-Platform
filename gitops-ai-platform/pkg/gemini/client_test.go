package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateJSON_ParsesModelOutput(t *testing.T) {
	// The mock server plays the role of the real Gemini API: it receives
	// our request and returns a candidate whose text part is itself a
	// JSON string, exactly as the real API does in JSON mode.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("server: decoding request: %v", err)
		}
		if req.GenerationConfig.ResponseMIMEType != "application/json" {
			t.Errorf("expected JSON mode to be requested, got %q", req.GenerationConfig.ResponseMIMEType)
		}

		resp := generateResponse{
			Candidates: []struct {
				Content content `json:"content"`
			}{
				{Content: content{Parts: []part{{Text: `{"riskScore": 42, "summary": "looks fine"}`}}}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := &Client{APIKey: "test-key", BaseURL: srv.URL, Model: "gemini-2.5-flash", HTTPClient: srv.Client()}

	var out struct {
		RiskScore int    `json:"riskScore"`
		Summary   string `json:"summary"`
	}
	if err := c.GenerateJSON(context.Background(), "system prompt", "user prompt", &out); err != nil {
		t.Fatalf("GenerateJSON() error = %v", err)
	}
	if out.RiskScore != 42 {
		t.Errorf("RiskScore = %d, want 42", out.RiskScore)
	}
	if out.Summary != "looks fine" {
		t.Errorf("Summary = %q, want %q", out.Summary, "looks fine")
	}
}

func TestGenerateJSON_RequiresAPIKey(t *testing.T) {
	c := &Client{APIKey: ""}
	var out map[string]interface{}
	if err := c.GenerateJSON(context.Background(), "sys", "user", &out); err == nil {
		t.Fatal("expected an error when APIKey is empty, got nil")
	}
}

func TestGenerateJSON_SurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "rate limited"}}`))
	}))
	defer srv.Close()

	c := &Client{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()}
	var out map[string]interface{}
	err := c.GenerateJSON(context.Background(), "sys", "user", &out)
	if err == nil {
		t.Fatal("expected an error for non-200 response, got nil")
	}
}
