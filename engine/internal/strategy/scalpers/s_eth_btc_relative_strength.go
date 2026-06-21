package scalpers

import (
	"fmt"
	"sync"

	"antigravity-engine/internal/dominance"
)

// S25 — Dominance_Relative_Strength
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h (BTC's own EMA trend) + dominance feed (hourly)
//
// DATA SOURCE DECISION: a CoinGecko-backed BTC dominance fetcher
// (engine/internal/dominance/fetcher.go, DominanceFetcher) ALREADY EXISTS in
// this codebase — it polls the free, no-auth CoinGecko /api/v3/global
// endpoint hourly and is already started in engine/cmd/antigravity/main.go
// ("Wiring 9: BTC dominance tracker"), but its output (DominanceData: BTC_D_Pct,
// Trend, Delta1h, Delta24h, EMA14) was not previously consumed by any
// strategy. This makes the true-CoinGecko-dominance path strictly faster to
// wire up than building a brand-new ETH/BTC fallback feed from scratch, so
// S25 uses it directly: that satisfies the original spec's primary ask
// ("Dominance_Relative_Strength") without needing the documented
// ETH/BTC-relative-strength fallback. No new market-data feed file was
// created — see SetDominanceFetcher below for the minimal wiring needed.
//
// Logic: BTC dominance RISING (capital rotating into BTC, out of alts) +
// BTC's own 1h EMA9>EMA21 trend confirms LONG conviction (BTC outperforming
// the broader crypto market while in its own uptrend). BTC dominance FALLING
// (capital rotating into alts, BTC underperforming) + BTC's own 1h
// EMA9<EMA21 downtrend confirms SHORT conviction. FLAT dominance or
// contradicting signals -> NoSignal.
//
// Graceful degradation: if the dominance fetcher has not yet produced a
// reading (GetLatest() == nil) or its trend is unavailable, the strategy
// falls back to BTC's own 1h EMA trend alone at reduced confidence rather
// than permanently refusing to trade — the dominance feed retries hourly via
// its own StartPolling loop, so the gap is self-healing, not permanent.

// dominanceFetcherMu guards the package-level dominance fetcher reference.
// Wired once at startup via SetDominanceFetcher; read-locked on every
// Evaluate() call from the (single-threaded) trading loop.
var (
	dominanceFetcherMu     sync.RWMutex
	sharedDominanceFetcher *dominance.DominanceFetcher
)

// SetDominanceFetcher wires the already-running dominance.DominanceFetcher
// (started in cmd/antigravity/main.go "Wiring 9") into the scalpers package
// so S25 can read its latest BTC.D reading. Call once at startup, e.g.:
//
//	scalpers.SetDominanceFetcher(dominanceFetcher)
//
// Safe to leave unset — S25 degrades to BTC-only-trend mode if so (see
// Evaluate() below).
func SetDominanceFetcher(f *dominance.DominanceFetcher) {
	dominanceFetcherMu.Lock()
	sharedDominanceFetcher = f
	dominanceFetcherMu.Unlock()
}

func getDominanceFetcher() *dominance.DominanceFetcher {
	dominanceFetcherMu.RLock()
	defer dominanceFetcherMu.RUnlock()
	return sharedDominanceFetcher
}

type DominanceRelativeStrength struct{}

func (s *DominanceRelativeStrength) Name() string { return "Dominance_Relative_Strength" }

func (s *DominanceRelativeStrength) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *DominanceRelativeStrength) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}

	ema9 := EMA(ctx.Candles1h, 9)
	ema21 := EMA(ctx.Candles1h, 21)
	if ema9 == 0 || ema21 == 0 {
		return NoSignal(name)
	}
	btcUptrend := ema9 > ema21
	btcDowntrend := ema9 < ema21
	if !btcUptrend && !btcDowntrend {
		return NoSignal(name)
	}

	price := ctx.Price
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	fetcher := getDominanceFetcher()
	var data *dominance.DominanceData
	if fetcher != nil {
		data = fetcher.GetLatest()
	}

	// ── Graceful degradation: dominance feed unavailable/not yet populated ──
	if data == nil {
		// Fall back to BTC's own 1h trend alone, at reduced confidence. Not
		// a permanent NoSignal — the underlying fetcher retries hourly.
		if btcUptrend {
			sl := price - maxF(1.0*atr1h, 0.003*price)
			risk := price - sl
			tp := price + 2.0*risk
			if risk <= 0 {
				return NoSignal(name)
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionLong,
				Confidence: 0.55,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason:     "Dominance feed unavailable (degraded mode): BTC 1h EMA9>EMA21 uptrend only -> LONG at reduced confidence",
			}
		}
		sl := price + maxF(1.0*atr1h, 0.003*price)
		risk := sl - price
		tp := price - 2.0*risk
		if risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.55,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason:     "Dominance feed unavailable (degraded mode): BTC 1h EMA9<EMA21 downtrend only -> SHORT at reduced confidence",
		}
	}

	// ── Full-confidence path: dominance trend confirms BTC's own trend ──────
	dominanceRising := data.Trend == "RISING"
	dominanceFalling := data.Trend == "FALLING"

	if btcUptrend && dominanceRising {
		sl := price - maxF(1.0*atr1h, 0.003*price)
		risk := price - sl
		tp := price + 2.2*risk
		if risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"BTC.D %.2f%% RISING (delta1h=%.3f) + BTC 1h EMA9=%.0f>EMA21=%.0f uptrend -> capital rotating to BTC, LONG",
				data.BTC_D_Pct, data.Delta1h, ema9, ema21,
			),
		}
	}

	if btcDowntrend && dominanceFalling {
		sl := price + maxF(1.0*atr1h, 0.003*price)
		risk := sl - price
		tp := price - 2.2*risk
		if risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"BTC.D %.2f%% FALLING (delta1h=%.3f) + BTC 1h EMA9=%.0f<EMA21=%.0f downtrend -> capital rotating to alts, SHORT",
				data.BTC_D_Pct, data.Delta1h, ema9, ema21,
			),
		}
	}

	return NoSignal(name)
}
