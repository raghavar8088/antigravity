// Command inception_reset re-bases the Live Engine's ROI baseline to the
// wallet's current balance.
//
// A one-shot operator action, not an API. Re-basing destroys the denominator
// every past return was measured against, so it should be a thing someone runs
// deliberately with a reason attached — not a button that can be clicked twice
// by accident on a real-money page.
//
// It goes through liveengine.ResetInception rather than writing the file
// itself. The state is a struct with reset bookkeeping in it, and a
// hand-written JSON file is free to drift from the struct that reads it: the
// fields would simply arrive as zero, which is indistinguishable from "never
// reset" — the exact ambiguity the bookkeeping exists to remove.
//
//	ENGINE_DATA_DIR=./data go run ./cmd/inception_reset -reason "roster replaced"
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"antigravity-engine/internal/delta"
	"antigravity-engine/internal/liveengine"
)

func main() {
	reason := flag.String("reason", "", "why the baseline is being re-based (recorded on disk)")
	equity := flag.Float64("equity", 0, "baseline to set; 0 reads the live Delta wallet")
	flag.Parse()

	if *reason == "" {
		fmt.Println("refusing: -reason is required, and is stored so the reset can be explained later")
		os.Exit(2)
	}

	eq := *equity
	if eq <= 0 {
		c, err := delta.NewClient()
		if err != nil {
			fmt.Println("delta client:", err)
			os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		bal, err := c.GetWallet(ctx)
		if err != nil {
			fmt.Println("wallet read failed:", err)
			os.Exit(1)
		}
		eq = bal
	}

	before, at, roiUSD, roiPct := liveengine.AccountROI(eq)
	fmt.Printf("BEFORE  baseline $%.4f (captured %s)  ROI %+.4f (%+.2f%%)\n",
		before, at.Format(time.RFC3339), roiUSD, roiPct)

	st, err := liveengine.ResetInception(eq, *reason)
	if err != nil {
		fmt.Println("reset failed:", err)
		os.Exit(1)
	}
	fmt.Printf("AFTER   baseline $%.4f  reset #%d  reason %q\n", st.EquityUSD, st.Resets, st.ResetReason)
	fmt.Println("the previous baseline is preserved beside it as .superseded-<timestamp>.json")
}
