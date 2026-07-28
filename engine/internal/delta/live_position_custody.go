package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Custody of real option positions.
//
// A position this application opened stays this application's responsibility
// until it is closed by SL/TP/expiry. Two failure modes previously broke that
// promise, orphaning real money:
//
//  1. live trades were in-memory only, so a restart forgot every position the
//     engine had opened — nothing then managed it to SL/TP; and
//  2. an untracked position on the exchange was never adopted, so it sat
//     unmanaged forever and permanently mismatched reconciliation.
//
// This file makes custody durable: trades persist across restarts, and any real
// option position the exchange reports that the engine is not already tracking
// is adopted so the monitor manages it to SL/TP.

const liveTradesFileName = "delta_live_trades.json"

// Exit thresholds the position monitor enforces on live long-option positions,
// as a fraction of the entry premium. Exported so the UI shows the same numbers
// the engine actually acts on — one source of truth, no drift between the
// displayed TP/SL and the levels that trigger a close.
const (
	LiveTakeProfitPct = 0.80 // close at +80% of premium paid
	LiveStopLossPct   = 0.50 // close at -50% of premium paid
)

// MinTimeToExpiryForNewEntry blocks live entries that are already too close to
// expiry to work. Positions opened inside this window could not reach the +80%
// target before the monitor force-closed them at near_expiry_30min, so the trade
// was structurally a theta donation: the first three such closes went 0-for-3.
// The paper engine still takes these signals; only the live mirror declines.
const MinTimeToExpiryForNewEntry = 2 * time.Hour

// strategyProfitCapExits are paper-strategy exit reasons that close a winner
// early. They are measured on the synthetic paper chain and fire far below the
// live +80% target, which is why take_profit_80pct had never once triggered in
// the first 20 live trades while stop_loss_50pct fired seven times: the upside
// was clipped at roughly +15% while the downside ran the full -50%. That
// asymmetry demanded an ~82% win rate to break even. Suppressing these hands the
// upside back to the custody monitor.
//
// Loss-cutting exits (SL, STRIKE_PRESSURE) are deliberately NOT in this set. A
// strategy stop that exits at ~-12% is strictly better than riding to the -50%
// custody stop, so it stays as the first line of defence with -50% as backstop.
// LATE_EXIT is ambiguous — it fires both on theta bleed (protective) and on late
// profit (capping) — and is kept, because holding a long option deep into its
// life is the more expensive mistake.
var strategyProfitCapExits = map[string]bool{
	"strategy_TP":          true,
	"strategy_TRAIL_STOP":  true,
	"strategy_PROFIT_LOCK": true,
}

// IsStrategyProfitCapExit reports whether a close reason is a paper-strategy
// profit-taking exit that the live custody layer should ignore, leaving the
// position to run to its own +80% take-profit or -50% stop.
func IsStrategyProfitCapExit(reason string) bool {
	return strategyProfitCapExits[reason]
}

// PositionExit describes where a long option position exits and what that is
// worth in USD, given the entry premium the monitor measures against.
type PositionExit struct {
	TakeProfitPrice float64 // premium level (USD/BTC) that triggers the TP close
	StopLossPrice   float64
	TakeProfitUSD   float64 // realised P&L in USD if TP is touched
	StopLossUSD     float64 // negative — realised P&L in USD if SL is touched
}

// ExitLevelsFor computes TP/SL levels and their USD outcomes for a long option.
// entryPremium is quoted in USD per BTC; contracts are 0.001 BTC each.
func ExitLevelsFor(entryPremium float64, contracts int) PositionExit {
	if entryPremium <= 0 || contracts == 0 {
		return PositionExit{}
	}
	btc := float64(contracts) * OptionContractSizeBTC
	tp := entryPremium * (1 + LiveTakeProfitPct)
	sl := entryPremium * (1 - LiveStopLossPct)
	return PositionExit{
		TakeProfitPrice: tp,
		StopLossPrice:   sl,
		TakeProfitUSD:   (tp - entryPremium) * btc,
		StopLossUSD:     (sl - entryPremium) * btc,
	}
}

// UnrealizedUSD is the mark-to-market P&L of a long option in USD. Delta's
// margined endpoint reports unrealised_pnl as 0 for these option positions, so
// the engine computes it from mark vs entry rather than showing a false zero.
func UnrealizedUSD(entryPremium, markPremium float64, contracts int) float64 {
	if entryPremium <= 0 || contracts == 0 {
		return 0
	}
	return (markPremium - entryPremium) * float64(contracts) * OptionContractSizeBTC
}

// liveTradesPath returns the durable path for live trade custody. ENGINE_DATA_DIR
// is a mounted volume in production, so this survives redeploys.
func liveTradesPath() string {
	dir := strings.TrimSpace(os.Getenv("ENGINE_DATA_DIR"))
	if dir == "" {
		dir = "./data"
	}
	return filepath.Join(dir, liveTradesFileName)
}

// persistTradesLocked writes live trades to disk. Caller holds b.mu.
func (b *Bridge) persistTradesLocked() {
	path := liveTradesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("[DELTA BRIDGE] custody: mkdir failed: %v", err)
		return
	}
	// Persist only trades that still need custody or recent history.
	data, err := json.Marshal(b.trades)
	if err != nil {
		log.Printf("[DELTA BRIDGE] custody: marshal failed: %v", err)
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		log.Printf("[DELTA BRIDGE] custody: write failed: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("[DELTA BRIDGE] custody: rename failed: %v", err)
	}
}

// PersistTrades saves live trade custody to disk.
func (b *Bridge) PersistTrades() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.persistTradesLocked()
}

// RestoreTrades reloads live trade custody from disk. Call once at startup,
// before the monitor starts, so positions opened before a restart keep being
// managed to SL/TP instead of being orphaned.
func (b *Bridge) RestoreTrades() {
	path := liveTradesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[DELTA BRIDGE] custody: read failed: %v", err)
		}
		return
	}
	var trades []LiveTrade
	if err := json.Unmarshal(data, &trades); err != nil {
		log.Printf("[DELTA BRIDGE] custody: unmarshal failed: %v", err)
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.trades = trades
	b.openByPaperID = make(map[string]string, len(trades))
	open := 0
	maxSeq := 0
	for _, t := range trades {
		if t.Status == "OPEN" {
			open++
			if t.PaperTradeID != "" {
				b.openByPaperID[t.PaperTradeID] = t.ID
			}
		}
		var n int
		if _, err := fmt.Sscanf(t.ID, "DLT-%d", &n); err == nil && n > maxSeq {
			maxSeq = n
		}
	}
	b.seq = maxSeq
	log.Printf("[DELTA BRIDGE] custody: restored %d live trade(s), %d still OPEN — SL/TP management resumes", len(trades), open)
}

// AdoptUntrackedPositions reconciles exchange truth into custody: any real BTC/ETH
// option position the exchange reports that the engine is not already tracking as
// OPEN is adopted as a managed trade, so the monitor closes it on SL/TP/expiry.
//
// This is deliberately conservative — it only adopts option products (the Live
// Engine's instrument), never perps or spot.
func (b *Bridge) AdoptUntrackedPositions(ctx context.Context) (adopted int, err error) {
	b.mu.RLock()
	client := b.client
	b.mu.RUnlock()
	if client == nil {
		return 0, fmt.Errorf("delta client not configured")
	}

	positions, err := client.GetPositions(ctx)
	if err != nil {
		return 0, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	tracked := make(map[int]bool)
	// knownProduct = a product this engine has traded at some point (any status).
	// Adoption is limited to these: the owner may hold their OWN manual option
	// positions on the same account, and the engine must never take those over
	// and close them on its SL/TP. Set LIVE_ADOPT_UNKNOWN=true to also adopt
	// positions the engine has no record of (e.g. after losing custody state).
	knownProduct := make(map[int]bool)
	for _, t := range b.trades {
		if t.ProductID != 0 {
			knownProduct[t.ProductID] = true
		}
		if t.Status == "OPEN" && t.ProductID != 0 {
			tracked[t.ProductID] = true
		}
	}
	adoptUnknown := strings.EqualFold(strings.TrimSpace(os.Getenv("LIVE_ADOPT_UNKNOWN")), "true")

	now := time.Now().UTC()
	for _, p := range positions {
		if p.Size == 0 || p.ProductID == 0 {
			continue
		}
		if !IsOptionSymbol(p.Symbol) {
			continue // only the Live Engine's instrument
		}
		if tracked[p.ProductID] {
			continue
		}
		if !knownProduct[p.ProductID] && !adoptUnknown {
			log.Printf("[DELTA BRIDGE] custody: NOT adopting %s (product=%d) — no record this engine opened it; leaving it alone (set LIVE_ADOPT_UNKNOWN=true to override)",
				p.Symbol, p.ProductID)
			continue
		}
		b.seq++
		id := fmt.Sprintf("DLT-%04d", b.seq)
		entry := p.EntryPrice
		if entry <= 0 {
			entry = p.MarkPrice
		}
		contracts := int(p.Size)
		if contracts < 0 {
			contracts = -contracts
		}
		b.trades = append([]LiveTrade{{
			ID:           id,
			PaperTradeID: "adopted-" + id,
			StrategyName: "ADOPTED_ORPHAN",
			OptionType:   optionTypeFromSymbol(p.Symbol),
			DeltaSymbol:  p.Symbol,
			ProductID:    p.ProductID,
			Contracts:    contracts,
			Side:         "buy",
			FillPrice:    entry,
			PremiumUSD:   entry * float64(contracts) * OptionContractSizeBTC,
			Status:       "OPEN",
			OpenedAt:     now,
		}}, b.trades...)
		adopted++
		log.Printf("[DELTA BRIDGE] custody: ADOPTED untracked live position %s (product=%d size=%d entry=%.4f) — now managed to SL/TP",
			p.Symbol, p.ProductID, contracts, entry)
	}

	if adopted > 0 {
		// Rebuild the open index after prepending.
		b.openByPaperID = make(map[string]string, len(b.trades))
		for _, t := range b.trades {
			if t.Status == "OPEN" && t.PaperTradeID != "" {
				b.openByPaperID[t.PaperTradeID] = t.ID
			}
		}
		b.persistTradesLocked()
	}
	return adopted, nil
}

// IsOptionSymbol reports whether a Delta symbol is a BTC/ETH option
// (e.g. C-BTC-64800-290726 / P-BTC-...).
func IsOptionSymbol(symbol string) bool {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	return strings.HasPrefix(s, "C-BTC-") || strings.HasPrefix(s, "P-BTC-") ||
		strings.HasPrefix(s, "C-ETH-") || strings.HasPrefix(s, "P-ETH-")
}

func optionTypeFromSymbol(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if strings.HasPrefix(s, "P-") {
		return "PUT"
	}
	return "CALL"
}
