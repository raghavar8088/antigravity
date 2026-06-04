package live

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ParityResult captures the outcome of a paper-vs-live execution parity check.
type ParityResult struct {
	Symbol           string
	Side             string
	Quantity         float64
	PaperFill        FillReport
	LiveFill         FillReport
	PriceDeltaPct    float64
	SlippageDeltaBps float64
	LatencyDeltaMs   int64
	FeesDeltaUSD     float64
	IsEquivalent     bool
	Findings         []string
	CheckedAt        time.Time
}

// ParityChecker compares paper and live execution outcomes for the same order request.
// Deviations beyond the configured tolerances are flagged as parity failures.
type ParityChecker struct {
	MaxPriceDeltaPct    float64 // fraction, e.g. 0.01 = 1%
	MaxSlippageDeltaBps float64
	MaxLatencyDeltaMs   int64
	MaxFeesDeltaUSD     float64
}

func NewParityChecker() *ParityChecker {
	return &ParityChecker{
		MaxPriceDeltaPct:    0.01,  // 1% price deviation
		MaxSlippageDeltaBps: 5.0,   // 5 bps slippage delta
		MaxLatencyDeltaMs:   500,   // 500 ms latency delta
		MaxFeesDeltaUSD:     1.0,   // $1 fee delta
	}
}

// Check submits the same request to both executors and compares the resulting fills.
// It is the caller's responsibility to use a suitably small quantity to limit live exposure.
func (pc *ParityChecker) Check(ctx context.Context, req OrderRequest, paper, liveExec Executor) (*ParityResult, error) {
	paperFill, err := paper.Submit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("parity check — paper submit: %w", err)
	}
	liveFill, err := liveExec.Submit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("parity check — live submit: %w", err)
	}
	return pc.compare(req, paperFill, liveFill), nil
}

func (pc *ParityChecker) compare(req OrderRequest, paper, live FillReport) *ParityResult {
	var priceDelta float64
	if paper.AvgPrice > 0 {
		priceDelta = math.Abs(live.AvgPrice-paper.AvgPrice) / paper.AvgPrice
	}
	slippageDelta := math.Abs(live.SlippageBps - paper.SlippageBps)
	latencyDelta := absInt64(live.LatencyMs - paper.LatencyMs)
	feesDelta := math.Abs(live.FeesUSD - paper.FeesUSD)

	var findings []string
	if priceDelta > pc.MaxPriceDeltaPct {
		findings = append(findings, fmt.Sprintf(
			"price delta %.3f%% exceeds tolerance %.3f%%", priceDelta*100, pc.MaxPriceDeltaPct*100))
	}
	if slippageDelta > pc.MaxSlippageDeltaBps {
		findings = append(findings, fmt.Sprintf(
			"slippage delta %.1f bps exceeds tolerance %.1f bps", slippageDelta, pc.MaxSlippageDeltaBps))
	}
	if latencyDelta > pc.MaxLatencyDeltaMs {
		findings = append(findings, fmt.Sprintf(
			"latency delta %d ms exceeds tolerance %d ms", latencyDelta, pc.MaxLatencyDeltaMs))
	}
	if feesDelta > pc.MaxFeesDeltaUSD {
		findings = append(findings, fmt.Sprintf(
			"fees delta $%.2f exceeds tolerance $%.2f", feesDelta, pc.MaxFeesDeltaUSD))
	}

	return &ParityResult{
		Symbol:           req.Symbol,
		Side:             req.Side,
		Quantity:         req.Quantity,
		PaperFill:        paper,
		LiveFill:         live,
		PriceDeltaPct:    priceDelta,
		SlippageDeltaBps: slippageDelta,
		LatencyDeltaMs:   latencyDelta,
		FeesDeltaUSD:     feesDelta,
		IsEquivalent:     len(findings) == 0,
		Findings:         findings,
		CheckedAt:        time.Now().UTC(),
	}
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
