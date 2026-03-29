package execution

import (
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
