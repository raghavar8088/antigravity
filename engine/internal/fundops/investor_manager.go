// Phase 20C — Investor Management System
// Registry for unlimited investors with capital segregation.
package fundops

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ─── Investor Manager ─────────────────────────────────────────────────────────

// InvestorManager manages the investor registry for a fund.
type InvestorManager struct {
	mu        sync.RWMutex
	investors map[string]*InvestorAccount // investorID → account
	store     EventStore
	fundID    string
}

// InvestorAccount is the full investor record including capital and units.
type InvestorAccount struct {
	InvestorID       string
	FundID           string
	Name             string
	EntityType       string // INDIVIDUAL, INSTITUTION, TRUST, FUND
	JurisdictionCode string
	TaxID            string
	Status           InvestorStatus
	Units            float64
	SubscribedCapital float64 // total capital subscribed
	RedeemedCapital  float64 // total capital redeemed
	Distributions    float64 // total distributions received
	NAVShare         float64 // current NAV attribution (= Units × NAVPerUnit)
	RealisedPnLUSD   float64
	CreatedAt        time.Time
	LastActivityAt   time.Time
}

// NewInvestorManager creates an investor manager for the given fund.
func NewInvestorManager(store EventStore, fundID string) *InvestorManager {
	return &InvestorManager{
		investors: make(map[string]*InvestorAccount),
		store:     store,
		fundID:    fundID,
	}
}

// LoadFromReplay populates the manager from a completed fund replay.
func (m *InvestorManager) LoadFromReplay(result ReplayResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, proj := range result.Investors {
		acct := &InvestorAccount{
			InvestorID:       proj.InvestorID,
			FundID:           proj.FundID,
			Name:             proj.Name,
			EntityType:       proj.EntityType,
			Status:           proj.Status,
			Units:            proj.Units,
			SubscribedCapital: proj.CapitalUSD,
			RedeemedCapital:  proj.RedemptionUSD,
			NAVShare:         proj.NAVShare,
			CreatedAt:        proj.CreatedAt,
			LastActivityAt:   proj.LastActivityAt,
		}
		m.investors[id] = acct
	}
}

// Register adds a new investor to the fund and persists the event.
func (m *InvestorManager) Register(ctx context.Context, input InvestorCreatedPayload) (*InvestorAccount, error) {
	if input.InvestorID == "" {
		return nil, errors.New("investor: id required")
	}
	if input.Name == "" {
		return nil, errors.New("investor: name required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.investors[input.InvestorID]; exists {
		return nil, fmt.Errorf("investor: %s already registered", input.InvestorID)
	}

	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggInvestor,
		AggregateID:   input.InvestorID,
		FundID:        m.fundID,
		EventType:     EvtInvestorCreated,
		Payload:       input,
	})
	if err != nil {
		return nil, err
	}
	if _, err := m.store.Append(ctx, ev); err != nil {
		return nil, fmt.Errorf("investor: persist InvestorCreated: %w", err)
	}

	acct := &InvestorAccount{
		InvestorID: input.InvestorID, FundID: input.FundID,
		Name: input.Name, EntityType: input.EntityType,
		JurisdictionCode: input.JurisdictionCode, TaxID: input.TaxID,
		Status: InvestorStatusActive, CreatedAt: time.Now().UTC(),
	}
	m.investors[input.InvestorID] = acct
	return acct, nil
}

// Get returns an investor account by ID.
func (m *InvestorManager) Get(investorID string) (*InvestorAccount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	acct, ok := m.investors[investorID]
	if !ok {
		return nil, fmt.Errorf("investor: not found: %s", investorID)
	}
	return acct, nil
}

// ApplySubscription updates investor account after a successful subscription.
func (m *InvestorManager) ApplySubscription(investorID string, amountUSD, units float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	acct, ok := m.investors[investorID]
	if !ok {
		return fmt.Errorf("investor: not found: %s", investorID)
	}
	acct.SubscribedCapital += amountUSD
	acct.Units += units
	acct.LastActivityAt = time.Now().UTC()
	return nil
}

// ApplyRedemption updates investor account after a successful redemption.
func (m *InvestorManager) ApplyRedemption(investorID string, amountUSD, units float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	acct, ok := m.investors[investorID]
	if !ok {
		return fmt.Errorf("investor: not found: %s", investorID)
	}
	if units > acct.Units+1e-9 {
		return fmt.Errorf("investor: cannot redeem %.6f units, only %.6f held", units, acct.Units)
	}
	acct.Units -= units
	acct.RedeemedCapital += amountUSD
	acct.LastActivityAt = time.Now().UTC()
	if acct.Units < 1e-9 {
		acct.Units = 0
		acct.Status = InvestorStatusRedeemed
	}
	return nil
}

// UpdateNAVShares recalculates each investor's current NAV attribution.
func (m *InvestorManager) UpdateNAVShares(navPerUnit float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, acct := range m.investors {
		acct.NAVShare = acct.Units * navPerUnit
	}
}

// Close marks an investor as redeemed if they have no remaining units.
func (m *InvestorManager) Close(ctx context.Context, investorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	acct, ok := m.investors[investorID]
	if !ok {
		return fmt.Errorf("investor: not found: %s", investorID)
	}
	if acct.Units > 1e-9 {
		return fmt.Errorf("investor: cannot close with %.6f outstanding units", acct.Units)
	}
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggInvestor,
		AggregateID:   investorID,
		FundID:        m.fundID,
		EventType:     EvtInvestorClosed,
		Payload:       map[string]any{"investor_id": investorID, "closed_at": time.Now().UTC()},
	})
	if err != nil {
		return err
	}
	if _, err := m.store.Append(ctx, ev); err != nil {
		return err
	}
	acct.Status = InvestorStatusRedeemed
	return nil
}

// List returns all investors sorted by subscribed capital (descending).
func (m *InvestorManager) List() []*InvestorAccount {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*InvestorAccount, 0, len(m.investors))
	for _, acct := range m.investors {
		cp := *acct
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SubscribedCapital > out[j].SubscribedCapital
	})
	return out
}

// TotalInvestors returns the total number of registered investors.
func (m *InvestorManager) TotalInvestors() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.investors)
}

// TotalAUM returns the sum of all investor NAV shares (AUM).
func (m *InvestorManager) TotalAUM() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := 0.0
	for _, acct := range m.investors {
		total += acct.NAVShare
	}
	return total
}

// OwnershipPct returns an investor's ownership percentage of the fund.
func (m *InvestorManager) OwnershipPct(investorID string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	totalUnits := 0.0
	for _, acct := range m.investors {
		totalUnits += acct.Units
	}
	if totalUnits <= 0 {
		return 0, nil
	}
	acct, ok := m.investors[investorID]
	if !ok {
		return 0, fmt.Errorf("investor: not found: %s", investorID)
	}
	return acct.Units / totalUnits, nil
}
