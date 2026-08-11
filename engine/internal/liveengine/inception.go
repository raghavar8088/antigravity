package liveengine

import (
	"encoding/json"
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
