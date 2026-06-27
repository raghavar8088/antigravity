package v2

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

type SizingScale string

const (
	Size100     SizingScale = "100%"
	Size75      SizingScale = "75%"
	Size50      SizingScale = "50%"
	Size25      SizingScale = "25%"
	Size10      SizingScale = "10%"
	SizeDisable SizingScale = "DISABLE"

	// MinExecutionSizeBTC is the default minimum executable notional in BTC terms.
	// Override via MIN_EXECUTION_SIZE_BTC env var for smaller paper accounts.
	MinExecutionSizeBTC = 0.01
)

// executionFloor returns the effective minimum size, allowing env-var override.
func executionFloor() float64 {
	if v := os.Getenv("MIN_EXECUTION_SIZE_BTC"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return f
		}
	}
	return MinExecutionSizeBTC
}

type SizingDecisionLog struct {
	Layer      string
	BeforeBTC  float64
	AfterBTC   float64
	Multiplier float64
	Reason     string
}

func scaleFromMultiplier(mult float64) SizingScale {
	switch {
	case mult <= 0:
		return SizeDisable
	case mult <= 0.10:
		return Size10
	case mult <= 0.25:
		return Size25
	case mult <= 0.50:
		return Size50
	case mult <= 0.75:
		return Size75
	default:
		return Size100
	}
}

// EnforceExecutionFloor rejects recommended sizes below the institutional floor.
// There is no fallback to a fixed-capital seed (e.g. $10k / price) — sub-floor
// Kelly output means the strategy does not justify a minimum executable position.
func EnforceExecutionFloor(strategyName string, rec, kellyFrac, dynMult float64) (float64, error) {
	floor := executionFloor()
	if rec >= floor {
		return rec, nil
	}
	log.Printf(
		"[RISK REJECTION] %s: recommended size %.6f BTC below execution floor %.6f BTC (Kelly=%.4f, DynMult=%.4f)",
		strategyName, rec, floor, kellyFrac, dynMult,
	)
	return 0, fmt.Errorf("RISK_REJECT: recommended size below execution floor")
}
