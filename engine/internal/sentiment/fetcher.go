package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// SentimentFetcher polls the local FinBERT sentiment server.
type SentimentFetcher struct {
	client    *http.Client
	serverURL string
	latest    *SentimentData
	mu        sync.RWMutex
}

// NewSentimentFetcher creates a fetcher targeting the given sentiment server URL.
func NewSentimentFetcher(client *http.Client, serverURL string) *SentimentFetcher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &SentimentFetcher{client: client, serverURL: serverURL}
}

// fetch retrieves the latest sentiment from the server.
func (f *SentimentFetcher) fetch(ctx context.Context) (*SentimentData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.serverURL+"/sentiment", nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sentiment server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sentiment server: HTTP %d", resp.StatusCode)
	}
	var raw struct {
		Score       float64  `json:"sentiment_score"`
		Label       string   `json:"sentiment_label"`
		HotKeywords []string `json:"hot_keywords"`
		Velocity    int      `json:"news_velocity"`
		Headlines   []string `json:"top_headlines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("sentiment decode: %w", err)
	}
	data := &SentimentData{
		Score:       raw.Score,
		Label:       raw.Label,
		HotKeywords: raw.HotKeywords,
		Velocity:    raw.Velocity,
		Headlines:   raw.Headlines,
		FetchedAt:   time.Now().UTC(),
	}
	f.mu.Lock()
	f.latest = data
	f.mu.Unlock()
	return data, nil
}

// GetLatest returns the most recently fetched sentiment data, or nil.
func (f *SentimentFetcher) GetLatest() *SentimentData {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.latest
}

// StartPolling polls the sentiment server at the given interval until ctx is cancelled.
// On error, logs a warning and continues using the last cached value.
func (f *SentimentFetcher) StartPolling(ctx context.Context, interval time.Duration) {
	go func() {
		if _, err := f.fetch(ctx); err != nil {
			slog.Warn("[sentiment] initial fetch failed — server may not be running",
				"server", f.serverURL, "error", err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := f.fetch(ctx); err != nil {
					slog.Warn("[sentiment] fetch failed — using cached", "error", err)
				}
			}
		}
	}()
}
