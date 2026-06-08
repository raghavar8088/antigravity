package execution

// Canonical Paper Desk fee model.
// NetPnL = GrossPnL - EntryFee - ExitFee

// TradeFeeBreakdown holds per-leg fees for a closed round-trip.
type TradeFeeBreakdown struct {
	EntryFee float64 `json:"entry_fee"`
	ExitFee  float64 `json:"exit_fee"`
	TotalFee float64 `json:"total_fee"`
}

// CanonicalTradeFees computes entry/exit taker fees from notional at each leg.
func CanonicalTradeFees(entryPrice, exitPrice, quantity float64) TradeFeeBreakdown {
	if entryPrice <= 0 || exitPrice <= 0 || quantity <= 0 {
		return TradeFeeBreakdown{}
	}
	entryNotional := entryPrice * quantity
	exitNotional := exitPrice * quantity
	entryFee := entryNotional * BinanceFuturesTakerFeePct
	exitFee := exitNotional * BinanceFuturesTakerFeePct
	return TradeFeeBreakdown{
		EntryFee: entryFee,
		ExitFee:  exitFee,
		TotalFee: entryFee + exitFee,
	}
}

// CanonicalNetPnL returns gross minus both fee legs.
func CanonicalNetPnL(grossPnL float64, fees TradeFeeBreakdown) float64 {
	return grossPnL - fees.EntryFee - fees.ExitFee
}
