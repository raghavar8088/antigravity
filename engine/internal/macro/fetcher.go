package macro

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// MacroFetcher calls a Python script hourly to compute macro correlation data.
type MacroFetcher struct {
	pythonPath string
	scriptPath string
	latest     *MacroData
	mu         sync.RWMutex
}

// NewMacroFetcher creates a fetcher that invokes pythonPath scriptPath on demand.
func NewMacroFetcher(pythonPath, scriptPath string) *MacroFetcher {
	return &MacroFetcher{pythonPath: pythonPath, scriptPath: scriptPath}
}

// fetch runs the Python script and parses macro data.
func (f *MacroFetcher) fetch(ctx context.Context) (*MacroData, error) {
	cmd := exec.CommandContext(ctx, f.pythonPath, f.scriptPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("macro_fetcher.py: %w", err)
	}
	var raw struct {
		SPYCorrelation float64 `json:"spy_correlation"`
		DXYCorrelation float64 `json:"dxy_correlation"`
		VIX            float64 `json:"vix"`
		MacroCoupled   bool    `json:"macro_coupled"`
		DXYTrend       string  `json:"dxy_trend"`
		SPYDir1h       string  `json:"spy_direction_1h"`
		MacroScore     float64 `json:"macro_score"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("macro parse: %w", err)
	}
	data := &MacroData{
		SPY_Correlation: raw.SPYCorrelation,
		DXY_Correlation: raw.DXYCorrelation,
		VIX:             raw.VIX,
		SPY_Dir_1h:      raw.SPYDir1h,
		DXY_Trend:       raw.DXYTrend,
		MacroCoupled:    raw.MacroCoupled,
		FetchedAt:       time.Now().UTC(),
	}
	data.Score = ComputeMacroScore(*data)
	f.mu.Lock()
	f.latest = data
	f.mu.Unlock()
	return data, nil
}

// GetLatest returns the most recently fetched macro data, or nil.
func (f *MacroFetcher) GetLatest() *MacroData {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.latest
}

// StartHourlyPoll runs Fetch every hour until ctx is cancelled.
// On error, logs a warning and continues using the last cached value.
func (f *MacroFetcher) StartHourlyPoll(ctx context.Context) {
	go func() {
		if _, err := f.fetch(ctx); err != nil {
			slog.Warn("[macro] initial fetch failed", "error", err)
		}
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if data, err := f.fetch(ctx); err != nil {
					slog.Warn("[macro] fetch failed — using cached", "error", err)
				} else {
					slog.Info("[macro] updated",
						"vix", data.VIX,
						"coupled", data.MacroCoupled,
						"spy_dir", data.SPY_Dir_1h,
						"score", data.Score,
					)
				}
			}
		}
	}()
}
