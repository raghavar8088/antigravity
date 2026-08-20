package delta

import "testing"

// A routed symbol must be fundable by RISK SIZING, not merely fit the book.
//
// These are two different limits and confusing them cost a wrong answer in
// review. The book ceiling asks whether one contract fits equity x leverage.
// Risk sizing asks something tighter: notional = risk / stopFraction, then
// contracts = int(notional / perContract) — and int() ROUNDS DOWN, so a symbol
// whose single contract costs more than the notional the strategy wants sizes
// to ZERO and is refused with ErrRiskTooSmall.
//
// ETHUSD is the worked example. At $23.08 a contract it sits comfortably under
// a $30 ceiling and is still unroutable, because a 0.553% stop asks for only
// $10.85 of notional. Every symbol the desk had traded until now cost between
// $0.17 and $9 a contract, so the ceiling was the only limit that ever bound
// and this one had never been observed.
func TestRoster_EverySymbolCanBeSizedByRisk(t *testing.T) {
	const equityUSD, riskFraction, maxLeverage = 10.0, 0.006, 3.0
	riskUSD := equityUSD * riskFraction

	// symbol -> (per-contract USD, measured vol stop fraction), 2026-08-20.
	type quote struct{ perContract, stopFrac float64 }
	measured := map[string]quote{
		"AVAXUSD": {7.08, 0.00404},
		"ETHUSD":  {23.08, 0.00553},
		"ZECUSD":  {56.45, 0.00307},
		"BTCUSD":  {72.44, 0.00446},
		"SOLUSD":  {87.07, 0.00454},
	}

	contractsFor := func(q quote) int {
		notional := riskUSD / q.stopFrac
		if ceiling := equityUSD * maxLeverage; notional > ceiling {
			notional = ceiling
		}
		return int(notional / q.perContract) // SizeContracts rounds down
	}

	for _, st := range ScalpLiveStreams() {
		q, ok := measured[st.Symbol]
		if !ok {
			t.Errorf("%s (%s) is routed but has no measured contract price/stop here; "+
				"add them so the sizing claim can be re-run", st.Symbol, st.Strategy)
			continue
		}
		if n := contractsFor(q); n < 1 {
			t.Errorf("%s is routed but risk sizing yields %d contracts: $%.2f of notional "+
				"against a $%.2f contract. It would be refused with ErrRiskTooSmall on every signal.",
				st.Symbol, n, riskUSD/q.stopFrac, q.perContract)
		}
	}

	// And the ones deliberately held back must still be unfundable, or the
	// reason they were excluded has expired and they deserve another look.
	for _, sym := range []string{"ETHUSD", "ZECUSD", "BTCUSD", "SOLUSD"} {
		if n := contractsFor(measured[sym]); n >= 1 {
			t.Errorf("%s now sizes to %d contract(s) at $%.0f equity and could be routed; "+
				"it is excluded on the grounds that it cannot be", sym, n, equityUSD)
		}
	}

	// ETHUSD specifically: fits the ceiling, fails the budget. If this ever
	// stops being true the comment in perp_roster.go explaining the difference
	// is describing something that no longer happens.
	eth := measured["ETHUSD"]
	if eth.perContract > equityUSD*maxLeverage {
		t.Errorf("ETHUSD at $%.2f no longer fits the $%.2f ceiling; it is the example of a symbol "+
			"refused by the RISK BUDGET while clearing the ceiling", eth.perContract, equityUSD*maxLeverage)
	}
}
