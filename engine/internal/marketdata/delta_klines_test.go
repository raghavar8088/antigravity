package marketdata

import (
	"testing"
	"time"
)

// The kline feed has two ways to corrupt a strategy's view of the market, and
// both are silent: emitting the bar that is still forming, and emitting the same
// bar twice.

// A forming bar's high, low, close and volume all change under the strategy that
// received it, and the next poll delivers a different candle for the same
// period. Only closed bars may be emitted.
func TestDeltaKlines_EmitsOnlyClosedBars(t *testing.T) {
	f := NewDeltaKlineFeed("BTCUSD", []string{"15m"})
	now := time.Now().UTC()

	// One bar closed 20 minutes ago, one still forming right now.
	closed := now.Add(-20 * time.Minute).Truncate(15 * time.Minute)
	forming := now.Truncate(15 * time.Minute)

	var got []Candle
	emit := func(res string, c Candle) { got = append(got, c) }

	f.emitFrom([]HistoricalCandle{
		{OpenTime: closed, Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 1000},
		{OpenTime: forming, Open: 2, High: 3, Low: 1.5, Close: 2.5, Volume: 500},
	}, "15m", now, emit)

	if len(got) != 1 {
		t.Fatalf("emitted %d candles, want 1 — the forming bar must be withheld", len(got))
	}
	if !got[0].OpenTime.Equal(closed) {
		t.Errorf("emitted the bar opening %s, want the closed one at %s", got[0].OpenTime, closed)
	}
}

// Re-emitting a bar double-counts its volume and re-fires any close-of-bar
// logic downstream. Polling means the same closed bar is seen on every tick
// until it ages out of the window, so the guard is load-bearing.
func TestDeltaKlines_DoesNotEmitTheSameBarTwice(t *testing.T) {
	f := NewDeltaKlineFeed("BTCUSD", []string{"1h"})
	now := time.Now().UTC()
	bar := now.Add(-2 * time.Hour).Truncate(time.Hour)

	count := 0
	emit := func(res string, c Candle) { count++ }
	raw := []HistoricalCandle{{OpenTime: bar, Open: 1, High: 2, Low: 1, Close: 2, Volume: 100}}

	for i := 0; i < 5; i++ {
		f.emitFrom(raw, "1h", now, emit)
	}
	if count != 1 {
		t.Fatalf("the same closed bar was emitted %d times; volume would be counted %dx", count, count)
	}
}

// Delta reports candle volume in CONTRACTS. Binance klines report it in BTC.
// Unscaled, the venue switch multiplies every volume reading by a thousand at
// the moment the feed changes.
func TestDeltaKlines_ScalesVolumeFromContractsToBTC(t *testing.T) {
	f := NewDeltaKlineFeed("BTCUSD", []string{"1h"})
	now := time.Now().UTC()
	bar := now.Add(-2 * time.Hour).Truncate(time.Hour)

	var got Candle
	f.emitFrom([]HistoricalCandle{{OpenTime: bar, Open: 1, High: 2, Low: 1, Close: 2, Volume: 222107}},
		"1h", now, func(res string, c Candle) { got = c })

	want := 222107 * DeltaContractValueBTC // 222.107 BTC
	if got.Volume != want {
		t.Errorf("volume %v, want %v — a raw contract count would read as 222,107 BTC", got.Volume, want)
	}
}

// The close time must be derived from the resolution. Delta stamps only the open
// time, and a zero-width candle defeats any downstream bar-boundary logic.
func TestDeltaKlines_DerivesCloseTimeFromResolution(t *testing.T) {
	f := NewDeltaKlineFeed("BTCUSD", []string{"15m"})
	now := time.Now().UTC()
	bar := now.Add(-1 * time.Hour).Truncate(15 * time.Minute)

	var got Candle
	f.emitFrom([]HistoricalCandle{{OpenTime: bar, Close: 2, Volume: 1}}, "15m", now,
		func(res string, c Candle) { got = c })

	if d := got.CloseTime.Sub(got.OpenTime); d != 15*time.Minute {
		t.Errorf("candle width %s, want 15m", d)
	}
}

// A caller configured with another venue's notation must still work, since the
// deployed desks set PRE_LIVE_FEED_SYMBOL=BTC-USD.
func TestDeltaKlines_TranslatesSymbolNotation(t *testing.T) {
	if got := NewDeltaKlineFeed("BTC-USD", []string{"1h"}).symbol; got != "BTCUSD" {
		t.Errorf("symbol %q, want BTCUSD", got)
	}
	if got := NewDeltaKlineFeed("BTCUSDT", []string{"1h"}).symbol; got != "BTCUSD" {
		t.Errorf("symbol %q, want BTCUSD", got)
	}
}

func TestResolutionDuration_CoversTheFeedsResolutions(t *testing.T) {
	for res, want := range map[string]time.Duration{
		"1m": time.Minute, "5m": 5 * time.Minute, "15m": 15 * time.Minute,
		"1h": time.Hour, "4h": 4 * time.Hour, "1d": 24 * time.Hour,
	} {
		if got := resolutionDuration(res); got != want {
			t.Errorf("resolutionDuration(%q) = %s, want %s", res, got, want)
		}
	}
}
