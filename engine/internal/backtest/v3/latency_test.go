package v3

import (
	"testing"
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
)

func TestLatencyEngine_FilledAfterGenerated(t *testing.T) {
	engine := btExecution.LatencyEngine{}
	generatedAt := time.Now()
	input := btExecution.LatencyInput{
		Tier:          btExecution.Latency50ms,
		SignalEdgeBps: 8,
	}
	result := engine.Simulate(generatedAt, input)
	if !result.FilledAt.After(generatedAt) {
		t.Fatal("FilledAt must be after GeneratedAt")
	}
	if result.EntryLatency < time.Duration(btExecution.Latency50ms) {
		t.Fatalf("entry latency %v should be >= tier %v", result.EntryLatency, btExecution.Latency50ms)
	}
}

func TestLatencyEngine_DecayGrowsWithTier(t *testing.T) {
	engine := btExecution.LatencyEngine{}
	gen := time.Now()
	fast := engine.Simulate(gen, btExecution.LatencyInput{Tier: btExecution.Latency10ms, SignalEdgeBps: 10})
	slow := engine.Simulate(gen, btExecution.LatencyInput{Tier: btExecution.Latency1000ms, SignalEdgeBps: 10})
	if slow.SignalDecayBps <= fast.SignalDecayBps {
		t.Fatal("slower latency should cause more signal decay")
	}
}

func TestLatencyJitter_ProducesVariance(t *testing.T) {
	// Simulate jitter by varying ExecutionDelay.
	rng := newSimpleRNG(99)
	engine := btExecution.LatencyEngine{}
	gen := time.Now()
	samples := make([]time.Duration, 20)
	for i := range samples {
		jitter := 1 + rng.NormFloat64()*0.3
		if jitter < 0.1 {
			jitter = 0.1
		}
		execDelay := time.Duration(float64(btExecution.Latency50ms) * jitter)
		input := btExecution.LatencyInput{
			Tier:          btExecution.Latency50ms,
			ExecutionDelay: execDelay,
			SignalEdgeBps: 8,
		}
		r := engine.Simulate(gen, input)
		samples[i] = r.EntryLatency
	}
	// Check there's variance (not all identical).
	allSame := true
	for i := 1; i < len(samples); i++ {
		if samples[i] != samples[0] {
			allSame = false
			break
		}
	}
	if allSame {
		t.Fatal("jittered latency samples should not all be identical")
	}
}

func TestLatencyTiers_AllProducePositiveDuration(t *testing.T) {
	tiers := []btExecution.LatencyTier{
		btExecution.Latency10ms,
		btExecution.Latency25ms,
		btExecution.Latency50ms,
		btExecution.Latency100ms,
		btExecution.Latency250ms,
		btExecution.Latency500ms,
		btExecution.Latency1000ms,
	}
	engine := btExecution.LatencyEngine{}
	for _, tier := range tiers {
		r := engine.Simulate(time.Now(), btExecution.LatencyInput{Tier: tier})
		if r.EntryLatency <= 0 {
			t.Fatalf("tier %v produced zero latency", tier)
		}
	}
}
