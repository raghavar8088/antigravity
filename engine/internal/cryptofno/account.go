package cryptofno

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Named paper accounts for the crypto F&O desk.
//
// Each account is an independent book: its own capital, its own positions, its
// own margin. Nothing nets across accounts — two accounts holding opposite legs
// are not hedged, because a real broker would not net them either.
//
// Capital is USD because Delta settles in USD. The Indian F&O desk this mirrors
// defaults to Rs 1 crore; the crypto equivalent is $100,000, and like the Indian
// module it is editable at any time.

// DefaultAccountCapitalUSD is the starting balance for a new account.
const DefaultAccountCapitalUSD = 100_000.0

// Account is one named paper book.
type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// InitialCapitalUSD is what the account was funded with. Editing it adjusts
	// the book by the DIFFERENCE rather than resetting cash, so an account with
	// open positions can be topped up without silently erasing its P&L.
	InitialCapitalUSD float64 `json:"initialCapitalUsd"`
	// RealisedPnlUSD accumulates closed-position results.
	RealisedPnlUSD float64 `json:"realisedPnlUsd"`
}

// AccountView is an account plus its live derived figures.
type AccountView struct {
	Account
	// MarginUsedUSD is the portfolio requirement across all open baskets.
	MarginUsedUSD float64 `json:"marginUsedUsd"`
	// AvailableUSD is what a new basket may consume.
	AvailableUSD float64 `json:"availableUsd"`
	// UnrealisedPnlUSD marks open positions to the live chain.
	UnrealisedPnlUSD float64 `json:"unrealisedPnlUsd"`
	// EquityUSD is capital + realised + unrealised.
	EquityUSD    float64 `json:"equityUsd"`
	OpenBaskets  int     `json:"openBaskets"`
	OpenPosition int     `json:"openPositions"`
}

// Book holds every account and its positions.
type Book struct {
	mu       sync.RWMutex
	accounts map[string]*Account
	// positions are keyed by account ID.
	positions map[string][]*Position
	seq       int
	now       func() time.Time
}

// NewBook creates an empty book with one default account, so the desk is usable
// immediately rather than presenting an empty state that cannot be traded.
func NewBook() *Book {
	b := &Book{
		accounts:  map[string]*Account{},
		positions: map[string][]*Position{},
		now:       func() time.Time { return time.Now().UTC() },
	}
	_, _ = b.CreateAccount("Default", DefaultAccountCapitalUSD)
	return b
}

func (b *Book) nextID(prefix string) string {
	b.seq++
	return fmt.Sprintf("%s-%04d", prefix, b.seq)
}

// CreateAccount opens a new named book.
func (b *Book) CreateAccount(name string, capitalUSD float64) (Account, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Account{}, fmt.Errorf("account name is required")
	}
	if capitalUSD <= 0 {
		capitalUSD = DefaultAccountCapitalUSD
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, a := range b.accounts {
		if strings.EqualFold(a.Name, name) {
			return Account{}, fmt.Errorf("an account named %q already exists", name)
		}
	}

	now := b.now()
	a := &Account{
		ID: b.nextID("ACC"), Name: name,
		CreatedAt: now, UpdatedAt: now,
		InitialCapitalUSD: capitalUSD,
	}
	b.accounts[a.ID] = a
	return *a, nil
}

// EditAccount renames an account and/or changes its capital.
//
// Capital is adjusted, not reset. An account that has traded carries realised
// P&L; overwriting the balance with a new number would erase that record while
// appearing to be a simple top-up. Passing 0 leaves capital unchanged, so a
// rename cannot accidentally re-fund the book.
func (b *Book) EditAccount(id, newName string, newCapitalUSD float64) (Account, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	a, ok := b.accounts[id]
	if !ok {
		return Account{}, fmt.Errorf("account %s not found", id)
	}

	if n := strings.TrimSpace(newName); n != "" && !strings.EqualFold(n, a.Name) {
		for _, other := range b.accounts {
			if other.ID != id && strings.EqualFold(other.Name, n) {
				return Account{}, fmt.Errorf("an account named %q already exists", n)
			}
		}
		a.Name = n
	}

	if newCapitalUSD > 0 {
		// Reducing capital below what open positions already reserve would put
		// the book instantly short of margin, so it is refused rather than
		// silently creating an under-margined account.
		used := b.marginUsedLocked(id)
		if newCapitalUSD < used {
			return Account{}, fmt.Errorf(
				"cannot set capital to $%.2f: open positions already reserve $%.2f — close positions first",
				newCapitalUSD, used)
		}
		a.InitialCapitalUSD = newCapitalUSD
	}

	a.UpdatedAt = b.now()
	return *a, nil
}

// DeleteAccount removes an account. Refuses while positions are open, because
// deleting the book that owns them would orphan the positions rather than close
// them.
func (b *Book) DeleteAccount(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.accounts[id]; !ok {
		return fmt.Errorf("account %s not found", id)
	}
	if n := len(b.openLocked(id)); n > 0 {
		return fmt.Errorf("account has %d open position(s) — close them before deleting", n)
	}
	if len(b.accounts) == 1 {
		return fmt.Errorf("cannot delete the last account")
	}
	delete(b.accounts, id)
	delete(b.positions, id)
	return nil
}

// ResetAccount clears every position and P&L, optionally re-funding.
func (b *Book) ResetAccount(id string, capitalUSD float64) (Account, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	a, ok := b.accounts[id]
	if !ok {
		return Account{}, fmt.Errorf("account %s not found", id)
	}
	if capitalUSD > 0 {
		a.InitialCapitalUSD = capitalUSD
	}
	a.RealisedPnlUSD = 0
	a.UpdatedAt = b.now()
	b.positions[id] = nil
	return *a, nil
}

// Accounts lists every account, newest last, with live figures.
func (b *Book) Accounts(spot map[string]float64) []AccountView {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]AccountView, 0, len(b.accounts))
	for id, a := range b.accounts {
		out = append(out, b.viewLocked(*a, id, spot))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// AccountView returns one account's live figures.
func (b *Book) AccountView(id string, spot map[string]float64) (AccountView, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	a, ok := b.accounts[id]
	if !ok {
		return AccountView{}, fmt.Errorf("account %s not found", id)
	}
	return b.viewLocked(*a, id, spot), nil
}

func (b *Book) viewLocked(a Account, id string, spot map[string]float64) AccountView {
	used := b.marginUsedLocked(id)
	open := b.openLocked(id)

	unrealised := 0.0
	for _, p := range open {
		unrealised += p.UnrealisedUSD(spot[p.Underlying])
	}

	v := AccountView{Account: a}
	v.MarginUsedUSD = used
	v.UnrealisedPnlUSD = unrealised
	v.EquityUSD = a.InitialCapitalUSD + a.RealisedPnlUSD + unrealised
	// Available is measured against EQUITY, not the original funding: an account
	// that has lost money must not keep its original buying power, and one that
	// has gained should be able to use the gain.
	v.AvailableUSD = v.EquityUSD - used
	if v.AvailableUSD < 0 {
		v.AvailableUSD = 0
	}
	v.OpenBaskets = len(open)
	for _, p := range open {
		v.OpenPosition += len(p.Legs)
	}
	return v
}

// marginUsedLocked sums the portfolio requirement across open baskets, grouped
// by underlying so hedges net within an underlying but never across them.
func (b *Book) marginUsedLocked(accountID string) float64 {
	open := b.openLocked(accountID)
	if len(open) == 0 {
		return 0
	}

	// All open legs for this account, grouped by underlying. Grouping across
	// BASKETS matters: two separately-entered baskets on the same underlying do
	// hedge each other in a real book, and charging them independently would
	// overstate the requirement.
	byUnderlying := map[string][]Leg{}
	spotOf := map[string]float64{}
	for _, p := range open {
		byUnderlying[p.Underlying] = append(byUnderlying[p.Underlying], p.Legs...)
		spotOf[p.Underlying] = p.EntrySpot
	}

	total := 0.0
	for u, legs := range byUnderlying {
		total += PortfolioMargin(legs, spotOf[u], DefaultMarginParams).RequiredUSD
	}
	return total
}

func (b *Book) openLocked(accountID string) []*Position {
	out := make([]*Position, 0, len(b.positions[accountID]))
	for _, p := range b.positions[accountID] {
		if p.Status == StatusOpen {
			out = append(out, p)
		}
	}
	return out
}
