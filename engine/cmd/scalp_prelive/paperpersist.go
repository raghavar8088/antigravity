package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Paper book persistence.
//
// The books were in-memory only, so every deploy wiped them. A desk whose whole
// purpose is accumulating evidence toward a promotion decision cannot lose that
// evidence each time the binary is rebuilt — and it lost it silently, showing a
// fresh $100 and one trade as though that were the whole history.
//
// Same write-then-rename discipline the perp bridge uses: a crash mid-write
// leaves the previous good file rather than a truncated one.

type paperBookSnapshot struct {
	Account  string                   `json:"account"`
	Equity   float64                  `json:"equity"`
	Accounts map[string]*paperAccount `json:"accounts"`
	Open     map[string]*paperPos     `json:"open"`
	Closed   []paperTrade             `json:"closed"`
	Started  time.Time                `json:"started"`
}

func paperPersistPath(dir string) string { return filepath.Join(dir, "live_paper.json") }

// savePaperBooks writes every book atomically.
func savePaperBooks(dir string) error {
	out := make([]paperBookSnapshot, 0, len(livePaperBooks))
	for id, d := range livePaperBooks {
		d.mu.Lock()
		snap := paperBookSnapshot{
			Account: id, Equity: d.equity, Started: d.started,
			Accounts: map[string]*paperAccount{},
			Open:     map[string]*paperPos{},
			Closed:   append([]paperTrade(nil), d.closed...),
		}
		for k, v := range d.accounts {
			c := *v
			snap.Accounts[k] = &c
		}
		for k, v := range d.open {
			c := *v
			snap.Open[k] = &c
		}
		d.mu.Unlock()
		out = append(out, snap)
	}

	blob, err := json.Marshal(out)
	if err != nil {
		return err
	}
	tmp := paperPersistPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, paperPersistPath(dir))
}

// loadPaperBooks restores what was saved, keeping the CURRENT watch list.
//
// Accounts are re-seeded from configuration first and the saved figures merged
// onto them, so a stream added since the last save appears at zero and one
// removed disappears. Restoring the saved account map wholesale would resurrect
// streams that are no longer watched, and their old P&L would keep counting
// toward a balance nobody is trading.
func loadPaperBooks(dir string) {
	blob, err := os.ReadFile(paperPersistPath(dir))
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[LIVE PAPER] could not read saved books: %v — starting fresh", err)
		}
		return
	}
	var saved []paperBookSnapshot
	if err := json.Unmarshal(blob, &saved); err != nil {
		log.Printf("[LIVE PAPER] saved books are unreadable (%v) — starting fresh", err)
		return
	}

	for _, s := range saved {
		d := livePaperBooks[s.Account]
		if d == nil {
			// An account that no longer exists. Dropped rather than recreated:
			// its streams are not being watched, so its balance is not a
			// position anyone holds.
			log.Printf("[LIVE PAPER] saved account %q is no longer configured — dropped", s.Account)
			continue
		}
		d.mu.Lock()
		d.equity = s.Equity
		d.started = s.Started
		d.closed = s.Closed
		for k, v := range s.Open {
			if _, watched := d.accounts[k]; watched {
				d.open[k] = v
			}
		}
		for k, v := range s.Accounts {
			cur, watched := d.accounts[k]
			if !watched {
				continue
			}
			cur.Trades, cur.Wins = v.Trades, v.Wins
			cur.GrossUSD, cur.FeesUSD, cur.NetUSD = v.GrossUSD, v.FeesUSD, v.NetUSD
			cur.ShareOfEquityPct = v.ShareOfEquityPct
		}
		n, open := len(d.closed), len(d.open)
		eq := d.equity
		d.mu.Unlock()
		log.Printf("[LIVE PAPER %s] restored: $%.2f equity, %d closed trades, %d open", s.Account, eq, n, open)
	}
}

// persistPaperBooks saves on a timer and once more on shutdown.
//
// Every 10s rather than on every close: a book can close several trades in one
// bar, and rewriting per trade turns a bounded cost into one that scales with
// activity. The interval is the exposure window for an UNGRACEFUL kill only —
// a normal stop or restart saves via the SIGTERM path and loses nothing.
func persistPaperBooks(dir string, every time.Duration) {
	if every <= 0 {
		every = 10 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for range t.C {
		if err := savePaperBooks(dir); err != nil {
			log.Printf("[LIVE PAPER] save failed: %v", err)
		}
	}
}
