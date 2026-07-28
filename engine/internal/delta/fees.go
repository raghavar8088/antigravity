package delta

import "fmt"

// Delta Exchange option fee schedule, and the entry economics that follow from
// it. This file is the single source of truth for what a live option round trip
// actually costs; before it existed the live path modelled no fee at all, so
// every realised P&L, every strategy statistic, and every go-live decision was
// computed on gross numbers and systematically flattered the desk.
//
// The schedule: per side, Delta charges OptionFeeRateOfNotional of the
// UNDERLYING notional, capped at OptionFeeCapOfPremium of the option premium.
// The cap is what makes cheap options structurally expensive — below a premium
// of OptionFeeRateOfNotional/OptionFeeCapOfPremium of notional (0.3%) the cap
// binds and the fee becomes a flat 10% of premium per side no matter how small
// the ticket. Sizing cannot dilute it; only buying a richer option can.
const (
	// OptionFeeRateOfNotional is the per-side fee as a fraction of underlying notional.
	OptionFeeRateOfNotional = 0.0003 // 0.03%
	// OptionFeeCapOfPremium caps the per-side fee at this fraction of the premium.
	OptionFeeCapOfPremium = 0.10 // 10%
)

// OptionNotionalUSD is the underlying value behind a position: spot × BTC size.
func OptionNotionalUSD(spotBTC float64, contracts int) float64 {
	if spotBTC <= 0 || contracts == 0 {
		return 0
	}
	return spotBTC * absInt(contracts) * OptionContractSizeBTC
}

// OptionPremiumUSD converts a quoted premium (USD per BTC) into the USD cash
// value of the position. A contract is OptionContractSizeBTC of BTC.
func OptionPremiumUSD(premiumPerBTC float64, contracts int) float64 {
	if premiumPerBTC <= 0 || contracts == 0 {
		return 0
	}
	return premiumPerBTC * absInt(contracts) * OptionContractSizeBTC
}

// OptionFeeUSD is the fee for ONE side (open or close) in USD: the notional rate,
// capped at a share of that side's premium. Both legs of a round trip are priced
// at their own premium, so a winning exit costs more in fees than the entry did.
//
// spotBTC is the underlying price used for the notional leg. When it is unknown
// (an adopted orphan, a stale record) callers pass 0 and the cap alone applies,
// which is the conservative choice for the cheap options this desk trades.
func OptionFeeUSD(premiumPerBTC, spotBTC float64, contracts int) float64 {
	premiumUSD := OptionPremiumUSD(premiumPerBTC, contracts)
	if premiumUSD <= 0 {
		return 0
	}
	cap := premiumUSD * OptionFeeCapOfPremium
	notionalUSD := OptionNotionalUSD(spotBTC, contracts)
	if notionalUSD <= 0 {
		return cap
	}
	byNotional := notionalUSD * OptionFeeRateOfNotional
	if byNotional < cap {
		return byNotional
	}
	return cap
}

// RoundTripFeeUSD is the total fee for opening at entryPremium and closing at
// exitPremium, both quoted USD per BTC.
func RoundTripFeeUSD(entryPremium, exitPremium, spotBTC float64, contracts int) float64 {
	return OptionFeeUSD(entryPremium, spotBTC, contracts) +
		OptionFeeUSD(exitPremium, spotBTC, contracts)
}

// RoundTripFeePctOfEntryPremium expresses the round-trip cost of a trade that
// runs to its take-profit as a percentage of the premium paid. This is the
// number that decides whether an option is worth buying at all: at the 10% cap
// with a +80% target it is a flat 28%, meaning the position must gain 28% before
// the trader keeps a cent.
func RoundTripFeePctOfEntryPremium(entryPremium, spotBTC float64, contracts int, targetGainPct float64) float64 {
	premiumUSD := OptionPremiumUSD(entryPremium, contracts)
	if premiumUSD <= 0 {
		return 0
	}
	exitPremium := entryPremium * (1 + targetGainPct)
	return RoundTripFeeUSD(entryPremium, exitPremium, spotBTC, contracts) / premiumUSD * 100
}

// MaxRoundTripFeePctOfPremium is the entry economics guard: an option whose
// round-trip fee eats more than this share of the premium is declined before it
// is ever bought. The desk's live history sat at 28% — a tax that demanded a
// 55.6% win rate from a long-option book that ran at 11%. The default admits
// only options rich enough that the notional rate, not the 10% cap, is binding.
//
// Override with DELTA_MAX_ROUNDTRIP_FEE_PCT. Setting it to 0 or a negative value
// disables the guard, which restores the pre-guard behaviour.
const DefaultMaxRoundTripFeePctOfPremium = 8.0

// MaxRoundTripFeePctOfPremium returns the configured entry guard threshold.
func MaxRoundTripFeePctOfPremium() float64 {
	return parseEnvFloat("DELTA_MAX_ROUNDTRIP_FEE_PCT", DefaultMaxRoundTripFeePctOfPremium)
}

// EntryEconomics is the fee verdict for a proposed entry, in the form the bridge
// logs and the operator can read: what the round trip costs, what the guard
// allows, and what premium would have been rich enough to pass.
type EntryEconomics struct {
	PremiumUSD       float64
	NotionalUSD      float64
	RoundTripFeePct  float64
	LimitPct         float64
	MinPremiumPerBTC float64
	Acceptable       bool
	Reason           string
}

// EvaluateEntryEconomics decides whether an option is cheap enough in fee terms
// to be worth buying. It is deliberately independent of the signal: a strategy
// may be right about direction and still lose because the instrument it wants to
// express that view in is untradeable after costs.
func EvaluateEntryEconomics(entryPremium, spotBTC float64, contracts int, targetGainPct float64) EntryEconomics {
	limit := MaxRoundTripFeePctOfPremium()
	e := EntryEconomics{
		PremiumUSD:  OptionPremiumUSD(entryPremium, contracts),
		NotionalUSD: OptionNotionalUSD(spotBTC, contracts),
		LimitPct:    limit,
	}
	e.RoundTripFeePct = RoundTripFeePctOfEntryPremium(entryPremium, spotBTC, contracts, targetGainPct)
	e.MinPremiumPerBTC = MinPremiumPerBTCForFeeLimit(spotBTC, targetGainPct, limit)

	if limit <= 0 {
		e.Acceptable = true
		e.Reason = "fee guard disabled"
		return e
	}
	if e.PremiumUSD <= 0 {
		e.Acceptable = false
		e.Reason = "premium unknown — cannot price the round trip"
		return e
	}
	// Compared with a relative tolerance so the boundary is inclusive in practice.
	// Without it MinPremiumPerBTCForFeeLimit advertises a minimum premium that
	// lands a few ULPs above the limit and is then rejected by this very check —
	// the guard would tell the operator to buy something it still declines.
	if e.RoundTripFeePct <= limit*(1+1e-9) {
		e.Acceptable = true
		e.Reason = fmt.Sprintf("round-trip fee %.1f%% of premium within the %.1f%% limit", e.RoundTripFeePct, limit)
		return e
	}
	e.Acceptable = false
	e.Reason = fmt.Sprintf(
		"round-trip fee %.1f%% of premium exceeds the %.1f%% limit — premium %.2f/BTC is too cheap; needs ≥ %.0f/BTC at this spot",
		e.RoundTripFeePct, limit, entryPremium, e.MinPremiumPerBTC)
	return e
}

// MinPremiumPerBTCForFeeLimit inverts the fee formula: the cheapest premium
// (quoted USD per BTC) whose round-trip cost still fits inside limitPct. Below
// the cap-binding point no premium qualifies, so this reports the level at which
// the notional rate takes over.
//
// With the cap not binding, round-trip fee = notional×rate×2 and the premium
// scales out, so: limit = 2·rate·spot / premium ⇒ premium = 2·rate·spot / limit.
// (The exit leg is priced at the target premium, which only ever lowers the
// requirement, so this is the conservative bound.)
func MinPremiumPerBTCForFeeLimit(spotBTC, targetGainPct, limitPct float64) float64 {
	if spotBTC <= 0 || limitPct <= 0 {
		return 0
	}
	_ = targetGainPct
	return 2 * OptionFeeRateOfNotional * spotBTC / (limitPct / 100)
}

// BreakEvenWinRatePct is the win rate a long-option book needs purely to break
// even, given its take-profit, stop-loss and the fee schedule. It exists so the
// operator sees the bar before capital is committed rather than inferring it
// from a drawdown. Returns 0 when the geometry cannot break even at any rate.
func BreakEvenWinRatePct(entryPremium, spotBTC float64, contracts int, takeProfitPct, stopLossPct float64) float64 {
	premiumUSD := OptionPremiumUSD(entryPremium, contracts)
	if premiumUSD <= 0 {
		return 0
	}
	winNet := premiumUSD*takeProfitPct - RoundTripFeeUSD(entryPremium, entryPremium*(1+takeProfitPct), spotBTC, contracts)
	lossNet := premiumUSD*stopLossPct + RoundTripFeeUSD(entryPremium, entryPremium*(1-stopLossPct), spotBTC, contracts)
	if winNet <= 0 {
		return 0 // no win rate can pay for the fees at this geometry
	}
	return 100 * lossNet / (winNet + lossNet)
}

func absInt(v int) float64 {
	if v < 0 {
		return float64(-v)
	}
	return float64(v)
}
