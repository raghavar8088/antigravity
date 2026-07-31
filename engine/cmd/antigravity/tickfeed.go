package main

import (
	"context"
	"log"
	"os"
	"strings"

	"antigravity-engine/internal/marketdata"
)

// Which venue the live tick stream comes from.
//
// This engine executes on Delta Exchange: the Live Engine buys Delta options,
// the options desks price against the Delta chain, and the scalp hunt desk
// already reads Delta candles. The tick feed was the last piece still pointed
// somewhere else — Coinbase spot BTC-USD — so 600 strategies were scored on one
// venue's trades while the orders went to another's book.
//
// That is not a small mismatch. Coinbase BTC-USD is spot; Delta BTCUSD is a
// perpetual future with its own basis, funding, liquidity and microstructure.
// The two track each other closely enough that the substitution never looked
// broken, and differ enough that a threshold fitted on one is not the same
// threshold on the other.
//
// The venue is overridable so the change can be compared rather than taken on
// faith, and it is always logged, because a desk quietly running on the wrong
// book looks exactly like one running correctly.

// liveTickVenue reads MARKET_DATA_VENUE. Delta is the default.
func liveTickVenue() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("MARKET_DATA_VENUE")))
	if v == "" {
		return "delta"
	}
	return v
}

// liveTickSymbol is the instrument to stream, in the chosen venue's own
// notation. Delta calls the BTC perp BTCUSD; Coinbase calls spot BTC-USD.
func liveTickSymbol(venue string) string {
	if s := strings.TrimSpace(os.Getenv("MARKET_DATA_SYMBOL")); s != "" {
		return s
	}
	if venue == "coinbase" {
		return "BTC-USD"
	}
	return "BTCUSD"
}

// newLiveTickFeed builds and connects the live trade stream.
//
// It returns the MarketDataClient interface rather than a concrete type so the
// orchestrator, the execution watchdog and the shutdown path are all indifferent
// to which venue is behind it — the swap is a one-line change here, not a change
// spread through the engine.
func newLiveTickFeed(ctx context.Context) marketdata.MarketDataClient {
	venue := liveTickVenue()
	symbol := liveTickSymbol(venue)

	switch venue {
	case "coinbase":
		// Retained for A/B comparison against the historical record, which was
		// entirely earned on this feed.
		log.Printf("[MARKET DATA] ⚠️  tick feed pinned to COINBASE %s — this engine EXECUTES on Delta, so strategies are being scored on a book they do not trade", symbol)
		c := marketdata.NewCoinbaseClient()
		go func() {
			if err := c.Connect(ctx, []string{symbol}); err != nil {
				log.Fatalf("Fatal error connecting to Coinbase: %v", err)
			}
		}()
		return c

	default:
		if venue != "delta" {
			// An unrecognised value must not silently pick a venue. Defaulting
			// to the one the engine actually trades is the safe reading of a
			// typo; falling through to something else is not.
			log.Printf("[MARKET DATA] MARKET_DATA_VENUE=%q is not recognised — using delta", venue)
		}
		log.Printf("[MARKET DATA] tick feed: DELTA %s (the venue this engine executes on)", symbol)
		c := marketdata.NewDeltaTickClient()
		go func() {
			// Connect only validates arguments and starts the reconnect loop; a
			// venue outage is handled there with backoff rather than by killing
			// the process. Fatal here would mean a transient socket failure at
			// boot takes down an engine that may be holding positions.
			if err := c.Connect(ctx, []string{symbol}); err != nil {
				log.Printf("[MARKET DATA] FATAL: Delta tick feed could not start: %v", err)
			}
		}()
		return c
	}
}
