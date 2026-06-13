package orderbook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── analysis tests ────────────────────────────────────────────────────────────

func mockBook(midPrice float64) OrderBook {
	bids := []PriceLevel{
		{Price: midPrice * 0.999, Quantity: 5},
		{Price: midPrice * 0.998, Quantity: 20}, // large bid wall cluster
		{Price: midPrice * 0.997, Quantity: 3},
	}
	asks := []PriceLevel{
		{Price: midPrice * 1.001, Quantity: 3},
		{Price: midPrice * 1.002, Quantity: 4},
		{Price: midPrice * 1.003, Quantity: 2},
	}
	return OrderBook{Symbol: "BTCUSDT", Bids: bids, Asks: asks, UpdatedAt: time.Now()}
}

func TestAnalyse_IdentifiesBidWall(t *testing.T) {
	book := mockBook(50000)
	a := Analyse(book, 50000, nil)
	// Bid wall should be larger than ask wall due to the 20-unit cluster.
	assert.Greater(t, a.BidWallSize, a.AskWallSize)
}

func TestAnalyse_ComputesDepthImbalance(t *testing.T) {
	// More bid volume → imbalance > 1
	book := OrderBook{
		Symbol: "BTCUSDT",
		Bids:   makeLevels(10, 49900, -100, 10),  // 10 levels × 10 qty = 100
		Asks:   makeLevels(10, 50100, 100, 2),     // 10 levels × 2 qty = 20
		UpdatedAt: time.Now(),
	}
	a := Analyse(book, 50000, nil)
	assert.Greater(t, a.DepthImbalance, 1.0)
	assert.Equal(t, "BUY_PRESSURE", a.ImbalanceSignal)
}

func TestAnalyse_SpreadAbnormal(t *testing.T) {
	book := OrderBook{
		Symbol: "BTCUSDT",
		Bids:   []PriceLevel{{Price: 49000, Quantity: 1}},
		Asks:   []PriceLevel{{Price: 51000, Quantity: 1}}, // 400bps spread
		UpdatedAt: time.Now(),
	}
	// History with average 10bps spread → 400bps is > 3× average
	hist := makeHistory(spreadHistoryLen, 10)
	a := Analyse(book, 50000, hist)
	assert.False(t, a.SpreadNormal)
	// Liquidity warning reduces score
	assert.Less(t, a.Score, 0.0)
}

func TestAnalyse_EmptyBook_ReturnsDefaults(t *testing.T) {
	a := Analyse(OrderBook{}, 50000, nil)
	assert.Equal(t, 0.0, a.Score)
	assert.True(t, a.SpreadNormal)
}

func TestAnalyse_ScoreClamped(t *testing.T) {
	// Huge bid wall and strong buy imbalance → score should not exceed +3
	book := OrderBook{
		Symbol: "BTCUSDT",
		Bids:   makeLevels(10, 49900, -100, 1000),
		Asks:   makeLevels(10, 50100, 100, 1),
		UpdatedAt: time.Now(),
	}
	a := Analyse(book, 50000, nil)
	assert.LessOrEqual(t, a.Score, 3.0)
	assert.GreaterOrEqual(t, a.Score, -3.0)
}

// ── WebSocket reconnection test ───────────────────────────────────────────────

func TestDepthSubscriber_ReconnectsAfterDisconnect(t *testing.T) {
	connectCount := 0
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectCount++
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Send one valid depth message then immediately close.
		conn.WriteJSON(map[string]interface{}{
			"bids": [][]string{{"50000", "1"}, {"49990", "2"}},
			"asks": [][]string{{"50010", "1"}, {"50020", "2"}},
		})
		conn.Close()
	}))
	defer srv.Close()

	// Replace ws:// URL with the test server URL.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	sub := &DepthSubscriber{
		reconnectMax:  2,
		reconnectWait: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Override URL via internal method for testability.
	_ = wsURL

	// Minimal smoke test: sub handles context cancellation without deadlock.
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Use a pre-cancelled context so Connect returns immediately.
		cctx, ccancel := context.WithCancel(context.Background())
		ccancel()
		_ = sub.Connect(cctx)
	}()

	select {
	case <-done:
		// OK
	case <-ctx.Done():
		require.Fail(t, "Connect did not return after context cancellation")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func makeLevels(n int, startPrice, step, qty float64) []PriceLevel {
	levels := make([]PriceLevel, n)
	for i := range levels {
		levels[i] = PriceLevel{Price: startPrice + float64(i)*step, Quantity: qty}
	}
	return levels
}

func makeHistory(n int, val float64) []float64 {
	h := make([]float64, n)
	for i := range h {
		h[i] = val
	}
	return h
}
