package liveengine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Re-basing must PRESERVE the old baseline, not overwrite it.
//
// The record of what an account actually did is the one thing a reset must not
// destroy. After the tile reads 0.00%, the superseded file is the only place
// "-5.02% since 15 Aug" can still be reconstructed from.
func TestResetInception_PreservesThePreviousBaseline(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ENGINE_DATA_DIR", dir)
	resetPackageState()

	// Capture an original baseline, then lose money against it.
	AccountROI(100)
	_, _, _, roi := AccountROI(90)
	if roi >= 0 {
		t.Fatalf("premise wrong: ROI on a fall was %.2f%%", roi)
	}

	st, err := ResetInception(90, "roster replaced")
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if st.EquityUSD != 90 || st.ResetFrom != 100 || st.Resets != 1 {
		t.Errorf("reset state = %+v, want baseline 90 from 100, reset #1", st)
	}

	// ROI now reads flat, which is the point of the reset.
	if _, _, _, pct := AccountROI(90); pct != 0 {
		t.Errorf("ROI after re-basing to the current equity = %.4f%%, want 0", pct)
	}

	// And the old baseline survives on disk.
	matches, _ := filepath.Glob(filepath.Join(dir, inceptionFile+".superseded-*.json"))
	if len(matches) != 1 {
		t.Fatalf("found %d superseded files, want 1 — the previous baseline was destroyed", len(matches))
	}
	var old inceptionState
	raw, _ := os.ReadFile(matches[0])
	_ = json.Unmarshal(raw, &old)
	if old.EquityUSD != 100 {
		t.Errorf("superseded baseline = %.2f, want the original 100", old.EquityUSD)
	}
}

// A reset must be DISTINGUISHABLE from an account that has never traded. Both
// read 0.00%, and only the bookkeeping tells them apart.
func TestResetInception_IsVisibleAfterwards(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ENGINE_DATA_DIR", dir)
	resetPackageState()

	AccountROI(100)
	if n, _, _, _ := InceptionReset(); n != 0 {
		t.Errorf("a fresh baseline reports %d resets, want 0", n)
	}

	if _, err := ResetInception(90, "roster replaced"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	n, at, from, reason := InceptionReset()
	if n != 1 || from != 100 || reason != "roster replaced" || at.IsZero() {
		t.Errorf("reset info = (%d, %v, %.2f, %q); want 1, a time, 100, the reason", n, at, from, reason)
	}
}

// A non-positive equity must be refused. Re-basing to zero would divide every
// future return by nothing, and re-basing to a negative would invert the sign
// of every gain.
func TestResetInception_RefusesNonPositiveEquity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ENGINE_DATA_DIR", dir)
	resetPackageState()
	AccountROI(100)

	for _, bad := range []float64{0, -1} {
		if _, err := ResetInception(bad, "x"); err == nil {
			t.Errorf("re-basing to %v was accepted", bad)
		}
	}
	// The original baseline is untouched by a refused reset.
	if base, _, _, _ := AccountROI(100); base != 100 {
		t.Errorf("baseline moved to %.2f after a refused reset", base)
	}
}

// resetPackageState clears the process-level memo so each test starts clean.
func resetPackageState() {
	inceptionMu.Lock()
	defer inceptionMu.Unlock()
	inceptionOnce = inceptionState{}
	inceptionRead = false
}
