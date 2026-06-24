package scalpers

import (
	"fmt"
	"math"
)

// S18 — BTC_Equities_Correlation_Break
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h (BTC momentum + correlation pairing)
// Logic:
//
//	BTC's correlation with traditional equities (Nasdaq proxy) regime-shifts
//	over time — well documented in 2026 market commentary (CryptoSlate:
//	"Bitcoin's surging correlation with Nasdaq signals convergence with
//	traditional finance"; CME Group OpenMarkets: "Why Bitcoin's Relationship
//	with Equities Has Changed"; arXiv:2501.09911 "Institutional Adoption and
//	Correlation Dynamics: Bitcoin's Evolving Role in Financial Markets").
//	When correlation is HIGH (>0.6) and the Nasdaq proxy makes a strong move,
//	BTC tends to follow with a short lag as leveraged crypto desks react to
//	the macro print. We confirm BTC hasn't already fully caught up (its own
//	short-term momentum doesn't yet match the magnitude of the Nasdaq move)
//	before trading WITH the Nasdaq direction.
//
//	When correlation suddenly BREAKS DOWN (current reading <0.2 after a
//	recent higher reading), that's a regime-shift signal — historically
//	"decoupling" events (e.g. Crypto.com "Bitcoin and Nasdaq-100 Break
//	Correlation") — so we reduce conviction on correlation-based positioning
//	rather than reversing (we simply skip/lower confidence; no contrarian bet).
type BTCEquitiesCorrelationBreak struct{}

func (s *BTCEquitiesCorrelationBreak) Name() string { return "BTC_Equities_Correlation_Break" }

func (s *BTCEquitiesCorrelationBreak) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	corrBreakHighThreshold = 0.6 // correlation regime considered "high" / actionable
	corrBreakLowThreshold  = 0.2 // correlation regime considered "broken down"
	corrBreakNasdaqMoveMin = 0.5 // % Nasdaq-proxy move over the window required to act
)

func (s *BTCEquitiesCorrelationBreak) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}

	// Graceful degradation: macro feed must be populated and healthy, and
	// must not be a permanently-zero/INFEASIBLE feed.
	if !ctx.MacroFeedPopulated || !ctx.MacroFeedHealthy {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}

	correlation := ctx.BTCEquitiesCorrelation30d
	nasdaqMove := ctx.NasdaqProxyChangePct
	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	// Sudden correlation breakdown: skip / reduce conviction, do not reverse.
	// (Detected via a low current reading — without per-strategy state we
	// cannot diff against "a prior higher reading" directly, so we treat any
	// reading below the breakdown threshold as a breakdown state and refuse
	// to trade the correlation-based setup at all, which is the conservative
	// interpretation of "reduce conviction... or skip".)
	if correlation < corrBreakLowThreshold {
		return NoSignal(name)
	}

	if correlation <= corrBreakHighThreshold {
		return NoSignal(name)
	}
	if math.Abs(nasdaqMove) < corrBreakNasdaqMoveMin {
		return NoSignal(name)
	}

	// BTC's own short-term momentum (3-bar close-to-close % move) — require it
	// has NOT already fully caught up to the Nasdaq move (lag confirmation).
	last3 := ctx.Candles1h[len(ctx.Candles1h)-3:]
	btcMove3barPct := (last3[len(last3)-1].Close - last3[0].Close) / last3[0].Close * 100.0

	laggedConfirmation := math.Abs(btcMove3barPct) < math.Abs(nasdaqMove)*0.8

	if !laggedConfirmation {
		return NoSignal(name)
	}

	// Minimum 3-bar confirmation: require the Nasdaq direction has been
	// consistent in sign for context (use change pct sign as proxy — no
	// dedicated Nasdaq candle history is exposed, so we rely on the feed's
	// own % change plus the BTC 3-bar non-contradiction check above as the
	// confirmation gate).
	if nasdaqMove > 0 {
		sl := price - math.Max(1.0*atr1h, 0.003*price)
		slDist := price - sl
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.62,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"correlation break LONG: corr=%.2f>0.6, Nasdaq proxy +%.2f%%, BTC 3bar=%.2f%% (lag confirmed)",
				correlation, nasdaqMove, btcMove3barPct,
			),
		}
	}

	sl := price + math.Max(1.0*atr1h, 0.003*price)
	slDist := sl - price
	if slDist <= 0 {
		return NoSignal(name)
	}
	tp := price - 2.0*slDist
	risk := sl - price
	reward := price - tp
	if risk <= 0 || reward/risk < 2.0 {
		return NoSignal(name)
	}
	return Signal{
		Strategy:   name,
		Direction:  DirectionShort,
		Confidence: 0.62,
		StopLoss:   sl,
		TakeProfit: tp,
		Reason: fmt.Sprintf(
			"correlation break SHORT: corr=%.2f>0.6, Nasdaq proxy %.2f%%, BTC 3bar=%.2f%% (lag confirmed)",
			correlation, nasdaqMove, btcMove3barPct,
		),
	}
}
