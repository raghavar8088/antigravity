package execution

func SupportedLatencyTiers() []LatencyTier {
	return []LatencyTier{Latency10ms, Latency25ms, Latency50ms, Latency100ms, Latency250ms, Latency500ms, Latency1000ms}
}

func LatencyTierMilliseconds(tier LatencyTier) int64 {
	return int64(tier) / 1_000_000
}
