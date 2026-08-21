package liveengine

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// inceptionFile records the wallet balance ROI is measured from.
const inceptionFile = "live_engine_inception.json"

type inceptionState struct {
	EquityUSD float64   `json:"equityUsd"`
	At        time.Time `json:"at"`

	// Reset bookkeeping. Present only when the baseline has been deliberately
	// re-based, and reported to the UI so a 0.00% can never be mistaken for a
	// lifetime figure.
	//
	// The whole value of a baseline is that it does not move, so a reset has to
	// leave evidence. Without these fields a re-based account is
	// indistinguishable from one that has never traded, which is exactly the
	// flattering reading AccountROI's comment warns about.
	ResetFrom   float64   `json:"resetFrom,omitempty"`
	ResetAt     time.Time `json:"resetAt,omitempty"`
	Resets      int       `json:"resets,omitempty"`
	ResetReason string    `json:"resetReason,omitempty"`
}

var (
	inceptionMu   sync.Mutex
	inceptionOnce inceptionState
	inceptionRead bool
)

func inceptionPath() string {
	dir := strings.TrimSpace(os.Getenv("ENGINE_DATA_DIR"))
	if dir == "" {
		dir = "./data"
	}
	return filepath.Join(dir, inceptionFile)
}

// AccountROI returns the baseline and the return measured against it.
//
// The baseline is captured ONCE, the first time a real wallet balance is seen,
// and then never moves. A baseline that follows the balance would report 0%
// forever, which is the most flattering possible lie for a losing account.
//
// A zero or negative equity is not recorded: it would either divide by zero or
// fix the baseline at a moment the wallet could not be read, permanently
// misstating every future return.
func AccountROI(equityUSD float64) (baseline float64, at time.Time, roiUSD, roiPct float64) {
	if equityUSD <= 0 {
		return 0, time.Time{}, 0, 0
	}

	inceptionMu.Lock()
	defer inceptionMu.Unlock()

	if !inceptionRead {
		if raw, err := os.ReadFile(inceptionPath()); err == nil {
			_ = json.Unmarshal(raw, &inceptionOnce)
		}
		inceptionRead = true
	}

	if inceptionOnce.EquityUSD <= 0 {
		inceptionOnce = inceptionState{EquityUSD: equityUSD, At: time.Now().UTC()}
		if data, err := json.Marshal(inceptionOnce); err == nil {
			dir := filepath.Dir(inceptionPath())
			if err := os.MkdirAll(dir, 0o755); err == nil {
				tmp := inceptionPath() + ".tmp"
				if os.WriteFile(tmp, data, 0o644) == nil {
					_ = os.Rename(tmp, inceptionPath())
				}
			}
		}
		log.Printf("[LIVE ENGINE] ROI baseline captured: $%.4f — every return is measured from here", equityUSD)
	}

	baseline = inceptionOnce.EquityUSD
	at = inceptionOnce.At
	roiUSD = equityUSD - baseline
	roiPct = roiUSD / baseline * 100
	return baseline, at, roiUSD, roiPct
}

// InceptionReset reports whether the baseline has been re-based, and from what.
//
// Exposed so the UI can label the figure "since reset" rather than "since
// inception". A re-based account reads 0.00% at the moment of the reset, and
// that number is only honest if it says what it is measured from.
func InceptionReset() (resets int, at time.Time, from float64, reason string) {
	inceptionMu.Lock()
	defer inceptionMu.Unlock()
	return inceptionOnce.Resets, inceptionOnce.ResetAt, inceptionOnce.ResetFrom, inceptionOnce.ResetReason
}

// ResetInception re-bases the ROI baseline to the current equity.
//
// Deliberately a separate, named operation rather than a flag on AccountROI.
// AccountROI captures a baseline ONCE and must keep doing so — a baseline that
// followed the balance would report 0% forever, which is the most flattering
// possible lie for a losing account. Re-basing is a decision someone makes,
// with a reason, and it is recorded as such.
//
// The previous baseline is preserved on disk as a superseded file rather than
// overwritten. The record of what an account actually did is the one thing a
// reset must not destroy: it is why "-5.02% since 15 Aug" can still be
// reconstructed after the tile reads 0.00%.
func ResetInception(equityUSD float64, reason string) (inceptionState, error) {
	if equityUSD <= 0 {
		return inceptionState{}, fmt.Errorf("liveengine: refusing to re-base ROI to a non-positive equity (%.4f)", equityUSD)
	}

	inceptionMu.Lock()
	defer inceptionMu.Unlock()

	if !inceptionRead {
		if raw, err := os.ReadFile(inceptionPath()); err == nil {
			_ = json.Unmarshal(raw, &inceptionOnce)
		}
		inceptionRead = true
	}
	prev := inceptionOnce

	// Preserve the old baseline beside the new one, timestamped.
	if prev.EquityUSD > 0 {
		if raw, err := json.Marshal(prev); err == nil {
			stamp := time.Now().UTC().Format("20060102T150405Z")
			_ = os.WriteFile(inceptionPath()+".superseded-"+stamp+".json", raw, 0o644)
		}
	}

	next := inceptionState{
		EquityUSD:   equityUSD,
		At:          time.Now().UTC(),
		ResetFrom:   prev.EquityUSD,
		ResetAt:     time.Now().UTC(),
		Resets:      prev.Resets + 1,
		ResetReason: strings.TrimSpace(reason),
	}
	data, err := json.Marshal(next)
	if err != nil {
		return inceptionState{}, err
	}
	if err := os.MkdirAll(filepath.Dir(inceptionPath()), 0o755); err != nil {
		return inceptionState{}, err
	}
	tmp := inceptionPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return inceptionState{}, err
	}
	if err := os.Rename(tmp, inceptionPath()); err != nil {
		return inceptionState{}, err
	}
	inceptionOnce = next

	log.Printf("[LIVE ENGINE] ROI baseline RE-BASED $%.4f -> $%.4f (reset #%d, %q) — "+
		"returns from here are measured against the new figure, not the account's opening balance",
		prev.EquityUSD, equityUSD, next.Resets, next.ResetReason)
	return next, nil
}
