package fundops

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ─── CQRS Projections ─────────────────────────────────────────────────────────

// FundProjection is the CQRS read model for a fund, rebuilt O(n) from events.
type FundProjection struct {
	FundID          string
	Name            string
	Strategy        string
	Currency        string
	Status          FundStatus
	InceptionDate   time.Time
	ManagementFee   float64
	PerformanceFee  float64
	HurdleRate      float64
	TotalUnits      float64
	TotalNAV        float64
	NAVPerUnit      float64
	TotalInvestors  int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// InvestorProjection is the CQRS read model for a single investor.
type InvestorProjection struct {
	InvestorID     string
	FundID         string
	Name           string
	EntityType     string
	Status         InvestorStatus
	Units          float64
	CapitalUSD     float64    // total subscribed capital
	NAVShare       float64    // current NAV attribution
	TotalPnLUSD    float64
	RedemptionUSD  float64
	CreatedAt      time.Time
	LastActivityAt time.Time
}

// NAVProjection tracks NAV history for a fund.
type NAVProjection struct {
	FundID       string
	History      []NAVPoint
	CurrentNAV   float64
	CurrentPerUnit float64
	LastUpdated  time.Time
}

// NAVPoint is a single NAV observation.
type NAVPoint struct {
	AsOf        time.Time
	TotalNAV    float64
	NAVPerUnit  float64
	TotalUnits  float64
	PeriodReturn float64
	YTDReturn   float64
}

// FeeProjection tracks accrued and paid fees for a fund.
type FeeProjection struct {
	FundID            string
	AccruedMgmtFee    float64
	AccruedPerfFee    float64
	PaidMgmtFee       float64
	PaidPerfFee       float64
	HighWaterMark     float64
	LastFeeDate       time.Time
}

// TaxLotProjection tracks open and closed tax lots.
type TaxLotProjection struct {
	FundID        string
	OpenLots      []TaxLot
	ClosedLots    []ClosedTaxLot
	TotalRealizedGainUSD float64
}

// TaxLot is an open tax lot (holding).
type TaxLot struct {
	LotID           string
	InvestorID      string
	Symbol          string
	Quantity        float64
	CostBasisUSD    float64
	AcquisitionDate time.Time
}

// ClosedTaxLot is a closed (disposed) tax lot.
type ClosedTaxLot struct {
	LotID           string
	Symbol          string
	QuantityClosed  float64
	CostBasisUSD    float64
	ProceedsUSD     float64
	RealizedGainUSD float64
	HoldingDays     int
	IsLongTerm      bool
	ClosedDate      time.Time
}

// ComplianceProjection tracks compliance violations for a fund.
type ComplianceProjection struct {
	FundID           string
	ActiveViolations []ComplianceRecord
	HistoricalCount  int
	LastCheckAt      time.Time
}

// ComplianceRecord captures a compliance event.
type ComplianceRecord struct {
	RuleID        string
	RuleName      string
	ViolationType string
	ActualValue   float64
	Limit         float64
	Severity      string
	Symbol        string
	DetectedAt    time.Time
	ClearedAt     time.Time
	Active        bool
}

// CapitalFlowProjection tracks all capital movements for a fund.
type CapitalFlowProjection struct {
	FundID              string
	TotalSubscribed     float64
	TotalRedeemed       float64
	TotalDistributed    float64
	NetCapital          float64
	FlowHistory         []CapitalFlowRecord
}

// CapitalFlowRecord is a single capital movement.
type CapitalFlowRecord struct {
	FlowType   string // SUBSCRIPTION, REDEMPTION, DISTRIBUTION, TRANSFER
	InvestorID string
	AmountUSD  float64
	Units      float64
	Date       time.Time
}

// PerformanceProjection tracks attribution results over time.
type PerformanceProjection struct {
	FundID      string
	History     []AttributionCalculatedPayload
	LastUpdated time.Time
}

// ─── Full Replay Result ───────────────────────────────────────────────────────

// ReplayResult holds all projections rebuilt from a complete fund event replay.
type ReplayResult struct {
	Fund        FundProjection
	Investors   map[string]InvestorProjection
	NAV         NAVProjection
	Fees        FeeProjection
	TaxLots     TaxLotProjection
	Compliance  ComplianceProjection
	CapitalFlow CapitalFlowProjection
	Performance PerformanceProjection
	TotalEvents int
}

// ReplayFund rebuilds all fund projections from the event log.
// This is deterministic — calling it twice produces identical output.
func ReplayFund(ctx context.Context, store EventStore, fundID string) (ReplayResult, error) {
	evts, err := store.ReplayFund(ctx, fundID)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("fundops/replay: %w", err)
	}

	result := ReplayResult{
		Fund:      FundProjection{FundID: fundID},
		Investors: make(map[string]InvestorProjection),
		NAV:       NAVProjection{FundID: fundID},
		Fees:      FeeProjection{FundID: fundID},
		TaxLots:   TaxLotProjection{FundID: fundID},
		Compliance: ComplianceProjection{FundID: fundID},
		CapitalFlow: CapitalFlowProjection{FundID: fundID},
		Performance: PerformanceProjection{FundID: fundID},
		TotalEvents: len(evts),
	}

	for _, ev := range evts {
		applyEvent(&result, ev)
	}
	return result, nil
}

// applyEvent applies a single fund event to all projections.
func applyEvent(r *ReplayResult, ev FundEvent) {
	switch ev.EventType {

	// ── Fund lifecycle ──────────────────────────────────────────────────────
	case EvtFundCreated:
		var p FundCreatedPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			r.Fund.Name = p.Name
			r.Fund.Strategy = p.Strategy
			r.Fund.Currency = p.Currency
			r.Fund.InceptionDate = p.InceptionDate
			r.Fund.ManagementFee = p.ManagementFee
			r.Fund.PerformanceFee = p.PerformanceFee
			r.Fund.HurdleRate = p.HurdleRate
			r.Fund.TotalNAV = p.InitialNAV
			r.Fund.Status = FundStatusActive
			r.Fund.CreatedAt = ev.CreatedAt
			r.Fees.HighWaterMark = p.InitialNAV
		}
	case EvtFundClosed:
		r.Fund.Status = FundStatusClosed
		r.Fund.UpdatedAt = ev.CreatedAt

	// ── Investor lifecycle ───────────────────────────────────────────────────
	case EvtInvestorCreated:
		var p InvestorCreatedPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			inv := InvestorProjection{
				InvestorID: p.InvestorID, FundID: p.FundID,
				Name: p.Name, EntityType: p.EntityType,
				Status: InvestorStatusActive, CreatedAt: ev.CreatedAt,
			}
			r.Investors[p.InvestorID] = inv
			r.Fund.TotalInvestors++
		}
	case EvtInvestorClosed:
		if inv, ok := r.Investors[ev.AggregateID]; ok {
			inv.Status = InvestorStatusRedeemed
			inv.LastActivityAt = ev.CreatedAt
			r.Investors[ev.AggregateID] = inv
		}

	// ── Capital flows ────────────────────────────────────────────────────────
	case EvtCapitalSubscribed:
		var p CapitalSubscribedPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			if inv, ok := r.Investors[p.InvestorID]; ok {
				inv.Units += p.UnitsIssued
				inv.CapitalUSD += p.AmountUSD
				inv.LastActivityAt = ev.CreatedAt
				r.Investors[p.InvestorID] = inv
			}
			r.Fund.TotalUnits += p.UnitsIssued
			r.Fund.TotalNAV += p.AmountUSD
			r.Fund.UpdatedAt = ev.CreatedAt
			r.CapitalFlow.TotalSubscribed += p.AmountUSD
			r.CapitalFlow.NetCapital += p.AmountUSD
			r.CapitalFlow.FlowHistory = append(r.CapitalFlow.FlowHistory, CapitalFlowRecord{
				FlowType: "SUBSCRIPTION", InvestorID: p.InvestorID,
				AmountUSD: p.AmountUSD, Units: p.UnitsIssued, Date: p.EffectiveDate,
			})
		}
	case EvtCapitalRedeemed:
		var p CapitalRedeemedPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			if inv, ok := r.Investors[p.InvestorID]; ok {
				inv.Units -= p.UnitsRedeemed
				inv.RedemptionUSD += p.AmountUSD
				inv.LastActivityAt = ev.CreatedAt
				r.Investors[p.InvestorID] = inv
			}
			r.Fund.TotalUnits -= p.UnitsRedeemed
			r.Fund.TotalNAV -= p.AmountUSD
			r.Fund.UpdatedAt = ev.CreatedAt
			r.CapitalFlow.TotalRedeemed += p.AmountUSD
			r.CapitalFlow.NetCapital -= p.AmountUSD
			r.CapitalFlow.FlowHistory = append(r.CapitalFlow.FlowHistory, CapitalFlowRecord{
				FlowType: "REDEMPTION", InvestorID: p.InvestorID,
				AmountUSD: p.AmountUSD, Units: p.UnitsRedeemed, Date: p.EffectiveDate,
			})
		}
	case EvtCapitalTransferred:
		var p CapitalTransferredPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			if from, ok := r.Investors[p.FromInvestorID]; ok {
				from.Units -= p.UnitsTransferred
				r.Investors[p.FromInvestorID] = from
			}
			if to, ok := r.Investors[p.ToInvestorID]; ok {
				to.Units += p.UnitsTransferred
				r.Investors[p.ToInvestorID] = to
			}
			r.CapitalFlow.FlowHistory = append(r.CapitalFlow.FlowHistory, CapitalFlowRecord{
				FlowType: "TRANSFER", InvestorID: p.FromInvestorID,
				Units: p.UnitsTransferred, Date: p.EffectiveDate,
			})
		}
	case EvtDistributionPaid:
		var p DistributionPaidPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			r.CapitalFlow.TotalDistributed += p.AmountUSD
			r.CapitalFlow.NetCapital -= p.AmountUSD
			r.CapitalFlow.FlowHistory = append(r.CapitalFlow.FlowHistory, CapitalFlowRecord{
				FlowType: "DISTRIBUTION", InvestorID: p.InvestorID,
				AmountUSD: p.AmountUSD, Date: p.EffectiveDate,
			})
		}

	// ── NAV ──────────────────────────────────────────────────────────────────
	case EvtNAVCalculated:
		var p NAVCalculatedPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			r.Fund.TotalNAV = p.TotalNAV
			r.Fund.TotalUnits = p.TotalUnits
			r.Fund.NAVPerUnit = p.NAVPerUnit
			r.Fund.UpdatedAt = ev.CreatedAt
			r.NAV.CurrentNAV = p.TotalNAV
			r.NAV.CurrentPerUnit = p.NAVPerUnit
			r.NAV.LastUpdated = ev.CreatedAt
			r.NAV.History = append(r.NAV.History, NAVPoint{
				AsOf: p.AsOf, TotalNAV: p.TotalNAV, NAVPerUnit: p.NAVPerUnit,
				TotalUnits: p.TotalUnits, PeriodReturn: p.PeriodReturn, YTDReturn: p.YTDReturn,
			})
		}

	// ── Fees ─────────────────────────────────────────────────────────────────
	case EvtFeeAccrued:
		var p FeeAccruedPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			if p.FeeType == "MANAGEMENT" {
				r.Fees.AccruedMgmtFee += p.AmountUSD
			} else {
				r.Fees.AccruedPerfFee += p.AmountUSD
			}
			r.Fees.LastFeeDate = p.AsOf
		}
	case EvtFeePaid:
		var p FeePaidPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			if p.FeeType == "MANAGEMENT" {
				r.Fees.PaidMgmtFee += p.AmountUSD
				r.Fees.AccruedMgmtFee -= p.AmountUSD
			} else {
				r.Fees.PaidPerfFee += p.AmountUSD
				r.Fees.AccruedPerfFee -= p.AmountUSD
			}
		}
	case EvtHWMUpdated:
		var p struct{ HWM float64 `json:"hwm"` }
		if json.Unmarshal(ev.Payload, &p) == nil {
			r.Fees.HighWaterMark = p.HWM
		}

	// ── Tax lots ─────────────────────────────────────────────────────────────
	case EvtTaxLotCreated:
		var p TaxLotCreatedPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			r.TaxLots.OpenLots = append(r.TaxLots.OpenLots, TaxLot{
				LotID: p.LotID, InvestorID: p.InvestorID, Symbol: p.Symbol,
				Quantity: p.Quantity, CostBasisUSD: p.CostBasisUSD,
				AcquisitionDate: p.AcquisitionDate,
			})
		}
	case EvtTaxLotClosed:
		var p TaxLotClosedPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			// Remove from open lots.
			newOpen := r.TaxLots.OpenLots[:0]
			for _, lot := range r.TaxLots.OpenLots {
				if lot.LotID != p.LotID {
					newOpen = append(newOpen, lot)
				}
			}
			r.TaxLots.OpenLots = newOpen
			r.TaxLots.ClosedLots = append(r.TaxLots.ClosedLots, ClosedTaxLot{
				LotID: p.LotID, Symbol: p.Symbol,
				QuantityClosed: p.QuantityClosed, CostBasisUSD: p.CostBasisUSD,
				ProceedsUSD: p.ProceedsUSD, RealizedGainUSD: p.RealizedGainUSD,
				HoldingDays: p.HoldingDays, IsLongTerm: p.IsLongTerm, ClosedDate: p.ClosedDate,
			})
			r.TaxLots.TotalRealizedGainUSD += p.RealizedGainUSD
		}

	// ── Compliance ───────────────────────────────────────────────────────────
	case EvtComplianceViolation:
		var p ComplianceViolationPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			r.Compliance.ActiveViolations = append(r.Compliance.ActiveViolations, ComplianceRecord{
				RuleID: p.RuleID, RuleName: p.RuleName, ViolationType: p.ViolationType,
				ActualValue: p.ActualValue, Limit: p.Limit, Severity: p.Severity,
				Symbol: p.Symbol, DetectedAt: p.DetectedAt, Active: true,
			})
			r.Compliance.HistoricalCount++
			r.Compliance.LastCheckAt = ev.CreatedAt
		}
	case EvtComplianceCleared:
		var p struct{ RuleID string `json:"rule_id"` }
		if json.Unmarshal(ev.Payload, &p) == nil {
			for i := range r.Compliance.ActiveViolations {
				if r.Compliance.ActiveViolations[i].RuleID == p.RuleID {
					r.Compliance.ActiveViolations[i].Active = false
					r.Compliance.ActiveViolations[i].ClearedAt = ev.CreatedAt
				}
			}
		}

	// ── Attribution ──────────────────────────────────────────────────────────
	case EvtAttributionCalculated:
		var p AttributionCalculatedPayload
		if json.Unmarshal(ev.Payload, &p) == nil {
			r.Performance.History = append(r.Performance.History, p)
			r.Performance.LastUpdated = ev.CreatedAt
		}
	}
}

// ─── Investor NAV Share ───────────────────────────────────────────────────────

// UpdateInvestorNAVShares recalculates each investor's NAV attribution
// based on their unit holdings and the current NAV per unit.
func UpdateInvestorNAVShares(result *ReplayResult) {
	for id, inv := range result.Investors {
		inv.NAVShare = inv.Units * result.Fund.NAVPerUnit
		result.Investors[id] = inv
	}
}
