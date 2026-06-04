// Phase 20H — Audit Export Engine
// Generates external-auditor-ready packages with full event history,
// positions, NAV, investor activity, and capital movements.
// Formats: JSON (always), CSV and summary text.
package fundops

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ─── Audit Package ────────────────────────────────────────────────────────────

// AuditPackage is a complete export package for external auditors.
type AuditPackage struct {
	ExportID    string
	FundID      string
	FromDate    time.Time
	ToDate      time.Time
	GeneratedAt time.Time
	GeneratedBy string

	// Full event log.
	Events []FundEvent

	// Derived projections at export date.
	Fund       FundProjection
	Investors  map[string]InvestorProjection
	NAVHistory []NAVPoint
	TaxLots    TaxLotProjection
	Fees       FeeProjection
	Compliance ComplianceProjection
	CapFlows   CapitalFlowProjection

	RecordCount int64
}

// AuditExportEngine generates auditor-ready export packages.
type AuditExportEngine struct {
	store  EventStore
	fundID string
}

// NewAuditExportEngine creates an audit export engine.
func NewAuditExportEngine(store EventStore, fundID string) *AuditExportEngine {
	return &AuditExportEngine{store: store, fundID: fundID}
}

// GeneratePackage builds a complete audit package for the given date range.
func (e *AuditExportEngine) GeneratePackage(ctx context.Context, from, to time.Time, generatedBy string) (AuditPackage, error) {
	// Replay complete fund history.
	result, err := ReplayFund(ctx, e.store, e.fundID)
	if err != nil {
		return AuditPackage{}, fmt.Errorf("audit: replay: %w", err)
	}

	// Filter events to date range.
	allEvts, err := e.store.ReplayFund(ctx, e.fundID)
	if err != nil {
		return AuditPackage{}, fmt.Errorf("audit: load events: %w", err)
	}
	var filtered []FundEvent
	for _, ev := range allEvts {
		if !ev.CreatedAt.Before(from) && !ev.CreatedAt.After(to) {
			filtered = append(filtered, ev)
		}
	}

	exportID := fmt.Sprintf("audit_%s_%d", e.fundID, time.Now().UnixNano())
	pkg := AuditPackage{
		ExportID:    exportID,
		FundID:      e.fundID,
		FromDate:    from,
		ToDate:      to,
		GeneratedAt: time.Now().UTC(),
		GeneratedBy: generatedBy,
		Events:      filtered,
		Fund:        result.Fund,
		Investors:   result.Investors,
		NAVHistory:  result.NAV.History,
		TaxLots:     result.TaxLots,
		Fees:        result.Fees,
		Compliance:  result.Compliance,
		CapFlows:    result.CapitalFlow,
		RecordCount: int64(len(filtered)),
	}

	// Persist audit export event.
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggAudit,
		AggregateID:   e.fundID,
		FundID:        e.fundID,
		EventType:     EvtAuditExportGenerated,
		Payload: AuditExportPayload{
			FundID: e.fundID, ExportID: exportID,
			FromDate: from, ToDate: to,
			Format: "JSON+CSV", RecordCount: pkg.RecordCount,
			GeneratedBy: generatedBy,
		},
	})
	if err != nil {
		return pkg, err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return pkg, fmt.Errorf("audit: persist export event: %w", err)
	}
	return pkg, nil
}

// ToJSON serialises the audit package to JSON bytes.
func (p *AuditPackage) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// EventsToCSV serialises the audit event log to CSV bytes.
func (p *AuditPackage) EventsToCSV() ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	headers := []string{
		"event_id", "aggregate_type", "aggregate_id", "fund_id",
		"event_type", "sequence_no", "created_at", "payload_hash",
	}
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	for _, ev := range p.Events {
		row := []string{
			ev.EventID,
			string(ev.AggregateType),
			ev.AggregateID,
			ev.FundID,
			string(ev.EventType),
			strconv.FormatInt(ev.SequenceNo, 10),
			ev.CreatedAt.Format(time.RFC3339),
			ev.PayloadHash,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// NAVToCSV serialises the NAV history to CSV bytes.
func (p *AuditPackage) NAVToCSV() ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	headers := []string{"as_of", "total_nav_usd", "nav_per_unit", "total_units", "period_return_pct", "ytd_return_pct"}
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	for _, pt := range p.NAVHistory {
		row := []string{
			pt.AsOf.Format("2006-01-02"),
			fmt.Sprintf("%.2f", pt.TotalNAV),
			fmt.Sprintf("%.6f", pt.NAVPerUnit),
			fmt.Sprintf("%.6f", pt.TotalUnits),
			fmt.Sprintf("%.4f", pt.PeriodReturn),
			fmt.Sprintf("%.4f", pt.YTDReturn),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// CapitalFlowsToCSV serialises capital flow history to CSV.
func (p *AuditPackage) CapitalFlowsToCSV() ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	headers := []string{"date", "flow_type", "investor_id", "amount_usd", "units"}
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	for _, flow := range p.CapFlows.FlowHistory {
		row := []string{
			flow.Date.Format("2006-01-02"),
			flow.FlowType,
			flow.InvestorID,
			fmt.Sprintf("%.2f", flow.AmountUSD),
			fmt.Sprintf("%.6f", flow.Units),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// InvestorsToCSV serialises investor snapshot to CSV.
func (p *AuditPackage) InvestorsToCSV() ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	headers := []string{"investor_id", "name", "entity_type", "status", "units",
		"subscribed_usd", "redeemed_usd", "nav_share_usd"}
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	for _, inv := range p.Investors {
		row := []string{
			inv.InvestorID, inv.Name, inv.EntityType,
			string(inv.Status),
			fmt.Sprintf("%.6f", inv.Units),
			fmt.Sprintf("%.2f", inv.CapitalUSD),
			fmt.Sprintf("%.2f", inv.RedemptionUSD),
			fmt.Sprintf("%.2f", inv.NAVShare),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// TaxLotsToCSV serialises closed tax lots to CSV for tax reporting.
func (p *AuditPackage) TaxLotsToCSV() ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	headers := []string{"lot_id", "symbol", "qty_closed", "cost_basis_usd",
		"proceeds_usd", "realized_gain_usd", "holding_days", "is_long_term", "closed_date"}
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	for _, lot := range p.TaxLots.ClosedLots {
		row := []string{
			lot.LotID, lot.Symbol,
			fmt.Sprintf("%.6f", lot.QuantityClosed),
			fmt.Sprintf("%.2f", lot.CostBasisUSD),
			fmt.Sprintf("%.2f", lot.ProceedsUSD),
			fmt.Sprintf("%.2f", lot.RealizedGainUSD),
			strconv.Itoa(lot.HoldingDays),
			strconv.FormatBool(lot.IsLongTerm),
			lot.ClosedDate.Format("2006-01-02"),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// Summary returns a text summary of the audit package for quick review.
func (p *AuditPackage) Summary() string {
	return fmt.Sprintf(`AUDIT PACKAGE — %s
=================================
Fund:           %s (%s)
Period:         %s → %s
Generated:      %s by %s
Total Events:   %d
NAV Points:     %d
Investors:      %d
Capital Flows:  %d
Closed Tax Lots: %d
Violations:     %d
Total NAV:      $%.2f
`,
		p.ExportID,
		p.Fund.Name, p.FundID,
		p.FromDate.Format("2006-01-02"), p.ToDate.Format("2006-01-02"),
		p.GeneratedAt.Format(time.RFC3339), p.GeneratedBy,
		p.RecordCount,
		len(p.NAVHistory),
		len(p.Investors),
		len(p.CapFlows.FlowHistory),
		len(p.TaxLots.ClosedLots),
		len(p.Compliance.ActiveViolations),
		p.Fund.TotalNAV,
	)
}
