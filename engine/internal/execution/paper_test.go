package execution

import (
	"math"
	"testing"

	"antigravity-engine/internal/strategy"
)

func TestPlaceMarketOrderReturnsErrorOnInsufficientFunds(t *testing.T) {
	client := NewPaperClient(100)
	client.UpdateMarketState(10000)

	err := client.PlaceMarketOrder(strategy.Signal{
		Symbol:     "BTC-USD",
		Action:     strategy.ActionBuy,
		TargetSize: 1,
	})
	if err == nil {
		t.Fatal("expected insufficient funds error")
	}
}

func TestPaperClientTracksShortExposureAndEquity(t *testing.T) {
	client := NewPaperClient(100000)
	client.UpdateMarketState(100)

	err := client.PlaceMarketOrder(strategy.Signal{
		Symbol:     "BTC-USD",
		Action:     strategy.ActionSell,
		TargetSize: 1,
	})
	if err != nil {
		t.Fatalf("unexpected short order error: %v", err)
	}

	if got := client.GetPosition("BTC-USD"); got != -1 {
		t.Fatalf("expected signed short position -1, got %.4f", got)
	}

	expectedEquity := client.GetBalanceUSD() - 100
	if math.Abs(client.GetEquityUSD()-expectedEquity) > 1e-9 {
		t.Fatalf("expected equity %.5f, got %.5f", expectedEquity, client.GetEquityUSD())
	}

	client.SettlePosition(strategy.ActionSell, 1, 95)

	if got := client.GetPosition("BTCUSDT"); got != 0 {
		t.Fatalf("expected flat position after short cover, got %.4f", got)
	}
	if client.GetEquityUSD() <= 100000 {
		t.Fatalf("expected profitable short to increase equity, got %.5f", client.GetEquityUSD())
	}
}
