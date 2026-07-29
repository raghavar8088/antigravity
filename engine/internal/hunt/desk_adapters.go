package hunt

import (
	"time"

	"antigravity-engine/internal/options"
	"antigravity-engine/internal/options_selling"
)

// Adapters that expose each desk's closed trades to the hunt.
//
// These read the desks' existing exported state rather than hooking their trade
// paths. That keeps the accounting DERIVED — reproducible from the trade record,
// and impossible to desync from it — and means no engine's balance handling was
// modified to make the hunt work.

// BuyingDesk adapts the option-buying engine.
type BuyingDesk struct {
	Engine  *options.Engine
	Capital float64
}

func (d BuyingDesk) DeskName() string { return "buying" }

func (d BuyingDesk) StartingCapital() float64 {
	if d.Capital > 0 {
		return d.Capital
	}
	return DefaultStartingCapital
}

func (d BuyingDesk) HuntTrades() []Trade {
	if d.Engine == nil {
		return nil
	}
	st := d.Engine.ExportState()
	out := make([]Trade, 0, len(st.Trades))
	for _, t := range st.Trades {
		out = append(out, Trade{
			Strategy: t.StrategyName,
			// NetPnL on this desk is already after the engine's fee model.
			// Gross is reconstructed from the premium legs so fee drag is
			// visible rather than folded invisibly into one number.
			NetPnL:   t.NetPnL,
			GrossPnL: (t.ExitPremium - t.EntryPremium) * t.Quantity,
			Fees:     grossMinusNet((t.ExitPremium-t.EntryPremium)*t.Quantity, t.NetPnL),
			ClosedAt: closedAt(t.ExitTime, t.EntryTime),
		})
	}
	return out
}

// SellingDesk adapts the option-selling engine.
type SellingDesk struct {
	Engine  *options_selling.Engine
	Capital float64
}

func (d SellingDesk) DeskName() string { return "selling" }

func (d SellingDesk) StartingCapital() float64 {
	if d.Capital > 0 {
		return d.Capital
	}
	return DefaultStartingCapital
}

func (d SellingDesk) HuntTrades() []Trade {
	if d.Engine == nil {
		return nil
	}
	st := d.Engine.ExportState()
	out := make([]Trade, 0, len(st.Trades))
	for _, t := range st.Trades {
		// A seller's gross is the premium RECEIVED minus what it cost to buy
		// back — the mirror of the buying desk, so the sign convention is
		// explicit rather than assumed.
		gross := (t.EntryPremium - t.ExitPremium) * t.Quantity
		out = append(out, Trade{
			Strategy: t.StrategyName,
			NetPnL:   t.NetPnL,
			GrossPnL: gross,
			Fees:     grossMinusNet(gross, t.NetPnL),
			ClosedAt: closedAt(t.ExitTime, t.EntryTime),
		})
	}
	return out
}

// ScalpDesk adapts the scalp desk, which reports trades over HTTP because it
// runs as a separate process. The caller supplies the already-fetched trades.
type ScalpDesk struct {
	Trades  []Trade
	Capital float64
}

func (d ScalpDesk) DeskName() string { return "scalp" }

func (d ScalpDesk) StartingCapital() float64 {
	if d.Capital > 0 {
		return d.Capital
	}
	return DefaultStartingCapital
}

func (d ScalpDesk) HuntTrades() []Trade { return d.Trades }

// grossMinusNet derives the fee from the two figures the desks already report.
// Never negative: a net above gross means the desk's fee model credited
// something, and reporting a negative fee would flatter fee drag rather than
// expose it.
func grossMinusNet(gross, net float64) float64 {
	f := gross - net
	if f < 0 {
		return 0
	}
	return f
}

// closedAt prefers the exit time and falls back to entry, so a trade with a
// missing exit timestamp still lands in the right place in the equity walk
// instead of collapsing to the zero time and corrupting the half-split.
func closedAt(exit, entry time.Time) time.Time {
	if !exit.IsZero() {
		return exit
	}
	return entry
}
