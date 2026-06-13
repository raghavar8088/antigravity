package etf

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// ETFFetcher calls a Python script daily to retrieve ETF flow data via yfinance.
type ETFFetcher struct {
	pythonPath string
	scriptPath string
	latest     *ETFFlowData
	mu         sync.RWMutex
}

// NewETFFetcher creates a fetcher that invokes pythonPath scriptPath on demand.
func NewETFFetcher(pythonPath, scriptPath string) *ETFFetcher {
	return &ETFFetcher{pythonPath: pythonPath, scriptPath: scriptPath}
}

// Fetch executes the Python script and parses its JSON output.
func (f *ETFFetcher) Fetch(ctx context.Context) (*ETFFlowData, error) {
	cmd := exec.CommandContext(ctx, f.pythonPath, f.scriptPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("etf_fetcher.py: %w", err)
	}
	var raw struct {
		Date          string             `json:"date"`
		Flows         map[string]float64 `json:"flows"`
		TotalFlowUSD  float64            `json:"total_flow_usd"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("etf parse: %w", err)
	}

	data := &ETFFlowData{
		Date:      raw.Date,
		Flows:     raw.Flows,
		TotalFlowUSD: raw.TotalFlowUSD,
		FetchedAt: time.Now().UTC(),
		IsProxy:   true,
	}

	// Determine trend.
	switch {
	case raw.TotalFlowUSD > thresholdMildBullish:
		data.ETFFlowTrend = "ACCUMULATING"
	case raw.TotalFlowUSD < thresholdMildBearish:
		data.ETFFlowTrend = "DISTRIBUTING"
	default:
		data.ETFFlowTrend = "NEUTRAL"
	}

	// Find largest single ETF.
	var maxAbs float64
	for ticker, flow := range raw.Flows {
		abs := flow
		if abs < 0 {
			abs = -abs
		}
		if abs > maxAbs {
			maxAbs = abs
			data.LargestSingleETF = ticker
		}
	}

	// Update streak counters.
	f.mu.RLock()
	prev := f.latest
	f.mu.RUnlock()
	if prev != nil {
		if raw.TotalFlowUSD > 0 && prev.TotalFlowUSD > 0 {
			data.ConsecutiveInflow = prev.ConsecutiveInflow + 1
		}
		if raw.TotalFlowUSD < 0 && prev.TotalFlowUSD < 0 {
			data.ConsecutiveOutflow = prev.ConsecutiveOutflow + 1
		}
	} else if raw.TotalFlowUSD > 0 {
		data.ConsecutiveInflow = 1
	} else if raw.TotalFlowUSD < 0 {
		data.ConsecutiveOutflow = 1
	}

	f.mu.Lock()
	f.latest = data
	f.mu.Unlock()
	return data, nil
}

// GetLatest returns the most recently fetched data, or nil if none yet.
func (f *ETFFetcher) GetLatest() *ETFFlowData {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.latest
}

// StartDailyPoll runs Fetch every day at 09:30 UTC (30m after US market open).
func (f *ETFFetcher) StartDailyPoll(ctx context.Context) {
	go func() {
		for {
			// Calculate time to next 09:30 UTC.
			now := time.Now().UTC()
			target := time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, time.UTC)
			if now.After(target) {
				target = target.Add(24 * time.Hour)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(target)):
			}
			data, err := f.Fetch(ctx)
			if err != nil {
				slog.Warn("[ETF] fetch failed", "error", err)
				continue
			}
			score := ComputeETFScore(*data)
			slog.Info("[ETF] flows updated",
				"total_usd", data.TotalFlowUSD,
				"trend", data.ETFFlowTrend,
				"score", score,
			)
		}
	}()
}
