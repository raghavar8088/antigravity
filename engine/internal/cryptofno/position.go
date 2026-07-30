package cryptofno

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Baskets and the balance gate.
//
// A basket is executed atomically: either every leg fills or none does. A
// partial fill on a hedged structure is the dangerous outcome — filling the two
// short legs of an iron condor and failing the wings leaves a naked strangle
// that the account was never margined for.

// Status of a basket.
type Status string

const (
	StatusOpen   Status = "OPEN"
	StatusClosed Status = "CLOSED"
)

// Position is one executed basket.
type Position struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"`
	// Underlying is the netting key ("BTC", "ETH"). Margin nets within it only.
	Underlying string     `json:"underlying"`
	Legs       []Leg      `json:"legs"`
	Status     Status     `json:"status"`
	OpenedAt   time.Time  `json:"openedAt"`
	ClosedAt   *time.Time `json:"closedAt,omitempty"`

	// EntrySpot is the underlying price at entry, kept so the position can be
	// re-margined later without needing a live feed for a historical basket.
	EntrySpot float64 `json:"entrySpot"`
	// MarginUSD is what this basket reserved at entry.
	MarginUSD float64 `json:"marginUsd"`
	// NetPremiumUSD is positive for a credit received, negative for a debit paid.
	NetPremiumUSD float64 `json:"netPremiumUsd"`
	// FeesUSD is the round-trip cost charged at entry.
	FeesUSD float64 `json:"feesUsd"`

	RealisedPnlUSD float64 `json:"realisedPnlUsd,omitempty"`
	ExitSpot       float64 `json:"exitSpot,omitempty"`
	ExitReason     string  `json:"exitReason,omitempty"`

	// Label is what the UI shows ("Iron Condor", "Short Strangle"), inferred
	// from the leg shape so a user recognises what they built.
	Label string `json:"label"`
}

// UnrealisedUSD marks the basket to a given spot.
func (p *Position) UnrealisedUSD(spot float64) float64 {
	if spot <= 0 || p.Status != StatusOpen {
		return 0
	}
	now := valueBasket(p.Legs, spot, 0, time.Now())
	entry := valueBasket(p.Legs, p.EntrySpot, 0, p.OpenedAt)
	return now - entry
}

// ErrInsufficientCapital is returned when a basket's margin exceeds what the
// account has free. The basket is NOT partially executed.
type ErrInsufficientCapital struct {
	RequiredUSD  float64
	AvailableUSD float64
	Shortfall    float64
}

func (e ErrInsufficientCapital) Error() string {
	return fmt.Sprintf(
		"insufficient paper capital: basket needs $%.2f margin, account has $%.2f available (short $%.2f)",
		e.RequiredUSD, e.AvailableUSD, e.Shortfall)
}

// PreviewBasket computes what a basket would cost WITHOUT executing it.
//
// This is what the order ticket shows before the user commits, and it must use
// the identical code path as execution — a preview that disagrees with the fill
// is worse than no preview.
func (b *Book) PreviewBasket(accountID, underlying string, legs []Leg, spot float64, feeRate float64) (MarginResult, AccountView, error) {
	if len(legs) == 0 {
		return MarginResult{}, AccountView{}, fmt.Errorf("basket has no legs")
	}
	if spot <= 0 {
		return MarginResult{}, AccountView{}, fmt.Errorf("no live spot for %s — refusing to price a basket blind", underlying)
	}

	b.mu.RLock()
	acct, ok := b.accounts[accountID]
	if !ok {
		b.mu.RUnlock()
		return MarginResult{}, AccountView{}, fmt.Errorf("account %s not found", accountID)
	}
	// The requirement is for the account's WHOLE book after adding this basket,
	// not for the basket alone: a new short that hedges an existing long should
	// cost less than it would standalone.
	combined := append(b.openLegsLocked(accountID, underlying), legs...)
	view := b.viewLocked(*acct, accountID, map[string]float64{underlying: spot})
	b.mu.RUnlock()

	after := PortfolioMargin(combined, spot, DefaultMarginParams)
	return after, view, nil
}

// openLegsLocked returns every open leg for one underlying in an account.
func (b *Book) openLegsLocked(accountID, underlying string) []Leg {
	var out []Leg
	for _, p := range b.openLocked(accountID) {
		if p.Underlying == underlying {
			out = append(out, p.Legs...)
		}
	}
	return out
}

// ExecuteBasket fills every leg or none.
//
// The balance gate is enforced here, against the requirement for the account's
// combined book. Refusing is the whole point: a paper desk that lets a user
// exceed their capital teaches a habit the real exchange will reject.
func (b *Book) ExecuteBasket(accountID, underlying string, legs []Leg, spot, feeRate float64) (*Position, error) {
	if len(legs) == 0 {
		return nil, fmt.Errorf("basket has no legs")
	}
	if spot <= 0 {
		return nil, fmt.Errorf("no live spot for %s — refusing to execute blind", underlying)
	}
	for i, l := range legs {
		if l.Lots <= 0 {
			return nil, fmt.Errorf("leg %d (%s) has %d lots", i, l.Symbol, l.Lots)
		}
		if l.PremiumPerBTC <= 0 {
			return nil, fmt.Errorf("leg %d (%s) has no live premium — refusing to fill unpriced", i, l.Symbol)
		}
		if l.ContractValue <= 0 {
			return nil, fmt.Errorf("leg %d (%s) has no contract value", i, l.Symbol)
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	acct, ok := b.accounts[accountID]
	if !ok {
		return nil, fmt.Errorf("account %s not found", accountID)
	}

	combined := append(b.openLegsLocked(accountID, underlying), legs...)
	after := PortfolioMargin(combined, spot, DefaultMarginParams)

	// Available must be recomputed here rather than reused from a preview: the
	// book may have moved between the two, and the gate has to bind on the
	// state at execution time.
	view := b.viewLocked(*acct, accountID, map[string]float64{underlying: spot})
	// The already-reserved margin for this underlying is being replaced by the
	// combined figure, so compare against capital freed of the old reservation.
	existing := 0.0
	if legsNow := b.openLegsLocked(accountID, underlying); len(legsNow) > 0 {
		existing = PortfolioMargin(legsNow, spot, DefaultMarginParams).RequiredUSD
	}
	freeForThis := view.AvailableUSD + existing

	if after.RequiredUSD > freeForThis {
		return nil, ErrInsufficientCapital{
			RequiredUSD:  after.RequiredUSD,
			AvailableUSD: freeForThis,
			Shortfall:    after.RequiredUSD - freeForThis,
		}
	}

	// Fees are charged on notional at entry, matching how Delta bills.
	fees := 0.0
	netPremium := 0.0
	for _, l := range legs {
		fees += spot * l.ContractValue * float64(l.Lots) * feeRate
		if l.Side == SideBuy {
			netPremium -= l.PremiumUSD()
		} else {
			netPremium += l.PremiumUSD()
		}
	}

	now := b.now()
	p := &Position{
		ID: b.nextID("FNO"), AccountID: accountID, Underlying: underlying,
		Legs: append([]Leg(nil), legs...), Status: StatusOpen, OpenedAt: now,
		EntrySpot: spot, MarginUSD: after.RequiredUSD,
		NetPremiumUSD: netPremium, FeesUSD: fees,
		Label: LabelFor(legs),
	}
	// Entry fees hit realised P&L immediately — they are paid whether or not the
	// basket ever profits, and hiding them until exit flatters every open book.
	acct.RealisedPnlUSD -= fees
	acct.UpdatedAt = now

	b.positions[accountID] = append(b.positions[accountID], p)
	return p, nil
}

// CloseBasket exits a basket at the given spot and books the result.
func (b *Book) CloseBasket(accountID, positionID string, spot, feeRate float64, reason string) (*Position, error) {
	if spot <= 0 {
		return nil, fmt.Errorf("no live spot — refusing to close blind")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	acct, ok := b.accounts[accountID]
	if !ok {
		return nil, fmt.Errorf("account %s not found", accountID)
	}
	for _, p := range b.positions[accountID] {
		if p.ID != positionID {
			continue
		}
		if p.Status != StatusOpen {
			return nil, fmt.Errorf("position %s is already %s", p.ID, p.Status)
		}

		gross := p.UnrealisedUSD(spot)
		exitFees := 0.0
		for _, l := range p.Legs {
			exitFees += spot * l.ContractValue * float64(l.Lots) * feeRate
		}

		now := b.now()
		p.Status = StatusClosed
		p.ClosedAt = &now
		p.ExitSpot = spot
		p.ExitReason = reason
		p.FeesUSD += exitFees
		p.RealisedPnlUSD = gross - exitFees

		acct.RealisedPnlUSD += p.RealisedPnlUSD
		acct.UpdatedAt = now
		return p, nil
	}
	return nil, fmt.Errorf("position %s not found in account %s", positionID, accountID)
}

// Positions returns an account's baskets, newest first.
func (b *Book) Positions(accountID string, openOnly bool) []Position {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]Position, 0, len(b.positions[accountID]))
	for _, p := range b.positions[accountID] {
		if openOnly && p.Status != StatusOpen {
			continue
		}
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenedAt.After(out[j].OpenedAt) })
	return out
}

// LabelFor names a basket from its leg shape, so the UI shows "Iron Condor"
// rather than four rows the user has to decode.
func LabelFor(legs []Leg) string {
	if len(legs) == 0 {
		return ""
	}
	if len(legs) == 1 {
		l := legs[0]
		verb := "Long"
		if l.Side == SideSell {
			verb = "Short"
		}
		return fmt.Sprintf("%s %s", verb, strings.Title(strings.ToLower(string(l.Type))))
	}

	var shortCalls, shortPuts, longCalls, longPuts int
	for _, l := range legs {
		switch {
		case l.Type == TypeCall && l.Side == SideSell:
			shortCalls++
		case l.Type == TypeCall && l.Side == SideBuy:
			longCalls++
		case l.Type == TypePut && l.Side == SideSell:
			shortPuts++
		case l.Type == TypePut && l.Side == SideBuy:
			longPuts++
		}
	}

	switch {
	case shortCalls > 0 && shortPuts > 0 && longCalls > 0 && longPuts > 0:
		return "Iron Condor"
	case shortCalls > 0 && shortPuts > 0:
		return "Short Strangle"
	case longCalls > 0 && longPuts > 0 && shortCalls == 0 && shortPuts == 0:
		return "Long Strangle"
	case shortCalls > 0 && longCalls > 0:
		return "Call Spread"
	case shortPuts > 0 && longPuts > 0:
		return "Put Spread"
	default:
		return fmt.Sprintf("%d-leg basket", len(legs))
	}
}
