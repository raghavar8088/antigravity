package delta

import "math"

// Perpetual P&L accounting.
//
// A forensic audit on 2026-08-01 found the bridge reporting +$0.9424 for a day
// the venue recorded as -$3.5405 — a $4.4829 overstatement, decomposing exactly
// into four defects that all biased the same way:
//
//	$0.4425  closes booked at the trigger MARK, not the fill price
//	$0.8632  an externally-closed position booked at ENTRY, forcing P&L to 0
//	$1.2686  round trips lost before the book was persisted
//	$1.9086  trading fees never subtracted
//
// The common thread is that P&L was DERIVED from prices this process chose,
// rather than taken from what the venue actually did. Every check verified the
// bridge against itself — stats agreed with trades agreed with the leaderboard,
// all three computed from the same wrong numbers — so nothing disagreed until a
// human compared two systems.
//
// The rules below exist so that cannot recur.

// PerpTakerFeeRate is Delta India's taker fee on perpetual futures, INCLUDING
// the 18% GST that appears on every fill.
//
// 0.05% base + 18% GST = 0.059% of notional per side. Derived from the venue's
// own order log rather than from documentation:
//
//	order value 286.2528 -> fee 0.16888916  = 0.059%
//	order value  57.1890 -> fee 0.03374151  = 0.059%
//
// It is a CONSTANT here only because the bridge places taker (market/ioc)
// orders exclusively. A maker path would need a different rate, and using this
// one for it would understate a rebate as a charge.
const PerpTakerFeeRate = 0.00059

// PerpFeeUSD is the fee charged on one side of a perpetual trade.
//
// Fees are small against turnover — 0.059% — but they are NOT small against the
// edges being measured. These strategies target a few cents per trade, so a
// round trip's fees are comparable to the entire signal. Reporting gross as if
// it were net does not shade the result; it inverts it.
func PerpFeeUSD(price float64, contracts int, contractValue float64) float64 {
	if price <= 0 || contracts == 0 || contractValue <= 0 {
		return 0
	}
	n := contracts
	if n < 0 {
		n = -n
	}
	notional := price * float64(n) * contractValue
	return notional * PerpTakerFeeRate
}

// PerpResult is one closed perpetual trade, stated honestly.
//
// Gross, fees and net are separate fields rather than one number, for the same
// reason the options desk reports them separately: a desk can look profitable
// gross and shrink net, and collapsing them hides exactly that.
type PerpResult struct {
	Gross    float64
	EntryFee float64
	ExitFee  float64
	Net      float64
}

// ComputePerpResult books a closed perpetual from the prices that were actually
// FILLED, not from the marks that triggered the decision.
//
// entryFill and exitFill must both come from the venue's order responses. The
// bug this replaces used the exit MARK — the price that triggered the close —
// while the market/ioc order filled elsewhere. On one 1,099-contract ADAUSD
// trade that difference (0.17263 vs 0.17290) turned a recorded +$0.0751 into a
// real -$0.1395: not a rounding error, a sign flip.
func ComputePerpResult(entryFill, exitFill float64, contracts int, contractValue float64, long bool) PerpResult {
	n := contracts
	if n < 0 {
		n = -n
	}
	dir := 1.0
	if !long {
		dir = -1.0
	}
	gross := (exitFill - entryFill) * dir * float64(n) * contractValue
	ef := PerpFeeUSD(entryFill, n, contractValue)
	xf := PerpFeeUSD(exitFill, n, contractValue)
	return PerpResult{Gross: gross, EntryFee: ef, ExitFee: xf, Net: gross - ef - xf}
}

// Exit reasons that describe how a position left the book. These are values
// rather than free strings because the difference between them is the
// difference between a measured result and an unknown one.
const (
	// ExitReasonUnreconciled marks a position the venue closed without this
	// bridge seeing the price.
	//
	// The previous behaviour booked these at ENTRY, producing exactly $0.00 —
	// which reads identically to a flat trade. One such position was a
	// LIQUIDATION at -$0.8632, recorded as nothing. An unknown that is visible
	// is recoverable; a loss disguised as zero is not.
	ExitReasonUnreconciled = "UNRECONCILED"
	// ExitReasonLiquidated marks a venue liquidation. Distinct from a stop,
	// because a liquidation means the margin model was wrong, not that the
	// strategy was.
	ExitReasonLiquidated = "LIQUIDATED"
)

// PerpReconciliation compares this bridge's booked P&L against the venue's over
// the same window.
//
// This is the check whose absence let all four defects run for a full day. Every
// existing test verified the bridge against itself; none asked the venue whether
// it agreed.
type PerpReconciliation struct {
	BridgeNetUSD float64 `json:"bridgeNetUsd"`
	VenueNetUSD  float64 `json:"venueNetUsd"`
	DriftUSD     float64 `json:"driftUsd"`
	Matched      bool    `json:"matched"`
	Trades       int     `json:"trades"`
}

// perpDriftTolerance is how far bridge and venue may differ before it is a
// finding. Tight on purpose: the whole point is to catch a systematic bias, and
// a systematic bias is small per trade before it is large in aggregate.
const perpDriftTolerance = 0.02

// Reconcile compares two totals and reports whether they agree.
func ReconcilePerpPnL(bridgeNet, venueNet float64, trades int) PerpReconciliation {
	drift := bridgeNet - venueNet
	return PerpReconciliation{
		BridgeNetUSD: bridgeNet,
		VenueNetUSD:  venueNet,
		DriftUSD:     drift,
		Matched:      math.Abs(drift) <= perpDriftTolerance,
		Trades:       trades,
	}
}

// ── liquidation-distance guard ───────────────────────────────────────────────
//
// The defect this prevents: on 2026-08-01 two positions were force-closed by
// Delta at EXACTLY 0.500% adverse, while the strategies' own stops sat at 0.93%
// and 0.98%. ADAUSD ships at default_leverage 100 with maintenance_margin 0.5%,
// so the liquidation price sat INSIDE the stop. The venue closed every losing
// trade before the strategy's risk management could act, and the desk recorded
// one of them as a routine external close worth $0.00.
//
// A stop that cannot be reached is not a stop. It is a number on a screen.

// PerpLeverage is the account leverage this desk sets per product.
//
// 10x puts the liquidation distance around 9-10% of entry, which is ten to
// thirty times wider than the 0.35%-0.98% stops these strategies use — so the
// STRATEGY decides the exit, not the venue. The cost is margin: 10% of notional
// instead of 1%. On the 3x aggregate cap that is ~30% of a $100 account, which
// the risk config was already sized to allow.
const PerpLeverage = 10

// liquidationSafetyFactor is how much room the liquidation distance must have
// beyond the stop. 2x, so an adverse gap that overshoots the stop still resolves
// as a stop rather than a liquidation.
const liquidationSafetyFactor = 2.0

// ErrStopBeyondLiquidation means the venue would liquidate before the strategy's
// stop is reached, so the trade cannot be risk-managed as designed.
var ErrStopBeyondLiquidation = errStopBeyondLiquidation{}

type errStopBeyondLiquidation struct{}

func (errStopBeyondLiquidation) Error() string {
	return "delta: stop sits beyond the liquidation distance — the venue would close the position first"
}

// LiquidationDistanceFraction is how far price may move against a position
// before the venue liquidates it, as a fraction of entry.
//
// Approximated as (initial margin - maintenance margin) at the configured
// leverage. At 10x with a 0.5% maintenance requirement that is ~9.5%.
func LiquidationDistanceFraction(leverage int, maintenanceMarginPct float64) float64 {
	if leverage <= 0 {
		return 0
	}
	initial := 1.0 / float64(leverage)
	d := initial - maintenanceMarginPct/100
	if d < 0 {
		return 0
	}
	return d
}

// StopIsReachable reports whether a stop can be hit before liquidation.
//
// Called before an order is placed. Refusing here is deliberate: a trade whose
// stop the venue will pre-empt is not the trade the strategy was measured on,
// and taking it produces a record of liquidations dressed as stop-outs.
func StopIsReachable(entry, stop float64, leverage int, maintenanceMarginPct float64) bool {
	if entry <= 0 || stop <= 0 {
		return false
	}
	stopFrac := math.Abs(entry-stop) / entry
	liq := LiquidationDistanceFraction(leverage, maintenanceMarginPct)
	if liq <= 0 {
		return false
	}
	return stopFrac*liquidationSafetyFactor <= liq
}
