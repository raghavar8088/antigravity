package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// gemini-2.0-flash: fast, cheap, strong reasoning — ideal for macro overlay
	geminiModel  = "gemini-2.0-flash-001"
	geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/" + geminiModel + ":generateContent"
)

// GeminiClient wraps the Google Generative Language REST API.
// It degrades gracefully when GEMINI_API_KEY is not set.
type GeminiClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewGeminiClient creates a Gemini client from the GEMINI_API_KEY env var.
// Returns nil if the key is not set — the engine runs without the Macro Agent.
func NewGeminiClient() *GeminiClient {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil
	}
	return &GeminiClient{
		apiKey: key,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (g *GeminiClient) IsAvailable() bool {
	return g != nil && g.apiKey != ""
}

// ── Request / Response structs ──────────────────────────────────────

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  *geminiGenCfg   `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenCfg struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// Chat sends a system prompt + user message to Gemini and returns raw text.
func (g *GeminiClient) Chat(ctx context.Context, system, userMessage string, maxTokens int) (string, error) {
	if !g.IsAvailable() {
		return "", fmt.Errorf("gemini client not initialized (missing GEMINI_API_KEY)")
	}

	reqBody := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: system}},
		},
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: userMessage}},
			},
		},
		GenerationConfig: &geminiGenCfg{
			MaxOutputTokens: maxTokens,
			Temperature:     0.2, // Low temperature for consistent JSON
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal gemini request: %w", err)
	}

	url := geminiAPIURL + "?key=" + g.apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read gemini response: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("unmarshal gemini response: %w", err)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("gemini api error [%d %s]: %s",
			geminiResp.Error.Code, geminiResp.Error.Status, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// ChatForMacro calls Gemini Flash for macro-level market analysis.
func (g *GeminiClient) ChatForMacro(ctx context.Context, system, prompt string) (string, error) {
	return g.Chat(ctx, system, prompt, 700)
}

// ChatForSignal calls Gemini Flash for trading signal generation.
func (g *GeminiClient) ChatForSignal(ctx context.Context, system, prompt string) (string, error) {
	return g.Chat(ctx, system, prompt, 350)
}

// ChatForRisk calls Gemini Flash for risk arbitration.
func (g *GeminiClient) ChatForRisk(ctx context.Context, system, prompt string) (string, error) {
	return g.Chat(ctx, system, prompt, 400)
}
