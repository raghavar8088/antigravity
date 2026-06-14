package scalpers

import "fmt"

// S2 — Bollinger Band Mean Reversion
//
// Regime:     RANGING only
// Timeframes: 15m (regime/BB width filter) + 5m (entry trigger)
// Logic:      BB width percentile < 40th (market compressed). Price touches
//             lower/upper band. RSI oversold/overbought. Order book confirms
//             with bid/ask wall. CVD exhausted.

type BollingerMeanReversion struct{}

func (s *BollingerMeanReversion) Name() string { return "Bollinger_Mean_Reversion" }

func (s *BollingerMeanReversion) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *BollingerMeanReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 60 || len(ctx.Candles5m) < 25 {
		return NoSignal(name)
	}

	bbWidthPct := BBWidthPercentile(ctx.Candles15m, 20, 50)
	if bbWidthPct > 0.40 {
		return NoSignal(name)
	}

	bb5m := BB(ctx.Candles5m, 20)
	rsi5m := RSI(ctx.Candles5m, 14)
	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 || bb5m.Middle == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	ob := ctx.OrderBook

	atLowerBand := price <= bb5m.Lower+0.1*atr5m
	rsiOversold := rsi5m < 35
	bidWallPresent := ob.BidWallSize > ob.AskWallSize*1.5
	cvdExhaustedSell := ctx.CVD >= ctx.CVDPrev

	if atLowerBand && rsiOversold && bidWallPresent && cvdExhaustedSell {
		sl := bb5m.Lower - 0.3*atr5m
		tp1 := bb5m.Middle
		tp2 := bb5m.Upper
		if price-sl < 0.0005*price {
			return NoSignal(name)
		}
		risk := price - sl
		reward := tp1 - price
		if reward < 2*risk {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.75,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"RANGING: BB width pct=%.2f (compressed), price %.0f at lower BB=%.0f, "+
					"RSI=%.1f oversold, bid wall=%.2f>ask=%.2f, CVD stabilising",
				bbWidthPct, price, bb5m.Lower, rsi5m, ob.BidWallSize, ob.AskWallSize,
			),
		}
	}

	atUpperBand := price >= bb5m.Upper-0.1*atr5m
	rsiOverbought := rsi5m > 65
	askWallPresent := ob.AskWallSize > ob.BidWallSize*1.5
	cvdExhaustedBuy := ctx.CVD <= ctx.CVDPrev

	if atUpperBand && rsiOverbought && askWallPresent && cvdExhaustedBuy {
		sl := bb5m.Upper + 0.3*atr5m
		tp1 := bb5m.Middle
		tp2 := bb5m.Lower
		if sl-price < 0.0005*price {
			return NoSignal(name)
		}
		risk := sl - price
		reward := price - tp1
		if reward < 2*risk {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.75,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"RANGING: BB width pct=%.2f (compressed), price %.0f at upper BB=%.0f, "+
					"RSI=%.1f overbought, ask wall=%.2f>bid=%.2f, CVD weakening",
				bbWidthPct, price, bb5m.Upper, rsi5m, ob.AskWallSize, ob.BidWallSize,
			),
		}
	}

	return NoSignal(name)
}
