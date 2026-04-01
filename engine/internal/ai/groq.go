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
	groqAPIURL    = "https://api.groq.com/openai/v1/chat/completions"
	modelGroqFree = "llama-3.3-70b-versatile" // Fast, high-reasoning, and generous free tier
)

// GroqClient wraps the Groq Chat Completions API.
type GroqClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewGroqClient creates a Groq API client from the GROQ_API_KEY env var.
// Returns nil if the key is not set.
func NewGroqClient() *GroqClient {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		return nil
	}
	return &GroqClient{
		apiKey: key,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *GroqClient) IsAvailable() bool {
	return c != nil && c.apiKey != ""
}

// ChatForAudit calls llama-3-70b for fast, free signal vetting.
func (c *GroqClient) ChatForAudit(ctx context.Context, system, prompt string) (string, error) {
	if !c.IsAvailable() {
		return "", fmt.Errorf("groq client not initialized (missing GROQ_API_KEY)")
	}

	reqBody := openaiRequest{
		Model: modelGroqFree,
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   350,
		Temperature: 0.1,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqAPIURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("groq api error status %d: %s", resp.StatusCode, string(body))
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
		return "", fmt.Errorf("empty response from groq")
	}

	return oaResp.Choices[0].Message.Content, nil
}

// ChatForSignal calls llama-3-70b/8b for ultra-fast trading signal generation.
func (c *GroqClient) ChatForSignal(ctx context.Context, system, prompt string) (string, error) {
	return c.ChatForAudit(ctx, system, prompt) // Shares the same logic as auditing
}

// ChatForRisk calls llama-3-70b for fast risk arbitration.
func (c *GroqClient) ChatForRisk(ctx context.Context, system, prompt string) (string, error) {
	return c.ChatForAudit(ctx, system, prompt)
}

// ChatForMacro provides macro context if Gemini is down.
func (c *GroqClient) ChatForMacro(ctx context.Context, system, prompt string) (string, error) {
	return c.ChatForAudit(ctx, system, prompt)
}
