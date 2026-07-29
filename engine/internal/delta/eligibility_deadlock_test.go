package delta

import (
	"testing"
	"time"
)

// OnOpen deadlocked the bridge permanently the first time a live signal arrived
// for an allow-listed strategy: it held b.mu.Lock and then called the injected
// eligibility test, which reads the trade history via LiveStrategyRecord and so
// takes b.mu.RLock. Go's RWMutex is not reentrant, so the goroutine blocked on
// itself and never released b.mu — every later bridge read (positions, account,
// state) hung forever and the control plane returned 502 while /health still
// answered in milliseconds.
//
// These tests run OnOpen on a goroutine and fail on timeout, because a deadlock
// cannot be detected by an assertion — only by not finishing.

const onOpenDeadlockTimeout = 5 * time.Second

// runOnOpen returns false if OnOpen failed to return within the timeout.
func runOnOpen(b *Bridge, sig OpenSignal) bool {
	done := make(chan struct{})
	go func() {
		defer close(done)
		b.OnOpen(sig)
	}()
	select {
	case <-done:
		return true
	case <-time.After(onOpenDeadlockTimeout):
		return false
	}
}

// The eligibility callback re-enters the bridge, exactly as the real wiring does.
func TestOnOpen_EligibilityCallbackMayReadBridge(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}, configured: true, enabled: true, buyingMode: true}
	b.SetLiveAllowList([]string{"s1"})

	called := false
	b.SetLiveEligibility(func(name string) (bool, string) {
		called = true
		// This is what liveengine_wiring.go actually does — it takes b.mu.RLock.
		_ = b.LiveStrategyRecord(name)
		return true, "ok"
	})

	sig := pricedSignal("paper-1", "s1", 1, time.Now().Add(6*time.Hour))
	if !runOnOpen(b, sig) {
		t.Fatal("OnOpen deadlocked: the eligibility test must not run while holding b.mu")
	}
	if !called {
		t.Fatal("eligibility test was never invoked")
	}
	if len(b.trades) != 1 {
		t.Fatalf("an eligible signal must open one live trade, got %d", len(b.trades))
	}
}

// After OnOpen returns, the bridge must still be readable — a leaked lock would
// only show up on the next read, which is how this presented in production.
func TestOnOpen_BridgeStaysReadableAfterOpen(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}, configured: true, enabled: true, buyingMode: true}
	b.SetLiveAllowList([]string{"s1"})
	b.SetLiveEligibility(func(name string) (bool, string) {
		_ = b.LiveStrategyRecord(name)
		return true, "ok"
	})

	if !runOnOpen(b, pricedSignal("paper-1", "s1", 1, time.Now().Add(6*time.Hour))) {
		t.Fatal("OnOpen deadlocked")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = b.OpenTrades()
		_ = b.Trades()
		_ = b.LiveAllowList()
	}()
	select {
	case <-done:
	case <-time.After(onOpenDeadlockTimeout):
		t.Fatal("bridge reads blocked after OnOpen — b.mu was left held")
	}
}

// A blocking gate must also return cleanly, not deadlock on the way out.
func TestOnOpen_EnforcedGateBlockWithoutDeadlock(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}, configured: true, enabled: true, buyingMode: true}
	b.SetLiveAllowList([]string{"s1"})
	b.SetGateEnforcement(true)
	b.SetLiveEligibility(func(name string) (bool, string) {
		_ = b.LiveStrategyRecord(name)
		return false, "no real fills yet"
	})

	if !runOnOpen(b, pricedSignal("paper-1", "s1", 1, time.Now().Add(6*time.Hour))) {
		t.Fatal("OnOpen deadlocked on the gate-blocked path")
	}
	if len(b.trades) != 0 {
		t.Fatalf("an enforced gate block must place no live trade, got %d", len(b.trades))
	}
}

// With no eligibility test wired the gate must fail closed, and still not hang.
func TestOnOpen_UnwiredGateFailsClosed(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}, configured: true, enabled: true, buyingMode: true}
	b.SetLiveAllowList([]string{"s1"})
	b.SetGateEnforcement(true)

	if !runOnOpen(b, pricedSignal("paper-1", "s1", 1, time.Now().Add(6*time.Hour))) {
		t.Fatal("OnOpen deadlocked with no eligibility test wired")
	}
	if len(b.trades) != 0 {
		t.Fatal("an unwired gate must fail closed, never open")
	}
}
