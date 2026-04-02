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
	mistralAPIURL  = "https://api.mistral.ai/v1/chat/completions"
	modelMistral   = "mistral-small-latest" // Free tier, strong reasoning
)

// MistralClient wraps the Mistral AI Chat Completions API.
type MistralClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewMistralClient creates a Mistral client from the MISTRAL_API_KEY env var.
func NewMistralClient() *MistralClient {
	key := os.Getenv("MISTRAL_API_KEY")
	if key == "" {
		return nil
	}
	return &MistralClient{
		apiKey: key,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (c *MistralClient) IsAvailable() bool {
	return c != nil && c.apiKey != ""
}

// ChatForAudit calls Mistral for signal vetting.
func (c *MistralClient) ChatForAudit(ctx context.Context, system, prompt string) (string, error) {
	if !c.IsAvailable() {
		return "", fmt.Errorf("mistral client not initialized (missing MISTRAL_API_KEY)")
	}

	reqBody := openaiRequest{ // Mistral uses the same OpenAI-compatible format
		Model: modelMistral,
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   350,
		Temperature: 0.1,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal mistral request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mistralAPIURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create mistral request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("mistral http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("mistral api error status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read mistral response: %w", err)
	}

	var oaResp openaiResponse
	if err := json.Unmarshal(body, &oaResp); err != nil {
		return "", fmt.Errorf("unmarshal mistral response: %w", err)
	}

	if len(oaResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from mistral")
	}

	return oaResp.Choices[0].Message.Content, nil
}

func (c *MistralClient) ChatForSignal(ctx context.Context, system, prompt string) (string, error) {
	return c.ChatForAudit(ctx, system, prompt)
}

func (c *MistralClient) ChatForRisk(ctx context.Context, system, prompt string) (string, error) {
	return c.ChatForAudit(ctx, system, prompt)
}

func (c *MistralClient) ChatForMacro(ctx context.Context, system, prompt string) (string, error) {
	return c.ChatForAudit(ctx, system, prompt)
}
