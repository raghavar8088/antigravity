package v3

import (
	"testing"
	"time"
)

func TestOrderBook_BestBidAsk(t *testing.T) {
	book := NewOrderBook(50000, 0.20, 0.65, time.Now())
	bid := book.BestBid()
	ask := book.BestAsk()
	if bid.Price <= 0 {
		t.Fatalf("best bid must be > 0, got %f", bid.Price)
	}
	if ask.Price <= bid.Price {
		t.Fatalf("best ask (%f) must be > best bid (%f)", ask.Price, bid.Price)
	}
}

func TestOrderBook_Spread_IncreasesWithVol(t *testing.T) {
	ts := time.Now()
	low := NewOrderBook(50000, 0.10, 0.80, ts)
	high := NewOrderBook(50000, 1.50, 0.30, ts)
	spreadLow := low.SpreadPriceAbs()
	spreadHigh := high.SpreadPriceAbs()
	if spreadHigh <= spreadLow {
		t.Fatalf("high-vol spread (%f) should exceed low-vol spread (%f)", spreadHigh, spreadLow)
	}
}

func TestOrderBook_Depth_DecreasesWithLowLiquidity(t *testing.T) {
	ts := time.Now()
	liquid := NewOrderBook(50000, 0.20, 0.90, ts)
	illiquid := NewOrderBook(50000, 0.20, 0.10, ts)
	if illiquid.TotalAskDepthUSD(5) >= liquid.TotalAskDepthUSD(5) {
		t.Fatal("illiquid book should have less depth than liquid book")
	}
}

func TestOrderBook_WalkAsks_MarketBuy(t *testing.T) {
	book := NewOrderBook(50000, 0.20, 0.80, time.Now())
	totalAskDepth := book.TotalAskDepthUSD(10)
	// Buy 10% of book depth.
	buyQty := totalAskDepth / 50000 * 0.1
	result := book.WalkAsks(buyQty)
	if result.FilledQuantity <= 0 {
		t.Fatal("market buy should fill some quantity")
	}
	if result.AverageFillPrice < book.BestAsk().Price {
		t.Fatal("average fill price must be >= best ask")
	}
}

func TestOrderBook_WalkBids_MarketSell(t *testing.T) {
	book := NewOrderBook(50000, 0.20, 0.80, time.Now())
	bidDepth := book.TotalBidDepthUSD(10)
	sellQty := bidDepth / 50000 * 0.1
	result := book.WalkBids(sellQty)
	if result.FilledQuantity <= 0 {
		t.Fatal("market sell should fill some quantity")
	}
	// Allow 1bps tolerance for float64 rounding when filling entirely in level 0.
	tolerance := book.BestBid().Price * 0.0001
	if result.AverageFillPrice > book.BestBid().Price+tolerance {
		t.Fatalf("average fill price %f must be <= best bid %f (+%f tolerance)",
			result.AverageFillPrice, book.BestBid().Price, tolerance)
	}
}

func TestOrderBook_ApplyStress(t *testing.T) {
	ts := time.Now()
	book := NewOrderBook(50000, 0.20, 0.80, ts)
	normalSpread := book.SpreadPriceAbs()
	normalDepth := book.TotalAskDepthUSD(5)
	book.ApplyStress(8.0, 0.05)
	stressedSpread := book.SpreadPriceAbs()
	stressedDepth := book.TotalAskDepthUSD(5)
	if stressedSpread <= normalSpread {
		t.Fatalf("stress should widen spread: before=%f after=%f", normalSpread, stressedSpread)
	}
	if stressedDepth >= normalDepth {
		t.Fatalf("stress should reduce depth: before=%f after=%f", normalDepth, stressedDepth)
	}
}

func TestOrderBook_LiquidityScore(t *testing.T) {
	ts := time.Now()
	book := NewOrderBook(50000, 0.20, 0.90, ts)
	score := book.LiquidityScore()
	if score <= 0 || score > 1 {
		t.Fatalf("liquidity score must be in (0,1], got %f", score)
	}
}

func TestSyntheticSpread_Clamped(t *testing.T) {
	// Should never return negative or zero spread.
	for vol := 0.0; vol <= 5.0; vol += 0.1 {
		for liq := 0.01; liq <= 1.0; liq += 0.1 {
			s := syntheticSpreadBps(vol, liq)
			if s < 0.5 {
				t.Fatalf("spread below minimum: vol=%f liq=%f spread=%f", vol, liq, s)
			}
		}
	}
}

func TestMidPriceHistory_RealizedVol(t *testing.T) {
	h := NewMidPriceHistory(100)
	if vol := h.RealizedVolPct(); vol != 0.20 {
		t.Fatalf("empty history should return default 0.20, got %f", vol)
	}
	price := 50000.0
	for i := 0; i < 60; i++ {
		price *= 1.001
		h.Add(price)
	}
	vol := h.RealizedVolPct()
	if vol <= 0 {
		t.Fatal("realized vol must be > 0 after adding prices")
	}
}
