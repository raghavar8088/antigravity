// Phase 20D — Capital Inflow / Outflow Engine
// Handles subscriptions, redemptions, transfers, capital calls, distributions.
package fundops

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ─── Capital Flow Engine ──────────────────────────────────────────────────────

// CapitalFlowEngine manages all capital movements for a fund.
type CapitalFlowEngine struct {
	store    EventStore
	fund     *Fund
	investors *InvestorManager
}

// NewCapitalFlowEngine creates a capital flow engine.
func NewCapitalFlowEngine(store EventStore, fund *Fund, investors *InvestorManager) *CapitalFlowEngine {
	return &CapitalFlowEngine{store: store, fund: fund, investors: investors}
}

// ─── Subscription ─────────────────────────────────────────────────────────────

// SubscribeInput carries the parameters for a new capital subscription.
type SubscribeInput struct {
	InvestorID     string
	AmountUSD      float64
	EffectiveDate  time.Time
	SubscriptionID string
}

// Subscribe processes a capital subscription: issues new units and updates accounting.
func (e *CapitalFlowEngine) Subscribe(ctx context.Context, in SubscribeInput) (CapitalSubscribedPayload, error) {
	if in.AmountUSD <= 0 {
		return CapitalSubscribedPayload{}, errors.New("capitalflow: subscription amount must be positive")
	}
	if in.InvestorID == "" {
		return CapitalSubscribedPayload{}, errors.New("capitalflow: investor id required")
	}
	if e.fund.Status != FundStatusActive {
		return CapitalSubscribedPayload{}, fmt.Errorf("capitalflow: fund is not active (status: %s)", e.fund.Status)
	}
	// Validate investor exists.
	if _, err := e.investors.Get(in.InvestorID); err != nil {
		return CapitalSubscribedPayload{}, err
	}

	navPerUnit := e.fund.NAVPerUnit
	if navPerUnit <= 0 {
		navPerUnit = 1000.0 // fallback: initial NAV per unit
	}
	unitsIssued := in.AmountUSD / navPerUnit
	if in.SubscriptionID == "" {
		id, _ := randomID()
		in.SubscriptionID = "sub_" + id
	}

	payload := CapitalSubscribedPayload{
		InvestorID:     in.InvestorID,
		FundID:         e.fund.FundID,
		AmountUSD:      in.AmountUSD,
		UnitsIssued:    unitsIssued,
		NAVPerUnit:     navPerUnit,
		EffectiveDate:  in.EffectiveDate,
		SubscriptionID: in.SubscriptionID,
	}
	ev, err := NewFundEvent(NewEventInput{
		AggregateType:  AggCapital,
		AggregateID:    in.InvestorID,
		FundID:         e.fund.FundID,
		EventType:      EvtCapitalSubscribed,
		IdempotencyKey: in.SubscriptionID,
		Payload:        payload,
	})
	if err != nil {
		return CapitalSubscribedPayload{}, err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return CapitalSubscribedPayload{}, fmt.Errorf("capitalflow: persist subscription: %w", err)
	}

	// Update in-memory state.
	e.fund.IssueUnits(unitsIssued, navPerUnit)
	if err := e.investors.ApplySubscription(in.InvestorID, in.AmountUSD, unitsIssued); err != nil {
		return CapitalSubscribedPayload{}, err
	}
	return payload, nil
}

// ─── Redemption ───────────────────────────────────────────────────────────────

// RedeemInput carries the parameters for a redemption.
type RedeemInput struct {
	InvestorID   string
	AmountUSD    float64 // either AmountUSD or Units must be set
	Units        float64
	EffectiveDate time.Time
	RedemptionID  string
}

// Redeem processes a capital redemption: retires units and returns capital.
func (e *CapitalFlowEngine) Redeem(ctx context.Context, in RedeemInput) (CapitalRedeemedPayload, error) {
	if in.InvestorID == "" {
		return CapitalRedeemedPayload{}, errors.New("capitalflow: investor id required")
	}
	if e.fund.Status != FundStatusActive {
		return CapitalRedeemedPayload{}, fmt.Errorf("capitalflow: fund not active (status: %s)", e.fund.Status)
	}

	acct, err := e.investors.Get(in.InvestorID)
	if err != nil {
		return CapitalRedeemedPayload{}, err
	}

	navPerUnit := e.fund.NAVPerUnit
	if navPerUnit <= 0 {
		return CapitalRedeemedPayload{}, errors.New("capitalflow: invalid NAV per unit")
	}

	// Resolve amount/units.
	var units, amountUSD float64
	switch {
	case in.Units > 0:
		units = in.Units
		amountUSD = units * navPerUnit
	case in.AmountUSD > 0:
		amountUSD = in.AmountUSD
		units = amountUSD / navPerUnit
	default:
		// Full redemption.
		units = acct.Units
		amountUSD = units * navPerUnit
	}

	if units > acct.Units+1e-9 {
		return CapitalRedeemedPayload{}, fmt.Errorf(
			"capitalflow: investor has %.6f units, cannot redeem %.6f", acct.Units, units)
	}
	if in.RedemptionID == "" {
		id, _ := randomID()
		in.RedemptionID = "red_" + id
	}

	payload := CapitalRedeemedPayload{
		InvestorID:    in.InvestorID,
		FundID:        e.fund.FundID,
		AmountUSD:     amountUSD,
		UnitsRedeemed: units,
		NAVPerUnit:    navPerUnit,
		EffectiveDate: in.EffectiveDate,
		RedemptionID:  in.RedemptionID,
	}
	ev, err := NewFundEvent(NewEventInput{
		AggregateType:  AggCapital,
		AggregateID:    in.InvestorID,
		FundID:         e.fund.FundID,
		EventType:      EvtCapitalRedeemed,
		IdempotencyKey: in.RedemptionID,
		Payload:        payload,
	})
	if err != nil {
		return CapitalRedeemedPayload{}, err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return CapitalRedeemedPayload{}, fmt.Errorf("capitalflow: persist redemption: %w", err)
	}

	if err := e.fund.RetireUnits(units, navPerUnit); err != nil {
		return CapitalRedeemedPayload{}, err
	}
	if err := e.investors.ApplyRedemption(in.InvestorID, amountUSD, units); err != nil {
		return CapitalRedeemedPayload{}, err
	}
	return payload, nil
}

// ─── Transfer ─────────────────────────────────────────────────────────────────

// Transfer moves units between investors (e.g., estate transfer).
func (e *CapitalFlowEngine) Transfer(ctx context.Context, fromID, toID string, units float64) error {
	if units <= 0 {
		return errors.New("capitalflow: transfer units must be positive")
	}
	fromAcct, err := e.investors.Get(fromID)
	if err != nil {
		return err
	}
	if units > fromAcct.Units+1e-9 {
		return fmt.Errorf("capitalflow: insufficient units for transfer")
	}
	if _, err := e.investors.Get(toID); err != nil {
		return err
	}

	payload := CapitalTransferredPayload{
		FromInvestorID:   fromID,
		ToInvestorID:     toID,
		FundID:           e.fund.FundID,
		UnitsTransferred: units,
		EffectiveDate:    time.Now().UTC(),
	}
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggCapital,
		AggregateID:   fromID,
		FundID:        e.fund.FundID,
		EventType:     EvtCapitalTransferred,
		Payload:       payload,
	})
	if err != nil {
		return err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return fmt.Errorf("capitalflow: persist transfer: %w", err)
	}
	// Apply in-memory.
	_ = e.investors.ApplyRedemption(fromID, units*e.fund.NAVPerUnit, units)
	_ = e.investors.ApplySubscription(toID, units*e.fund.NAVPerUnit, units)
	return nil
}

// ─── Distribution ─────────────────────────────────────────────────────────────

// DistributeInput carries parameters for a distribution payment.
type DistributeInput struct {
	InvestorID    string
	PerUnitAmount float64
	DistribType   string // INCOME, CAPITAL_GAIN, RETURN_OF_CAPITAL
	EffectiveDate time.Time
}

// Distribute pays a distribution to one investor.
func (e *CapitalFlowEngine) Distribute(ctx context.Context, in DistributeInput) error {
	acct, err := e.investors.Get(in.InvestorID)
	if err != nil {
		return err
	}
	totalAmount := acct.Units * in.PerUnitAmount
	if totalAmount <= 0 {
		return nil // nothing to distribute
	}

	payload := DistributionPaidPayload{
		FundID:        e.fund.FundID,
		InvestorID:    in.InvestorID,
		AmountUSD:     totalAmount,
		PerUnitAmount: in.PerUnitAmount,
		EffectiveDate: in.EffectiveDate,
		DistribType:   in.DistribType,
	}
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggCapital,
		AggregateID:   in.InvestorID,
		FundID:        e.fund.FundID,
		EventType:     EvtDistributionPaid,
		Payload:       payload,
	})
	if err != nil {
		return err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return fmt.Errorf("capitalflow: persist distribution: %w", err)
	}
	return nil
}

// ─── Capital Call ─────────────────────────────────────────────────────────────

// IssueCapitalCall notifies investors of an upcoming capital call (closed-end funds).
func (e *CapitalFlowEngine) IssueCapitalCall(ctx context.Context, investorID string, amountUSD float64, dueDate time.Time) error {
	payload := map[string]any{
		"investor_id": investorID,
		"fund_id":     e.fund.FundID,
		"amount_usd":  amountUSD,
		"due_date":    dueDate,
		"issued_at":   time.Now().UTC(),
	}
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggCapital,
		AggregateID:   investorID,
		FundID:        e.fund.FundID,
		EventType:     EvtCapitalCallIssued,
		Payload:       payload,
	})
	if err != nil {
		return err
	}
	_, err = e.store.Append(ctx, ev)
	return err
}
