package temporal

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

const minTradesForPattern = 10

// TemporalAnalyser uses historical win-rate patterns to size trades by time of day.
type TemporalAnalyser struct {
	hourPatterns map[int]TemporalPattern // key: hour UTC (0-23)
	dayPatterns  map[int]TemporalPattern // key: day of week (0=Sun)
	mu           sync.RWMutex
}

// NewTemporalAnalyser creates an empty analyser. Call LoadPatterns() to populate it.
func NewTemporalAnalyser() *TemporalAnalyser {
	return &TemporalAnalyser{
		hourPatterns: make(map[int]TemporalPattern),
		dayPatterns:  make(map[int]TemporalPattern),
	}
}

// LoadPatterns reads patterns from a JSON file produced by pattern_builder.go.
func (a *TemporalAnalyser) LoadPatterns(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("temporal: load %s: %w", path, err)
	}
	var stored struct {
		HourPatterns []TemporalPattern `json:"hour_patterns"`
		DayPatterns  []TemporalPattern `json:"day_patterns"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("temporal: parse: %w", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, p := range stored.HourPatterns {
		a.hourPatterns[p.HourUTC] = p
	}
	for _, p := range stored.DayPatterns {
		a.dayPatterns[p.DayOfWeek] = p
	}
	return nil
}

// Analyse returns a TemporalAnalysis for the given timestamp (should be UTC).
func (a *TemporalAnalyser) Analyse(t time.Time) TemporalAnalysis {
	t = t.UTC()
	hour := t.Hour()
	dow := int(t.Weekday()) // 0=Sun

	a.mu.RLock()
	hourPat := a.hourPatterns[hour]
	dayPat := a.dayPatterns[dow]
	a.mu.RUnlock()

	// Determine bias from win rates.
	hourOK := hourPat.TradeCount >= minTradesForPattern
	dayOK := dayPat.TradeCount >= minTradesForPattern
	bias := TemporalNeutral
	sizeMod := 1.0
	switch {
	case hourOK && dayOK && hourPat.WinRate > 0.60 && dayPat.WinRate > 0.60:
		bias = TemporalFavorable
		sizeMod = 1.1
	case hourOK && dayOK && hourPat.WinRate < 0.40 && dayPat.WinRate < 0.40:
		bias = TemporalUnfavorable
		sizeMod = 0.7
	}

	// CME window modifiers.
	cmeActive, cmeMod, cmeDesc := cmeWindow(t)

	desc := fmt.Sprintf("bias=%s mod=%.2f cme=%v", bias, sizeMod*cmeMod, cmeActive)
	if cmeActive {
		desc += " " + cmeDesc
	}

	return TemporalAnalysis{
		CurrentHourPattern: hourPat,
		CurrentDayPattern:  dayPat,
		Bias:               bias,
		SizeModifier:       sizeMod,
		CMEWindowActive:    cmeActive,
		CMEModifier:        cmeMod,
		EffectiveModifier:  sizeMod * cmeMod,
		Description:        desc,
	}
}

// GetLatest returns a fresh analysis for time.Now().UTC().
func (a *TemporalAnalyser) GetLatest() TemporalAnalysis {
	return a.Analyse(time.Now().UTC())
}

// cmeWindow returns whether the current UTC time falls in a CME volatility window.
func cmeWindow(t time.Time) (active bool, modifier float64, desc string) {
	dow := t.Weekday()
	h := t.Hour()
	m := t.Minute()
	// Sunday 21:00–23:59 — CME open approaching
	if dow == time.Sunday && h >= 21 {
		return true, 0.7, "CME_OPEN_APPROACHING"
	}
	// Monday 00:00–02:00 — CME post-open
	if dow == time.Monday && (h < 2 || (h == 2 && m == 0)) {
		return true, 0.85, "CME_POST_OPEN"
	}
	// Friday 20:00–22:00 — CME close approaching
	if dow == time.Friday && h >= 20 && h < 22 {
		return true, 0.7, "CME_CLOSE_APPROACHING"
	}
	// Friday 22:00–23:59 — CME post-close
	if dow == time.Friday && h >= 22 {
		return true, 0.85, "CME_POST_CLOSE"
	}
	return false, 1.0, ""
}
