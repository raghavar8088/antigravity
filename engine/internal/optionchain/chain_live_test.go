package optionchain

import (
	"context"
	"os"
	"testing"
	"time"

	"antigravity-engine/internal/delta"
)

// Live check against the real Delta chain. Skipped unless OPTIONCHAIN_LIVE=1.
//
//	OPTIONCHAIN_LIVE=1 go test ./internal/optionchain/ -run Live -v
func TestLive_RealDeltaChain(t *testing.T) {
	if os.Getenv("OPTIONCHAIN_LIVE") != "1" {
		t.Skip("set OPTIONCHAIN_LIVE=1 to run against the live chain")
	}
	client, err := delta.NewClient()
	if err != nil {
		t.Skipf("delta client not configured: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c := New(client, "BTC", time.Minute, 5*time.Minute)
	c.Refresh(ctx)

	contracts, quoted, takenAt, lastErr := c.Stats()
	t.Logf("chain: %d contracts, %d quoted, taken %s, err=%q",
		contracts, quoted, takenAt.Format(time.RFC3339), lastErr)
	if contracts == 0 {
		t.Fatalf("no contracts returned: %s", lastErr)
	}
	if quoted == 0 {
		t.Fatalf("no marks returned: %s", lastErr)
	}

	// Resolve an at-the-money-ish call on the nearest expiry.
	spot := 64000.0
	q, err := c.Resolve("CALL", spot, time.Now().UTC().Add(24*time.Hour), DefaultTolerance)
	if err != nil {
		t.Fatalf("resolve ATM call: %v", err)
	}
	t.Logf("resolved %s strike=%.0f expiry=%s mark=%.2f/BTC bid=%.2f ask=%.2f spread=%.2f%% drift=%.3f%%",
		q.Symbol, q.Strike, q.Expiry.Format("2006-01-02"), q.MarkPerBTC, q.Bid, q.Ask,
		q.SpreadPct()*100, q.StrikeDriftPct*100)

	if q.MarkPerBTC <= 0 {
		t.Error("resolved contract has no mark")
	}
	perContract := q.MarkPerBTC * delta.OptionContractSizeBTC
	t.Logf("premium per contract = $%.4f (contract = %.3f BTC)", perContract, delta.OptionContractSizeBTC)
}
