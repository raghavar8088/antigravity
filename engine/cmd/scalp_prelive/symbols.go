package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Symbol universe.
//
// The desk shipped with eight hand-picked symbols. Delta India lists 220
// perpetual futures, and the eight covered ranks 1, 3, 5, 11, 17, 19, 21 and 40
// by turnover — good coverage of the liquid end, and blind to everything else.
//
// Discovery is from the venue rather than a hardcoded list, so a contract Delta
// adds or delists does not require a redeploy to appear or disappear.

// deltaPerpUniverse fetches every live perpetual future, newest liquidity first.
//
// minTurnoverUSD filters on 24h turnover. It exists because the tail is very
// long and very thin: of 220 contracts, 10 clear $10M/day, 29 clear $1M, and
// 144 sit under $100k. A fill modelled on a book that thin is not a fill —
// slippage on exit would dominate any edge the strategy claims to have, and the
// desk's maker-fill model has no way to represent that. Symbols below the floor
// are excluded rather than traded and quietly mis-measured.
func deltaPerpUniverse(ctx context.Context, minTurnoverUSD float64) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.india.delta.exchange/v2/tickers?contract_types=perpetual_futures", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("delta tickers: HTTP %d", resp.StatusCode)
	}

	var body struct {
		Result []struct {
			Symbol       string `json:"symbol"`
			ContractType string `json:"contract_type"`
			TurnoverUSD  any    `json:"turnover_usd"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	type row struct {
		sym string
		vol float64
	}
	rows := make([]row, 0, len(body.Result))
	for _, r := range body.Result {
		if r.ContractType != "perpetual_futures" || strings.TrimSpace(r.Symbol) == "" {
			continue
		}
		v := 0.0
		switch t := r.TurnoverUSD.(type) {
		case float64:
			v = t
		case string:
			v, _ = strconv.ParseFloat(t, 64)
		}
		if v < minTurnoverUSD {
			continue
		}
		rows = append(rows, row{sym: strings.ToUpper(r.Symbol), vol: v})
	}
	// Most liquid first, so a truncation keeps the tradeable end.
	sort.Slice(rows, func(i, j int) bool { return rows[i].vol > rows[j].vol })

	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.sym)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("delta returned no perpetuals above the $%.0f turnover floor", minTurnoverUSD)
	}
	return out, nil
}

// resolveSymbols turns the -symbols flag into the desk's universe.
//
// "auto" discovers from Delta. Anything else is treated as an explicit list, so
// the flag keeps working exactly as it did.
//
// Discovery failing falls back to the explicit default rather than starting with
// no symbols. A desk with an empty universe boots fine, logs nothing unusual and
// trades nothing — the silent-zero failure this codebase keeps producing.
func resolveSymbols(spec string) []string {
	spec = strings.TrimSpace(spec)
	if !strings.EqualFold(spec, "auto") {
		out := []string{}
		for _, s := range strings.Split(spec, ",") {
			if s = strings.ToUpper(strings.TrimSpace(s)); s != "" {
				out = append(out, s)
			}
		}
		return out
	}

	floor := 0.0
	if raw := strings.TrimSpace(os.Getenv("SCALP_MIN_TURNOVER_USD")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 {
			floor = v
		} else {
			log.Printf("[SCALP] SCALP_MIN_TURNOVER_USD=%q is not a number — using $0 (all perpetuals)", raw)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	syms, err := deltaPerpUniverse(ctx, floor)
	if err != nil {
		log.Printf("[SCALP] ⚠️  symbol discovery failed (%v) — falling back to the built-in list", err)
		return resolveSymbols(defaultSymbolsCSV)
	}

	if max := strings.TrimSpace(os.Getenv("SCALP_MAX_SYMBOLS")); max != "" {
		if n, err := strconv.Atoi(max); err == nil && n > 0 && n < len(syms) {
			log.Printf("[SCALP] capping universe at the %d most liquid of %d perpetuals", n, len(syms))
			syms = syms[:n]
		}
	}
	log.Printf("[SCALP] discovered %d Delta perpetual futures (turnover floor $%.0f)", len(syms), floor)
	return syms
}

// defaultSymbolsCSV is the original hand-picked eight, kept as the fallback.
const defaultSymbolsCSV = "BTCUSD,ETHUSD,SOLUSD,BNBUSD,XRPUSD,DOGEUSD,ADAUSD,AVAXUSD"
