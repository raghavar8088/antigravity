package mocks

import "sync"

// MockKillSwitch records kill switch activation for test assertions.
type MockKillSwitch struct {
	mu        sync.Mutex
	activated bool
	reason    string
}

// Activate records a kill switch activation.
func (k *MockKillSwitch) Activate(reason string) {
	k.mu.Lock()
	k.activated = true
	k.reason = reason
	k.mu.Unlock()
}

// WasActivated returns true if Activate was called at least once.
func (k *MockKillSwitch) WasActivated() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.activated
}

// ActivationReason returns the reason passed to the last Activate call.
func (k *MockKillSwitch) ActivationReason() string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.reason
}
