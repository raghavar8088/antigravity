package main

import "fmt"

// Grade is the 1-5 qualification band.
type Grade int

const (
	GradeRejected  Grade = 1
	GradeWeak      Grade = 2
	GradePromising Grade = 3
	GradeStrong    Grade = 4
	GradeExcellent Grade = 5
)

func (g Grade) String() string {
	switch g {
	case GradeExcellent:
		return "5 EXCEPTIONAL"
	case GradeStrong:
		return "4 STRONG"
	case GradePromising:
		return "3 PROMISING"
	case GradeWeak:
		return "2 WEAK"
	}
	return "1 REJECTED"
}

// minOOSTrades is the sample floor for a grade above REJECTED.
//
// 30, not the 200 a statistician would want, and the gap is worth being honest
// about. At 1:6 a strategy takes very few trades by construction — that is the
// point of the filter — so a 200-trade bar would reject the entire library and
// tell us nothing. 30 is enough to distinguish "never works" from "sometimes
// works" and nowhere near enough to estimate an edge. Nothing above GRADE 3 is
// claimed on sample size alone, and the report prints the count next to every
// grade so it can never be read without it.
const minOOSTrades = 30

// Verdict is one strategy's full qualification record.
type Verdict struct {
	Strategy  string  `json:"strategy"`
	Family    string  `json:"family"`
	TF        string  `json:"tf"`
	Symbol    string  `json:"symbol"`
	Train     Metrics `json:"train"`
	OOS       Metrics `json:"oos"`
	Grade     Grade   `json:"grade"`
	GradeText string  `json:"grade_text"`
	Reason    string  `json:"reason"`

	Signals     int `json:"signals"`
	RejectedRR  int `json:"rejected_rr"`
	RejectedBad int `json:"rejected_bad"`

	// EligibilityRate is the share of signals that cleared the 1:6 bar. The
	// measurement the 1:6 rule is judged by: a strategy that passes 2% of its
	// own signals is being redefined by the filter, not filtered.
	EligibilityRate float64 `json:"eligibility_rate"`
}

// Qualify grades a strategy from its TRAIN and OUT-OF-SAMPLE records.
//
// The grade is decided by OOS ONLY. Train is used for exactly one thing: to
// establish that the strategy had a reason to be looked at. Every roster this
// desk has ever shipped was picked off an in-sample leaderboard, and the
// out-of-sample check on 401 live trades showed the selected streams performing
// WORSE than the ones passed over (25.0% vs 27.8%). Ranking on train is not a
// weaker version of ranking on OOS; it is measurably worse than not ranking.
func Qualify(v *Verdict) {
	t, o := v.Train, v.OOS

	switch {
	case o.Trades == 0:
		v.Grade, v.Reason = GradeRejected, "no out-of-sample trades"
	case o.Trades < minOOSTrades:
		v.Grade = GradeWeak
		v.Reason = fmt.Sprintf("only %d OOS trades (need %d to grade)", o.Trades, minOOSTrades)
	case o.Expectancy <= 0:
		v.Grade = GradeRejected
		v.Reason = fmt.Sprintf("negative expectancy out of sample (%.3f%%/trade)", o.Expectancy)
	case t.Expectancy <= 0:
		// Positive OOS on a negative train is not a pass. It is the shape luck
		// takes when a strategy has no edge in either window and one of them
		// happened to land up.
		v.Grade = GradeWeak
		v.Reason = "profitable out of sample but not in train — inconsistent, most likely noise"
	case o.ProfitFactor > 0 && o.ProfitFactor < 1.2:
		v.Grade = GradePromising
		v.Reason = fmt.Sprintf("positive but thin (PF %.2f)", o.ProfitFactor)
	case o.MaxDrawdownPct > 3*o.NetPct && o.NetPct > 0:
		// Earned less than a third of what it gave back at the worst point.
		v.Grade = GradePromising
		v.Reason = fmt.Sprintf("drawdown %.1f%% against %.1f%% net — too rough to size", o.MaxDrawdownPct, o.NetPct)
	case o.Sharpe < 0.1:
		v.Grade = GradePromising
		v.Reason = fmt.Sprintf("positive but noisy (Sharpe %.2f)", o.Sharpe)
	case o.Trades >= 100 && o.ProfitFactor >= 1.5 && o.Sharpe >= 0.2:
		v.Grade = GradeExcellent
		v.Reason = fmt.Sprintf("PF %.2f, Sharpe %.2f over %d OOS trades", o.ProfitFactor, o.Sharpe, o.Trades)
	default:
		v.Grade = GradeStrong
		v.Reason = fmt.Sprintf("PF %.2f, Sharpe %.2f, %d OOS trades", o.ProfitFactor, o.Sharpe, o.Trades)
	}
	v.GradeText = v.Grade.String()
	if v.Signals > 0 {
		v.EligibilityRate = float64(o.Trades+t.Trades) / float64(v.Signals) * 100
	}
}

// PaperEligible reports whether a strategy may be enabled for paper trading.
//
// GRADE 3 and above. Deliberately NOT grade 4: paper trading is how a
// promising strategy earns the evidence to be graded properly, so gating it at
// the same bar the evidence is supposed to produce would close the loop on
// itself. Real money is a separate decision and a higher bar.
func PaperEligible(v Verdict) bool { return v.Grade >= GradePromising }
