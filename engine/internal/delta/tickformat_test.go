package delta

import "testing"

// Prices must print on the venue's grid, exactly.
//
// roundToTick does float arithmetic and leaves representation noise;
// FormatFloat with precision -1 prints all of it. Delta rejected
// 0.012860000000000001 as an invalid value, the position opened with no
// bracket, and the 15-second monitor closed it 1.280% adverse against a 0.626%
// stop — a 2.04x overshoot and -$4.15, caused by three extra printed digits.
func TestFormatTickPrice_PrintsOnTheGrid(t *testing.T) {
	for _, tc := range []struct {
		price, tick float64
		want        string
	}{
		// The exact case that was rejected: COOKIEUSD, 0.00001 tick.
		{0.0128565, 0.00001, "0.01286"},
		{0.012777, 0.00001, "0.01278"},
		// Delta reports ticks with trailing zeros — "0.000010000000000000".
		// Those are not precision and must not widen the output.
		{0.0128565, 0.000010000000000000, "0.01286"},
		// Coarser and finer grids.
		{6.48312, 0.0001, "6.4831"},
		{0.19846342, 0.00001, "0.19846"},
		{123.456, 0.01, "123.46"},
		{1.5, 1, "2"},
	} {
		got := formatTickPrice(tc.price, tc.tick)
		if got != tc.want {
			t.Errorf("formatTickPrice(%v, %v) = %q, want %q", tc.price, tc.tick, got, tc.want)
		}
	}
}

// No output may carry float noise. This is the property that broke, stated
// directly: whatever the input, the string must be short enough to be a real
// price on that grid.
func TestFormatTickPrice_NeverEmitsFloatNoise(t *testing.T) {
	for _, tick := range []float64{0.00001, 0.0001, 0.001, 0.01, 0.1, 1} {
		for _, p := range []float64{0.0128565, 0.19846342, 6.48312287, 64933.94442198} {
			got := formatTickPrice(p, tick)
			if len(got) > 12 {
				t.Errorf("formatTickPrice(%v, %v) = %q — that is float noise, not a price", p, tick, got)
			}
			if got == "" {
				t.Errorf("formatTickPrice(%v, %v) returned empty", p, tick)
			}
		}
	}
}

// Degenerate inputs must not produce a price. An empty string is refused by the
// caller; a wrong one opens an unprotected position.
func TestFormatTickPrice_RefusesNonPrices(t *testing.T) {
	if got := formatTickPrice(0, 0.001); got != "" {
		t.Errorf("a zero price formatted to %q", got)
	}
	if got := formatTickPrice(-1, 0.001); got != "" {
		t.Errorf("a negative price formatted to %q", got)
	}
	// An unknown tick must still bound the precision — -1 is what caused the
	// rejection in the first place.
	if got := formatTickPrice(0.0128565, 0); len(got) > 12 {
		t.Errorf("unknown tick produced %q; precision must stay bounded", got)
	}
}

func TestTickDecimals(t *testing.T) {
	for _, tc := range []struct {
		tick float64
		want int
	}{
		{0.00001, 5}, {0.000010000000000000, 5}, {0.0001, 4},
		{0.001, 3}, {0.01, 2}, {0.1, 1}, {1, 0}, {0, 8},
	} {
		if got := tickDecimals(tc.tick); got != tc.want {
			t.Errorf("tickDecimals(%v) = %d, want %d", tc.tick, got, tc.want)
		}
	}
}
