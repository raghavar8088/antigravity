package execution

import (
	"math"
	"time"
)

type Session string

const (
	SessionAsia    Session = "ASIA"
	SessionLondon  Session = "LONDON"
	SessionNewYork Session = "NEW_YORK"
)

type SpreadSnapshot struct {
	Bid           float64
	Ask           float64
	Mid           float64
	SpreadBps     float64
	SpreadPercent float64
	Timestamp     time.Time
}

type SpreadModel struct {
	BaseBps float64
}

func NewSpreadModel(baseBps float64) SpreadModel {
	if baseBps <= 0 {
		baseBps = 1.5
	}
	return SpreadModel{BaseBps: baseBps}
}

func (m SpreadModel) Snapshot(mid, volatilityPct, liquidityScore float64, ts time.Time) SpreadSnapshot {
	if mid <= 0 {
		return SpreadSnapshot{Timestamp: ts}
	}
	if liquidityScore <= 0 {
		liquidityScore = 0.5
	}
	sessionMul := sessionSpreadMultiplier(ts)
	volBps := math.Max(0, volatilityPct) * 1.8
	liqBps := (1 - clamp(liquidityScore, 0.05, 1)) * 8
	spreadBps := (m.BaseBps + volBps + liqBps) * sessionMul
	half := mid * (spreadBps / 10_000) / 2
	return SpreadSnapshot{
		Bid:           mid - half,
		Ask:           mid + half,
		Mid:           mid,
		SpreadBps:     spreadBps,
		SpreadPercent: spreadBps / 100,
		Timestamp:     ts,
	}
}

func sessionSpreadMultiplier(ts time.Time) float64 {
	switch CurrentSession(ts) {
	case SessionLondon, SessionNewYork:
		return 0.85
	default:
		return 1.15
	}
}

func CurrentSession(ts time.Time) Session {
	h := ts.UTC().Hour()
	switch {
	case h < 8:
		return SessionAsia
	case h < 13:
		return SessionLondon
	default:
		return SessionNewYork
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
