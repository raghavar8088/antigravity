package liveengine

import (
	"os"
	"testing"
	"time"
)

func resetInception(t *testing.T) {
	t.Helper()
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())
	inceptionMu.Lock()
	inceptionOnce = inceptionState{}
	inceptionRead = false
	inceptionMu.Unlock()
}

// The baseline is captured once and never moves.
//
// A baseline that follows the balance reports 0% forever, which is the most
// flattering possible lie for a losing account.
func TestAccountROI_BaselineIsFixedAtFirstSight(t *testing.T) {
	resetInception(t)

	base, _, roiUSD, roiPct := AccountROI(61.40)
	if base != 61.40 || roiUSD != 0 || roiPct != 0 {
		t.Fatalf("first sight: base %.4f roi %.4f (%.2f%%) — the opening balance must show no return yet", base, roiUSD, roiPct)
	}

	// The account loses money.
	base, _, roiUSD, roiPct = AccountROI(55.26)
	if base != 61.40 {
		t.Errorf("baseline moved to %.4f — it must stay at the opening balance", base)
	}
	if roiUSD >= 0 || roiPct >= 0 {
		t.Errorf("a fall from 61.40 to 55.26 reported roi %.4f (%.2f%%)", roiUSD, roiPct)
	}
	if want := -10.0; roiPct < want-0.1 || roiPct > want+0.1 {
		t.Errorf("roi %.2f%%, want about %.2f%%", roiPct, want)
	}
}

// An unreadable wallet must not become the baseline: it would either divide by
// zero or fix the reference at a moment the balance was unknown, permanently
// misstating every future return.
func TestAccountROI_ZeroEquityIsNotABaseline(t *testing.T) {
	resetInception(t)

	if base, _, _, pct := AccountROI(0); base != 0 || pct != 0 {
		t.Errorf("zero equity produced a baseline %.4f / %.2f%%", base, pct)
	}
	// A real balance afterwards must still become the baseline.
	if base, _, _, _ := AccountROI(61.40); base != 61.40 {
		t.Errorf("baseline after a failed read = %.4f, want 61.40", base)
	}
}

// The baseline must survive a restart, or ROI resets to 0% on every deploy.
func TestAccountROI_SurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ENGINE_DATA_DIR", dir)

	inceptionMu.Lock()
	inceptionOnce = inceptionState{}
	inceptionRead = false
	inceptionMu.Unlock()
	AccountROI(61.40)

	// Simulate a fresh process over the same data directory.
	inceptionMu.Lock()
	inceptionOnce = inceptionState{}
	inceptionRead = false
	inceptionMu.Unlock()

	base, at, _, pct := AccountROI(55.26)
	if base != 61.40 {
		t.Errorf("baseline after restart = %.4f, want 61.40 — ROI would reset on every deploy", base)
	}
	if at.IsZero() || time.Since(at) > time.Minute {
		t.Errorf("inception timestamp not restored: %v", at)
	}
	if pct > -9 {
		t.Errorf("roi %.2f%% after restart, want about -10%%", pct)
	}
	if _, err := os.Stat(inceptionPath()); err != nil {
		t.Errorf("baseline file missing: %v", err)
	}
}
