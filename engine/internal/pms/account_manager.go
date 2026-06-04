package pms

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// AccountType classifies the operational purpose of an account.
type AccountType string

const (
	AccountTypeMaster AccountType = "MASTER"
	AccountTypeSub    AccountType = "SUB"
	AccountTypeProp   AccountType = "PROP"
	AccountTypeClient AccountType = "CLIENT"
	AccountTypePaper  AccountType = "PAPER"
)

// AccountStatus tracks the account lifecycle.
type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "ACTIVE"
	AccountStatusSuspended AccountStatus = "SUSPENDED"
	AccountStatusClosed    AccountStatus = "CLOSED"
)

// ManagedAccount is the authoritative record for one trading account.
// Capital, risk, positions, and PnL are fully segregated between accounts.
type ManagedAccount struct {
	AccountID   string        `json:"account_id"`
	Name        string        `json:"name"`
	Type        AccountType   `json:"type"`
	PortfolioID string        `json:"portfolio_id"`
	ParentID    string        `json:"parent_id,omitempty"` // master account ID for sub accounts
	Currency    string        `json:"currency"`
	Status      AccountStatus `json:"status"`

	// Capital (all in base currency USD)
	InitialNAV     float64 `json:"initial_nav_usd"`
	CurrentNAV     float64 `json:"current_nav_usd"`
	AvailableCash  float64 `json:"available_cash_usd"`
	AllocatedUSD   float64 `json:"allocated_usd"`
	RealisedPnLUSD float64 `json:"realised_pnl_usd"`
	UnrealisedPnLUSD float64 `json:"unrealised_pnl_usd"`

	// Position tracking (position count only; full positions live in OMS/positions)
	OpenPositionCount int `json:"open_position_count"`
	TotalTrades       int `json:"total_trades"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AccountSnapshot is an immutable read-only view.
type AccountSnapshot struct {
	AccountID        string
	Name             string
	Type             AccountType
	PortfolioID      string
	ParentID         string
	Status           AccountStatus
	InitialNAV       float64
	CurrentNAV       float64
	AvailableCash    float64
	AllocatedUSD     float64
	RealisedPnLUSD   float64
	UnrealisedPnLUSD float64
	OpenPositionCount int
	TotalTrades      int
	UpdatedAt        time.Time
}

var (
	ErrAccountNotFound    = errors.New("pms: account not found")
	ErrAccountClosed      = errors.New("pms: account is closed")
	ErrAccountSuspended   = errors.New("pms: account is suspended")
	ErrInsufficientFunds  = errors.New("pms: insufficient available capital")
	ErrCrossAccountAccess = errors.New("pms: cross-account capital access denied")
)

// AccountManager is the authoritative registry for all trading accounts.
// It enforces full capital, risk, and position segregation between accounts.
// No account may access another account's capital or positions.
type AccountManager struct {
	mu       sync.RWMutex
	accounts map[string]*ManagedAccount // keyed by accountID
	store    ledger.Store
}

// NewAccountManager constructs an AccountManager backed by a ledger store.
func NewAccountManager(store ledger.Store) *AccountManager {
	return &AccountManager{
		accounts: make(map[string]*ManagedAccount),
		store:    store,
	}
}

// CreateAccount registers a new account and emits a creation event.
func (m *AccountManager) CreateAccount(ctx context.Context, a ManagedAccount) (*ManagedAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.accounts[a.AccountID]; exists {
		return nil, fmt.Errorf("pms: account %s already exists", a.AccountID)
	}
	a.Status = AccountStatusActive
	a.CurrentNAV = a.InitialNAV
	a.AvailableCash = a.InitialNAV
	a.CreatedAt = time.Now().UTC()
	a.UpdatedAt = a.CreatedAt

	payload := AccountCreatedPayload{
		AccountID:   a.AccountID,
		Name:        a.Name,
		Type:        string(a.Type),
		PortfolioID: a.PortfolioID,
		InitialNAV:  a.InitialNAV,
		Currency:    a.Currency,
		CreatedBy:   "pms.account_manager",
	}
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregateAccount,
		AggregateID:   a.AccountID,
		EventType:     EventAccountCreated,
		AccountID:     a.AccountID,
		Payload:       payload,
		Source:        "pms.accounts",
		CreatedAt:     a.CreatedAt,
	})
	if err != nil {
		return nil, err
	}
	if m.store != nil {
		m.store.Append(ctx, ev) //nolint:errcheck
	}
	m.accounts[a.AccountID] = &a
	return &a, nil
}

// Get returns the account for the given ID.
func (m *AccountManager) Get(accountID string) (*ManagedAccount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.accounts[accountID]
	if !ok {
		return nil, ErrAccountNotFound
	}
	return a, nil
}

// Snapshot returns a read-only view of one account.
func (m *AccountManager) Snapshot(accountID string) (AccountSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.accounts[accountID]
	if !ok {
		return AccountSnapshot{}, ErrAccountNotFound
	}
	return AccountSnapshot{
		AccountID:         a.AccountID,
		Name:              a.Name,
		Type:              a.Type,
		PortfolioID:       a.PortfolioID,
		ParentID:          a.ParentID,
		Status:            a.Status,
		InitialNAV:        a.InitialNAV,
		CurrentNAV:        a.CurrentNAV,
		AvailableCash:     a.AvailableCash,
		AllocatedUSD:      a.AllocatedUSD,
		RealisedPnLUSD:    a.RealisedPnLUSD,
		UnrealisedPnLUSD:  a.UnrealisedPnLUSD,
		OpenPositionCount: a.OpenPositionCount,
		TotalTrades:       a.TotalTrades,
		UpdatedAt:         a.UpdatedAt,
	}, nil
}

// ReserveCapital reserves USD for a position open. Returns ErrInsufficientFunds
// if the account doesn't have enough available cash.
// Cross-account access is prohibited; callers must use the correct accountID.
func (m *AccountManager) ReserveCapital(accountID string, usd float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[accountID]
	if !ok {
		return ErrAccountNotFound
	}
	if a.Status == AccountStatusClosed {
		return ErrAccountClosed
	}
	if a.Status == AccountStatusSuspended {
		return ErrAccountSuspended
	}
	if a.AvailableCash < usd {
		return fmt.Errorf("%w: account=%s available=$%.2f requested=$%.2f",
			ErrInsufficientFunds, accountID, a.AvailableCash, usd)
	}
	a.AvailableCash -= usd
	a.AllocatedUSD += usd
	a.OpenPositionCount++
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// ReleaseCapital releases reserved capital back to available cash on position close.
func (m *AccountManager) ReleaseCapital(accountID string, reservedUSD, realisedPnLUSD float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[accountID]
	if !ok {
		return
	}
	a.AllocatedUSD = max64(0, a.AllocatedUSD-reservedUSD)
	a.AvailableCash += reservedUSD + realisedPnLUSD
	a.RealisedPnLUSD += realisedPnLUSD
	if a.OpenPositionCount > 0 {
		a.OpenPositionCount--
	}
	a.TotalTrades++
	a.CurrentNAV = a.AvailableCash + a.AllocatedUSD + a.UnrealisedPnLUSD
	a.UpdatedAt = time.Now().UTC()
}

// UpdateUnrealisedPnL refreshes the mark-to-market PnL for an account.
func (m *AccountManager) UpdateUnrealisedPnL(accountID string, unrealisedUSD float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[accountID]
	if !ok {
		return
	}
	a.UnrealisedPnLUSD = unrealisedUSD
	a.CurrentNAV = a.AvailableCash + a.AllocatedUSD + unrealisedUSD
	a.UpdatedAt = time.Now().UTC()
}

// SuspendAccount suspends trading for an account.
func (m *AccountManager) SuspendAccount(ctx context.Context, accountID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[accountID]
	if !ok {
		return ErrAccountNotFound
	}
	a.Status = AccountStatusSuspended
	a.UpdatedAt = time.Now().UTC()

	payload := map[string]string{"account_id": accountID, "reason": reason}
	ev, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregateAccount,
		AggregateID:   accountID,
		EventType:     EventAccountUpdated,
		AccountID:     accountID,
		Payload:       payload,
		Source:        "pms.accounts",
	})
	if m.store != nil {
		m.store.Append(ctx, ev) //nolint:errcheck
	}
	return nil
}

// CloseAccount closes an account, preventing any further trading.
func (m *AccountManager) CloseAccount(ctx context.Context, accountID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[accountID]
	if !ok {
		return ErrAccountNotFound
	}
	a.Status = AccountStatusClosed
	a.UpdatedAt = time.Now().UTC()

	payload := AccountClosedPayload{
		AccountID:   accountID,
		PortfolioID: a.PortfolioID,
		FinalNAV:    a.CurrentNAV,
		Reason:      reason,
	}
	ev, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregateAccount,
		AggregateID:   accountID,
		EventType:     EventAccountClosed,
		AccountID:     accountID,
		Payload:       payload,
		Source:        "pms.accounts",
	})
	if m.store != nil {
		m.store.Append(ctx, ev) //nolint:errcheck
	}
	return nil
}

// AllSnapshots returns snapshots of all managed accounts.
func (m *AccountManager) AllSnapshots() []AccountSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AccountSnapshot, 0, len(m.accounts))
	for _, a := range m.accounts {
		out = append(out, AccountSnapshot{
			AccountID:         a.AccountID,
			Name:              a.Name,
			Type:              a.Type,
			PortfolioID:       a.PortfolioID,
			ParentID:          a.ParentID,
			Status:            a.Status,
			InitialNAV:        a.InitialNAV,
			CurrentNAV:        a.CurrentNAV,
			AvailableCash:     a.AvailableCash,
			AllocatedUSD:      a.AllocatedUSD,
			RealisedPnLUSD:    a.RealisedPnLUSD,
			UnrealisedPnLUSD:  a.UnrealisedPnLUSD,
			OpenPositionCount: a.OpenPositionCount,
			TotalTrades:       a.TotalTrades,
			UpdatedAt:         a.UpdatedAt,
		})
	}
	return out
}

// SubAccountsOf returns all accounts whose ParentID equals the given masterID.
func (m *AccountManager) SubAccountsOf(masterID string) []AccountSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AccountSnapshot, 0)
	for _, a := range m.accounts {
		if a.ParentID == masterID {
			out = append(out, AccountSnapshot{
				AccountID:        a.AccountID,
				Name:             a.Name,
				Type:             a.Type,
				PortfolioID:      a.PortfolioID,
				ParentID:         a.ParentID,
				Status:           a.Status,
				CurrentNAV:       a.CurrentNAV,
				AvailableCash:    a.AvailableCash,
				RealisedPnLUSD:   a.RealisedPnLUSD,
				UnrealisedPnLUSD: a.UnrealisedPnLUSD,
				UpdatedAt:        a.UpdatedAt,
			})
		}
	}
	return out
}

// TotalNAV sums the current NAV across all active accounts.
func (m *AccountManager) TotalNAV() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := 0.0
	for _, a := range m.accounts {
		if a.Status == AccountStatusActive {
			total += a.CurrentNAV
		}
	}
	return total
}
