package scalpers

import (
	"fmt"
	"math"
)

// S40 — Ornstein-Uhlenbeck Reversion
//
// Research:   Vasicek (1977); Avellaneda & Lee (2010) "Statistical Arbitrage
//             in the US Equities Market."
// Regime:     RANGING only
// Timeframes: 15m (OU parameter estimation + VWAP deviation)
// Logic:      Fit an OU process to price deviations from rolling VWAP on
//             Candles15m (period ~30). Only signal when Theta>0 (mean
//             reversion statistically detected) and the current deviation
//             from VWAP is large relative to the fitted Sigma (>=2 sigma).
//             Trade back toward VWAP.
// Edge:       Most mean-reversion strategies assume reversion; this one only
//             fires when the OU fit actually confirms theta > 0 for the
//             current window, avoiding fading genuine trends.

type OrnsteinUhlenbeckReversion struct{}

func (s *OrnsteinUhlenbeckReversion) Name() string { return "Ornstein_Uhlenbeck_Reversion" }

func (s *OrnsteinUhlenbeckReversion) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *OrnsteinUhlenbeckReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 32 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	ou := OUParameters(ctx.Candles15m, 30)
	if ou.Theta <= 0 || ou.Sigma == 0 {
		// No detectable mean reversion in this window.
		return NoSignal(name)
	}

	vwap := VWAP(ctx.Candles15m[len(ctx.Candles15m)-30:])
	if vwap == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	deviation := price - vwap
	zScore := deviation / ou.Sigma

	const zThreshold = 2.0

	if zScore <= -zThreshold {
		// Price is far below VWAP relative to OU sigma — expect reversion up.
		sl := price - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		tp := vwap
		risk := slDist
		reward := tp - price
		if reward < 2.0*risk {
			tp = price + 2.0*risk
			reward = tp - price
		}
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: OU theta=%.4f (mean-reverting), z-score=%.2f below -%.1f, "+
					"price=%.0f vs VWAP=%.0f, sigma=%.2f, half-life=%.1f bars",
				ou.Theta, zScore, zThreshold, price, vwap, ou.Sigma, ou.HalfLifeBars,
			),
		}
	}

	if zScore >= zThreshold {
		// Price is far above VWAP relative to OU sigma — expect reversion down.
		sl := price + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		tp := vwap
		risk := slDist
		reward := price - tp
		if reward < 2.0*risk {
			tp = price - 2.0*risk
			reward = price - tp
		}
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: OU theta=%.4f (mean-reverting), z-score=%.2f above %.1f, "+
					"price=%.0f vs VWAP=%.0f, sigma=%.2f, half-life=%.1f bars",
				ou.Theta, zScore, zThreshold, price, vwap, ou.Sigma, ou.HalfLifeBars,
			),
		}
	}

	return NoSignal(name)
}
