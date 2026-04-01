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
	openRouterAPIURL     = "https://openrouter.ai/api/v1/chat/completions"
	modelOpenRouterFree  = "meta-llama/llama-3.3-70b-instruct:free" // Highest quality free model on OpenRouter
)

// OpenRouterClient wraps the OpenRouter Chat Completions API.
type OpenRouterClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewOpenRouterClient creates an OpenRouter API client from the OPENROUTER_API_KEY env var.
func NewOpenRouterClient() *OpenRouterClient {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		return nil
	}
	return &OpenRouterClient{
		apiKey: key,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *OpenRouterClient) IsAvailable() bool {
	return c != nil && c.apiKey != ""
}

// ChatForAudit calls OpenRouter for resilient, free model access.
func (c *OpenRouterClient) ChatForAudit(ctx context.Context, system, prompt string) (string, error) {
	if !c.IsAvailable() {
		return "", fmt.Errorf("openrouter client not initialized (missing OPENROUTER_API_KEY)")
	}

	reqBody := openaiRequest{
		Model: modelOpenRouterFree,
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   500, // Batch audits might need slightly more tokens
		Temperature: 0.1,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterAPIURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/raghavar8088/antigravity") // Required by OpenRouter
	req.Header.Set("X-Title", "Antigravity Trading Engine")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openrouter api error status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var oaResp openaiResponse
	if err := json.Unmarshal(body, &oaResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(oaResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from openrouter")
	}

	return oaResp.Choices[0].Message.Content, nil
}
