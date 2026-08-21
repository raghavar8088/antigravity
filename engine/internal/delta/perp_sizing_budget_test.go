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

	// No routed symbol currently relies on volatility to become fundable.
	//
	// ETHUSD was in this set on 2026-08-20 and is not routed any more. Kept as
	// an empty map rather than deleted, because the DISTINCTION it encodes is
	// still live: a symbol whose contract fits the ceiling can be funded by a
	// tighter stop, and one whose contract exceeds it never can. The next
	// borderline symbol belongs here, not in the unconditional check below.
	volatilityGated := map[string]bool{}

	for _, st := range ScalpLiveStreams() {
		q, ok := measured[st.Symbol]
		if !ok {
			t.Errorf("%s (%s) is routed but has no measured contract price/stop here; "+
				"add them so the sizing claim can be re-run", st.Symbol, st.Strategy)
			continue
		}
		if volatilityGated[st.Symbol] {
			// The exemption is only legitimate while the contract fits the
			// ceiling. Past that, no volatility helps and the row would refuse
			// forever, which is the state this file exists to prevent.
			if q.perContract > equityUSD*maxLeverage {
				t.Errorf("%s is marked volatility-gated but its $%.2f contract exceeds the $%.2f "+
					"ceiling — it can never size, so it is not gated, it is impossible",
					st.Symbol, q.perContract, equityUSD*maxLeverage)
			}
			continue
		}
		if n := contractsFor(q); n < 1 {
			t.Errorf("%s is routed but risk sizing yields %d contracts: $%.2f of notional "+
				"against a $%.2f contract. It would be refused with ErrRiskTooSmall on every signal.",
				st.Symbol, n, riskUSD/q.stopFrac, q.perContract)
		}
	}

	// The six held back are held back because no volatility can rescue them.
	// Asserted on the CEILING rather than on today's stop, because that is the
	// permanent property their exclusion rests on.
	for _, sym := range []string{"ZECUSD", "BTCUSD", "SOLUSD"} {
		if measured[sym].perContract <= equityUSD*maxLeverage {
			t.Errorf("%s at $%.2f now fits the $%.2f ceiling and could become routable; "+
				"it is excluded on the grounds that it never can",
				sym, measured[sym].perContract, equityUSD*maxLeverage)
		}
	}

	// ETHUSD is excluded for a DIFFERENT reason and the difference must not
	// blur: it fits the ceiling and fails only today's stop width. Asserting
	// both halves keeps the two kinds of exclusion distinct, so a future reader
	// does not lump it in with the three that are permanently impossible.
	eth := measured["ETHUSD"]
	if eth.perContract > equityUSD*maxLeverage {
		t.Errorf("ETHUSD at $%.2f no longer fits the ceiling; it is documented as fundable at a "+
			"tighter stop, which would stop being true", eth.perContract)
	}
	if contractsFor(eth) >= 1 {
		t.Errorf("ETHUSD now sizes to %d contract(s) and could be routed; it is off the roster on "+
			"the grounds that it refuses continuously", contractsFor(eth))
	}

}
