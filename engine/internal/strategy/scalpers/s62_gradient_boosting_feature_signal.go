package scalpers

import (
	"fmt"
	"math"
)

// S62 — Gradient Boosting Feature Signal
//
// Research: Krauss, C., Do, X.A. & Huck, N. (2017) "Deep Neural Networks,
// Gradient-Boosted Trees, Random Forests: Statistical Arbitrage on the S&P
// 500", European Journal of Operational Research — multi-horizon
// rate-of-change/momentum features were among the strongest GBT splits.
// No AI/LLM/ML-library calls — explicit rule-based replication of the cited
// research's feature signal: this strategy hand-codes the multi-horizon
// rate-of-change + momentum features a GBT model would split on and requires
// explicit agreement across horizons rather than running an actual booster.
//
// Regime:     TRENDING
// Timeframes: 5m (short), 15m (medium), 1h (long)
// Logic:      Compute rate-of-change sign + momentum sign at each of 3
//             horizons. Strong agreement (all 3 horizons agree in both ROC
//             and momentum direction) → signal. Partial agreement → skip
//             (weak signal, avoid trading on it).
// Edge:       Krauss et al. found multi-horizon momentum/ROC features more
//             predictive in combination than any single horizon alone —
//             this requires unanimous cross-horizon agreement as a simple
//             proxy for that combined-feature strength.

type GradientBoostingFeatureSignal struct{}

func (s *GradientBoostingFeatureSignal) Name() string { return "Gradient_Boosting_Feature_Signal" }

func (s *GradientBoostingFeatureSignal) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

// rocAndMomentumSign returns (rocSign, momentumSign) where roc is the percent
// change over the last `period` bars and momentum is the sign of the most
// recent bar's close-vs-open compared to the prior bar's close-vs-open
// (acceleration proxy). +1 bull, -1 bear, 0 neutral.
func rocAndMomentumSign(candles []Candle, period int) (int, int) {
	n := len(candles)
	if n < period+2 {
		return 0, 0
	}
	cur := candles[n-1].Close
	past := candles[n-1-period].Close
	rocSign := 0
	if past > 0 {
		roc := (cur - past) / past
		if roc > 0.0005 {
			rocSign = 1
		} else if roc < -0.0005 {
			rocSign = -1
		}
	}

	lastBody := candles[n-1].Close - candles[n-1].Open
	prevBody := candles[n-2].Close - candles[n-2].Open
	momSign := 0
	if lastBody > 0 && lastBody > prevBody {
		momSign = 1
	} else if lastBody < 0 && lastBody < prevBody {
		momSign = -1
	}
	return rocSign, momSign
}

func (s *GradientBoostingFeatureSignal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 10 || len(ctx.Candles15m) < 10 || len(ctx.Candles1h) < 10 {
		return NoSignal(name)
	}

	shortROC, shortMom := rocAndMomentumSign(ctx.Candles5m, 5)
	medROC, medMom := rocAndMomentumSign(ctx.Candles15m, 5)
	longROC, longMom := rocAndMomentumSign(ctx.Candles1h, 5)

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	allBullish := shortROC == 1 && medROC == 1 && longROC == 1 &&
		shortMom == 1 && medMom == 1 && longMom == 1
	allBearish := shortROC == -1 && medROC == -1 && longROC == -1 &&
		shortMom == -1 && medMom == -1 && longMom == -1

	if allBullish {
		sl := price - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.75,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"GBT-proxy multi-horizon LONG: short/medium/long ROC+momentum all bullish (5m,15m,1h unanimous)",
			),
		}
	}

	if allBearish {
		sl := price + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.75,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"GBT-proxy multi-horizon SHORT: short/medium/long ROC+momentum all bearish (5m,15m,1h unanimous)",
			),
		}
	}

	// Partial agreement = weak signal, skip per spec.
	return NoSignal(name)
}
