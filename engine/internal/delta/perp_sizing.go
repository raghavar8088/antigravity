package delta

import (
	"fmt"
	"math"
	"strings"
)

// Sizing a scalp signal for a real, small account.
//
// The paper scalp desk trades a flat $3,000 notional per stream and holds up to
// ~374 positions at once across 2,416 streams. On a $100 account neither number
// is reachable: $3,000 is 30x the whole balance, one 0.35% stop on it is $10.50
// — more than a tenth of the account per trade — and 374 concurrent positions is
// not arithmetic that terminates.
//
// So "trade exactly like the scalp desk" is honoured where it means the SIGNAL
// and the EXIT GEOMETRY — same post-only limit entry, same stop and target
// distances, same time stop — and deliberately NOT where it means the paper
// desk's position size. Copying $3,000 onto $100 would not reproduce the desk's
// results at smaller scale; it would reproduce a margin call.
//
// Size is therefore derived from RISK: the distance to the stop is known before
// entry, so the notional that puts a fixed fraction of the account at stake on
// that stop is a division, not a guess.
//
//	notional = (equity x riskFraction) / stopDistanceFraction
//
// A $100 account risking 2% with a 0.35% stop takes $571 of notional — about
// 5.7x leverage, sized so a stop costs $2 rather than $10.50.

// PerpRiskConfig bounds what a small live account may do.
type PerpRiskConfig struct {
	// EquityUSD is the account's own money. Everything scales from it.
	EquityUSD float64
	// RiskPerTradeFraction is the share of equity lost if a trade stops out.
	RiskPerTradeFraction float64
	// MaxLeverage caps notional / equity regardless of what the risk maths
	// says. A very tight stop would otherwise imply enormous notional: at a
	// 0.05% stop, 2% risk asks for 40x. The stop is not a guarantee — it is a
	// resting order that can gap through — so leverage is bounded independently.
	MaxLeverage float64
	// MaxConcurrentPositions caps how many live positions may be open at once.
	// The paper desk's hundreds are not fundable here, and each open position
	// consumes margin that the next one then cannot have.
	MaxConcurrentPositions int

	// MaxPositionsPerSymbol caps how many live positions may share one
	// instrument.
	//
	// Without it, "3 concurrent positions" was three bets on the same coin.
	// Measured live: ANTI_Recurrence_Quantification_Signal, ANTI_D20_VWAP_
	// Reversion and ANTI_Recurrence again all opened COOKIEUSD SHORTS inside
	// 13 minutes and all three stopped out. Nominally three strategies
	// diversifying; actually one position in three pieces, and the symbol's
	// coarse tick grid — the thing that made those stops overshoot — applied
	// to every piece at once.
	//
	// Strategy names are not a diversification guarantee. Correlated entries
	// are the normal case for signals reading the same bars, so the cap is on
	// the instrument, which is what the risk is actually denominated in.
	MaxPositionsPerSymbol int

	// FixedContracts, when > 0, sends exactly this many contracts and ignores
	// risk-based sizing entirely.
	//
	// For running the desk at the smallest size the venue accepts, to test
	// whether the signals and the plumbing are right at a cost that does not
	// matter. It deliberately bypasses the risk maths: the point is a known,
	// minimal quantity, not a quantity derived from an account that is no
	// longer meaningfully at risk.
	//
	// Notional then varies with price — 1 contract is $0.03 of SOLVUSD and
	// $1.21 of LABUSD — so dollar P&L is NOT comparable between symbols at this
	// setting. Win rate, stop overshoot and fee drag are ratios and stay
	// comparable, which is what this mode exists to measure.
	FixedContracts int

	// TargetNotionalUSD sizes each order to roughly this position value,
	// overriding FixedContracts when set.
	//
	// One contract means wildly different risk per symbol: per-contract cost
	// spans 744x across the roster, from $0.014 of SAGAUSD to $10.40 of
	// BEATUSD. A stop-out on the cheap end costs a fraction of a rupee and on
	// the dear end costs twenty, so results were not comparable and the desk's
	// P&L was decided by whichever expensive coin happened to trade.
	//
	// Sizing to a common value makes each position carry similar risk, which is
	// what makes per-stream results mean anything.
	//
	// Symbols whose single contract already exceeds the target stay at one
	// contract — Delta has no fractional contracts, so one is the floor and
	// those positions simply run large.
	TargetNotionalUSD float64
	// MaxNotionalUSD is a hard per-order ceiling, independent of the above.
	MaxNotionalUSD float64
	// MaxAggregateLeverage caps notional across ALL open positions at once.
	//
	// Without it the per-order cap is misleading: 3 concurrent positions at 5x
	// each is 15x on the account, which on $116 is $1,750 of notional needing
	// far more margin than exists. The orders would simply be rejected — but a
	// margin rejection reads as an infrastructure problem, not as a risk config
	// that was never fundable.
	MaxAggregateLeverage float64
	// LeverageForOrder is sent to the venue so the margin each position consumes
	// is predictable rather than whatever the account happens to be set to.
	LeverageForOrder int
}

// DefaultPerpRiskConfig is the posture for a $100 live account.
//
// 2% risk per trade, 5x leverage cap, 3 concurrent positions. These are
// deliberately conservative: this account exists to find out whether a scalp
// strategy survives contact with a real book, and losing the account in a day
// answers nothing.
func DefaultPerpRiskConfig(equityUSD float64) PerpRiskConfig {
	return PerpRiskConfig{
		EquityUSD:            equityUSD,
		RiskPerTradeFraction: 0.02,
		MaxLeverage:          3.0,
		// Three positions at 3x each would be 9x aggregate; the aggregate cap is
		// what actually binds, and it is set so the whole book fits inside the
		// account with room for the options engine, which shares this wallet.
		MaxAggregateLeverage:   3.0,
		MaxConcurrentPositions: 3,
		// One per symbol: the concentration observed was same-symbol AND
		// same-direction, so anything above 1 re-admits the failure in smaller
		// form.
		MaxPositionsPerSymbol: 1,
		MaxNotionalUSD:        equityUSD * 3.0,
		// 10x on the order keeps margin at a tenth of notional, so a 3x book
		// consumes ~30% of equity as margin rather than all of it. The 0.35%
		// stop sits far inside the liquidation distance this implies.
		LeverageForOrder: 10,
	}
}

// PerpOrderPlan is a fully-resolved order, ready to send.
type PerpOrderPlan struct {
	Symbol      string
	ProductID   int
	Contracts   int
	Side        OrderSide
	LimitPrice  float64
	StopPrice   float64
	TargetPrice float64
	// NotionalUSD and RiskUSD are what this order actually commits, after every
	// cap has been applied. They are recorded so a fill can be audited against
	// the intent rather than re-derived later.
	NotionalUSD float64
	RiskUSD     float64
	Leverage    float64
}

// ErrRiskTooSmall means every cap has been applied and the result is below one
// contract. The signal is skipped rather than rounded up: rounding up is how a
// "$2 risk" becomes a $12 risk on a cheap account.
var ErrRiskTooSmall = fmt.Errorf("delta: risk-sized order is below one contract")

// ErrAggregateExposureReached means the book is already at its total leverage
// ceiling, so no further position can be funded regardless of its own size.
var ErrAggregateExposureReached = fmt.Errorf("delta: aggregate perpetual exposure ceiling reached")

// ErrTooManyPositions means the concurrency cap is reached.
var ErrTooManyPositions = fmt.Errorf("delta: max concurrent perpetual positions reached")

// minNotionalFraction is the smallest share of a trade's intended notional that
// is still worth opening.
//
// Below it the position is dust: full fees, a concurrency slot consumed, and a
// record entry that looks like evidence while carrying none. 25% keeps a
// genuinely partial position — the aggregate cap doing its job — while refusing
// the 0.08%-of-intent fills that prompted this.
const minNotionalFraction = 0.25

// PlanPerpOrder turns a scalp signal into a sized order, or explains why it
// cannot.
//
// entry, stop and target come from the scalp desk unchanged — the geometry is
// the strategy's, not this function's. All this decides is HOW MANY contracts
// that geometry is worth on a small account.
func PlanPerpOrder(
	reg *PerpRegistry,
	cfg PerpRiskConfig,
	symbol string,
	long bool,
	entry, stop, target float64,
	openPositions int,
	openNotionalUSD float64,
) (PerpOrderPlan, error) {
	if cfg.EquityUSD <= 0 {
		return PerpOrderPlan{}, fmt.Errorf("delta: account equity is %.2f — nothing to size against", cfg.EquityUSD)
	}
	if openPositions >= cfg.MaxConcurrentPositions {
		return PerpOrderPlan{}, fmt.Errorf("%w (%d open, cap %d)", ErrTooManyPositions, openPositions, cfg.MaxConcurrentPositions)
	}
	if entry <= 0 || stop <= 0 {
		return PerpOrderPlan{}, fmt.Errorf("delta: entry %.6f / stop %.6f are not usable prices", entry, stop)
	}
	// The stop must be on the correct side of entry, or the "risk" is a gain and
	// the sizing maths silently inverts.
	if long && stop >= entry {
		return PerpOrderPlan{}, fmt.Errorf("delta: long stop %.6f is not below entry %.6f", stop, entry)
	}
	if !long && stop <= entry {
		return PerpOrderPlan{}, fmt.Errorf("delta: short stop %.6f is not above entry %.6f", stop, entry)
	}

	stopFrac := math.Abs(entry-stop) / entry
	if stopFrac <= 0 {
		return PerpOrderPlan{}, fmt.Errorf("delta: stop distance is zero")
	}

	// Target-notional mode: size to a common position value.
	//
	// Unlike fixed-size, this DOES respect the aggregate cap below rather than
	// bypassing it. A $3 target across ten concurrent positions is $30, exactly
	// the book ceiling, so the cap is load-bearing here — skipping it would let
	// the book run to whatever the roster asked for.
	if cfg.TargetNotionalUSD > 0 {
		prod, ok := reg.Lookup(symbol)
		if !ok {
			return PerpOrderPlan{}, fmt.Errorf("delta: %s is not a known product", symbol)
		}
		perContract := prod.NotionalPerContract(entry)
		if perContract <= 0 {
			return PerpOrderPlan{}, fmt.Errorf("delta: %s has no usable contract value", symbol)
		}
		// Rounded, with a floor of one. Rounding down would send zero contracts
		// for every symbol dearer than the target; the floor keeps those
		// tradable at their minimum instead of silently dropping them.
		contracts := int(math.Round(cfg.TargetNotionalUSD / perContract))
		if contracts < 1 {
			contracts = 1
		}
		notional := float64(contracts) * perContract

		// The book ceiling still binds. A position that does not fit is refused
		// rather than shrunk: shrinking a target-sized order reintroduces the
		// dust it exists to prevent.
		if cfg.MaxAggregateLeverage > 0 {
			ceiling := cfg.EquityUSD * cfg.MaxAggregateLeverage
			if room := ceiling - openNotionalUSD; notional > room {
				return PerpOrderPlan{}, fmt.Errorf(
					"%w: %s needs $%.2f but only $%.2f is left against a $%.2f ceiling",
					ErrAggregateExposureReached, symbol, notional, room, ceiling)
			}
		}

		side := OrderSide("buy")
		if !long {
			side = OrderSide("sell")
		}
		return PerpOrderPlan{
			Symbol:      prod.Symbol,
			ProductID:   prod.ProductID,
			Side:        side,
			Contracts:   contracts,
			LimitPrice:  entry,
			StopPrice:   stop,
			TargetPrice: target,
			NotionalUSD: notional,
			RiskUSD:     float64(contracts) * math.Abs(entry-stop) * prod.ContractValue,
			Leverage:    float64(cfg.LeverageForOrder),
		}, nil
	}

	// Fixed-size mode short-circuits the risk maths and every notional cap.
	//
	// The caps exist to stop a position being too LARGE; a fixed minimum size
	// cannot be. Running it through them would let the aggregate ceiling or the
	// dust guard refuse a one-contract order — the dust guard especially, since
	// it compares against an intended notional this mode never computes. The
	// concurrency and per-symbol caps above still apply, because those are
	// about how many positions exist, not how big they are.
	if cfg.FixedContracts > 0 {
		prod, ok := reg.Lookup(symbol)
		if !ok {
			return PerpOrderPlan{}, fmt.Errorf("delta: %s is not a known product", symbol)
		}
		side := OrderSide("buy")
		if !long {
			side = OrderSide("sell")
		}
		return PerpOrderPlan{
			Symbol:      prod.Symbol,
			ProductID:   prod.ProductID,
			Side:        side,
			Contracts:   cfg.FixedContracts,
			LimitPrice:  entry,
			StopPrice:   stop,
			TargetPrice: target,
			NotionalUSD: float64(cfg.FixedContracts) * prod.NotionalPerContract(entry),
			RiskUSD:     float64(cfg.FixedContracts) * math.Abs(entry-stop) * prod.ContractValue,
			Leverage:    float64(cfg.LeverageForOrder),
		}, nil
	}

	riskUSD := cfg.EquityUSD * cfg.RiskPerTradeFraction
	notional := riskUSD / stopFrac

	// Independent caps. Leverage first, because a very tight stop implies
	// enormous notional and a resting stop can gap through.
	if maxByLev := cfg.EquityUSD * cfg.MaxLeverage; notional > maxByLev {
		notional = maxByLev
	}
	if cfg.MaxNotionalUSD > 0 && notional > cfg.MaxNotionalUSD {
		notional = cfg.MaxNotionalUSD
	}
	// AGGREGATE cap. What is already open counts against the same account, so
	// the room left is the book's ceiling minus the book.
	if cfg.MaxAggregateLeverage > 0 {
		room := cfg.EquityUSD*cfg.MaxAggregateLeverage - openNotionalUSD
		if room <= 0 {
			return PerpOrderPlan{}, fmt.Errorf("%w: $%.2f already open against a $%.2f book ceiling",
				ErrAggregateExposureReached, openNotionalUSD, cfg.EquityUSD*cfg.MaxAggregateLeverage)
		}
		if notional > room {
			// Shrinking to fit is right down to a point and wrong past it.
			//
			// Measured live: SKYAIUSD opened at $299 of a $300 book ceiling,
			// and the next two signals were sized into the remainder — 3
			// contracts of COOKIEUSD ($0.44) and 1 of MUBARAKUSD ($0.25),
			// against an intended $300. Those are not smaller versions of the
			// trade. They pay full round-trip fees, occupy one of three
			// concurrency slots, and write a "fill" into the strategy's record
			// that measures nothing — a -$0.0023 result carries no information
			// about the strategy but counts as evidence on the leaderboard.
			//
			// So a partial position is allowed while it is still a position,
			// and refused once it is dust.
			if room < notional*minNotionalFraction {
				return PerpOrderPlan{}, fmt.Errorf(
					"%w: only $%.2f of room left against a $%.2f ceiling, under %.0f%% of the $%.2f this trade needs",
					ErrAggregateExposureReached, room, cfg.EquityUSD*cfg.MaxAggregateLeverage,
					minNotionalFraction*100, notional)
			}
			notional = room
		}
	}

	contracts, prod, err := reg.SizeContracts(symbol, notional, entry)
	if err != nil {
		if isBelowOneContract(err) {
			return PerpOrderPlan{}, fmt.Errorf("%w: %s at $%.2f notional", ErrRiskTooSmall, symbol, notional)
		}
		return PerpOrderPlan{}, err
	}

	side := OrderSide("buy")
	if !long {
		side = OrderSide("sell")
	}

	// Recompute from the contracts actually being sent, not from the requested
	// notional. Rounding down means the real exposure is smaller, and the audit
	// trail should say what was sent.
	actualNotional := float64(contracts) * prod.NotionalPerContract(entry)
	return PerpOrderPlan{
		Symbol:      prod.Symbol,
		ProductID:   prod.ProductID,
		Contracts:   contracts,
		Side:        side,
		LimitPrice:  roundToTick(entry, prod.TickSize),
		StopPrice:   roundToTick(stop, prod.TickSize),
		TargetPrice: roundToTick(target, prod.TickSize),
		NotionalUSD: actualNotional,
		RiskUSD:     actualNotional * stopFrac,
		Leverage:    actualNotional / cfg.EquityUSD,
	}, nil
}

// isBelowOneContract recognises the registry's sub-contract refusal.
func isBelowOneContract(err error) bool {
	return err != nil && strings.Contains(err.Error(), "below one")
}

// roundToTick snaps a price to the venue's tick size. Delta rejects a limit
// price off the tick, and a rejected entry looks like a strategy that declined
// to trade rather than an order that was never valid.
func roundToTick(price, tick float64) float64 {
	if tick <= 0 || price <= 0 {
		return price
	}
	return math.Round(price/tick) * tick
}
