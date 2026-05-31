package execution

import "time"

type LatencyTier time.Duration

const (
	Latency10ms   LatencyTier = LatencyTier(10 * time.Millisecond)
	Latency25ms   LatencyTier = LatencyTier(25 * time.Millisecond)
	Latency50ms   LatencyTier = LatencyTier(50 * time.Millisecond)
	Latency100ms  LatencyTier = LatencyTier(100 * time.Millisecond)
	Latency250ms  LatencyTier = LatencyTier(250 * time.Millisecond)
	Latency500ms  LatencyTier = LatencyTier(500 * time.Millisecond)
	Latency1000ms LatencyTier = LatencyTier(time.Second)
)

type LatencyInput struct {
	Tier                 LatencyTier
	QueueDelay           time.Duration
	NetworkDelay         time.Duration
	ExecutionDelay       time.Duration
	SignalEdgeBps        float64
	MomentumPctPerSecond float64
}

type LatencyMetrics struct {
	SignalGeneratedAt time.Time
	QueuedAt          time.Time
	SentAt            time.Time
	FilledAt          time.Time
	EntryLatency      time.Duration
	ExitLatency       time.Duration
	SignalDecayBps    float64
	MissedAlphaBps    float64
}

type LatencyEngine struct{}

func (LatencyEngine) Simulate(generatedAt time.Time, input LatencyInput) LatencyMetrics {
	base := time.Duration(input.Tier)
	if base <= 0 {
		base = 50 * time.Millisecond
	}
	queuedAt := generatedAt.Add(input.QueueDelay)
	sentAt := queuedAt.Add(input.NetworkDelay)
	total := input.QueueDelay + input.NetworkDelay + input.ExecutionDelay + base
	filledAt := generatedAt.Add(total)
	seconds := total.Seconds()
	decay := input.SignalEdgeBps * clamp(seconds/2, 0, 1)
	missed := input.MomentumPctPerSecond * seconds * 100
	return LatencyMetrics{
		SignalGeneratedAt: generatedAt,
		QueuedAt:          queuedAt,
		SentAt:            sentAt,
		FilledAt:          filledAt,
		EntryLatency:      total,
		ExitLatency:       total,
		SignalDecayBps:    decay,
		MissedAlphaBps:    missed,
	}
}
