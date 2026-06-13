package temporal

import (
	"testing"
	"time"
)

func TestAnalyse_EmptyPatterns_Neutral(t *testing.T) {
	a := NewTemporalAnalyser()
	result := a.Analyse(time.Now().UTC())
	if result.Bias != TemporalNeutral {
		t.Errorf("expected NEUTRAL with no patterns, got %s", result.Bias)
	}
	if result.SizeModifier != 1.0 {
		t.Errorf("expected size modifier 1.0, got %v", result.SizeModifier)
	}
}

func TestAnalyse_Favorable(t *testing.T) {
	a := NewTemporalAnalyser()
	a.mu.Lock()
	a.hourPatterns[14] = TemporalPattern{HourUTC: 14, WinRate: 0.70, TradeCount: 20}
	a.dayPatterns[1] = TemporalPattern{DayOfWeek: 1, WinRate: 0.65, TradeCount: 15}
	a.mu.Unlock()

	// Monday at 14:00 UTC
	t1 := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC) // Monday
	result := a.Analyse(t1)
	if result.Bias != TemporalFavorable {
		t.Errorf("expected FAVORABLE, got %s", result.Bias)
	}
	if result.SizeModifier != 1.1 {
		t.Errorf("expected 1.1, got %v", result.SizeModifier)
	}
}

func TestAnalyse_Unfavorable(t *testing.T) {
	a := NewTemporalAnalyser()
	a.mu.Lock()
	a.hourPatterns[3] = TemporalPattern{HourUTC: 3, WinRate: 0.30, TradeCount: 12}
	a.dayPatterns[0] = TemporalPattern{DayOfWeek: 0, WinRate: 0.35, TradeCount: 11}
	a.mu.Unlock()

	// Sunday at 03:00 UTC
	t1 := time.Date(2026, 6, 14, 3, 0, 0, 0, time.UTC) // Sunday
	result := a.Analyse(t1)
	if result.Bias != TemporalUnfavorable {
		t.Errorf("expected UNFAVORABLE, got %s", result.Bias)
	}
	if result.SizeModifier != 0.7 {
		t.Errorf("expected 0.7, got %v", result.SizeModifier)
	}
}

func TestCMEWindowSundayEvening(t *testing.T) {
	// Sunday 22:00 UTC — CME open approaching
	sunday := time.Date(2026, 6, 14, 22, 0, 0, 0, time.UTC)
	active, mod, desc := cmeWindow(sunday)
	if !active || mod != 0.7 || desc != "CME_OPEN_APPROACHING" {
		t.Errorf("expected CME_OPEN_APPROACHING 0.7, got active=%v mod=%v desc=%s", active, mod, desc)
	}
}

func TestCMEWindowFridayClose(t *testing.T) {
	// Friday 21:00 UTC — CME close approaching
	friday := time.Date(2026, 6, 12, 21, 0, 0, 0, time.UTC)
	active, mod, desc := cmeWindow(friday)
	if !active || mod != 0.7 || desc != "CME_CLOSE_APPROACHING" {
		t.Errorf("expected CME_CLOSE_APPROACHING 0.7, got active=%v mod=%v desc=%s", active, mod, desc)
	}
}

func TestEffectiveModifier_CMEReducesFavorable(t *testing.T) {
	a := NewTemporalAnalyser()
	a.mu.Lock()
	a.hourPatterns[21] = TemporalPattern{HourUTC: 21, WinRate: 0.70, TradeCount: 20}
	a.dayPatterns[0] = TemporalPattern{DayOfWeek: 0, WinRate: 0.65, TradeCount: 15}
	a.mu.Unlock()

	// Sunday 21:00 — FAVORABLE pattern but CME window
	t1 := time.Date(2026, 6, 14, 21, 0, 0, 0, time.UTC)
	result := a.Analyse(t1)
	// 1.1 (favorable) × 0.7 (CME) = 0.77
	if result.EffectiveModifier < 0.76 || result.EffectiveModifier > 0.78 {
		t.Errorf("expected effective modifier ~0.77, got %v", result.EffectiveModifier)
	}
}
