// Package gemini is a minimal client for the Gemini API's generateContent
// endpoint, used by pkg/riskreview and pkg/healthwatcher for the two
// judgment calls that don't have a deterministic answer: "does this diff
// look risky" and "why did this rollout fail". Every other AI-adjacent
// task in this platform (Dockerfile/manifest generation) is intentionally
// template-based, not model-generated — see pkg/manifest for why.
//
// Deliberately stdlib-only (no SDK dependency) so the module has zero
// external dependencies. BaseURL is overridable so callers can point it at
// an httptest server in unit tests instead of the real API.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
const defaultModel = "gemini-2.5-flash"

// Client talks to the Gemini API.
type Client struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// New returns a Client configured with sane defaults. apiKey is required;
// pass the value of the GEMINI_API_KEY environment variable.
func New(apiKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		BaseURL:    defaultBaseURL,
		Model:      defaultModel,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type generateRequest struct {
	SystemInstruction *content         `json:"system_instruction,omitempty"`
	Contents          []content        `json:"contents"`
	GenerationConfig  generationConfig `json:"generationConfig"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generationConfig struct {
	ResponseMIMEType string  `json:"responseMimeType,omitempty"`
	Temperature      float64 `json:"temperature"`
}

type generateResponse struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GenerateJSON sends systemPrompt + userPrompt to Gemini with JSON-mode
// output enabled, and unmarshals the model's response into out. Callers
// should design systemPrompt to fully specify the expected JSON shape,
// since Gemini's JSON mode constrains syntax but not the schema itself.
func (c *Client) GenerateJSON(ctx context.Context, systemPrompt, userPrompt string, out interface{}) error {
	if c.APIKey == "" {
		return fmt.Errorf("gemini: APIKey is required")
	}
	model := c.Model
	if model == "" {
		model = defaultModel
	}
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	reqBody := generateRequest{
		SystemInstruction: &content{Parts: []part{{Text: systemPrompt}}},
		Contents:          []content{{Role: "user", Parts: []part{{Text: userPrompt}}}},
		GenerationConfig: generationConfig{
			ResponseMIMEType: "application/json",
			Temperature:      0.1, // low temperature: this is a judgment call that should be reproducible, not creative
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("gemini: marshaling request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, model, c.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("gemini: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("gemini: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("gemini: reading response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("gemini: API returned status %d: %s", httpResp.StatusCode, string(body))
	}

	var resp generateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("gemini: decoding response envelope: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("gemini: API error: %s", resp.Error.Message)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("gemini: no candidates returned")
	}

	text := resp.Candidates[0].Content.Parts[0].Text
	if err := json.Unmarshal([]byte(text), out); err != nil {
		return fmt.Errorf("gemini: model output was not valid JSON matching the expected shape: %w (raw: %s)", err, text)
	}
	return nil
}
