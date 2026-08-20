// Command wallet_check prints the Delta wallet balance.
//
// Credentials are read from the environment by delta.NewClient and are never
// printed — only the balances are. It exists because the scalp desk's equity is
// a CONFIGURED number (SCALP_LIVE_EQUITY_USD), not a reading of the venue, so
// "what the desk may risk" and "what is actually there to risk" are two
// different figures that nothing was comparing.
package main

import (
	"context"
	"fmt"
	"time"

	"antigravity-engine/internal/delta"
)

func main() {
	c, err := delta.NewClient()
	if err != nil {
		fmt.Println("client:", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	entries, err := c.GetWalletAll(ctx)
	if err != nil {
		fmt.Println("wallet read failed:", err)
		return
	}
	if len(entries) == 0 {
		fmt.Println("  no non-zero balances")
		return
	}
	for _, e := range entries {
		fmt.Printf("  %-8s balance %12.4f  available %12.4f  blocked %10.4f\n",
			e.Asset, e.Balance, e.AvailableBalance, e.BlockedBalance)
	}
}
