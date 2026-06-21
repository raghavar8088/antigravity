package scalpers

import "fmt"

// S12 — Vol_Crush_Fade
//
// Regime:     RANGING
// Timeframes: 1h (DVOL spike/crush detection) + 15m (Bollinger mean reversion)
// Rollout:    Phase 3 (see rollout_phase.go)
//
// Architecture decision: this strategy is implemented as its OWN
// self-contained signal generator using Bollinger Bands directly, rather than
// as a confidence modifier bolted onto S2 (Bollinger mean reversion). Reason:
// coupling to another strategy file would require S12 to either (a) call
// S2's Evaluate() and post-process its Signal (fragile — couples to S2's
// internal thresholds/regime gates changing independently), or (b) duplicate
// S2's BB logic anyway to compute a "would-have-fired" signal before
// modifying it. Both are more complex than just using BB() directly here.
// A self-contained generator keeps S12's vol-crush-specific gating (DVOL
// spike-then-crush window) and its mean-reversion entry logic in one
// auditable place, with no hidden coupling to another strategy's future edits.
//
// Logic: after a DVOL spike, detect DVOL crushing down >15% from its recent
// (2hr / 2-bar-on-1h) peak. During that crush window, take Bollinger Band
// mean-reversion entries (price at/beyond a band, RSI confirming exhaustion)
// with a boosted confidence multiplier — vol crush periods historically see
// fast mean reversion back toward the BB midline as realized vol compresses.
//
// Graceful degradation: if DVOL is unpopulated/unhealthy, this strategy
// degrades to a plain BB mean-reversion entry (no crush-confirmation boost,
// lower base confidence) rather than refusing to trade.

const (
	volCrushDropPct  = 0.15 // DVOL must be down >=15% from its 2hr peak
	volCrushLookback = 2    // 2 x 1h bars = 2hr window for the "recent peak"
)

type VolCrushFade struct{}

func (s *VolCrushFade) Name() string { return "Vol_Crush_Fade" }

func (s *VolCrushFade) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *VolCrushFade) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	bb := BB(ctx.Candles15m, 20)
	if bb.Middle == 0 {
		return NoSignal(name)
	}
	rsi := RSI(ctx.Candles15m, 14)
	price := ctx.Price
	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	// 3-bar confirmation: require the band touch to persist across the last 3
	// closed candles (not a single-bar wick poke) — check the last 3 closes
	// against the (recomputed, slightly trailing) bands.
	c15 := ctx.Candles15m
	last3 := c15[len(c15)-3:]
	lowerTouches, upperTouches := 0, 0
	for i := range last3 {
		end := len(c15) - 3 + i + 1
		bbAt := BB(c15[:end], 20)
		if bbAt.Lower == 0 {
			continue
		}
		if last3[i].Close <= bbAt.Lower {
			lowerTouches++
		}
		if last3[i].Close >= bbAt.Upper {
			upperTouches++
		}
	}

	inCrushWindow := false
	crushDesc := "DVOL_unavailable, plain BB mean-reversion fallback"
	usingDVOL := ctx.DVOLPopulated && ctx.DVOLHealthy && len(ctx.DVOLHistory) >= volCrushLookback+1
	if usingDVOL {
		hist := ctx.DVOLHistory
		peak := hist[len(hist)-volCrushLookback-1]
		for i := len(hist) - volCrushLookback - 1; i < len(hist); i++ {
			if hist[i] > peak {
				peak = hist[i]
			}
		}
		if peak > 0 {
			dropPct := (peak - ctx.DVOL) / peak
			inCrushWindow = dropPct >= volCrushDropPct
			crushDesc = fmt.Sprintf("DVOL crush: peak=%.1f now=%.1f drop=%.1f%%", peak, ctx.DVOL, dropPct*100)
		}
	}

	baseConfidence := 0.55
	if inCrushWindow {
		baseConfidence = 0.72 // boosted confidence during confirmed vol-crush window
	} else if !usingDVOL {
		baseConfidence = 0.50 // fallback, no crush confirmation available
	} else {
		// DVOL is healthy but we're not in a crush window — still allow plain
		// BB mean reversion, but without the boost.
		baseConfidence = 0.55
	}

	slDist := max64(1.0*atr15m, 0.003*price)

	if lowerTouches >= 3 && rsi < 35 {
		sl := price - slDist
		tp1 := bb.Middle
		// Enforce R:R >= 2:1 — if BB midline target doesn't reach 2x SL distance,
		// extend TP to satisfy the gate.
		if tp1-price < 2.0*slDist {
			tp1 = price + 2.0*slDist
		}
		tp2 := price + 3.0*slDist
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  baseConfidence,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"Vol crush fade LONG: %s, price at lower BB (3-bar confirmed, %d/3 touches), RSI=%.1f<35",
				crushDesc, lowerTouches, rsi,
			),
		}
	}

	if upperTouches >= 3 && rsi > 65 {
		sl := price + slDist
		tp1 := bb.Middle
		if price-tp1 < 2.0*slDist {
			tp1 = price - 2.0*slDist
		}
		tp2 := price - 3.0*slDist
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  baseConfidence,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"Vol crush fade SHORT: %s, price at upper BB (3-bar confirmed, %d/3 touches), RSI=%.1f>65",
				crushDesc, upperTouches, rsi,
			),
		}
	}

	return NoSignal(name)
}
