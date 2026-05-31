package v2

import "time"

type DrawdownDecision struct {
	DrawdownPct         float64
	SizeMultiplier      float64
	OnlyEliteStrategies bool
	TradingHalt         bool
	RecoveryProgressPct float64
	Duration            time.Duration
	Severity            string
	Action              string
}

func EvaluateDrawdown(account AccountState) DrawdownDecision {
	account = normalizeAccount(account)
	dd := 0.0
	if account.HighWatermarkUSD > 0 && account.EquityUSD < account.HighWatermarkUSD {
		dd = (account.HighWatermarkUSD - account.EquityUSD) / account.HighWatermarkUSD * 100
	}
	out := DrawdownDecision{DrawdownPct: dd, SizeMultiplier: 1, Severity: "NORMAL", Action: "normal"}
	switch {
	case dd >= 10:
		out.SizeMultiplier = 0
		out.TradingHalt = true
		out.Severity = "HALT"
		out.Action = "trading halt"
	case dd >= 8:
		out.SizeMultiplier = 0.10
		out.OnlyEliteStrategies = true
		out.Severity = "ELITE_ONLY"
		out.Action = "only elite strategies"
	case dd >= 6:
		out.SizeMultiplier = 0.25
		out.Severity = "CRITICAL"
		out.Action = "reduce size 75%"
	case dd >= 4:
		out.SizeMultiplier = 0.50
		out.Severity = "WARNING"
		out.Action = "reduce size 50%"
	case dd >= 2:
		out.SizeMultiplier = 0.75
		out.Severity = "CAUTION"
		out.Action = "reduce size 25%"
	}
	if account.HighWatermarkUSD > 0 {
		out.RecoveryProgressPct = clamp(account.EquityUSD/account.HighWatermarkUSD*100, 0, 100)
	}
	return out
}
