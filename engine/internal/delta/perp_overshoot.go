package delta

import "math"

// perpStopOvershoot reports realised risk over planned risk for a stop-out.
//
// 1.00 is a stop that closed exactly where it was placed. The measure exists
// because this desk has hit stop failures five distinct ways, and each fix
// looked complete when it shipped — a fix is only credible across a run of
// closes, which requires the ratio to be recorded rather than reconstructed.
//
// Only stop-outs are measured. A target or timeout exit has no planned-risk
// denominator, and inventing one would fill the record with meaningless 1.0s
// that dilute the very average being watched.
func perpStopOvershoot(t *PerpLiveTrade, exit float64) float64 {
	if t == nil || exit <= 0 || t.EntryPrice <= 0 || t.StopPrice <= 0 {
		return 0
	}
	switch t.ExitReason {
	case "SL", "SL_BACKSTOP", "STOP", "stop_loss":
	default:
		return 0
	}
	planned := math.Abs(t.EntryPrice - t.StopPrice)
	if planned <= 0 {
		return 0
	}
	realised := math.Abs(t.EntryPrice - exit)
	return realised / planned
}
