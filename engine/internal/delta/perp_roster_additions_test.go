package delta

import "testing"

// The 2026-08-10 additions must be present AND the original 31 untouched.
//
// The owner's instruction was explicit: add alongside, do not replace. Every
// previous roster change on this desk was a replacement, so an append that
// quietly drops the old book is the mistake worth guarding against.
func TestDefaultScalpLiveStreams_AdditionsDoNotDisplaceTheOriginals(t *testing.T) {
	got := map[string]bool{}
	for _, s := range defaultScalpLiveStreams {
		got[perpStreamKey(s.Strategy, s.Symbol)] = true
	}

	// 31 original + 14 (2026-08-10) + 98 + 30 (2026-08-11).
	if len(defaultScalpLiveStreams) != 173 {
		t.Fatalf("roster has %d streams, want 173", len(defaultScalpLiveStreams))
	}

	// A sample of the original book, including the stream that was open when
	// the additions were made.
	for _, orig := range []struct{ strategy, symbol string }{
		{"ANTI_M1_NR7_Expand_T20_Long", "LABUSD"},
		{"ANTI_D20_MACD_Cross", "COOKIEUSD"},
		{"ANTI_Recurrence_Quantification_Signal", "MUBARAKUSD"},
		{"ANTI_Ornstein_Uhlenbeck_Reversion", "SOLVUSD"},
	} {
		if !got[perpStreamKey(orig.strategy, orig.symbol)] {
			t.Errorf("original stream %s on %s was lost", orig.strategy, orig.symbol)
		}
	}

	for _, add := range []struct{ strategy, symbol string }{
		// 2026-08-11 additions, including the three new symbols.
		{"ANTI_D20_RSI_Reversion", "MUBARAKUSD"},
		{"ANTI_M1_VWAP_Rev_40bp_Long", "BEATUSD"},
		{"Hidden_Liquidity_Detection", "LABUSD"},
		{"M1_PinBar_W2_Short", "BLESSUSD"},
		{"ANTI_M1_InsideBar_V20_Short", "GRIFFAINUSD"},
		{"ANTI_M1_HMA21_Flip_Long", "BLESSUSD"},
		{"ANTI_Recurrence_Quantification_Signal", "TSTUSD"},
		{"ANTI_Ornstein_Uhlenbeck_Reversion", "BMTUSD"},
		{"ANTI_M1_HMA34_Flip_Short", "XANUSD"},
		{"M1_VWAP_Rev_70bp_Short", "LABUSD"},
	} {
		if !got[perpStreamKey(add.strategy, add.symbol)] {
			t.Errorf("added stream %s on %s is missing", add.strategy, add.symbol)
		}
	}
}

// A duplicated stream would double a strategy's weight on one symbol while
// looking like two independent results on the board.
func TestDefaultScalpLiveStreams_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range defaultScalpLiveStreams {
		k := perpStreamKey(s.Strategy, s.Symbol)
		if seen[k] {
			t.Errorf("duplicate stream: %s on %s", s.Strategy, s.Symbol)
		}
		seen[k] = true
	}
}

// Two additions drop the ANTI_ prefix. That is not a typo — M1_VWAP_Rev_40bp_
// Short and its 70bp sibling are the ORIGINAL strategies, not the mirrors, and
// silently "correcting" them to ANTI_ would trade the opposite signal.
func TestDefaultScalpLiveStreams_KeepsNonAntiNamesExact(t *testing.T) {
	want := map[string]bool{
		perpStreamKey("M1_VWAP_Rev_40bp_Short", "LABUSD"): false,
		perpStreamKey("M1_VWAP_Rev_70bp_Short", "LABUSD"): false,
	}
	for _, s := range defaultScalpLiveStreams {
		k := perpStreamKey(s.Strategy, s.Symbol)
		if _, ok := want[k]; ok {
			want[k] = true
		}
	}
	for k, found := range want {
		if !found {
			t.Errorf("%s missing — a non-ANTI name may have been rewritten to its mirror", k)
		}
	}
}
