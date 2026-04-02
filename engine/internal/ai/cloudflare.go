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

// Cloudflare Workers AI — free tier with generous limits
// Uses Llama-3.1-70B: best free model on CF, strong JSON output
const cfModel = "@cf/meta/llama-3.1-70b-instruct"

// CloudflareClient wraps the Cloudflare Workers AI REST API.
type CloudflareClient struct {
	apiKey     string
	accountID  string
	httpClient *http.Client
}

// NewCloudflareClient reads CLOUDFLARE_API_KEY and CLOUDFLARE_ACCOUNT_ID env vars.
func NewCloudflareClient() *CloudflareClient {
	key := os.Getenv("CLOUDFLARE_API_KEY")
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	if key == "" || accountID == "" {
		return nil
	}
	return &CloudflareClient{
		apiKey:    key,
		accountID: accountID,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *CloudflareClient) IsAvailable() bool {
	return c != nil && c.apiKey != "" && c.accountID != ""
}

type cfRequest struct {
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type cfResponse struct {
	Result struct {
		Response string `json:"response"`
	} `json:"result"`
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *CloudflareClient) chat(ctx context.Context, system, prompt string, maxTokens int) (string, error) {
	if !c.IsAvailable() {
		return "", fmt.Errorf("cloudflare client not initialized")
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s",
		c.accountID, cfModel)

	reqBody := cfRequest{
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.1,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal cf request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create cf request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cf http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read cf response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cf api error status %d: %s", resp.StatusCode, string(body))
	}

	var cfResp cfResponse
	if err := json.Unmarshal(body, &cfResp); err != nil {
		return "", fmt.Errorf("unmarshal cf response: %w", err)
	}

	if !cfResp.Success {
		if len(cfResp.Errors) > 0 {
			return "", fmt.Errorf("cf api error: %s", cfResp.Errors[0].Message)
		}
		return "", fmt.Errorf("cf api returned success=false")
	}

	return cfResp.Result.Response, nil
}

func (c *CloudflareClient) ChatForAudit(ctx context.Context, system, prompt string) (string, error) {
	return c.chat(ctx, system, prompt, 350)
}

func (c *CloudflareClient) ChatForSignal(ctx context.Context, system, prompt string) (string, error) {
	return c.chat(ctx, system, prompt, 350)
}

func (c *CloudflareClient) ChatForRisk(ctx context.Context, system, prompt string) (string, error) {
	return c.chat(ctx, system, prompt, 400)
}

func (c *CloudflareClient) ChatForMacro(ctx context.Context, system, prompt string) (string, error) {
	return c.chat(ctx, system, prompt, 700)
}
