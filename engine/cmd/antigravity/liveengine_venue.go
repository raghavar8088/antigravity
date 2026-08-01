package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"antigravity-engine/internal/delta"
)

// /api/live-engine/venue — the account as DELTA sees it.
//
// Every other surface on the Live Engine page is the engine's own read model:
// its book, its P&L, its idea of what is open. That is exactly the arrangement
// the 2026-08-01 audit found to be unfalsifiable — the bridge reported +$0.9424
// for a day the venue recorded as -$3.5405, and stats, trades and leaderboard
// all agreed with each other because all three were computed from the same
// wrong numbers.
//
// This endpoint answers a different question: not "what does the engine think",
// but "what does the exchange say". Nothing here is derived, adjusted, filtered
// by strategy, or attributed to a desk. Where this disagrees with the rest of
// the page, this is right — it is the venue holding the money.
//
// Read-only. Six private endpoints, fetched in parallel, each failing
// independently so one outage cannot blank the others.
type venuePayload struct {
	AsOf      time.Time               `json:"asOf"`
	Balances  []delta.WalletEntry     `json:"balances"`
	Positions []delta.LivePosition    `json:"positions"`
	Open      []delta.OpenOrder       `json:"openOrders"`
	History   []delta.HistoricalOrder `json:"orderHistory"`
	Fills     []delta.Fill            `json:"fills"`
	Ledger    []delta.LedgerEntry     `json:"ledger"`
	// Errors is per-section, keyed by section name. A partial failure must be
	// visible AS a failure — an empty table and a broken table look identical,
	// and the difference is whether you have no positions or no idea.
	Errors map[string]string `json:"errors,omitempty"`
}

func serveLiveEngineVenue(bridge *delta.Bridge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		client := bridge.Client()
		out := venuePayload{AsOf: time.Now().UTC(), Errors: map[string]string{}}
		if client == nil {
			out.Errors["all"] = "delta client not configured — no API keys"
			writeVenueJSON(w, out)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()

		var mu sync.Mutex
		fail := func(section string, err error) {
			mu.Lock()
			out.Errors[section] = err.Error()
			mu.Unlock()
		}

		var wg sync.WaitGroup
		wg.Add(6)
		go func() {
			defer wg.Done()
			v, err := client.GetWalletAll(ctx)
			if err != nil {
				fail("balances", err)
				return
			}
			mu.Lock()
			out.Balances = v
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			v, err := client.GetPositions(ctx)
			if err != nil {
				fail("positions", err)
				return
			}
			mu.Lock()
			out.Positions = v
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			v, err := client.GetOpenOrders(ctx)
			if err != nil {
				fail("openOrders", err)
				return
			}
			mu.Lock()
			out.Open = v
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			v, err := client.GetOrderHistory(ctx, 100)
			if err != nil {
				fail("orderHistory", err)
				return
			}
			mu.Lock()
			out.History = v
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			v, err := client.GetFills(ctx, 100)
			if err != nil {
				fail("fills", err)
				return
			}
			mu.Lock()
			out.Fills = v
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			v, err := client.GetLedger(ctx, 100)
			if err != nil {
				fail("ledger", err)
				return
			}
			mu.Lock()
			out.Ledger = v
			mu.Unlock()
		}()
		wg.Wait()

		if len(out.Errors) == 0 {
			out.Errors = nil
		}
		writeVenueJSON(w, out)
	}
}

func writeVenueJSON(w http.ResponseWriter, v venuePayload) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
