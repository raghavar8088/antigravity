package scalpers

import (
	"fmt"
	"math"
)

// S60 — Feature Confluence Score
//
// Research: Multiple academic random-forest feature-importance studies on
// crypto price prediction (2019-2024) consistently identify ~8 dominant
// technical features (momentum oscillators, MACD, volume ratio, volatility
// trend, moving-average distance, band position, order-flow delta, funding)
// as the top splits learned by tree ensembles.
// No AI/LLM/ML-library calls — explicit rule-based replication of the cited
// research's feature signal: this strategy hand-codes the 8 features an RF
// model would learn as splits and sums them into a deterministic vote score.
//
// Regime:     TRENDING, RANGING
// Timeframes: 15m
// Logic:      Compute 8 explicit bull(+1)/bear(-1)/neutral(0) votes from
//             RSI(14), MACD histogram sign, Volume vs AvgVolume ratio, ATR
//             trend, price vs EMA21, BB position, CVD vs CVDPrev, and
//             FundingRate sign. Sum >= +5 → LONG, <= -5 → SHORT.
// Edge:       Replicates what an RF/GBM ensemble would learn as its top
//             feature splits, without requiring an actual trained model —
//             confluence across independent feature families reduces
//             single-indicator false positives.

type FeatureConfluenceScore struct{}

func (s *FeatureConfluenceScore) Name() string { return "Feature_Confluence_Score" }

func (s *FeatureConfluenceScore) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *FeatureConfluenceScore) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 30 {
		return NoSignal(name)
	}

	c15 := ctx.Candles15m
	n := len(c15)

	atr := ATR(c15, 14)
	if atr == 0 {
		return NoSignal(name)
	}
	atrPrev := ATR(c15[:n-1], 14)

	price := ctx.Price

	rsi := RSI(c15, 14)
	macd := MACD(c15)
	vol := c15[n-1].Volume
	avgVol := AvgVolume(c15, 20)
	ema21 := EMA(c15, 21)
	bb := BB(c15, 20)

	score := 0

	// 1. RSI(14)
	if rsi > 55 {
		score++
	} else if rsi < 45 {
		score--
	}

	// 2. MACD histogram sign
	if macd.Histogram > 0 {
		score++
	} else if macd.Histogram < 0 {
		score--
	}

	// 3. Volume vs AvgVolume ratio (confirms whichever direction last bar moved)
	if avgVol > 0 && vol > 1.2*avgVol {
		if c15[n-1].Close > c15[n-1].Open {
			score++
		} else if c15[n-1].Close < c15[n-1].Open {
			score--
		}
	}

	// 4. ATR trend (expanding = confirm momentum direction)
	if atr > atrPrev {
		if c15[n-1].Close > c15[n-1].Open {
			score++
		} else if c15[n-1].Close < c15[n-1].Open {
			score--
		}
	}

	// 5. Price vs EMA21
	if ema21 > 0 {
		if price > ema21 {
			score++
		} else if price < ema21 {
			score--
		}
	}

	// 6. BB position
	if bb.Upper > 0 {
		if price >= bb.Upper {
			score++
		} else if price <= bb.Lower {
			score--
		}
	}

	// 7. CVD vs CVDPrev
	if ctx.CVD > ctx.CVDPrev {
		score++
	} else if ctx.CVD < ctx.CVDPrev {
		score--
	}

	// 8. FundingRate sign (positive funding = crowded longs = bearish tilt;
	// negative funding = crowded shorts = bullish tilt — contrarian vote)
	if ctx.FundingRate > 0 {
		score--
	} else if ctx.FundingRate < 0 {
		score++
	}

	if score >= 5 {
		sl := price - math.Max(1.0*atr, 0.003*price)
		slDist := price - sl
		tp := price + 2.0*slDist
		if slDist <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.68 + 0.02*float64(score-5),
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Feature confluence LONG: score=%d/8 (RSI=%.1f MACDh=%.4f vol=%.0f/%.0f ATR %.1f>%.1f price/EMA21=%.0f/%.0f CVD %.0f/%.0f funding=%.5f)",
				score, rsi, macd.Histogram, vol, avgVol, atr, atrPrev, price, ema21, ctx.CVD, ctx.CVDPrev, ctx.FundingRate,
			),
		}
	}

	if score <= -5 {
		sl := price + math.Max(1.0*atr, 0.003*price)
		slDist := sl - price
		tp := price - 2.0*slDist
		if slDist <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.68 + 0.02*float64(-score-5),
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Feature confluence SHORT: score=%d/8 (RSI=%.1f MACDh=%.4f vol=%.0f/%.0f ATR %.1f>%.1f price/EMA21=%.0f/%.0f CVD %.0f/%.0f funding=%.5f)",
				score, rsi, macd.Histogram, vol, avgVol, atr, atrPrev, price, ema21, ctx.CVD, ctx.CVDPrev, ctx.FundingRate,
			),
		}
	}

	return NoSignal(name)
}
