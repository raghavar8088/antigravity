package trading

import (
	"fmt"
	"sync"

	"antigravity-engine/internal/aiscoring"
	"antigravity-engine/internal/calibration"
	"antigravity-engine/internal/dataquality"
	"antigravity-engine/internal/derivatives"
	"antigravity-engine/internal/dominance"
	"antigravity-engine/internal/etf"
	"antigravity-engine/internal/eventstore"
	"antigravity-engine/internal/kelly"
	"antigravity-engine/internal/learning"
	"antigravity-engine/internal/macro"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/ml"
	"antigravity-engine/internal/orderbook"
	"antigravity-engine/internal/regime"
	"antigravity-engine/internal/sentiment"
	"antigravity-engine/internal/temporal"
)

// LoopDeps carries all Phase 1–5 + Phase C + Phase D upgrade dependencies into the trading loop.
// All dependencies must flow through this struct — never via package-level globals.
type LoopDeps struct {
	// Phase 1–5 (required)
	DataValidator    *dataquality.Validator
	AsyncScorer      *aiscoring.AsyncScorer
	FallbackScorer   *aiscoring.FallbackScorer
	RegimeClassifier *regime.Classifier
	StrategyGate     *regime.StrategyGate
	CycleGuard       *CycleGuard
	FundingFetcher   *derivatives.FundingFetcher
	OIFetcher        *derivatives.OIFetcher
	DepthSubscriber  *orderbook.DepthSubscriber
	// DVOLHolder is optional (nil = DVOL-dependent strategies fall back to
	// RealizedVol). Polled in the background by DeribitDVOLHolder.StartPolling.
	DVOLHolder *marketdata.DeribitDVOLHolder
	// LiquidationHolder is optional (nil = S14 Liquidation_Cascade_Fade is
	// suppressed). Streamed in the background by
	// BinanceLiquidationHolder.StartStreaming.
	LiquidationHolder *marketdata.BinanceLiquidationHolder
	// PerpPriceHolder is optional (nil = S16 Perp_Spot_Basis_Momentum is
	// suppressed). Polled in the background by
	// DeltaPerpPriceHolder.StartPolling.
	PerpPriceHolder *marketdata.DeltaPerpPriceHolder
	// MacroFeedHolder is optional (nil = S18-S21 macro family strategies
	// degrade to NoSignal/reduced confidence). Polled in the background by
	// MacroFeedHolder.StartPolling (Nasdaq futures proxy + DXY).
	MacroFeedHolder *marketdata.MacroFeedHolder
	Ledger          kelly.LedgerInterface
	PortfolioValue  float64

	// Phase C market signal fetchers (optional — nil = signal skipped, score = 0)
	ETFFetcher       *etf.ETFFetcher
	DominanceFetcher *dominance.DominanceFetcher
	MacroFetcher     *macro.MacroFetcher
	SentimentFetcher *sentiment.SentimentFetcher

	// Phase C temporal engine (required — falls back gracefully if patterns empty)
	TemporalAnalyser *temporal.TemporalAnalyser

	// Phase D AI intelligence (optional — nil disables feature)
	LessonGenerator *learning.LessonGenerator // post-trade self-learning loop

	// Phase E optional components (nil-safe — engine operates identically without them)
	EventStore  *eventstore.EventWriter // dual-write audit trail; non-blocking
	MLPrescorer *ml.MLPrescorer         // local XGBoost pre-filter; skipped when nil

	// calibResult holds the most recent confidence calibration result.
	// Thread-safe via calibMu; updated by ScheduleRecalibration callback in main.go.
	calibResult *calibration.CalibrationResult
	calibMu     sync.RWMutex
}

// UpdateCalibration stores a new calibration result thread-safely.
func (d *LoopDeps) UpdateCalibration(r *calibration.CalibrationResult) {
	d.calibMu.Lock()
	d.calibResult = r
	d.calibMu.Unlock()
}

// GetCalibration returns the current calibration result (may be nil before first run).
func (d *LoopDeps) GetCalibration() *calibration.CalibrationResult {
	d.calibMu.RLock()
	defer d.calibMu.RUnlock()
	return d.calibResult
}

// Validate checks that the required dependencies are present.
func (d *LoopDeps) Validate() error {
	if d == nil {
		return fmt.Errorf("LoopDeps is nil")
	}
	if d.DataValidator == nil {
		return fmt.Errorf("LoopDeps: DataValidator is required")
	}
	if d.CycleGuard == nil {
		return fmt.Errorf("LoopDeps: CycleGuard is required")
	}
	if d.AsyncScorer == nil {
		return fmt.Errorf("LoopDeps: AsyncScorer is required")
	}
	if d.FallbackScorer == nil {
		return fmt.Errorf("LoopDeps: FallbackScorer is required")
	}
	if d.TemporalAnalyser == nil {
		return fmt.Errorf("LoopDeps: TemporalAnalyser is required")
	}
	return nil
}
