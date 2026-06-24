package scalpers

import (
	"fmt"
	"math"
)

// S20 — Risk_On_Off_Regime_Proxy
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h
// Logic:
//
//	A composite "risk-on/risk-off" macro regime proxy: DXY falling (dollar
//	weakness) + Nasdaq proxy rising (equities risk appetite) + funding rate
//	not extreme (no crowded one-sided positioning) = risk-on -> favor BTC
//	long bias. The mirror image (DXY rising + Nasdaq falling + funding not
//	extreme) = risk-off -> favor BTC short bias. This composite framing is
//	consistent with how risk-on/risk-off regimes are discussed in crypto
//	macro commentary (e.g. Nasdaq.com "Bitcoin's Potential Rally Amid U.S.
//	Dollar Weakness"; OSL DXY/BTC correlation analysis).
//
// Design note (per task spec): the registry/walkforward harness expects a
// Strategy that itself returns a Signal from Evaluate() — there is no
// "bolt-on modifier" hook in the Strategy interface (see types.go: Strategy
// has only Name/ValidRegimes/Evaluate, and RegistryEntry wraps a single
// Strategy instance). So this is implemented as a standalone signal
// generator: the macro composite determines directional bias AND confidence
// weighting, while a short-term price-action trigger (EMA9/EMA21 cross
// direction agreeing with the macro bias, plus RSI not already extreme in
// the bias direction) supplies the actual entry timing/trigger. This avoids
// firing purely off a slow-moving macro composite with no entry timing.
type RiskOnOffRegimeProxy struct{}

func (s *RiskOnOffRegimeProxy) Name() string { return "Risk_On_Off_Regime_Proxy" }

func (s *RiskOnOffRegimeProxy) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	riskRegimeDXYMoveMin     = 0.1    // % DXY move to count as "falling"/"rising"
	riskRegimeNasdaqMoveMin  = 0.2    // % Nasdaq proxy move to count as "rising"/"falling"
	riskRegimeFundingExtreme = 0.0005 // raw decimal — funding considered "extreme" beyond this (0.05%/8h)
)

func (s *RiskOnOffRegimeProxy) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if !ctx.MacroFeedPopulated || !ctx.MacroFeedHealthy {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}

	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	// FundingRate is raw decimal (e.g. 0.0001 = 0.01% per 8h) — no conversion.
	fundingNotExtreme := math.Abs(ctx.FundingRate) < riskRegimeFundingExtreme

	dxyFalling := ctx.DXYChangePct < -riskRegimeDXYMoveMin
	dxyRising := ctx.DXYChangePct > riskRegimeDXYMoveMin
	nasdaqRising := ctx.NasdaqProxyChangePct > riskRegimeNasdaqMoveMin
	nasdaqFalling := ctx.NasdaqProxyChangePct < -riskRegimeNasdaqMoveMin

	riskOn := dxyFalling && nasdaqRising && fundingNotExtreme
	riskOff := dxyRising && nasdaqFalling && fundingNotExtreme

	if !riskOn && !riskOff {
		return NoSignal(name)
	}

	ema9 := EMA(ctx.Candles1h, 9)
	ema21 := EMA(ctx.Candles1h, 21)
	rsi := RSI(ctx.Candles1h, 14)
	if ema9 == 0 || ema21 == 0 {
		return NoSignal(name)
	}

	if riskOn {
		// Entry trigger: BTC short-term trend agrees (EMA9>EMA21) and RSI not
		// already overbought (>75) — avoid chasing an exhausted move.
		if ema9 <= ema21 || rsi > 60 {
			return NoSignal(name)
		}
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
			Confidence: 0.64,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"risk-on composite LONG: DXY=%.2f%%, Nasdaq=%.2f%%, funding=%.5f (not extreme), EMA9>EMA21, RSI=%.1f",
				ctx.DXYChangePct, ctx.NasdaqProxyChangePct, ctx.FundingRate, rsi,
			),
		}
	}

	// riskOff
	if ema9 >= ema21 || rsi < 40 {
		return NoSignal(name)
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
		Confidence: 0.64,
		StopLoss:   sl,
		TakeProfit: tp,
		Reason: fmt.Sprintf(
			"risk-off composite SHORT: DXY=%.2f%%, Nasdaq=%.2f%%, funding=%.5f (not extreme), EMA9<EMA21, RSI=%.1f",
			ctx.DXYChangePct, ctx.NasdaqProxyChangePct, ctx.FundingRate, rsi,
		),
	}
}
