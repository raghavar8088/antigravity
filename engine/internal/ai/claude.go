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
	anthropicAPIURL = "https://api.anthropic.com/v1/messages"
	anthropicVersion = "2023-06-01"

	// claude-haiku-4-5: fastest + cheapest — ideal for real-time trading signals
	modelSignals = "claude-haiku-4-5-20251001"
	// claude-sonnet-4-6: stronger reasoning — used for risk arbitration
	modelRisk = "claude-sonnet-4-6"
)

// ClaudeClient wraps the Anthropic Messages API.
type ClaudeClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewClaudeClient creates a Claude API client from the ANTHROPIC_API_KEY env var.
// Returns nil if the key is not set (engine degrades gracefully to rules-only mode).
func NewClaudeClient() *ClaudeClient {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil
	}
	return &ClaudeClient{
		apiKey: key,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *ClaudeClient) IsAvailable() bool {
	return c != nil && c.apiKey != ""
}

// claudeRequest is the Anthropic Messages API request body.
type claudeRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system,omitempty"`
	Messages  []claudeMessage  `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// claudeResponse is the Anthropic Messages API response.
type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat sends a message to Claude and returns the text response.
func (c *ClaudeClient) Chat(ctx context.Context, model, system, userMessage string, maxTokens int) (string, error) {
	if !c.IsAvailable() {
		return "", fmt.Errorf("claude client not initialized (missing ANTHROPIC_API_KEY)")
	}

	reqBody := claudeRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    system,
		Messages: []claudeMessage{
			{Role: "user", Content: userMessage},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if claudeResp.Error != nil {
		return "", fmt.Errorf("claude api error [%s]: %s", claudeResp.Error.Type, claudeResp.Error.Message)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("empty response from claude")
	}

	return claudeResp.Content[0].Text, nil
}

// ChatForSignal calls claude-haiku for fast trading signal generation.
func (c *ClaudeClient) ChatForSignal(ctx context.Context, system, prompt string) (string, error) {
	return c.Chat(ctx, modelSignals, system, prompt, 600)
}

// ChatForRisk calls claude-sonnet for more careful risk arbitration.
func (c *ClaudeClient) ChatForRisk(ctx context.Context, system, prompt string) (string, error) {
	return c.Chat(ctx, modelRisk, system, prompt, 400)
}
