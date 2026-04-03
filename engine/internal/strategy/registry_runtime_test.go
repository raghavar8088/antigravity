package strategy

import "testing"

func TestNormalizeCategoryForIntradayFamilies(t *testing.T) {
	tests := []struct {
		name     string
		category string
		strategy string
		want     string
	}{
		{name: "ema intraday", category: "Intraday", strategy: "ID_EMA20_50_5m", want: "Trend Elite"},
		{name: "macd intraday", category: "Intraday", strategy: "ID_MACD_Cross12_26_9_5m", want: "Momentum Elite"},
		{name: "vwap deviation intraday", category: "Intraday", strategy: "ID_VWAP_Dev0p5_5m", want: "Mean Rev Elite"},
		{name: "breakout intraday", category: "Intraday", strategy: "ID_BB_Break20_2_5m", want: "Breakout Elite"},
		{name: "width intraday", category: "Intraday", strategy: "ID_BB_Width20_2_5m", want: "Volatility Elite"},
		{name: "plain category", category: "Trend", strategy: "Anything", want: "Trend"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeCategory(tc.category, tc.strategy)
			if got != tc.want {
				t.Fatalf("NormalizeCategory(%q, %q) = %q, want %q", tc.category, tc.strategy, got, tc.want)
			}
		})
	}
}

func TestGroupByTimeframeIncludes15mBucket(t *testing.T) {
	entries := []RegistryEntry{
		{Strategy: NewTestScalper(), Category: "Test", Timeframe: "tick"},
		{Strategy: NewEMACrossScalper(8, 21), Category: "Trend", Timeframe: "1m"},
		{Strategy: NewDonchianScalper(20), Category: "Breakout", Timeframe: "5m"},
		{Strategy: NewID_EMA10_30_15m(), Category: "Intraday", Timeframe: "15m"},
		{Strategy: NewPivotScalper(60), Category: "Breakout", Timeframe: "1h"},
	}

	groups := GroupByTimeframe(entries)
	if len(groups.Tick) != 1 || len(groups.M1) != 1 || len(groups.M5) != 1 || len(groups.M15) != 1 || len(groups.H1) != 1 {
		t.Fatalf("unexpected group sizes: tick=%d 1m=%d 5m=%d 15m=%d 1h=%d",
			len(groups.Tick), len(groups.M1), len(groups.M5), len(groups.M15), len(groups.H1))
	}
}
