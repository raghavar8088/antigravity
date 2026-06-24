// Package kelly implements the half-Kelly position sizing formula with
// regime, data-quality, and trade-history safety guards.
//
// All callers should use Compute(KellyInputs) and read FinalPositionUSD.
// Never use the raw Kelly fraction directly — the half-Kelly, regime, and
// quality multipliers are mandatory risk controls.
package kelly

import "fmt"

// hardMaxPositionPct is the absolute ceiling enforced regardless of any config.
// Reduced from 10% to 2% to account for up to 80 strategies trading simultaneously.
// 80 strategies × 2% = 160% max theoretical exposure (bounded by Kelly math below this).
const hardMaxPositionPct = 0.02 // 2%

// KellyInputs carries all inputs required for one sizing computation.
type KellyInputs struct {
	WinRate           float64 // rolling win rate, last N closed trades (0–1)
	AvgWinPct         float64 // average win as % of position value
	AvgLossPct        float64 // average loss as % of position value (positive)
	PortfolioValue    float64 // current portfolio value in USD
	MaxPositionPct    float64 // hard ceiling from env var (e.g. 0.10 = 10%)
	RegimeMult        float64 // from regime engine: 0.5, 0.75, 1.0, or 1.25
	SessionMult       float64 // from session detection: 1.0 London/NY, 0.5 Asia, 0.25 Off
	DataQualityScore  float64 // from data quality validator: 0–100
	MinTradesRequired int     // minimum trades before Kelly is applied (default 30)
	TradeCount        int     // number of closed trades in history
}

// KellyResult holds the sizing recommendation and full audit trail.
type KellyResult struct {
	RawKelly          float64     // f* = (p×b − q) / b
	HalfKelly         float64     // RawKelly × 0.5
	AfterRegime       float64     // HalfKelly × RegimeMult
	AfterSession      float64     // AfterRegime × SessionMult
	AfterDataQuality  float64     // AfterSession × dataQualityMult
	FinalPositionPct  float64     // after all constraints (0–0.10)
	FinalPositionUSD  float64     // FinalPositionPct × PortfolioValue
	WasConstrained    bool        // true if a ceiling was applied
	ConstraintReason  string      // which constraint fired
	Inputs            KellyInputs
}

// Compute derives the optimal (half-Kelly) position size from trade history
// and market context. It applies safety guards in strict order so no single
// factor can override the hard limits.
func Compute(inputs KellyInputs) (KellyResult, error) {
	min := inputs.MinTradesRequired
	if min == 0 {
		min = 30
	}

	// ── Step 1: Minimum trade count ───────────────────────────────────────────
	if inputs.TradeCount < min {
		pct := clamp(0.005, 0, effectiveMax(inputs))
		return KellyResult{
			FinalPositionPct: pct,
			FinalPositionUSD: pct * inputs.PortfolioValue,
			WasConstrained:   true,
			ConstraintReason: "insufficient_history",
			Inputs:           inputs,
		}, nil
	}

	// ── Step 2: Input validation ──────────────────────────────────────────────
	if inputs.WinRate <= 0 || inputs.WinRate >= 1 {
		return KellyResult{}, fmt.Errorf("kelly: WinRate must be in (0, 1), got %.4f", inputs.WinRate)
	}
	if inputs.AvgLossPct <= 0 {
		inputs.AvgLossPct = 0.02
	}

	// ── Step 3: Full Kelly formula ────────────────────────────────────────────
	b := inputs.AvgWinPct / inputs.AvgLossPct
	p := inputs.WinRate
	q := 1 - p
	rawKelly := (p*b - q) / b

	if rawKelly <= 0 {
		return KellyResult{
			RawKelly:         rawKelly,
			FinalPositionPct: 0,
			FinalPositionUSD: 0,
			WasConstrained:   true,
			ConstraintReason: "negative_kelly",
			Inputs:           inputs,
		}, nil
	}

	// ── Step 4: Half-Kelly (safety factor) ───────────────────────────────────
	halfKelly := rawKelly * 0.5

	// ── Step 5: Regime multiplier ─────────────────────────────────────────────
	regimeMult := inputs.RegimeMult
	if regimeMult <= 0 {
		regimeMult = 0.75 // conservative default
	}
	afterRegime := halfKelly * regimeMult

	// ── Step 5b: Session multiplier ───────────────────────────────────────────
	sessionMult := inputs.SessionMult
	if sessionMult <= 0 {
		sessionMult = 1.0 // default to full size when not set
	}
	afterSession := afterRegime * sessionMult

	// ── Step 6: Data quality multiplier ──────────────────────────────────────
	qMult := dataQualityMult(inputs.DataQualityScore)
	afterQuality := afterSession * qMult

	// ── Step 7: Floor and hard ceiling ────────────────────────────────────────
	floor := max64(afterQuality, 0.01)
	ceiling := min64(floor, effectiveMax(inputs))
	final := min64(ceiling, hardMaxPositionPct)

	wasConstrained := final < afterQuality
	reason := ""
	if wasConstrained {
		if final <= hardMaxPositionPct && afterQuality > hardMaxPositionPct {
			reason = "hard_ceiling_10pct"
		} else {
			reason = "max_position_pct"
		}
	}

	// ── Step 8: USD amount ────────────────────────────────────────────────────
	finalUSD := final * inputs.PortfolioValue

	return KellyResult{
		RawKelly:         rawKelly,
		HalfKelly:        halfKelly,
		AfterRegime:      afterRegime,
		AfterSession:     afterSession,
		AfterDataQuality: afterQuality,
		FinalPositionPct: final,
		FinalPositionUSD: finalUSD,
		WasConstrained:   wasConstrained,
		ConstraintReason: reason,
		Inputs:           inputs,
	}, nil
}

// LedgerInterface is the minimal interface for querying closed trade history.
type LedgerInterface interface {
	// ClosedTrades returns the last n closed trades for the given account.
	ClosedTrades(accountID string, n int) ([]ClosedTrade, error)
}

// ClosedTrade is the minimal shape of a closed trade needed for Kelly inputs.
type ClosedTrade struct {
	PnLPct float64 // realised PnL as % of entry capital (positive = win)
}

// GetKellyInputs computes KellyInputs from the last `lookback` closed trades.
func GetKellyInputs(ledger LedgerInterface, accountID string, lookback int, portfolio, maxPositionPct, regimeMult, dataQualityScore float64) (KellyInputs, error) {
	trades, err := ledger.ClosedTrades(accountID, lookback)
	if err != nil {
		return KellyInputs{}, fmt.Errorf("kelly: fetch trades: %w", err)
	}

	wins, losses := 0, 0
	totalWinPct, totalLossPct := 0.0, 0.0
	for _, t := range trades {
		if t.PnLPct > 0 {
			wins++
			totalWinPct += t.PnLPct
		} else if t.PnLPct < 0 {
			losses++
			totalLossPct += -t.PnLPct
		}
	}

	total := len(trades)
	winRate := 0.0
	if total > 0 {
		winRate = float64(wins) / float64(total)
	}
	avgWinPct := 0.0
	if wins > 0 {
		avgWinPct = totalWinPct / float64(wins)
	}
	avgLossPct := 0.02 // fallback
	if losses > 0 {
		avgLossPct = totalLossPct / float64(losses)
	}

	return KellyInputs{
		WinRate:           winRate,
		AvgWinPct:         avgWinPct,
		AvgLossPct:        avgLossPct,
		PortfolioValue:    portfolio,
		MaxPositionPct:    maxPositionPct,
		RegimeMult:        regimeMult,
		DataQualityScore:  dataQualityScore,
		MinTradesRequired: 30,
		TradeCount:        total,
	}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func effectiveMax(inputs KellyInputs) float64 {
	if inputs.MaxPositionPct > 0 && inputs.MaxPositionPct < hardMaxPositionPct {
		return inputs.MaxPositionPct
	}
	return hardMaxPositionPct
}

func dataQualityMult(score float64) float64 {
	switch {
	case score >= 80:
		return 1.0
	case score >= 60:
		return 0.75
	default:
		return 0.50
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

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
