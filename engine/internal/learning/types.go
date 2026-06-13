// Package learning implements a post-trade self-learning loop that analyses
// closed trade outcomes and generates strategy-level lessons for the AI scorer.
package learning

import "time"

// TradeOutcome classifies the result of a closed trade.
type TradeOutcome string

const (
	OutcomeWin       TradeOutcome = "WIN"
	OutcomeLoss      TradeOutcome = "LOSS"
	OutcomeBreakeven TradeOutcome = "BREAKEVEN"
)

// TradeLesson encapsulates what the system learned from one closed trade.
type TradeLesson struct {
	TradeID         string
	Strategy        string
	Bias            string
	Outcome         TradeOutcome
	PnLPct          float64
	EntryRegime     string
	EntryConfidence float64
	EntryMCEV       float64  // Monte Carlo expected value at entry (if available)
	ExitReason      string
	WhatWorked      string   // populated by lesson generator
	WhatFailed      string   // populated by lesson generator
	LessonText      string   // human-readable summary for AI prompt
	GeneratedAt     time.Time
}

// LessonSummary aggregates lessons across all strategies for the AI prompt.
type LessonSummary struct {
	TotalLessons     int
	RecentWinRate    float64
	TopWinStrategy   string
	TopLossStrategy  string
	RegimeInsights   map[string]float64 // regime → win rate
	Lessons          []TradeLesson
	ComputedAt       time.Time
}
