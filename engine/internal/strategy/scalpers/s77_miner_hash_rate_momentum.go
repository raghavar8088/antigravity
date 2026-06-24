package scalpers

import (
	"fmt"
	"math"
)

// S77 — Miner_Hash_Rate_Momentum
//
// Research:   Hashrate Index research on miner hashprice/revenue as a macro
//
//	indicator — sustained hash rate growth signals miner
//	confidence/capex commitment (bullish macro backdrop), while
//	sharp hash rate declines can signal miner capitulation
//	(forced selling pressure, bearish backdrop).
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h (macro bias only — not a scalping trigger by itself)
// Logic:      Uses ctx.HashRateTrend30dPct (check HashRatePopulated first).
//
//	Strongly positive 30d hash rate trend → LONG bias; strongly
//	negative → SHORT bias. Confirmed by EMA9 vs EMA21 on Candles1h
//	not contradicting the macro bias.
//
// Edge:       Hash rate trend is a slow-moving macro confirmation signal,
//
//	not a timing trigger — used here purely as a bias filter
//	requiring price-action confirmation before entry, consistent
//	with how the existing macro family (S18-S21) treats other
//	macro proxies (DXY, Nasdaq correlation).
//
// Data dependency: PENDING — ctx.HashRateTrend30dPct is defined in
//
//	MarketContext but the live fetcher is NOT yet wired
//	(ctx.HashRatePopulated reads false in production today). This
//	strategy checks HashRatePopulated first and returns NoSignal
//	cleanly when false. Intended source for whoever wires this
//	next: a free hash-rate API such as mempool.space
//	(GET https://mempool.space/api/v1/mining/hashrate/3y) or
//	blockchain.com's charts API
//	(GET https://api.blockchain.info/charts/hash-rate?format=json),
//	polled at most once/day (hash rate is a slow-moving series).
type MinerHashRateMomentum struct{}

func (s *MinerHashRateMomentum) Name() string { return "Miner_Hash_Rate_Momentum" }

func (s *MinerHashRateMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	mhrmStrongPositiveTrend = 0.05  // +5% 30d hash rate growth = strong bullish macro bias
	mhrmStrongNegativeTrend = -0.05 // -5% = strong bearish macro bias (capitulation risk)
)

func (s *MinerHashRateMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if !ctx.HashRatePopulated {
		// Feed not wired yet — see header "Data dependency: PENDING".
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}

	trend := ctx.HashRateTrend30dPct
	ema9 := EMA(ctx.Candles1h, 9)
	ema21 := EMA(ctx.Candles1h, 21)
	if ema9 == 0 || ema21 == 0 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	slDist := math.Max(1.0*atr1h, 0.003*price)

	if trend >= mhrmStrongPositiveTrend && ema9 > ema21 {
		sl := price - slDist
		tp1 := price + 2.0*slDist
		tp2 := price + 3.0*slDist
		if sl >= price || tp1 <= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.60,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"hash rate momentum LONG: 30d trend=%.2f%% (>=%.2f%%), EMA9>EMA21 confirms",
				trend*100, mhrmStrongPositiveTrend*100,
			),
		}
	}

	if trend <= mhrmStrongNegativeTrend && ema9 < ema21 {
		sl := price + slDist
		tp1 := price - 2.0*slDist
		tp2 := price - 3.0*slDist
		if sl <= price || tp1 >= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.60,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"hash rate momentum SHORT: 30d trend=%.2f%% (<=%.2f%%), EMA9<EMA21 confirms",
				trend*100, mhrmStrongNegativeTrend*100,
			),
		}
	}

	return NoSignal(name)
}
