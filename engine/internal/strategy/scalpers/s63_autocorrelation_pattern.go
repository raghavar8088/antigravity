package scalpers

import (
	"fmt"
	"math"
)

// S63 — Autocorrelation Pattern
//
// Research: Biais, B., Foucault, T. & Moinas, S. (2016) "Equilibrium Fast
// Trading", Journal of Financial Economics — documents short-horizon return
// autocorrelation structure exploitable by fast/algorithmic traders.
// No AI/LLM/ML-library calls — explicit rule-based replication of the cited
// research's feature signal: uses the deterministic Autocorrelation()
// indicator (Pearson autocorrelation of returns) computed directly, no
// learned model.
//
// Regime:     TRENDING, RANGING
// Timeframes: 1m (short-term momentum path), 5m (medium-term mean-reversion path)
// Logic:      Lag-1 autocorrelation > 0.3 on Candles1m → momentum continuation
//             signal (positive serial correlation = trend persistence).
//             Lag-5 autocorrelation < -0.3 on Candles5m → mean-reversion
//             signal (negative serial correlation = reversal tendency). Two
//             distinct signal paths in one Evaluate(); if both fire, momentum
//             takes priority.
// Edge:       Positive short-lag autocorrelation indicates order-flow-driven
//             persistence (worth riding); strong negative lag-5 autocorrelation
//             indicates overreaction/reversion (worth fading) — both are
//             well-documented microstructure effects in fast markets.

type AutocorrelationPattern struct{}

func (s *AutocorrelationPattern) Name() string { return "Autocorrelation_Pattern" }

func (s *AutocorrelationPattern) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *AutocorrelationPattern) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}

	price := ctx.Price

	// ── Momentum path: lag-1 autocorrelation on 1m, priority if it fires ──────
	if len(ctx.Candles1m) >= 35 {
		acf1 := Autocorrelation(ctx.Candles1m, 1, 30)
		atr1m := ATR(ctx.Candles1m, 14)
		if atr1m > 0 && math.Abs(acf1) > 0.3 {
			// 3-bar confirmation: last 3 1m bars must move in the same direction
			c1 := ctx.Candles1m
			n := len(c1)
			last3Up := c1[n-1].Close > c1[n-1].Open && c1[n-2].Close > c1[n-2].Open && c1[n-3].Close > c1[n-3].Open
			last3Down := c1[n-1].Close < c1[n-1].Open && c1[n-2].Close < c1[n-2].Open && c1[n-3].Close < c1[n-3].Open

			if acf1 > 0.3 && last3Up {
				sl := price - math.Max(1.0*atr1m, 0.003*price)
				slDist := price - sl
				tp := price + 2.0*slDist
				return Signal{
					Strategy:   name,
					Direction:  DirectionLong,
					Confidence: 0.66,
					StopLoss:   sl,
					TakeProfit: tp,
					Reason: fmt.Sprintf(
						"Autocorrelation momentum LONG: 1m lag-1 ACF=%.2f >0.3, 3-bar confirmed uptrend",
						acf1,
					),
				}
			}
			if acf1 > 0.3 && last3Down {
				sl := price + math.Max(1.0*atr1m, 0.003*price)
				slDist := sl - price
				tp := price - 2.0*slDist
				return Signal{
					Strategy:   name,
					Direction:  DirectionShort,
					Confidence: 0.66,
					StopLoss:   sl,
					TakeProfit: tp,
					Reason: fmt.Sprintf(
						"Autocorrelation momentum SHORT: 1m lag-1 ACF=%.2f >0.3, 3-bar confirmed downtrend",
						acf1,
					),
				}
			}
		}
	}

	// ── Mean-reversion path: lag-5 autocorrelation on 5m ──────────────────────
	if len(ctx.Candles5m) >= 40 {
		acf5 := Autocorrelation(ctx.Candles5m, 5, 30)
		atr5m := ATR(ctx.Candles5m, 14)
		if atr5m > 0 && acf5 < -0.3 {
			c5 := ctx.Candles5m
			n := len(c5)
			// 3-bar confirmation: require last 3 bars consistently extending the
			// move being faded (so we're catching an overextended, exhausting move)
			last3Up := c5[n-1].Close > c5[n-1].Open && c5[n-2].Close > c5[n-2].Open && c5[n-3].Close > c5[n-3].Open
			last3Down := c5[n-1].Close < c5[n-1].Open && c5[n-2].Close < c5[n-2].Open && c5[n-3].Close < c5[n-3].Open

			if last3Up {
				// Fade the up move — expect mean reversion down
				sl := price + math.Max(1.0*atr5m, 0.003*price)
				slDist := sl - price
				tp := price - 2.0*slDist
				return Signal{
					Strategy:   name,
					Direction:  DirectionShort,
					Confidence: 0.65,
					StopLoss:   sl,
					TakeProfit: tp,
					Reason: fmt.Sprintf(
						"Autocorrelation mean-reversion SHORT: 5m lag-5 ACF=%.2f <-0.3, 3-bar confirmed overextended uptrend",
						acf5,
					),
				}
			}
			if last3Down {
				sl := price - math.Max(1.0*atr5m, 0.003*price)
				slDist := price - sl
				tp := price + 2.0*slDist
				return Signal{
					Strategy:   name,
					Direction:  DirectionLong,
					Confidence: 0.65,
					StopLoss:   sl,
					TakeProfit: tp,
					Reason: fmt.Sprintf(
						"Autocorrelation mean-reversion LONG: 5m lag-5 ACF=%.2f <-0.3, 3-bar confirmed overextended downtrend",
						acf5,
					),
				}
			}
		}
	}

	return NoSignal(name)
}
