// Package trading — SignalQualityEngine
//
// Replaces AI agent judgment (Bull + Bear + Macro agents) with a deterministic,
// explainable scoring function.  Every decision is reproducible: the same
// market context always produces the same score and the same reasoning string.
//
// Score components (total 100 pts):
//
//	[25 pts] Trend Strength    — ADX magnitude and DI alignment
//	[20 pts] Volume Context    — volume ratio vs N-bar average
//	[20 pts] Regime Alignment  — strategy family vs current regime
//	[15 pts] Volatility Fit    — ATR vs fee hurdle
//	[10 pts] Strategy Health   — historical win rate (from tracker)
//	[10 pts] Confidence Pass   — raw strategy confidence ≥ threshold
//
// Minimum score to pass:  50 / 100  (configurable via ScoringConfig)
package trading

import (
	"fmt"
	"math"
	"strings"

	"antigravity-engine/internal/regime"
)

// ── Score result ───────────────────────────────────────────────────────────────

// ScoreResult carries the numeric score and an auditable breakdown.
type ScoreResult struct {
	Total          int    // 0–100
	Passed         bool   // Total >= MinPassScore
	TrendScore     int    // 0–25
	VolumeScore    int    // 0–20
	RegimeScore    int    // 0–20
	VolatilityScore int   // 0–15
	HealthScore    int    // 0–10
	ConfidenceScore int   // 0–10
	RejectReason   string // populated when Passed == false
	Explanation    string // human-readable audit trail
}

// ── Scoring config ─────────────────────────────────────────────────────────────

// ScoringConfig controls thresholds used by the engine.
type ScoringConfig struct {
	MinPassScore    int     // minimum total to allow execution  (default 50)
	MinConfidence   float64 // strategy confidence floor          (default 0.65)
	MinATRPct       float64 // ATR% floor — fees must be coverable (default 0.10)
	FeePct          float64 // round-trip fee + slippage as % of notional (default 0.12)
	MinHealthWinRate float64 // historical win rate floor          (default 0.30)
}

func DefaultScoringConfig() ScoringConfig {
	return ScoringConfig{
		MinPassScore:     50,
		MinConfidence:    0.65,
		MinATRPct:        0.10,
		FeePct:           0.12,
		MinHealthWinRate: 0.30,
	}
}

// ── Market context fed to Score() ─────────────────────────────────────────────

// MarketContext carries the live market state required for scoring.
type MarketContext struct {
	Regime       regime.Regime
	ADX          float64
	PlusDI       float64
	MinusDI      float64
	ATRPct       float64 // ATR as % of price
	VolumeRatio  float64 // current bar volume / N-bar average
	Direction    string  // "BUY" | "SELL"
	Confidence   float64 // strategy-reported confidence (0–1)
	StrategyName string
	WinRate      float64 // historical win rate from strategy tracker (0–1); 0 = unknown
	TotalTrades  int     // total historical trades; 0 = untested
}

// ── Engine ─────────────────────────────────────────────────────────────────────

// SignalQualityEngine scores signals deterministically.
// It is stateless and safe for concurrent calls.
type SignalQualityEngine struct {
	cfg    ScoringConfig
	router *regime.Router
}

// NewSignalQualityEngine creates a scorer with the given config.
func NewSignalQualityEngine(cfg ScoringConfig, router *regime.Router) *SignalQualityEngine {
	return &SignalQualityEngine{cfg: cfg, router: router}
}

// NewSignalQualityEngineDefault creates a scorer with production defaults.
func NewSignalQualityEngineDefault() *SignalQualityEngine {
	return NewSignalQualityEngine(DefaultScoringConfig(), regime.NewRouter())
}

// Score evaluates a signal given market context and returns a ScoreResult.
// This call is allocation-light and deterministic — no IO, no goroutines.
func (e *SignalQualityEngine) Score(ctx MarketContext) ScoreResult {
	var sb strings.Builder
	r := ScoreResult{}

	// ── [25 pts] Trend Strength ────────────────────────────────────────────────
	ts := e.scoreTrend(ctx.ADX, ctx.PlusDI, ctx.MinusDI, ctx.Direction)
	r.TrendScore = ts
	fmt.Fprintf(&sb, "trend=%d/25(ADX=%.1f +DI=%.1f -DI=%.1f) ", ts, ctx.ADX, ctx.PlusDI, ctx.MinusDI)

	// ── [20 pts] Volume Context ────────────────────────────────────────────────
	vs := e.scoreVolume(ctx.VolumeRatio)
	r.VolumeScore = vs
	fmt.Fprintf(&sb, "vol=%d/20(ratio=%.2f) ", vs, ctx.VolumeRatio)

	// ── [20 pts] Regime Alignment ──────────────────────────────────────────────
	family := regime.FamilyFromName(ctx.StrategyName)
	rs := e.scoreRegime(ctx.Regime, family, ctx.Direction)
	r.RegimeScore = rs
	fmt.Fprintf(&sb, "regime=%d/20(%s/%s) ", rs, ctx.Regime, family)

	// ── [15 pts] Volatility Fit ────────────────────────────────────────────────
	voltScore := e.scoreVolatility(ctx.ATRPct)
	r.VolatilityScore = voltScore
	fmt.Fprintf(&sb, "atr=%d/15(%.3f%%) ", voltScore, ctx.ATRPct)

	// ── [10 pts] Strategy Health ───────────────────────────────────────────────
	hs := e.scoreHealth(ctx.WinRate, ctx.TotalTrades)
	r.HealthScore = hs
	fmt.Fprintf(&sb, "health=%d/10(wr=%.0f%%,n=%d) ", hs, ctx.WinRate*100, ctx.TotalTrades)

	// ── [10 pts] Confidence ────────────────────────────────────────────────────
	cs := e.scoreConfidence(ctx.Confidence)
	r.ConfidenceScore = cs
	fmt.Fprintf(&sb, "conf=%d/10(%.2f)", cs, ctx.Confidence)

	total := ts + vs + rs + voltScore + hs + cs
	r.Total = total
	r.Explanation = sb.String()

	// ── Rejection reasons (ordered by precedence) ──────────────────────────────
	switch {
	case ctx.Regime == regime.RegimeUnknown:
		r.RejectReason = "REGIME_UNKNOWN"
	case rs == 0 && ctx.Regime != regime.RegimeUnknown:
		r.RejectReason = "REGIME_MISALIGN"
	case ctx.Confidence < e.cfg.MinConfidence:
		r.RejectReason = "LOW_CONFIDENCE"
	case ctx.ATRPct < e.cfg.MinATRPct:
		r.RejectReason = "ATR_FEES"
	case total < e.cfg.MinPassScore:
		r.RejectReason = "LOW_SIGNAL_SCORE"
	}

	r.Passed = r.RejectReason == ""
	return r
}

// ── Component scorers ──────────────────────────────────────────────────────────

// scoreTrend awards up to 25 points.
//
//	ADX 25–35 + aligned DI  → 25
//	ADX 20–25 + aligned DI  → 18
//	ADX 15–20 aligned       → 12
//	ADX < 15                →  0 (choppy, no trend credit)
//	DI conflict             → halve the trend score
func (e *SignalQualityEngine) scoreTrend(adx, plusDI, minusDI float64, direction string) int {
	if adx < 15 {
		return 0
	}
	var base int
	switch {
	case adx >= 35:
		base = 25
	case adx >= 28:
		base = 22
	case adx >= 22:
		base = 18
	case adx >= 18:
		base = 12
	default:
		base = 6
	}

	// DI alignment bonus/penalty
	aligned := (direction == "BUY" && plusDI > minusDI) || (direction == "SELL" && minusDI > plusDI)
	if !aligned {
		base = base / 2
	}
	return clamp(base, 0, 25)
}

// scoreVolume awards up to 20 points based on volume ratio.
//
//	> 2.0× avg  → 20 (strong confirmation)
//	1.5–2.0×    → 15
//	1.0–1.5×    → 10
//	0.7–1.0×    →  5 (below average but tradeable)
//	< 0.7×      →  0 (thin volume, skip)
func (e *SignalQualityEngine) scoreVolume(ratio float64) int {
	switch {
	case ratio >= 2.0:
		return 20
	case ratio >= 1.5:
		return 15
	case ratio >= 1.0:
		return 10
	case ratio >= 0.7:
		return 5
	default:
		return 0
	}
}

// scoreRegime awards up to 20 points.
// Returns 0 if the strategy family is explicitly inactive in the current regime
// (hard block — ensures regime routing works even if total score somehow passes).
func (e *SignalQualityEngine) scoreRegime(r regime.Regime, family regime.StrategyFamily, direction string) int {
	// Block inactive families outright
	if !e.router.IsActive(r, family) {
		return 0
	}

	// Full credit for perfect alignment
	switch r {
	case regime.RegimeTrendingBull:
		if direction == "BUY" {
			return 20
		}
		return 8 // counter-trend short allowed but lower credit
	case regime.RegimeTrendingBear:
		if direction == "SELL" {
			return 20
		}
		return 8
	case regime.RegimeRanging:
		return 16 // mean-reversion works both ways in ranging
	case regime.RegimeHighVol:
		return 12 // scalp/vol strategies in high-vol
	case regime.RegimeLowVol:
		return 12
	default:
		return 0
	}
}

// scoreVolatility awards up to 15 points based on ATR-vs-fee coverage.
// The fee hurdle (round-trip + slippage ≈ 0.12%) must be covered by the ATR.
func (e *SignalQualityEngine) scoreVolatility(atrPct float64) int {
	if atrPct < e.cfg.MinATRPct {
		return 0
	}
	coverage := atrPct / e.cfg.FeePct
	switch {
	case coverage >= 4.0:
		return 15
	case coverage >= 3.0:
		return 12
	case coverage >= 2.0:
		return 9
	case coverage >= 1.5:
		return 6
	default:
		return 3
	}
}

// scoreHealth awards up to 10 points based on historical performance.
// Untested strategies (TotalTrades == 0) receive 5 — neutral, not penalised.
func (e *SignalQualityEngine) scoreHealth(winRate float64, trades int) int {
	if trades == 0 {
		return 5 // untested — give benefit of the doubt
	}
	if winRate < e.cfg.MinHealthWinRate {
		return 0
	}
	switch {
	case winRate >= 0.60:
		return 10
	case winRate >= 0.50:
		return 8
	case winRate >= 0.45:
		return 6
	case winRate >= 0.40:
		return 4
	default:
		return 2
	}
}

// scoreConfidence awards up to 10 points.
func (e *SignalQualityEngine) scoreConfidence(conf float64) int {
	switch {
	case conf >= 0.90:
		return 10
	case conf >= 0.80:
		return 8
	case conf >= 0.70:
		return 6
	case conf >= 0.65:
		return 4
	default:
		return 0
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────

func clamp(v, lo, hi int) int {
	return int(math.Max(float64(lo), math.Min(float64(hi), float64(v))))
}
