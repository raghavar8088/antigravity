package learning

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	lessonWindow      = 50  // number of recent trades to analyse
	minLessonsForSummary = 5
)

// LessonGenerator reads closed trades from MongoDB and produces TradeLesson records.
type LessonGenerator struct {
	db      *mongo.Database
	latest  *LessonSummary
	mu      sync.RWMutex
}

// NewLessonGenerator creates a generator backed by the given database.
func NewLessonGenerator(db *mongo.Database) *LessonGenerator {
	return &LessonGenerator{db: db}
}

// GenerateLessons queries the last lessonWindow closed trades and builds lessons.
func (g *LessonGenerator) GenerateLessons(ctx context.Context) (*LessonSummary, error) {
	coll := g.db.Collection("paper_trades")
	opts := options.Find().
		SetSort(bson.M{"exit_time": -1}).
		SetLimit(int64(lessonWindow)).
		SetProjection(bson.M{
			"strategy_name":  1,
			"direction":      1,
			"pnl_pct":        1,
			"pnl_usd":        1,
			"entry_regime":   1,
			"ai_confidence":  1,
			"exit_reason":    1,
			"mc_ev":          1,
		})

	cursor, err := coll.Find(ctx, bson.M{"status": "CLOSED"}, opts)
	if err != nil {
		return nil, fmt.Errorf("learning: find: %w", err)
	}
	defer cursor.Close(ctx)

	type row struct {
		Strategy     string  `bson:"strategy_name"`
		Bias         string  `bson:"direction"`
		PnLPct       float64 `bson:"pnl_pct"`
		PnLUSD       float64 `bson:"pnl_usd"`
		EntryRegime  string  `bson:"entry_regime"`
		AIConfidence float64 `bson:"ai_confidence"`
		ExitReason   string  `bson:"exit_reason"`
		MCEV         float64 `bson:"mc_ev"`
	}

	var rows []row
	for cursor.Next(ctx) {
		var r row
		if err := cursor.Decode(&r); err != nil {
			continue
		}
		rows = append(rows, r)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("learning: cursor: %w", err)
	}

	if len(rows) < minLessonsForSummary {
		return nil, fmt.Errorf("learning: only %d trades, need %d", len(rows), minLessonsForSummary)
	}

	summary := &LessonSummary{
		RegimeInsights: make(map[string]float64),
		ComputedAt:     time.Now().UTC(),
	}

	stratWins := make(map[string]int)
	stratTotal := make(map[string]int)
	regimeWins := make(map[string]int)
	regimeTotal := make(map[string]int)
	wins := 0

	for _, r := range rows {
		outcome := OutcomeLoss
		if r.PnLUSD > 0 {
			outcome = OutcomeWin
			wins++
			stratWins[r.Strategy]++
			regimeWins[r.EntryRegime]++
		} else if r.PnLUSD == 0 {
			outcome = OutcomeBreakeven
		}
		stratTotal[r.Strategy]++
		regimeTotal[r.EntryRegime]++

		lesson := TradeLesson{
			Strategy:        r.Strategy,
			Bias:            r.Bias,
			Outcome:         outcome,
			PnLPct:          r.PnLPct,
			EntryRegime:     r.EntryRegime,
			EntryConfidence: r.AIConfidence,
			EntryMCEV:       r.MCEV,
			ExitReason:      r.ExitReason,
			GeneratedAt:     time.Now().UTC(),
		}
		lesson.WhatWorked, lesson.WhatFailed = generateLessonText(r.Strategy, outcome, r.EntryRegime, r.ExitReason)
		lesson.LessonText = fmt.Sprintf("%s %s %s %.2f%% in %s (exit: %s) — %s",
			lesson.Bias, lesson.Strategy, string(outcome), r.PnLPct*100,
			r.EntryRegime, r.ExitReason, lesson.WhatWorked)
		summary.Lessons = append(summary.Lessons, lesson)
	}

	summary.TotalLessons = len(rows)
	summary.RecentWinRate = float64(wins) / float64(len(rows))

	// Find top winning and losing strategies.
	bestWR, worstWR := -1.0, 2.0
	for s, tot := range stratTotal {
		wr := float64(stratWins[s]) / float64(tot)
		if wr > bestWR {
			bestWR = wr
			summary.TopWinStrategy = s
		}
		if wr < worstWR {
			worstWR = wr
			summary.TopLossStrategy = s
		}
	}

	for regime, tot := range regimeTotal {
		summary.RegimeInsights[regime] = float64(regimeWins[regime]) / float64(tot)
	}

	g.mu.Lock()
	g.latest = summary
	g.mu.Unlock()
	return summary, nil
}

// GetLatestSummary returns the most recently generated lesson summary.
func (g *LessonGenerator) GetLatestSummary() *LessonSummary {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.latest
}

// StartPeriodicLearning runs GenerateLessons every interval until ctx is cancelled.
func (g *LessonGenerator) StartPeriodicLearning(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				summary, err := g.GenerateLessons(ctx)
				if err != nil {
					slog.Warn("[learning] lesson generation failed", "error", err)
					continue
				}
				slog.Info("[learning] lessons updated",
					"win_rate", summary.RecentWinRate,
					"lessons", summary.TotalLessons,
					"top_win", summary.TopWinStrategy,
				)
			}
		}
	}()
}

func generateLessonText(strategy string, outcome TradeOutcome, regime, exitReason string) (worked, failed string) {
	switch outcome {
	case OutcomeWin:
		worked = fmt.Sprintf("%s performed well in %s regime", strategy, regime)
		failed = ""
	case OutcomeLoss:
		worked = ""
		failed = fmt.Sprintf("%s underperformed in %s (exit: %s)", strategy, regime, exitReason)
	default:
		worked = fmt.Sprintf("%s breakeven in %s", strategy, regime)
		failed = ""
	}
	return
}
