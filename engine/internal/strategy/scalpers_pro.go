package strategy

import (
	"math"

	"antigravity-engine/internal/marketdata"
)

// =============================================================================
// PRO STRATEGIES — High-confidence, multi-condition entries
// =============================================================================

// TrendMomentumScoreScalper builds a 5-component bullish/bearish score and
// only fires when 4 or 5 components agree. Uses ATR-scaled SL/TP so the risk
// adapts to current market volatility rather than using hard-coded percentages.
//
// Components:
//  1. EMA ribbon (fast > mid > slow → bullish)
//  2. MACD histogram direction
//  3. RSI zone (< 55 bullish, > 45 bearish overlap intentional)
//  4. ADX trend strength gate (> 20 required to act)
//  5. Price vs VWAP position
type TrendMomentumScoreScalper struct {
	baseScalper
	volumes      []float64
	cooldownBars int
	lastBar      int
}

func NewTrendMomentumScoreScalper() *TrendMomentumScoreScalper {
	return &TrendMomentumScoreScalper{
		baseScalper:  baseScalper{name: "TrendMomentum_Score_Scalp", maxBuf: defaultBufSize},
		cooldownBars: 5,
	}
}

func (s *TrendMomentumScoreScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *TrendMomentumScoreScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	vol := candle.Quantity
	if vol <= 0 {
		vol = 1
	}
	s.volumes = appendRollingFloat(s.volumes, vol, defaultBufSize)

	n := len(s.prices)
	if n < 60 || len(s.volumes) < 30 {
		return holdSignal()
	}
	if n-s.lastBar < s.cooldownBars {
		return holdSignal()
	}

	adx := ADX(s.prices, 14)
	if adx < 20 {
		return holdSignal() // Choppy — skip
	}

	fastEMA := EMA(s.prices, 8)
	midEMA := EMA(s.prices, 21)
	slowEMA := EMA(s.prices, 55)
	_, _, macdHist := MACD(s.prices, 12, 26, 9)
	rsi := RSI(s.prices, 14)
	vwap := RollingVWAP(s.prices, s.volumes, 40)
	if vwap == 0 {
		return holdSignal()
	}

	atr := ATR(s.prices, 14)
	if atr <= 0 || candle.Price <= 0 {
		return holdSignal()
	}

	// Score: +1 per bullish component, -1 per bearish
	score := 0.0

	// 1. EMA ribbon
	if fastEMA > midEMA && midEMA > slowEMA {
		score += 1
	} else if fastEMA < midEMA && midEMA < slowEMA {
		score -= 1
	}

	// 2. MACD
	if macdHist > 0 {
		score += 1
	} else if macdHist < 0 {
		score -= 1
	}

	// 3. RSI
	if rsi < 55 && rsi > 40 {
		score += 0.5 // Neutral bullish zone
	} else if rsi < 40 {
		score += 1
	} else if rsi > 60 && rsi < 75 {
		score -= 0.5
	} else if rsi >= 75 {
		score -= 1
	}

	// 4. Price vs VWAP
	if candle.Price > vwap {
		score += 1
	} else {
		score -= 1
	}

	// 5. ADX boost: strong trend amplifies the signal
	adxBonus := math.Min((adx-20)/40, 0.5) // 0 to 0.5
	if score > 0 {
		score += adxBonus
	} else if score < 0 {
		score -= adxBonus
	}

	// ATR-based SL/TP
	atrPct := (atr / candle.Price) * 100
	slPct := math.Max(atrPct*1.10, 0.20)
	if slPct > 0.80 {
		slPct = 0.80
	}
	tpPct := slPct * 2.2
	confidence := 0.92 + math.Min(math.Abs(score)*0.04, 0.28)

	if score >= 3.0 {
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionBuy, slPct, tpPct, confidence)
	}
	if score <= -3.0 {
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionSell, slPct, tpPct, confidence)
	}
	return holdSignal()
}

// VWAPBounceScalper enters on pullbacks to VWAP within an established trend.
// It requires: confirmed trend via EMA20 > EMA50 (or inverted), price temporarily
// below VWAP, RSI confirming mild oversold, volume at least average, and price
// already reversing back up through VWAP. Classic institutional-level entry.
type VWAPBounceScalper struct {
	baseScalper
	volumes      []float64
	cooldownBars int
	lastBar      int
}

func NewVWAPBounceScalper() *VWAPBounceScalper {
	return &VWAPBounceScalper{
		baseScalper:  baseScalper{name: "VWAP_Bounce_Pro_Scalp", maxBuf: defaultBufSize},
		cooldownBars: 6,
	}
}

func (s *VWAPBounceScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *VWAPBounceScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	vol := candle.Quantity
	if vol <= 0 {
		vol = 1
	}
	s.volumes = appendRollingFloat(s.volumes, vol, defaultBufSize)

	n := len(s.prices)
	if n < 55 || len(s.volumes) < 30 {
		return holdSignal()
	}
	if n-s.lastBar < s.cooldownBars {
		return holdSignal()
	}

	ema20 := EMA(s.prices, 20)
	ema50 := EMA(s.prices, 50)
	vwap := RollingVWAP(s.prices, s.volumes, 40)
	rsi := RSI(s.prices, 7) // Short RSI for fast oversold detection
	atr := ATR(s.prices, 14)
	avgVol := tailAverage(s.volumes, 20)

	if vwap == 0 || atr == 0 || avgVol == 0 {
		return holdSignal()
	}

	// Volume gate: at least 90% of average (don't require spike — it's a pullback setup)
	volRatio := candle.Quantity / avgVol
	if volRatio < 0.90 {
		return holdSignal()
	}

	prevPrice := s.prices[n-2]
	atrPct := (atr / candle.Price) * 100
	slPct := math.Max(atrPct*1.05, 0.18)
	if slPct > 0.70 {
		slPct = 0.70
	}
	tpPct := slPct * 2.0

	// LONG bounce: uptrend, price dipped below VWAP and now reclaims it
	if ema20 > ema50 {
		belowVWAP := prevPrice < vwap
		nowAbove := candle.Price >= vwap
		if belowVWAP && nowAbove && rsi < 48 && candle.Price > prevPrice {
			confidence := 0.94 + math.Min((48-rsi)/60+math.Max(volRatio-1, 0)*0.08, 0.26)
			s.lastBar = n
			return signalWithConfidence(candle.Symbol, ActionBuy, slPct, tpPct, confidence)
		}
	}

	// SHORT bounce: downtrend, price rallied above VWAP and now fails back
	if ema20 < ema50 {
		aboveVWAP := prevPrice > vwap
		nowBelow := candle.Price <= vwap
		if aboveVWAP && nowBelow && rsi > 52 && candle.Price < prevPrice {
			confidence := 0.94 + math.Min((rsi-52)/60+math.Max(volRatio-1, 0)*0.08, 0.26)
			s.lastBar = n
			return signalWithConfidence(candle.Symbol, ActionSell, slPct, tpPct, confidence)
		}
	}

	return holdSignal()
}
