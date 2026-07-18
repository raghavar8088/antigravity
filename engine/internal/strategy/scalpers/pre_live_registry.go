package scalpers

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// preLiveQualified is the set of strategies loaded into the Pre-Live Trade
// Engine. Only these trade (on the pre-live paper account).
//
// ─── Whitelist realigned to the honest backtest pipeline: 2026-07-07 ──────────
// The previous 42-name list (rebuilt 2026-07-02) was built with the OLD
// backtest metrics that the 2026-07-05 honesty audit invalidated:
//   • Sharpe annualisation treated per-trade returns as 15m bars (×√35040),
//     inflating Sharpe by ~20-40× — that is why the old comments below showed
//     Sharpe 16-76 for scalpers whose HONEST Sharpe is ~1.0-2.5.
//   • Cost model used maker fees (2+2 bps) instead of the taker fees (4 bps/leg,
//     8 bps round trip) the engine actually pays on market orders.
//   • Single-window fit (2023-01 → 2024-12) with no out-of-sample confirmation,
//     so 87 near-clone *_Short variants curve-fit the same window.
// Running that list — even on paper — produced a misleading validation signal.
//
// This list now matches the LIVE trade-engine whitelist (see
// curated_registry.go tradeEngineEnabled): the ONLY two of 303 strategies that
// qualified in the train window (2021-06-26 → 2024-06-30) AND confirmed
// out-of-sample (2024-07-01 → 2026-06-26) under taker fees + honest Sharpe.
// See BACKTEST.md §5b. To validate additional candidates, run them in the main
// engine's shadow tier (tradeEngineShadow) and promote via ShadowPromoter —
// do NOT re-add single-window backtest names here.
var preLiveQualified = map[string]bool{
	"BB_Squeeze_EFI_ADX_Short": true, // train: sh 1.18 pf 1.46 n=276 | val: sh 1.00 pf 1.35 n=210
	"WMA_Bear_Cross_Short":     true, // train: sh 1.24 pf 2.39 n=59  | val: sh 1.52 pf 2.88 n=30
}

// PRE_LIVE_WHITELIST_FILE optionally replaces the built-in preLiveQualified
// whitelist with names loaded from a qualification-run JSON (the
// {"whitelist":[...]} shape written by cmd/btc_qualify_25). This is what lets a
// SECOND pre_live instance (the BTC Pre-Live Engine, Phase 3) trade its own
// qualified basket without forking this binary or touching the default
// instance, whose behavior is unchanged when the env var is unset.
var (
	whitelistOnce   sync.Once
	activeWhitelist map[string]bool
)

func effectiveWhitelist() map[string]bool {
	whitelistOnce.Do(func() {
		activeWhitelist = preLiveQualified
		path := os.Getenv("PRE_LIVE_WHITELIST_FILE")
		if path == "" {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			// Fail LOUD and hard: a missing whitelist file must never silently
			// fall back to the 2-name default — the operator explicitly asked
			// for a different basket.
			log.Fatalf("[PRE-LIVE] PRE_LIVE_WHITELIST_FILE=%s could not be read: %v", path, err)
		}
		var parsed struct {
			Whitelist []string `json:"whitelist"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil || len(parsed.Whitelist) == 0 {
			log.Fatalf("[PRE-LIVE] PRE_LIVE_WHITELIST_FILE=%s has no usable {\"whitelist\":[...]} array (err=%v)", path, err)
		}
		wl := make(map[string]bool, len(parsed.Whitelist))
		for _, name := range parsed.Whitelist {
			wl[name] = true
		}
		activeWhitelist = wl
		log.Printf("[PRE-LIVE] whitelist loaded from %s: %d strategies (replaces built-in %d)",
			path, len(wl), len(preLiveQualified))
	})
	return activeWhitelist
}

// PreLiveWhitelistSize returns the number of names in the active whitelist.
// Used at startup to detect silent strategy-name mismatches between
// the whitelist and the strategy builder functions.
func PreLiveWhitelistSize() int {
	return len(effectiveWhitelist())
}

// BuildPreLiveStrategies returns the backtested-qualified strategies for the
// Pre-Live Trade Engine. It pulls from all available strategy sources (ported +
// curated + research) and filters by the qualifying whitelist. No shadow overlay,
// no FilterWinnersOnly — the whitelist IS the quality gate.
func BuildPreLiveStrategies() []RegistryEntry {
	// Collect all candidate strategies from both sources.
	var all []RegistryEntry
	all = append(all, BuildPortedStrategies()...)
	all = append(all, BuildAllScalpers()...)
	all = append(all, buildExpansionPack()...)
	all = append(all, buildImmortalEditionPack()...)
	all = append(all, buildResearchStrategies()...)
	all = append(all, BuildDelta20Pack()...)
	all = append(all, BuildM1Pack()...)

	// Deduplicate by name (ported strategies may also appear in curated).
	seen := make(map[string]bool, len(all))
	var deduped []RegistryEntry
	for _, e := range all {
		name := e.Strategy.Name()
		if seen[name] {
			continue
		}
		seen[name] = true
		deduped = append(deduped, e)
	}

	// Filter to the qualifying whitelist (built-in, or PRE_LIVE_WHITELIST_FILE).
	wl := effectiveWhitelist()
	var result []RegistryEntry
	for _, e := range deduped {
		if wl[e.Strategy.Name()] {
			result = append(result, e)
		}
	}
	return result
}
