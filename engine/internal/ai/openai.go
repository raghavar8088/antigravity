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
	openaiAPIURL = "https://api.openai.com/v1/chat/completions"

	// gpt-4o-mini: ultra-fast, low latency — ideal for real-time trading signals
	modelSignalsGPT = "gpt-4o-mini"
	// gpt-4o: strongest reasoning — used for risk arbitration
	modelRiskGPT = "gpt-4o"
)

// OpenAIClient wraps the OpenAI Chat Completions API.
type OpenAIClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewOpenAIClient creates an OpenAI API client from the OPENAI_API_KEY env var.
// Returns nil if the key is not set.
func NewOpenAIClient() *OpenAIClient {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil
	}
	return &OpenAIClient{
		apiKey: key,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *OpenAIClient) IsAvailable() bool {
	return c != nil && c.apiKey != ""
}

// openaiRequest is the OpenAI Chat Completions API request body.
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse is the OpenAI Chat Completions API response.
type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Chat sends a message to OpenAI and returns the text response.
func (c *OpenAIClient) Chat(ctx context.Context, model, system, userMessage string, maxTokens int) (string, error) {
	if !c.IsAvailable() {
		return "", fmt.Errorf("openai client not initialized (missing OPENAI_API_KEY)")
	}

	reqBody := openaiRequest{
		Model: model,
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: userMessage},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.1, // Low temperature for consistent JSON output
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiAPIURL, bytes.NewReader(data))
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var oaResp openaiResponse
	if err := json.Unmarshal(body, &oaResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if oaResp.Error != nil {
		return "", fmt.Errorf("openai api error [%s]: %s", oaResp.Error.Type, oaResp.Error.Message)
	}

	if len(oaResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from openai")
	}

	return oaResp.Choices[0].Message.Content, nil
}

// ChatForSignal calls gpt-4o-mini for fast trading signal generation.
func (c *OpenAIClient) ChatForSignal(ctx context.Context, system, prompt string) (string, error) {
	return c.Chat(ctx, modelSignalsGPT, system, prompt, 600)
}

// ChatForRisk calls gpt-4o for more careful risk arbitration.
func (c *OpenAIClient) ChatForRisk(ctx context.Context, system, prompt string) (string, error) {
	return c.Chat(ctx, modelRiskGPT, system, prompt, 400)
}
