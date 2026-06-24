package scalpers

import (
	"fmt"
	"math"
)

// S61 — Regime Probability Composite
//
// Research: Hamilton, J.D. (1989) "A New Approach to the Economic Analysis
// of Nonstationary Time Series and the Business Cycle" — the foundational
// Markov-switching / HMM regime-detection framework.
// No AI/LLM/ML-library calls — explicit rule-based replication of the cited
// research's feature signal: instead of fitting an actual HMM, this strategy
// hand-codes 5 deterministic regime-classifying votes (the same features an
// HMM's emission distributions would key off of) and uses simple majority
// vote as a regime-probability proxy.
//
// Regime:     TRENDING, RANGING, VOLATILE
// Timeframes: 15m, 1h
// Logic:      5 votes (ATR percentile, ADX, volume momentum, CVD trend,
//             spread/OB proxy) each cast TRENDING/RANGING/VOLATILE. Only
//             signal when the majority-vote regime agrees with ctx.Regime
//             (higher conviction than baseline regime classification alone),
//             then apply trend-following logic in TRENDING majority or
//             mean-reversion logic in RANGING majority.
// Edge:       Cross-checking the engine's own regime classifier against an
//             independent 5-feature vote reduces false-regime trades — only
//             trades when two independent regime estimates agree.

type RegimeProbabilityComposite struct{}

func (s *RegimeProbabilityComposite) Name() string { return "Regime_Probability_Composite" }

func (s *RegimeProbabilityComposite) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *RegimeProbabilityComposite) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeRanging && ctx.Regime != RegimeVolatile {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 40 || len(ctx.Candles1h) < 20 {
		return NoSignal(name)
	}

	c15 := ctx.Candles15m

	atr := ATR(c15, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	votes := map[Regime]int{RegimeTrending: 0, RegimeRanging: 0, RegimeVolatile: 0}

	// 1. ATR/volatility percentile via BB width percentile
	bbPct := BBWidthPercentile(c15, 20, 20)
	if bbPct > 0.75 {
		votes[RegimeVolatile]++
	} else if bbPct < 0.35 {
		votes[RegimeRanging]++
	} else {
		votes[RegimeTrending]++
	}

	// 2. ADX trend strength
	adx := ADX(ctx.Candles1h, 14)
	if adx > 25 {
		votes[RegimeTrending]++
	} else if adx < 15 {
		votes[RegimeRanging]++
	} else {
		votes[RegimeVolatile]++
	}

	// 3. Volume momentum vs AvgVolume
	avgVol := AvgVolume(c15, 20)
	lastVol := c15[len(c15)-1].Volume
	if avgVol > 0 {
		ratio := lastVol / avgVol
		if ratio > 1.5 {
			votes[RegimeVolatile]++
		} else if ratio < 0.8 {
			votes[RegimeRanging]++
		} else {
			votes[RegimeTrending]++
		}
	}

	// 4. CVD trend direction (consistent direction across history = trending)
	cvdTrendVote := RegimeRanging
	if len(ctx.CVDHistory) >= 3 {
		h := ctx.CVDHistory
		nh := len(h)
		if h[nh-1] > h[nh-2] && h[nh-2] > h[nh-3] {
			cvdTrendVote = RegimeTrending
		} else if h[nh-1] < h[nh-2] && h[nh-2] < h[nh-3] {
			cvdTrendVote = RegimeTrending
		}
	}
	votes[cvdTrendVote]++

	// 5. Spread/OB proxy — wide effective spread or imbalance extremes imply
	// volatile/illiquid conditions; tight & balanced implies ranging/trending calm.
	ob := ctx.OrderBook
	if ob.IsPopulated() {
		if math.Abs(ob.Imbalance) > 0.6 {
			votes[RegimeVolatile]++
		} else if math.Abs(ob.Imbalance) < 0.2 {
			votes[RegimeRanging]++
		} else {
			votes[RegimeTrending]++
		}
	} else {
		// degrade gracefully: use realized vol vs DVOL proxy instead
		rv := RealizedVol(ctx.Candles1h, 24)
		if rv > 60 {
			votes[RegimeVolatile]++
		} else {
			votes[RegimeTrending]++
		}
	}

	// Determine majority regime
	majority := RegimeRanging
	best := -1
	for r, v := range votes {
		if v > best {
			best = v
			majority = r
		}
	}

	if majority != ctx.Regime {
		return NoSignal(name)
	}

	price := ctx.Price
	ema9 := EMA(c15, 9)
	ema21 := EMA(c15, 21)

	switch majority {
	case RegimeTrending:
		// Trend-following: EMA9 vs EMA21 alignment
		if ema9 > ema21 && price > ema9 {
			sl := price - math.Max(1.0*atr, 0.003*price)
			slDist := price - sl
			tp := price + 2.0*slDist
			return Signal{
				Strategy:   name,
				Direction:  DirectionLong,
				Confidence: 0.70,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"Regime composite TRENDING majority (votes T=%d R=%d V=%d) agrees with ctx.Regime — EMA9>EMA21 trend LONG",
					votes[RegimeTrending], votes[RegimeRanging], votes[RegimeVolatile],
				),
			}
		}
		if ema9 < ema21 && price < ema9 {
			sl := price + math.Max(1.0*atr, 0.003*price)
			slDist := sl - price
			tp := price - 2.0*slDist
			return Signal{
				Strategy:   name,
				Direction:  DirectionShort,
				Confidence: 0.70,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"Regime composite TRENDING majority (votes T=%d R=%d V=%d) agrees with ctx.Regime — EMA9<EMA21 trend SHORT",
					votes[RegimeTrending], votes[RegimeRanging], votes[RegimeVolatile],
				),
			}
		}
	case RegimeRanging:
		// Mean-reversion: BB extremes
		bb := BB(c15, 20)
		if bb.Upper == 0 {
			return NoSignal(name)
		}
		if price >= bb.Upper {
			sl := price + math.Max(1.0*atr, 0.003*price)
			slDist := sl - price
			tp := price - 2.0*slDist
			if tp <= bb.Middle-3*slDist { // sanity floor, unlikely to trigger
				tp = bb.Middle
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionShort,
				Confidence: 0.69,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"Regime composite RANGING majority (votes T=%d R=%d V=%d) agrees with ctx.Regime — price at BB upper, fade SHORT",
					votes[RegimeTrending], votes[RegimeRanging], votes[RegimeVolatile],
				),
			}
		}
		if price <= bb.Lower {
			sl := price - math.Max(1.0*atr, 0.003*price)
			slDist := price - sl
			tp := price + 2.0*slDist
			return Signal{
				Strategy:   name,
				Direction:  DirectionLong,
				Confidence: 0.69,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"Regime composite RANGING majority (votes T=%d R=%d V=%d) agrees with ctx.Regime — price at BB lower, fade LONG",
					votes[RegimeTrending], votes[RegimeRanging], votes[RegimeVolatile],
				),
			}
		}
	}

	return NoSignal(name)
}
