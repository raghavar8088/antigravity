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
	// HuggingFace serverless inference — OpenAI-compatible endpoint
	hfAPIURL  = "https://api-inference.huggingface.co/v1/chat/completions"
	// Qwen2.5-72B: top free model on HF, strong reasoning, JSON-reliable
	modelHF   = "Qwen/Qwen2.5-72B-Instruct"
)

// HuggingFaceClient wraps the HuggingFace Serverless Inference API.
type HuggingFaceClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewHuggingFaceClient creates a HF client from the HUGGINGFACE_API_KEY env var.
func NewHuggingFaceClient() *HuggingFaceClient {
	key := os.Getenv("HUGGINGFACE_API_KEY")
	if key == "" {
		return nil
	}
	return &HuggingFaceClient{
		apiKey: key,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *HuggingFaceClient) IsAvailable() bool {
	return c != nil && c.apiKey != ""
}

// ChatForAudit calls Qwen2.5-72B for signal vetting.
func (c *HuggingFaceClient) ChatForAudit(ctx context.Context, system, prompt string) (string, error) {
	if !c.IsAvailable() {
		return "", fmt.Errorf("huggingface client not initialized (missing HUGGINGFACE_API_KEY)")
	}

	reqBody := openaiRequest{ // HF uses OpenAI-compatible format
		Model: modelHF,
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   350,
		Temperature: 0.1,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal hf request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hfAPIURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create hf request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("hf http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("hf api error status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read hf response: %w", err)
	}

	var oaResp openaiResponse
	if err := json.Unmarshal(body, &oaResp); err != nil {
		return "", fmt.Errorf("unmarshal hf response: %w", err)
	}

	if len(oaResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from huggingface")
	}

	return oaResp.Choices[0].Message.Content, nil
}

func (c *HuggingFaceClient) ChatForSignal(ctx context.Context, system, prompt string) (string, error) {
	return c.ChatForAudit(ctx, system, prompt)
}

func (c *HuggingFaceClient) ChatForRisk(ctx context.Context, system, prompt string) (string, error) {
	return c.ChatForAudit(ctx, system, prompt)
}

func (c *HuggingFaceClient) ChatForMacro(ctx context.Context, system, prompt string) (string, error) {
	return c.ChatForAudit(ctx, system, prompt)
}
