// Phase 20G — Compliance Reporting Engine
// Every violation is recorded immutably. Every report is reproducible.
package fundops

import (
	"context"
	"fmt"
	"time"
)

// ─── Compliance Rule ──────────────────────────────────────────────────────────

type RuleType string

const (
	RulePositionLimit  RuleType = "POSITION_LIMIT"
	RuleExposureLimit  RuleType = "EXPOSURE_LIMIT"
	RuleLeverageLimit  RuleType = "LEVERAGE_LIMIT"
	RuleConcentration  RuleType = "CONCENTRATION_LIMIT"
	RuleRestrictedAsset RuleType = "RESTRICTED_ASSET"
	RuleVaRLimit       RuleType = "VAR_LIMIT"
	RuleDrawdownLimit  RuleType = "DRAWDOWN_LIMIT"
)

type Severity string

const (
	SeverityWarning  Severity = "WARNING"
	SeverityBreach   Severity = "BREACH"
	SeverityCritical Severity = "CRITICAL"
)

// ComplianceRule defines a single compliance check.
type ComplianceRule struct {
	RuleID      string
	Name        string
	Type        RuleType
	Symbol      string  // if empty, applies fund-wide
	Limit       float64 // the limit value (e.g. 0.10 = 10%)
	WarnAt      float64 // warning threshold (e.g. 0.08 = 80% of limit)
	Description string
	Active      bool
}

// ComplianceCheckInput carries the fund state for compliance evaluation.
type ComplianceCheckInput struct {
	FundID        string
	AsOf          time.Time
	TotalNAV      float64
	Leverage      float64
	GrossExposure float64
	NetExposure   float64
	CurrentVaR    float64
	DrawdownPct   float64
	Positions     []CompliancePosition
}

// CompliancePosition is a position for compliance evaluation.
type CompliancePosition struct {
	Symbol        string
	NotionalUSD   float64
	WeightInFund  float64 // notional / NAV
	IsRestricted  bool
}

// ComplianceReport is the output of one compliance evaluation cycle.
type ComplianceReport struct {
	FundID     string
	AsOf       time.Time
	Violations []ComplianceViolationPayload
	Passes     []string // rule IDs that passed
	Status     string   // "CLEAN" or "VIOLATIONS"
}

// ─── Compliance Engine ────────────────────────────────────────────────────────

// ComplianceEngine evaluates compliance rules against fund state.
type ComplianceEngine struct {
	rules  []ComplianceRule
	store  EventStore
	fundID string
}

// NewComplianceEngine creates a compliance engine with standard hedge fund rules.
func NewComplianceEngine(store EventStore, fundID string) *ComplianceEngine {
	return &ComplianceEngine{
		store:  store,
		fundID: fundID,
		rules:  defaultRules(),
	}
}

// AddRule adds a custom compliance rule.
func (e *ComplianceEngine) AddRule(rule ComplianceRule) {
	e.rules = append(e.rules, rule)
}

// Check evaluates all active rules against the given fund state.
func (e *ComplianceEngine) Check(ctx context.Context, input ComplianceCheckInput) (ComplianceReport, error) {
	report := ComplianceReport{
		FundID: input.FundID,
		AsOf:   input.AsOf,
		Status: "CLEAN",
	}

	for _, rule := range e.rules {
		if !rule.Active {
			continue
		}
		violation, violated := e.evaluate(rule, input)
		if violated {
			report.Violations = append(report.Violations, violation)
			report.Status = "VIOLATIONS"
			ev, err := NewFundEvent(NewEventInput{
				AggregateType: AggCompliance,
				AggregateID:   e.fundID,
				FundID:        e.fundID,
				EventType:     EvtComplianceViolation,
				Payload:       violation,
			})
			if err != nil {
				return report, err
			}
			if _, err := e.store.Append(ctx, ev); err != nil {
				return report, fmt.Errorf("compliance: persist violation: %w", err)
			}
		} else {
			report.Passes = append(report.Passes, rule.RuleID)
			ev, err := NewFundEvent(NewEventInput{
				AggregateType: AggCompliance,
				AggregateID:   e.fundID,
				FundID:        e.fundID,
				EventType:     EvtCompliancePass,
				Payload:       map[string]any{"rule_id": rule.RuleID, "as_of": input.AsOf},
			})
			if err != nil {
				return report, err
			}
			if _, err := e.store.Append(ctx, ev); err != nil {
				return report, fmt.Errorf("compliance: persist pass: %w", err)
			}
		}
	}
	return report, nil
}

// ClearViolation marks a compliance violation as resolved.
func (e *ComplianceEngine) ClearViolation(ctx context.Context, ruleID string) error {
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggCompliance,
		AggregateID:   e.fundID,
		FundID:        e.fundID,
		EventType:     EvtComplianceCleared,
		Payload:       map[string]any{"rule_id": ruleID, "cleared_at": time.Now().UTC()},
	})
	if err != nil {
		return err
	}
	_, err = e.store.Append(ctx, ev)
	return err
}

// ReplayViolations returns all historical violations from the event log.
func (e *ComplianceEngine) ReplayViolations(ctx context.Context) ([]ComplianceViolationPayload, error) {
	evts, err := e.store.Replay(ctx, AggCompliance, e.fundID)
	if err != nil {
		return nil, err
	}
	var violations []ComplianceViolationPayload
	for _, ev := range evts {
		if ev.EventType == EvtComplianceViolation {
			var p ComplianceViolationPayload
			if unmarshal(ev.Payload, &p) == nil {
				violations = append(violations, p)
			}
		}
	}
	return violations, nil
}

// evaluate tests one rule against the current fund state.
func (e *ComplianceEngine) evaluate(rule ComplianceRule, input ComplianceCheckInput) (ComplianceViolationPayload, bool) {
	var actualValue float64
	var violated bool
	var sevStr string

	switch rule.Type {
	case RuleLeverageLimit:
		actualValue = input.Leverage
		violated = actualValue > rule.Limit
		if violated {
			sevStr = severityStr(actualValue, rule.Limit, rule.WarnAt)
		} else if actualValue > rule.WarnAt {
			sevStr = string(SeverityWarning)
			violated = true
		}

	case RuleExposureLimit:
		actualValue = input.GrossExposure / maxFloat64(input.TotalNAV, 1)
		violated = actualValue > rule.Limit
		if violated {
			sevStr = severityStr(actualValue, rule.Limit, rule.WarnAt)
		} else if actualValue > rule.WarnAt {
			sevStr = string(SeverityWarning)
			violated = true
		}

	case RuleVaRLimit:
		actualValue = input.CurrentVaR / maxFloat64(input.TotalNAV, 1)
		violated = actualValue > rule.Limit
		if violated {
			sevStr = string(SeverityBreach)
		}

	case RuleDrawdownLimit:
		actualValue = input.DrawdownPct
		violated = actualValue > rule.Limit
		if violated {
			sevStr = severityStr(actualValue, rule.Limit, rule.WarnAt)
		}

	case RuleConcentration:
		// Check each position's concentration.
		for _, pos := range input.Positions {
			if pos.WeightInFund > rule.Limit {
				actualValue = pos.WeightInFund
				violated = true
				sevStr = severityStr(actualValue, rule.Limit, rule.WarnAt)
				break
			}
		}

	case RuleRestrictedAsset:
		for _, pos := range input.Positions {
			if pos.IsRestricted && pos.NotionalUSD > 0 {
				actualValue = pos.NotionalUSD
				violated = true
				sevStr = string(SeverityCritical)
				break
			}
		}
	}

	if !violated {
		return ComplianceViolationPayload{}, false
	}
	return ComplianceViolationPayload{
		FundID:        e.fundID,
		RuleID:        rule.RuleID,
		RuleName:      rule.Name,
		ViolationType: string(rule.Type),
		ActualValue:   actualValue,
		Limit:         rule.Limit,
		DetectedAt:    input.AsOf,
		Severity:      sevStr,
	}, true
}

// ─── Default Rules ────────────────────────────────────────────────────────────

func defaultRules() []ComplianceRule {
	return []ComplianceRule{
		{RuleID: "leverage_8x", Name: "Maximum Leverage 8×", Type: RuleLeverageLimit,
			Limit: 8.0, WarnAt: 6.0, Description: "Fund leverage must not exceed 8×", Active: true},
		{RuleID: "gross_exposure_500pct", Name: "Gross Exposure 500% NAV", Type: RuleExposureLimit,
			Limit: 5.0, WarnAt: 4.0, Description: "Gross exposure limit", Active: true},
		{RuleID: "concentration_20pct", Name: "Single Asset Concentration 20%", Type: RuleConcentration,
			Limit: 0.20, WarnAt: 0.15, Description: "No single asset > 20% of fund NAV", Active: true},
		{RuleID: "var_10pct_95", Name: "VaR Limit 10% (95% confidence)", Type: RuleVaRLimit,
			Limit: 0.10, WarnAt: 0.08, Description: "Daily 95% VaR must not exceed 10% of NAV", Active: true},
		{RuleID: "drawdown_30pct", Name: "Maximum Drawdown 30%", Type: RuleDrawdownLimit,
			Limit: 0.30, WarnAt: 0.20, Description: "Fund drawdown trigger for risk review", Active: true},
		{RuleID: "restricted_assets", Name: "Restricted Assets", Type: RuleRestrictedAsset,
			Limit: 0, WarnAt: 0, Description: "No restricted assets may be held", Active: true},
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func severityStr(actual, limit, warn float64) string {
	ratio := actual / maxFloat64(limit, 1e-9)
	if ratio > 1.2 {
		return string(SeverityCritical)
	} else if ratio > 1.0 {
		return string(SeverityBreach)
	}
	return string(SeverityWarning)
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
