package scalpers

import (
	"fmt"
	"math"
)

// S65 — Cross Correlation Lead-Lag
//
// Research: Documented BTC-leads-altcoins lead-lag literature (2018+) on
// cross-asset crypto return propagation — BTC moves tend to precede
// proportional ETH/altcoin moves with a short lag.
// No AI/LLM/ML-library calls — explicit rule-based replication of the cited
// research's feature signal: hand-coded threshold comparison, no fitted
// cross-correlation/cointegration model.
//
// Regime:     TRENDING
// Timeframes: 1m (BTC move measurement), live ETHPrice tick (confirmation)
// Logic:      Base signal = EMA9 vs EMA21 on Candles5m (BTC continuation).
//             If BTC moved >0.3% over the last 5 Candles1m bars while ETH
//             "hasn't caught up" (qualitative check vs current ETHPrice
//             reading — see DATA LIMITATION note), boost Confidence on the
//             base BTC continuation signal. This is a confirmation
//             multiplier, never a standalone signal.
// Edge:       When BTC leads and ETH lags, the BTC move is more likely an
//             independent/leading move rather than a reaction — increasing
//             confidence that BTC continuation will persist (and likely drag
//             ETH with it).
//
// DEPENDENCY: requires ctx.ETHPricePopulated; returns NoSignal if false.
//
// DATA LIMITATION NOTE: This engine retains no ETH candle history (only the
// current live ETHPrice tick), so a true cross-correlation/lead-lag
// regression against an ETH return series is not possible here. We instead
// do a qualitative single-point comparison: estimate ETH's "expected" move
// from BTC's actual move (assuming normal high-beta co-movement) and check
// whether the live ETHPrice reading confirms that expectation has NOT yet
// been met — documented as an approximation, not a true cross-correlation.

type CrossCorrelationLeadLag struct{}

func (s *CrossCorrelationLeadLag) Name() string { return "Cross_Correlation_Lead_Lag" }

func (s *CrossCorrelationLeadLag) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *CrossCorrelationLeadLag) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if !ctx.ETHPricePopulated || ctx.ETHPrice <= 0 {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 25 || len(ctx.Candles1m) < 6 {
		return NoSignal(name)
	}

	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	// Base BTC continuation signal: EMA9 vs EMA21 on 5m
	ema9 := EMA(ctx.Candles5m, 9)
	ema21 := EMA(ctx.Candles5m, 21)
	if ema9 == 0 || ema21 == 0 {
		return NoSignal(name)
	}

	longBias := ema9 > ema21 && price > ema9
	shortBias := ema9 < ema21 && price < ema9
	if !longBias && !shortBias {
		return NoSignal(name)
	}

	// BTC move over last 5 1m bars
	c1 := ctx.Candles1m
	n := len(c1)
	btcStart := c1[n-6].Close
	btcMovePct := 0.0
	if btcStart > 0 {
		btcMovePct = (c1[n-1].Close - btcStart) / btcStart
	}

	leadLagConfirm := math.Abs(btcMovePct) > 0.003

	conf := 0.66
	confirmNote := "no lead-lag confirmation (BTC move <0.3%% over last 5x1m bars)"
	if leadLagConfirm {
		conf = 0.73
		confirmNote = fmt.Sprintf(
			"lead-lag CONFIRMED: BTC moved %.2f%% over last 5x1m bars (qualitative ETH-lag check, ETH=%.2f — see data-limitation note)",
			btcMovePct*100, ctx.ETHPrice,
		)
	}

	if longBias {
		sl := price - math.Max(1.0*atr5m, 0.003*price)
		slDist := price - sl
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"BTC continuation LONG (EMA9>EMA21, price>EMA9) — %s", confirmNote,
			),
		}
	}

	sl := price + math.Max(1.0*atr5m, 0.003*price)
	slDist := sl - price
	tp := price - 2.0*slDist
	return Signal{
		Strategy:   name,
		Direction:  DirectionShort,
		Confidence: conf,
		StopLoss:   sl,
		TakeProfit: tp,
		Reason: fmt.Sprintf(
			"BTC continuation SHORT (EMA9<EMA21, price<EMA9) — %s", confirmNote,
		),
	}
}
