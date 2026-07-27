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
	b.openByPaperID = make(map[string]int, len(trades))
	open := 0
	maxSeq := 0
	for i, t := range trades {
		if t.Status == "OPEN" {
			open++
			if t.PaperTradeID != "" {
				b.openByPaperID[t.PaperTradeID] = i
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
	for _, t := range b.trades {
		if t.Status == "OPEN" && t.ProductID != 0 {
			tracked[t.ProductID] = true
		}
	}

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
		b.openByPaperID = make(map[string]int, len(b.trades))
		for i, t := range b.trades {
			if t.Status == "OPEN" && t.PaperTradeID != "" {
				b.openByPaperID[t.PaperTradeID] = i
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
