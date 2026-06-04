// Phase 20I — Investor Reporting Engine
// Generates institutional-grade investor reports: daily, monthly, quarterly, annual.
package fundops

import (
	"context"
	"fmt"
	"time"
)

// ─── Report Types ─────────────────────────────────────────────────────────────

type ReportType string

const (
	ReportDaily     ReportType = "DAILY"
	ReportMonthly   ReportType = "MONTHLY"
	ReportQuarterly ReportType = "QUARTERLY"
	ReportAnnual    ReportType = "ANNUAL"
)

// ─── Investor Report ──────────────────────────────────────────────────────────

// InvestorReport is the complete investor statement for a reporting period.
type InvestorReport struct {
	// Identity
	ReportID     string
	FundID       string
	FundName     string
	InvestorID   string
	InvestorName string
	ReportType   ReportType
	Period       string
	AsOf         time.Time
	GeneratedAt  time.Time

	// Units & Ownership
	OpeningUnits  float64
	ClosingUnits  float64
	OwnershipPct  float64

	// NAV & Returns
	OpeningNAVPerUnit  float64
	ClosingNAVPerUnit  float64
	OpeningNAVShare    float64
	ClosingNAVShare    float64
	PeriodReturn       float64 // (closing - opening) / opening
	YTDReturn          float64
	SinceInceptionReturn float64

	// Capital Activity
	SubscriptionsUSD    float64
	RedemptionsUSD      float64
	DistributionsUSD    float64
	NetCapitalActivity  float64

	// Performance
	GrossReturnUSD      float64
	ManagementFeeUSD    float64
	PerformanceFeeUSD   float64
	NetReturnUSD        float64

	// Risk
	MaxDrawdown    float64
	Volatility     float64 // annualised daily vol
	SharpeRatio    float64

	// Exposure
	GrossExposurePct float64
	NetExposurePct   float64
	Leverage         float64

	// Attribution summary (top 3 strategies)
	Attribution []AttributionEntry
}

// ─── Reporting Engine ─────────────────────────────────────────────────────────

// InvestorReportingEngine generates investor-facing reports.
type InvestorReportingEngine struct {
	store     EventStore
	fundID    string
	navEngine *NAVEngine
}

// NewInvestorReportingEngine creates a reporting engine.
func NewInvestorReportingEngine(store EventStore, fundID string) *InvestorReportingEngine {
	return &InvestorReportingEngine{
		store:     store,
		fundID:    fundID,
		navEngine: NewNAVEngine(store),
	}
}

// Generate produces an investor report for the specified period.
func (e *InvestorReportingEngine) Generate(
	ctx context.Context,
	investorID string,
	reportType ReportType,
	periodLabel string,
	asOf time.Time,
	result ReplayResult,
) (InvestorReport, error) {

	inv, ok := result.Investors[investorID]
	if !ok {
		return InvestorReport{}, fmt.Errorf("reporting: investor %s not found", investorID)
	}

	// NAV history for the period.
	navHistory := result.NAV.History
	openingNAVPerUnit := 1000.0
	if len(navHistory) > 0 {
		openingNAVPerUnit = navHistory[0].NAVPerUnit
	}
	closingNAVPerUnit := result.Fund.NAVPerUnit

	// Opening NAV share: approximate as current units × opening NAV.
	openingNAVShare := inv.Units * openingNAVPerUnit
	closingNAVShare := inv.Units * closingNAVPerUnit

	periodReturn := 0.0
	if openingNAVPerUnit > 0 {
		periodReturn = (closingNAVPerUnit - openingNAVPerUnit) / openingNAVPerUnit
	}

	// Capital flows for this investor.
	subsUSD, redUSD, distUSD := 0.0, 0.0, 0.0
	for _, flow := range result.CapitalFlow.FlowHistory {
		if flow.InvestorID != investorID {
			continue
		}
		switch flow.FlowType {
		case "SUBSCRIPTION":
			subsUSD += flow.AmountUSD
		case "REDEMPTION":
			redUSD += flow.AmountUSD
		case "DISTRIBUTION":
			distUSD += flow.AmountUSD
		}
	}

	// Risk metrics from NAV history.
	maxDD := MaxDrawdown(navHistory)
	sharpe := SharpeRatio(navHistory, 0.045) // 4.5% risk-free

	// Annualised vol from NAV history.
	annualVol := annualisedVol(navHistory)

	// Ownership percentage.
	ownershipPct := 0.0
	if result.Fund.TotalUnits > 0 {
		ownershipPct = inv.Units / result.Fund.TotalUnits
	}

	// Top 3 attribution entries.
	var topAttrib []AttributionEntry
	if len(result.Performance.History) > 0 {
		last := result.Performance.History[len(result.Performance.History)-1]
		topN := last.Attribution
		if len(topN) > 3 {
			topN = topN[:3]
		}
		topAttrib = topN
	}

	// YTD return: from first NAV of the year.
	ytdReturn := 0.0
	yearStart := time.Date(asOf.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	for _, pt := range navHistory {
		if !pt.AsOf.Before(yearStart) {
			if openingNAVPerUnit > 0 {
				ytdReturn = (closingNAVPerUnit - pt.NAVPerUnit) / pt.NAVPerUnit
			}
			break
		}
	}

	// Since-inception return.
	inceptionReturn := 0.0
	if openingNAVPerUnit > 0 && result.Fund.InceptionDate.Year() > 2000 {
		inceptionReturn = (closingNAVPerUnit - 1000.0) / 1000.0
	}

	reportID := fmt.Sprintf("rpt_%s_%s_%d", investorID, string(reportType), asOf.UnixNano())

	report := InvestorReport{
		ReportID:             reportID,
		FundID:               e.fundID,
		FundName:             result.Fund.Name,
		InvestorID:           investorID,
		InvestorName:         inv.Name,
		ReportType:           reportType,
		Period:               periodLabel,
		AsOf:                 asOf,
		GeneratedAt:          time.Now().UTC(),
		OpeningUnits:         inv.Units,
		ClosingUnits:         inv.Units,
		OwnershipPct:         ownershipPct * 100,
		OpeningNAVPerUnit:    openingNAVPerUnit,
		ClosingNAVPerUnit:    closingNAVPerUnit,
		OpeningNAVShare:      openingNAVShare,
		ClosingNAVShare:      closingNAVShare,
		PeriodReturn:         periodReturn * 100,
		YTDReturn:            ytdReturn * 100,
		SinceInceptionReturn: inceptionReturn * 100,
		SubscriptionsUSD:     subsUSD,
		RedemptionsUSD:       redUSD,
		DistributionsUSD:     distUSD,
		NetCapitalActivity:   subsUSD - redUSD - distUSD,
		GrossReturnUSD:       closingNAVShare - openingNAVShare,
		ManagementFeeUSD:     result.Fees.PaidMgmtFee * ownershipPct,
		PerformanceFeeUSD:    result.Fees.PaidPerfFee * ownershipPct,
		NetReturnUSD:         (closingNAVShare - openingNAVShare) - result.Fees.PaidMgmtFee*ownershipPct - result.Fees.PaidPerfFee*ownershipPct,
		MaxDrawdown:          maxDD * 100,
		Volatility:           annualVol * 100,
		SharpeRatio:          sharpe,
		Attribution:          topAttrib,
	}

	// Persist reporting event.
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggReport,
		AggregateID:   investorID,
		FundID:        e.fundID,
		EventType:     EvtInvestorReportGenerated,
		Payload: InvestorReportPayload{
			FundID:      e.fundID,
			InvestorID:  investorID,
			ReportType:  string(reportType),
			Period:      periodLabel,
			GeneratedAt: report.GeneratedAt,
		},
	})
	if err != nil {
		return report, err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return report, fmt.Errorf("reporting: persist event: %w", err)
	}
	return report, nil
}

// FormatText renders an investor report as a human-readable statement.
func (r InvestorReport) FormatText() string {
	return fmt.Sprintf(`
╔═══════════════════════════════════════════════════════════════════╗
║  INVESTOR STATEMENT — %s
╠═══════════════════════════════════════════════════════════════════╣
║  Fund:            %-44s  ║
║  Investor:        %-44s  ║
║  Period:          %-44s  ║
║  As Of:           %-44s  ║
╠═══════════════════════════════════════════════════════════════════╣
║  UNIT HOLDINGS                                                    ║
║  Units:           %-10.4f    Ownership: %-22.4f%%   ║
╠═══════════════════════════════════════════════════════════════════╣
║  NAV PER UNIT                                                     ║
║  Opening:         $%-10.4f   Closing: $%-22.4f  ║
╠═══════════════════════════════════════════════════════════════════╣
║  NAV SHARE                                                        ║
║  Opening:         $%-10.2f   Closing: $%-22.2f  ║
╠═══════════════════════════════════════════════════════════════════╣
║  PERFORMANCE                                                      ║
║  Period Return:   %-8.2f%%    YTD Return: %-20.2f%%   ║
║  Since Inception: %-8.2f%%    Sharpe:     %-22.4f  ║
║  Max Drawdown:    %-8.2f%%    Volatility: %-22.2f%%   ║
╠═══════════════════════════════════════════════════════════════════╣
║  CAPITAL ACTIVITY                                                 ║
║  Subscriptions:   $%-10.2f   Redemptions: $%-21.2f  ║
║  Distributions:   $%-10.2f   Net Return:  $%-21.2f  ║
╠═══════════════════════════════════════════════════════════════════╣
║  FEES                                                             ║
║  Management Fee:  $%-10.2f   Performance: $%-21.2f  ║
╚═══════════════════════════════════════════════════════════════════╝
`,
		r.ReportID,
		r.FundName, r.InvestorName, r.Period, r.AsOf.Format("2006-01-02"),
		r.ClosingUnits, r.OwnershipPct,
		r.OpeningNAVPerUnit, r.ClosingNAVPerUnit,
		r.OpeningNAVShare, r.ClosingNAVShare,
		r.PeriodReturn, r.YTDReturn,
		r.SinceInceptionReturn, r.SharpeRatio,
		r.MaxDrawdown, r.Volatility,
		r.SubscriptionsUSD, r.RedemptionsUSD,
		r.DistributionsUSD, r.NetReturnUSD,
		r.ManagementFeeUSD, r.PerformanceFeeUSD,
	)
}

// annualisedVol computes annualised volatility from NAV history.
func annualisedVol(history []NAVPoint) float64 {
	if len(history) < 2 {
		return 0
	}
	returns := make([]float64, 0, len(history)-1)
	for i := 1; i < len(history); i++ {
		if history[i-1].NAVPerUnit > 0 {
			r := (history[i].NAVPerUnit - history[i-1].NAVPerUnit) / history[i-1].NAVPerUnit
			returns = append(returns, r)
		}
	}
	if len(returns) == 0 {
		return 0
	}
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, r := range returns {
		d := r - mean
		variance += d * d
	}
	variance /= float64(len(returns))
	return variance * 365 * variance // approximate annualised
}
