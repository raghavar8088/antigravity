package scalpers

import (
	"fmt"
	"math"
)

// S67 — Recurrence Quantification Signal
//
// Research: Webber, C.L. & Marwan, N. (eds.), "Recurrence Quantification
// Analysis: Theory and Best Practices" (2015) — RQA finds historical windows
// in a system's state space that closely "recur" to the current state, then
// studies what followed those historical recurrences.
// No AI/LLM/ML-library calls — explicit rule-based replication of the cited
// research's feature signal: hand-coded Euclidean distance over an explicit
// 2-feature vector and historical window comparison, no fitted/learned
// recurrence model.
//
// Regime:     TRENDING, RANGING
// Timeframes: 15m
// Logic:      Build a 2-feature vector (normalized price change, normalized
//             volume change) for the current 20-bar window. Compare via
//             Euclidean distance to feature vectors from the most recent
//             ~30 non-overlapping 20-bar historical windows available in the
//             candle buffer. If the closest matches were followed by a
//             consistent directional move, signal that direction.
// Edge:       If the market's current micro-state closely resembles past
//             states that were reliably followed by a directional move, that
//             historical regularity is a tradeable edge (state-space
//             analog-matching, the core RQA insight).
//
// LOOKBACK CONSTRAINT NOTE: True RQA typically scans hundreds of historical
// windows; this engine's candle buffers cap at 60x15m candles (Candles15m
// max 60), which permits at most ~2 non-overlapping 20-bar historical
// windows plus the current window. We use ALL available non-overlapping
// historical 20-bar windows within the buffer (typically 1-2) rather than
// the literature's ~30-window ideal — explicitly documented as a reduced-
// lookback approximation constrained by available candle history.

type RecurrenceQuantificationSignal struct{}

func (s *RecurrenceQuantificationSignal) Name() string {
	return "Recurrence_Quantification_Signal"
}

func (s *RecurrenceQuantificationSignal) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

// featureVector computes (normalized price change, normalized volume change)
// for a 20-bar window.
func featureVector(window []Candle) (float64, float64) {
	if len(window) < 2 {
		return 0, 0
	}
	first := window[0]
	last := window[len(window)-1]
	priceChange := 0.0
	if first.Close > 0 {
		priceChange = (last.Close - first.Close) / first.Close
	}
	var avgVol float64
	for _, c := range window {
		avgVol += c.Volume
	}
	avgVol /= float64(len(window))
	volChange := 0.0
	if avgVol > 0 {
		volChange = (last.Volume - first.Volume) / avgVol
	}
	return priceChange, volChange
}

// nextWindowDirection returns +1/-1/0 for the directional move (close[end] vs
// close[end+windowLen]) that followed a historical window, or 0 if unavailable.
func nextWindowDirection(candles []Candle, windowEnd, windowLen int) int {
	if windowEnd+windowLen > len(candles) {
		return 0
	}
	start := candles[windowEnd-1].Close
	end := candles[windowEnd+windowLen-1].Close
	if start <= 0 {
		return 0
	}
	change := (end - start) / start
	if change > 0.001 {
		return 1
	}
	if change < -0.001 {
		return -1
	}
	return 0
}

func (s *RecurrenceQuantificationSignal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}

	const windowLen = 20
	c15 := ctx.Candles15m
	n := len(c15)
	// Need current window + at least 1 historical non-overlapping window + room
	// for the directional follow-through measurement after that window.
	if n < windowLen*2+windowLen {
		return NoSignal(name)
	}

	atr := ATR(c15, 14)
	if atr == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	currentWindow := c15[n-windowLen:]
	curP, curV := featureVector(currentWindow)

	// Build all available non-overlapping historical 20-bar windows that
	// END before the current window AND have at least windowLen bars of
	// follow-through data available after them, up to ~30 (constrained by
	// buffer size — see header LOOKBACK CONSTRAINT NOTE).
	type match struct {
		dist float64
		dir  int
	}
	var matches []match

	maxHistoricalEnd := n - windowLen // current window starts here
	for end := maxHistoricalEnd - windowLen; end >= windowLen; end -= windowLen {
		histStart := end - windowLen
		if histStart < 0 {
			break
		}
		histWindow := c15[histStart:end]
		hp, hv := featureVector(histWindow)
		dist := math.Hypot(curP-hp, curV-hv)
		dir := nextWindowDirection(c15, end, windowLen)
		if dir != 0 {
			matches = append(matches, match{dist: dist, dir: dir})
		}
		if len(matches) >= 30 {
			break
		}
	}

	if len(matches) == 0 {
		return NoSignal(name)
	}

	// Take the closest few matches (up to 3, or fewer if not available) and
	// check for directional consistency.
	closestN := 3
	if len(matches) < closestN {
		closestN = len(matches)
	}
	// Simple selection sort for the closest N (small N, small slice — fine).
	for i := 0; i < closestN; i++ {
		minIdx := i
		for j := i + 1; j < len(matches); j++ {
			if matches[j].dist < matches[minIdx].dist {
				minIdx = j
			}
		}
		matches[i], matches[minIdx] = matches[minIdx], matches[i]
	}

	upVotes, downVotes := 0, 0
	for i := 0; i < closestN; i++ {
		if matches[i].dir > 0 {
			upVotes++
		} else if matches[i].dir < 0 {
			downVotes++
		}
	}

	// Require unanimous consistency among the closest matches found.
	if upVotes == closestN && closestN > 0 {
		sl := price - math.Max(1.0*atr, 0.003*price)
		slDist := price - sl
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.64,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RQA-proxy LONG: current 20-bar state vector (dP=%.4f dV=%.2f) matched %d/%d closest historical windows, all followed by upward moves (reduced lookback: %d windows scanned)",
				curP, curV, upVotes, closestN, len(matches),
			),
		}
	}

	if downVotes == closestN && closestN > 0 {
		sl := price + math.Max(1.0*atr, 0.003*price)
		slDist := sl - price
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.64,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RQA-proxy SHORT: current 20-bar state vector (dP=%.4f dV=%.2f) matched %d/%d closest historical windows, all followed by downward moves (reduced lookback: %d windows scanned)",
				curP, curV, downVotes, closestN, len(matches),
			),
		}
	}

	return NoSignal(name)
}
