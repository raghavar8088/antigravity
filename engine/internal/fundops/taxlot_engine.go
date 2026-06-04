// Phase 20E — Tax Lot Accounting
// Tracks every position with acquisition date/price for FIFO/LIFO/AvgCost/SpecID.
// Every fill is linked to a tax lot. Replayable from events.
package fundops

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ─── Cost Basis Methods ───────────────────────────────────────────────────────

type CostBasisMethod string

const (
	MethodFIFO             CostBasisMethod = "FIFO"
	MethodLIFO             CostBasisMethod = "LIFO"
	MethodAverageCost      CostBasisMethod = "AVERAGE_COST"
	MethodSpecificIdentify CostBasisMethod = "SPECIFIC_IDENTIFY"
)

// LongTermDays is the minimum holding period for long-term capital gains treatment.
const LongTermDays = 365

// ─── Lot ──────────────────────────────────────────────────────────────────────

// Lot is an open tax lot (a tranche of shares with specific cost basis).
type Lot struct {
	LotID           string
	FundID          string
	InvestorID      string
	Symbol          string
	OriginalQty     float64
	RemainingQty    float64
	CostPerUnit     float64 // cost basis per unit/share
	AcquisitionDate time.Time
	FillID          string
}

// ─── Tax Lot Engine ───────────────────────────────────────────────────────────

// TaxLotEngine manages tax lot accounting for a fund using configurable cost basis method.
type TaxLotEngine struct {
	method    CostBasisMethod
	store     EventStore
	fundID    string
	openLots  map[string][]*Lot // symbol → open lots
}

// NewTaxLotEngine creates a tax lot engine with the given cost basis method.
func NewTaxLotEngine(store EventStore, fundID string, method CostBasisMethod) *TaxLotEngine {
	return &TaxLotEngine{
		method:   method,
		store:    store,
		fundID:   fundID,
		openLots: make(map[string][]*Lot),
	}
}

// OpenLot records a new acquisition (purchase fill) as a tax lot.
func (e *TaxLotEngine) OpenLot(ctx context.Context, investorID, symbol, fillID string, qty, price float64, acqDate time.Time) (*Lot, error) {
	if qty <= 0 {
		return nil, errors.New("taxlot: quantity must be positive")
	}
	if price <= 0 {
		return nil, errors.New("taxlot: cost basis must be positive")
	}

	lotID := fmt.Sprintf("lot_%s_%d", symbol, time.Now().UnixNano())
	lot := &Lot{
		LotID: lotID, FundID: e.fundID, InvestorID: investorID,
		Symbol: symbol, OriginalQty: qty, RemainingQty: qty,
		CostPerUnit: price, AcquisitionDate: acqDate, FillID: fillID,
	}

	// Persist event.
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggTaxLot,
		AggregateID:   lotID,
		FundID:        e.fundID,
		EventType:     EvtTaxLotCreated,
		Payload: TaxLotCreatedPayload{
			LotID: lotID, FundID: e.fundID, InvestorID: investorID,
			Symbol: symbol, Quantity: qty,
			CostBasisUSD:    qty * price,
			AcquisitionDate: acqDate, FillID: fillID,
		},
	})
	if err != nil {
		return nil, err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return nil, fmt.Errorf("taxlot: persist OpenLot: %w", err)
	}

	// Apply average cost adjustment if using average cost method.
	if e.method == MethodAverageCost {
		e.openLots[symbol] = e.mergeIntoAvgCost(symbol, lot)
	} else {
		e.openLots[symbol] = append(e.openLots[symbol], lot)
	}
	return lot, nil
}

// CloseLot processes a sale/disposal using the configured cost basis method.
// Returns the realised gain/loss records.
func (e *TaxLotEngine) CloseLot(ctx context.Context, symbol string, saleQty, salePrice float64, saleDate time.Time, specificLotID string) ([]TaxLotClosedPayload, error) {
	if saleQty <= 0 {
		return nil, errors.New("taxlot: sale quantity must be positive")
	}
	lots := e.openLots[symbol]
	if len(lots) == 0 {
		return nil, fmt.Errorf("taxlot: no open lots for symbol %s", symbol)
	}

	// Sort lots per method.
	e.sortLots(lots)
	if e.method == MethodSpecificIdentify {
		lots = e.filterToLot(lots, specificLotID)
	}

	var results []TaxLotClosedPayload
	remaining := saleQty

	for i := 0; i < len(lots) && remaining > 1e-9; i++ {
		lot := lots[i]
		if lot.RemainingQty <= 1e-9 {
			continue
		}
		closeQty := lot.RemainingQty
		if closeQty > remaining {
			closeQty = remaining
		}

		costBasis := closeQty * lot.CostPerUnit
		proceeds := closeQty * salePrice
		gain := proceeds - costBasis
		holdingDays := int(saleDate.Sub(lot.AcquisitionDate).Hours() / 24)
		isLongTerm := holdingDays >= LongTermDays

		result := TaxLotClosedPayload{
			LotID: lot.LotID, FundID: e.fundID, Symbol: symbol,
			QuantityClosed: closeQty, CostBasisUSD: costBasis,
			ProceedsUSD: proceeds, RealizedGainUSD: gain,
			HoldingDays: holdingDays, IsLongTerm: isLongTerm, ClosedDate: saleDate,
		}

		// Determine event type: full close or partial.
		evType := EvtTaxLotClosed
		if closeQty < lot.RemainingQty {
			evType = EvtTaxLotPartial
		}
		ev, err := NewFundEvent(NewEventInput{
			AggregateType: AggTaxLot,
			AggregateID:   lot.LotID,
			FundID:        e.fundID,
			EventType:     evType,
			Payload:       result,
		})
		if err != nil {
			return nil, err
		}
		if _, err := e.store.Append(ctx, ev); err != nil {
			return nil, fmt.Errorf("taxlot: persist CloseLot: %w", err)
		}

		lot.RemainingQty -= closeQty
		remaining -= closeQty
		results = append(results, result)
	}

	if remaining > 1e-9 {
		return results, fmt.Errorf("taxlot: insufficient lots — %.6f qty unmatched for %s", remaining, symbol)
	}

	// Remove exhausted lots.
	e.openLots[symbol] = e.filterExhausted(e.openLots[symbol])
	return results, nil
}

// TotalRealizedGain returns the sum of all realised gains/losses for a symbol.
func (e *TaxLotEngine) TotalRealizedGain(ctx context.Context, symbol string) (float64, error) {
	evts, err := e.store.Replay(ctx, AggTaxLot, e.fundID)
	if err != nil {
		return 0, err
	}
	total := 0.0
	for _, ev := range evts {
		if ev.EventType == EvtTaxLotClosed || ev.EventType == EvtTaxLotPartial {
			var p TaxLotClosedPayload
			if unmarshal(ev.Payload, &p) == nil && p.Symbol == symbol {
				total += p.RealizedGainUSD
			}
		}
	}
	return total, nil
}

// OpenLots returns all open tax lots for a symbol.
func (e *TaxLotEngine) OpenLots(symbol string) []*Lot {
	lots := e.openLots[symbol]
	out := make([]*Lot, 0, len(lots))
	for _, lot := range lots {
		if lot.RemainingQty > 1e-9 {
			cp := *lot
			out = append(out, &cp)
		}
	}
	return out
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (e *TaxLotEngine) sortLots(lots []*Lot) {
	switch e.method {
	case MethodFIFO:
		sort.Slice(lots, func(i, j int) bool {
			return lots[i].AcquisitionDate.Before(lots[j].AcquisitionDate)
		})
	case MethodLIFO:
		sort.Slice(lots, func(i, j int) bool {
			return lots[i].AcquisitionDate.After(lots[j].AcquisitionDate)
		})
	}
}

func (e *TaxLotEngine) filterToLot(lots []*Lot, lotID string) []*Lot {
	for _, l := range lots {
		if l.LotID == lotID {
			return []*Lot{l}
		}
	}
	return lots // fallback to FIFO if specific lot not found
}

func (e *TaxLotEngine) filterExhausted(lots []*Lot) []*Lot {
	out := lots[:0]
	for _, l := range lots {
		if l.RemainingQty > 1e-9 {
			out = append(out, l)
		}
	}
	return out
}

// mergeIntoAvgCost handles average cost method by merging new lots with existing.
func (e *TaxLotEngine) mergeIntoAvgCost(symbol string, newLot *Lot) []*Lot {
	existing := e.openLots[symbol]
	if len(existing) == 0 {
		return []*Lot{newLot}
	}
	// Compute new average cost.
	totalQty := newLot.RemainingQty
	totalCost := newLot.RemainingQty * newLot.CostPerUnit
	for _, lot := range existing {
		totalQty += lot.RemainingQty
		totalCost += lot.RemainingQty * lot.CostPerUnit
	}
	avgCost := totalCost / totalQty
	// Collapse to a single lot with the earliest acquisition date.
	earliest := newLot.AcquisitionDate
	for _, lot := range existing {
		if lot.AcquisitionDate.Before(earliest) {
			earliest = lot.AcquisitionDate
		}
	}
	merged := &Lot{
		LotID:           existing[0].LotID,
		FundID:          e.fundID,
		InvestorID:      existing[0].InvestorID,
		Symbol:          symbol,
		OriginalQty:     totalQty,
		RemainingQty:    totalQty,
		CostPerUnit:     avgCost,
		AcquisitionDate: earliest,
	}
	return []*Lot{merged}
}
