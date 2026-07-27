package reconciliationv2

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// isDeltaOptionSymbol reports whether a Delta symbol is a BTC/ETH option
// (e.g. C-BTC-64800-290726, P-BTC-...). These belong to the Live Engine's
// real-money option-buying module and are reconciled there, not against the
// paper perp/spot OMS — so this detector must ignore them.
func isDeltaOptionSymbol(symbol string) bool {
	s := strings.ToUpper(symbol)
	return strings.HasPrefix(s, "C-BTC-") || strings.HasPrefix(s, "P-BTC-") ||
		strings.HasPrefix(s, "C-ETH-") || strings.HasPrefix(s, "P-ETH-")
}

// Mismatch is a single detected discrepancy between exchange state and internal state.
type Mismatch struct {
	ID             string
	Domain         MismatchDomain
	Type           string
	Severity       DriftSeverity
	Symbol         string
	Reference      string
	ExchangeVal    string
	InternalVal    string
	DriftPct       float64
	Message        string
	AutoRepairable bool
	RepairHint     string
	DetectedAt     time.Time
}

// absDrift returns |a-b| / max(|a|, |b|, epsilon) * 100 as a percentage.
func absDrift(a, b float64) float64 {
	diff := math.Abs(a - b)
	if diff == 0 {
		return 0
	}
	denom := math.Max(math.Abs(a), math.Max(math.Abs(b), 1e-9))
	return (diff / denom) * 100.0
}

// classifySeverity maps a drift percentage to a severity level.
// Ghost/missing items must use classifySeverityAlways which returns at least HIGH.
func classifySeverity(driftPct float64) DriftSeverity {
	switch {
	case driftPct >= 1.0:
		return SeverityCritical
	case driftPct >= 0.1:
		return SeverityHigh
	case driftPct >= 0.01:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// ─── Balance Drift Detector ───────────────────────────────────────────────────

// BalanceDriftDetector detects discrepancies between exchange and OMS balances.
type BalanceDriftDetector struct{}

// Detect compares exchange asset balances against OMS balance snapshot.
// Equity, available margin, and margin-used are each checked independently.
func (d *BalanceDriftDetector) Detect(
	exchangeBalances []AssetBalance,
	oms OMSBalanceSnapshot,
	now time.Time,
) []Mismatch {
	var mismatches []Mismatch

	// Aggregate exchange totals across all assets
	var exchEquity, exchAvailable, exchMargin, exchUnreal float64
	for _, b := range exchangeBalances {
		exchEquity += b.EquityUSD
		exchAvailable += b.Available
		exchMargin += b.MarginUsed
		exchUnreal += b.UnrealizedPnL
	}

	if driftPct := absDrift(exchEquity, oms.EquityUSD); driftPct > 0 {
		mismatches = append(mismatches, Mismatch{
			ID:             newMismatchID(),
			Domain:         DomainBalance,
			Type:           "equity_drift",
			Severity:       classifySeverity(driftPct),
			ExchangeVal:    fmt.Sprintf("%.6f", exchEquity),
			InternalVal:    fmt.Sprintf("%.6f", oms.EquityUSD),
			DriftPct:       driftPct,
			Message:        fmt.Sprintf("equity drift %.4f%% — exchange=%.2f OMS=%.2f", driftPct, exchEquity, oms.EquityUSD),
			AutoRepairable: driftPct < 0.1,
			RepairHint:     "projection_rebuild",
			DetectedAt:     now,
		})
	}

	if driftPct := absDrift(exchAvailable, oms.AvailableUSD); driftPct > 0 {
		mismatches = append(mismatches, Mismatch{
			ID:             newMismatchID(),
			Domain:         DomainBalance,
			Type:           "available_margin_drift",
			Severity:       classifySeverity(driftPct),
			ExchangeVal:    fmt.Sprintf("%.6f", exchAvailable),
			InternalVal:    fmt.Sprintf("%.6f", oms.AvailableUSD),
			DriftPct:       driftPct,
			Message:        fmt.Sprintf("available margin drift %.4f%% — exchange=%.2f OMS=%.2f", driftPct, exchAvailable, oms.AvailableUSD),
			AutoRepairable: driftPct < 0.1,
			RepairHint:     "projection_rebuild",
			DetectedAt:     now,
		})
	}

	if driftPct := absDrift(exchMargin, oms.MarginUsedUSD); driftPct > 0 {
		mismatches = append(mismatches, Mismatch{
			ID:             newMismatchID(),
			Domain:         DomainBalance,
			Type:           "margin_used_drift",
			Severity:       classifySeverity(driftPct),
			ExchangeVal:    fmt.Sprintf("%.6f", exchMargin),
			InternalVal:    fmt.Sprintf("%.6f", oms.MarginUsedUSD),
			DriftPct:       driftPct,
			Message:        fmt.Sprintf("margin-used drift %.4f%% — exchange=%.2f OMS=%.2f", driftPct, exchMargin, oms.MarginUsedUSD),
			AutoRepairable: driftPct < 0.1,
			RepairHint:     "projection_rebuild",
			DetectedAt:     now,
		})
	}

	if driftPct := absDrift(exchUnreal, oms.UnrealizedPnL); driftPct > 0.5 {
		// Unrealised PnL drifts legitimately with mark price; only flag above threshold.
		mismatches = append(mismatches, Mismatch{
			ID:             newMismatchID(),
			Domain:         DomainPnL,
			Type:           "unrealized_pnl_drift",
			Severity:       classifySeverity(driftPct),
			ExchangeVal:    fmt.Sprintf("%.6f", exchUnreal),
			InternalVal:    fmt.Sprintf("%.6f", oms.UnrealizedPnL),
			DriftPct:       driftPct,
			Message:        fmt.Sprintf("unrealised PnL drift %.4f%% — exchange=%.2f OMS=%.2f", driftPct, exchUnreal, oms.UnrealizedPnL),
			AutoRepairable: true,
			RepairHint:     "projection_rebuild",
			DetectedAt:     now,
		})
	}

	return mismatches
}

// ─── Position Drift Detector ──────────────────────────────────────────────────

// PositionDriftDetector detects discrepancies between exchange and OMS positions.
type PositionDriftDetector struct{}

// Detect finds ghost positions, missing positions, and quantity/price/side mismatches.
func (d *PositionDriftDetector) Detect(
	exchangePositions []ExchangePosition,
	omsPositions []OMSPosition,
	now time.Time,
) []Mismatch {
	var mismatches []Mismatch

	// Index OMS positions by symbol+side for O(1) lookup.
	omsIdx := make(map[string]*OMSPosition, len(omsPositions))
	for i := range omsPositions {
		p := &omsPositions[i]
		if p.State == "OPEN" || p.State == "REDUCED" {
			key := positionSideKey(p.Symbol, p.Side)
			omsIdx[key] = p
		}
	}

	// Check exchange positions against OMS.
	seenKeys := make(map[string]bool)
	for _, ep := range exchangePositions {
		if ep.Quantity == 0 {
			continue // zero-size position = closed on exchange
		}
		if isDeltaOptionSymbol(ep.Symbol) {
			// Real BTC/ETH option positions belong to the Live Engine (buying-only
			// module), not this paper OMS which trades perps/spot. They are
			// reconciled by the Live Engine itself; treating them as ghosts here
			// halted the engine on every real fill. Skip them.
			continue
		}
		key := positionSideKey(ep.Symbol, ep.Side)
		seenKeys[key] = true

		omsPos, found := omsIdx[key]
		if !found {
			// Ghost position: exchange has a position OMS doesn't know about.
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainPosition,
				Type:           "ghost_position",
				Severity:       SeverityCritical,
				Symbol:         ep.Symbol,
				Reference:      key,
				ExchangeVal:    fmt.Sprintf("qty=%.6f side=%s price=%.2f", ep.Quantity, ep.Side, ep.EntryPrice),
				InternalVal:    "NOT_FOUND",
				DriftPct:       100.0,
				Message:        fmt.Sprintf("ghost position on exchange: %s %s qty=%.6f not in OMS", ep.Symbol, ep.Side, ep.Quantity),
				AutoRepairable: false,
				RepairHint:     "manual_investigation",
				DetectedAt:     now,
			})
			continue
		}

		// Check quantity.
		if driftPct := absDrift(ep.Quantity, omsPos.Quantity); driftPct > 0 {
			sev := SeverityHigh
			if driftPct >= 1.0 {
				sev = SeverityCritical
			}
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainPosition,
				Type:           "wrong_quantity",
				Severity:       sev,
				Symbol:         ep.Symbol,
				Reference:      omsPos.PositionID,
				ExchangeVal:    fmt.Sprintf("%.6f", ep.Quantity),
				InternalVal:    fmt.Sprintf("%.6f", omsPos.Quantity),
				DriftPct:       driftPct,
				Message:        fmt.Sprintf("position qty drift %.4f%% for %s: exchange=%.6f OMS=%.6f", driftPct, ep.Symbol, ep.Quantity, omsPos.Quantity),
				AutoRepairable: driftPct < 0.1,
				RepairHint:     "correction_event",
				DetectedAt:     now,
			})
		}

		// Check entry price.
		if driftPct := absDrift(ep.EntryPrice, omsPos.EntryPrice); driftPct > 0.05 {
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainPosition,
				Type:           "wrong_entry_price",
				Severity:       classifySeverity(driftPct),
				Symbol:         ep.Symbol,
				Reference:      omsPos.PositionID,
				ExchangeVal:    fmt.Sprintf("%.4f", ep.EntryPrice),
				InternalVal:    fmt.Sprintf("%.4f", omsPos.EntryPrice),
				DriftPct:       driftPct,
				Message:        fmt.Sprintf("entry price drift %.4f%% for %s: exchange=%.4f OMS=%.4f", driftPct, ep.Symbol, ep.EntryPrice, omsPos.EntryPrice),
				AutoRepairable: true,
				RepairHint:     "correction_event",
				DetectedAt:     now,
			})
		}
	}

	// Check for missing positions: OMS has open positions exchange doesn't confirm.
	for key, omsPos := range omsIdx {
		if seenKeys[key] {
			continue
		}
		mismatches = append(mismatches, Mismatch{
			ID:             newMismatchID(),
			Domain:         DomainPosition,
			Type:           "missing_position",
			Severity:       SeverityCritical,
			Symbol:         omsPos.Symbol,
			Reference:      omsPos.PositionID,
			ExchangeVal:    "NOT_FOUND",
			InternalVal:    fmt.Sprintf("qty=%.6f side=%s price=%.4f", omsPos.Quantity, omsPos.Side, omsPos.EntryPrice),
			DriftPct:       100.0,
			Message:        fmt.Sprintf("missing position: OMS has %s %s qty=%.6f but exchange doesn't", omsPos.Symbol, omsPos.Side, omsPos.Quantity),
			AutoRepairable: false,
			RepairHint:     "manual_investigation",
			DetectedAt:     now,
		})
	}

	// Check for duplicate OMS positions with same symbol+side (OMS data quality issue).
	seen := make(map[string]int)
	for _, p := range omsPositions {
		if p.State != "OPEN" && p.State != "REDUCED" {
			continue
		}
		k := positionSideKey(p.Symbol, p.Side)
		seen[k]++
		if seen[k] == 2 {
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainPosition,
				Type:           "duplicate_position",
				Severity:       SeverityCritical,
				Symbol:         p.Symbol,
				Reference:      k,
				ExchangeVal:    "N/A",
				InternalVal:    fmt.Sprintf("duplicate open positions for %s", k),
				DriftPct:       100.0,
				Message:        fmt.Sprintf("duplicate open OMS positions for %s", k),
				AutoRepairable: false,
				RepairHint:     "manual_investigation",
				DetectedAt:     now,
			})
		}
	}

	return mismatches
}

// ─── Order Drift Detector ─────────────────────────────────────────────────────

// OrderDriftDetector detects discrepancies between exchange and OMS orders.
type OrderDriftDetector struct{}

// Detect finds missing, duplicate, and status/quantity/fill mismatches in orders.
func (d *OrderDriftDetector) Detect(
	exchangeOrders []ExchangeOrder,
	omsOrders []OMSOrder,
	now time.Time,
) []Mismatch {
	var mismatches []Mismatch

	// Index OMS orders by ClientOrderID for O(1) lookup.
	omsByClient := make(map[string]*OMSOrder, len(omsOrders))
	for i := range omsOrders {
		omsByClient[omsOrders[i].ClientOrderID] = &omsOrders[i]
	}

	// Index OMS orders by ExchangeOrderID for cases where ClientOrderID is missing.
	omsByExchange := make(map[string]*OMSOrder, len(omsOrders))
	for i := range omsOrders {
		if omsOrders[i].ExchangeOrderID != "" {
			omsByExchange[omsOrders[i].ExchangeOrderID] = &omsOrders[i]
		}
	}

	seenClientIDs := make(map[string]bool)

	for _, eo := range exchangeOrders {
		// Resolve matching OMS order: prefer ClientOrderID, fall back to ExchangeOrderID.
		omsOrd := omsByClient[eo.ClientOrderID]
		if omsOrd == nil && eo.ExchangeOrderID != "" {
			omsOrd = omsByExchange[eo.ExchangeOrderID]
		}

		if omsOrd == nil {
			// Exchange has an order OMS doesn't track — this is a ghost order.
			// Only flag non-trivial statuses to avoid noise from old filled/cancelled orders.
			if eo.Status == "NEW" || eo.Status == "PARTIALLY_FILLED" {
				mismatches = append(mismatches, Mismatch{
					ID:             newMismatchID(),
					Domain:         DomainOrder,
					Type:           "ghost_order",
					Severity:       SeverityCritical,
					Symbol:         eo.Symbol,
					Reference:      eo.ExchangeOrderID,
					ExchangeVal:    fmt.Sprintf("status=%s qty=%.6f", eo.Status, eo.Quantity),
					InternalVal:    "NOT_FOUND",
					DriftPct:       100.0,
					Message:        fmt.Sprintf("ghost open order on exchange: %s %s not in OMS", eo.ExchangeOrderID, eo.Status),
					AutoRepairable: false,
					RepairHint:     "manual_investigation",
					DetectedAt:     now,
				})
			}
			continue
		}

		if eo.ClientOrderID != "" {
			seenClientIDs[eo.ClientOrderID] = true
		}

		// Status mismatch: exchange filled but OMS disagrees.
		if eo.Status == "FILLED" && omsOrd.State != "FILLED" && omsOrd.State != "PARTIALLY_FILLED" {
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainOrder,
				Type:           "wrong_status",
				Severity:       SeverityCritical,
				Symbol:         eo.Symbol,
				Reference:      eo.ClientOrderID,
				ExchangeVal:    "FILLED",
				InternalVal:    omsOrd.State,
				DriftPct:       100.0,
				Message:        fmt.Sprintf("order %s: exchange=FILLED but OMS=%s", eo.ClientOrderID, omsOrd.State),
				AutoRepairable: true,
				RepairHint:     "correction_event",
				DetectedAt:     now,
			})
		}

		// Filled quantity mismatch.
		if driftPct := absDrift(eo.FilledQuantity, omsOrd.FilledQuantity); driftPct > 0.01 {
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainOrder,
				Type:           "wrong_filled_qty",
				Severity:       classifySeverity(driftPct),
				Symbol:         eo.Symbol,
				Reference:      eo.ClientOrderID,
				ExchangeVal:    fmt.Sprintf("%.6f", eo.FilledQuantity),
				InternalVal:    fmt.Sprintf("%.6f", omsOrd.FilledQuantity),
				DriftPct:       driftPct,
				Message:        fmt.Sprintf("filled qty drift %.4f%% for %s: exchange=%.6f OMS=%.6f", driftPct, eo.ClientOrderID, eo.FilledQuantity, omsOrd.FilledQuantity),
				AutoRepairable: driftPct < 0.1,
				RepairHint:     "correction_event",
				DetectedAt:     now,
			})
		}

		// Average fill price mismatch.
		if eo.AverageFillPrice > 0 && omsOrd.AveragePrice > 0 {
			if driftPct := absDrift(eo.AverageFillPrice, omsOrd.AveragePrice); driftPct > 0.05 {
				mismatches = append(mismatches, Mismatch{
					ID:             newMismatchID(),
					Domain:         DomainOrder,
					Type:           "wrong_fill_price",
					Severity:       classifySeverity(driftPct),
					Symbol:         eo.Symbol,
					Reference:      eo.ClientOrderID,
					ExchangeVal:    fmt.Sprintf("%.4f", eo.AverageFillPrice),
					InternalVal:    fmt.Sprintf("%.4f", omsOrd.AveragePrice),
					DriftPct:       driftPct,
					Message:        fmt.Sprintf("avg fill price drift %.4f%% for %s", driftPct, eo.ClientOrderID),
					AutoRepairable: true,
					RepairHint:     "correction_event",
					DetectedAt:     now,
				})
			}
		}
	}

	// Find OMS open orders missing on exchange.
	for _, omsOrd := range omsOrders {
		if omsOrd.State != "SUBMITTED" && omsOrd.State != "ACKNOWLEDGED" && omsOrd.State != "PARTIALLY_FILLED" {
			continue
		}
		if seenClientIDs[omsOrd.ClientOrderID] {
			continue
		}
		if omsOrd.UpdatedAt.Before(time.Now().Add(-5 * time.Minute)) {
			// Orders older than 5 min that went missing need investigation.
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainOrder,
				Type:           "missing_order",
				Severity:       SeverityHigh,
				Symbol:         omsOrd.Symbol,
				Reference:      omsOrd.ClientOrderID,
				ExchangeVal:    "NOT_FOUND",
				InternalVal:    fmt.Sprintf("state=%s qty=%.6f", omsOrd.State, omsOrd.Quantity),
				DriftPct:       100.0,
				Message:        fmt.Sprintf("OMS order %s (state=%s) not found on exchange", omsOrd.ClientOrderID, omsOrd.State),
				AutoRepairable: false,
				RepairHint:     "manual_investigation",
				DetectedAt:     now,
			})
		}
	}

	return mismatches
}

// ─── Fill Drift Detector ──────────────────────────────────────────────────────

// FillDriftDetector detects fill discrepancies. Zero tolerance — fills must be exact.
type FillDriftDetector struct{}

// Detect finds missing, duplicate, and field-level fill mismatches.
func (d *FillDriftDetector) Detect(
	exchangeFills []ExchangeFill,
	omsFills []OMSFill,
	now time.Time,
) []Mismatch {
	var mismatches []Mismatch

	// Index OMS fills by FillID for exact matching.
	omsByFillID := make(map[string]*OMSFill, len(omsFills))
	omsByOrderID := make(map[string][]*OMSFill, len(omsFills))
	for i := range omsFills {
		f := &omsFills[i]
		if f.FillID != "" {
			omsByFillID[f.FillID] = f
		}
		omsByOrderID[f.ExchangeOrderID] = append(omsByOrderID[f.ExchangeOrderID], f)
	}

	seenFillIDs := make(map[string]bool)

	for _, ef := range exchangeFills {
		seenFillIDs[ef.FillID] = true

		omsF := omsByFillID[ef.FillID]
		if omsF == nil {
			// Missing fill: exchange executed a trade we didn't record.
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainFill,
				Type:           "missing_fill",
				Severity:       SeverityCritical,
				Symbol:         ef.Symbol,
				Reference:      ef.FillID,
				ExchangeVal:    fmt.Sprintf("price=%.4f qty=%.6f fee=%.6f", ef.Price, ef.Quantity, ef.FeeUSD),
				InternalVal:    "NOT_FOUND",
				DriftPct:       100.0,
				Message:        fmt.Sprintf("missing fill %s for order %s: price=%.4f qty=%.6f", ef.FillID, ef.ExchangeOrderID, ef.Price, ef.Quantity),
				AutoRepairable: false,
				RepairHint:     "manual_investigation",
				DetectedAt:     now,
			})
			continue
		}

		// Price exact match (fills must match exactly per institutional requirement).
		if driftPct := absDrift(ef.Price, omsF.Price); driftPct > 0 {
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainFill,
				Type:           "wrong_fill_price",
				Severity:       SeverityCritical,
				Symbol:         ef.Symbol,
				Reference:      ef.FillID,
				ExchangeVal:    fmt.Sprintf("%.6f", ef.Price),
				InternalVal:    fmt.Sprintf("%.6f", omsF.Price),
				DriftPct:       driftPct,
				Message:        fmt.Sprintf("fill price mismatch for %s: exchange=%.6f OMS=%.6f", ef.FillID, ef.Price, omsF.Price),
				AutoRepairable: false,
				RepairHint:     "manual_investigation",
				DetectedAt:     now,
			})
		}

		// Quantity exact match.
		if driftPct := absDrift(ef.Quantity, omsF.Quantity); driftPct > 0 {
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainFill,
				Type:           "wrong_fill_qty",
				Severity:       SeverityCritical,
				Symbol:         ef.Symbol,
				Reference:      ef.FillID,
				ExchangeVal:    fmt.Sprintf("%.6f", ef.Quantity),
				InternalVal:    fmt.Sprintf("%.6f", omsF.Quantity),
				DriftPct:       driftPct,
				Message:        fmt.Sprintf("fill qty mismatch for %s: exchange=%.6f OMS=%.6f", ef.FillID, ef.Quantity, omsF.Quantity),
				AutoRepairable: false,
				RepairHint:     "manual_investigation",
				DetectedAt:     now,
			})
		}

		// Fee mismatch (allow small rounding difference).
		if driftPct := absDrift(ef.FeeUSD, omsF.FeeUSD); driftPct > 1.0 {
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainFill,
				Type:           "wrong_fill_fee",
				Severity:       SeverityHigh,
				Symbol:         ef.Symbol,
				Reference:      ef.FillID,
				ExchangeVal:    fmt.Sprintf("%.6f", ef.FeeUSD),
				InternalVal:    fmt.Sprintf("%.6f", omsF.FeeUSD),
				DriftPct:       driftPct,
				Message:        fmt.Sprintf("fill fee mismatch for %s: exchange=%.6f OMS=%.6f", ef.FillID, ef.FeeUSD, omsF.FeeUSD),
				AutoRepairable: true,
				RepairHint:     "correction_event",
				DetectedAt:     now,
			})
		}
	}

	// Detect OMS fills with no matching exchange fill (duplicate/phantom fills).
	for _, omsF := range omsFills {
		if omsF.FillID != "" && !seenFillIDs[omsF.FillID] {
			if time.Since(omsF.Timestamp) > 10*time.Minute {
				mismatches = append(mismatches, Mismatch{
					ID:             newMismatchID(),
					Domain:         DomainFill,
					Type:           "phantom_fill",
					Severity:       SeverityHigh,
					Symbol:         omsF.Symbol,
					Reference:      omsF.FillID,
					ExchangeVal:    "NOT_FOUND",
					InternalVal:    fmt.Sprintf("price=%.4f qty=%.6f", omsF.Price, omsF.Quantity),
					DriftPct:       100.0,
					Message:        fmt.Sprintf("OMS has fill %s not confirmed by exchange (after 10m)", omsF.FillID),
					AutoRepairable: false,
					RepairHint:     "manual_investigation",
					DetectedAt:     now,
				})
			}
		}
	}

	return mismatches
}

// ─── Exposure Drift Detector ──────────────────────────────────────────────────

// ExposureDriftDetector detects portfolio exposure discrepancies.
type ExposureDriftDetector struct{}

// Detect compares exchange positions' gross/net notional against OMS snapshot.
func (d *ExposureDriftDetector) Detect(
	exchangePositions []ExchangePosition,
	omsSnapshot OMSSnapshot,
	riskEngineGross float64,
	now time.Time,
) []Mismatch {
	var mismatches []Mismatch

	// Compute exchange-side gross and net notional.
	var exchLong, exchShort float64
	for _, ep := range exchangePositions {
		if ep.Quantity == 0 {
			continue
		}
		notional := math.Abs(ep.NotionalUSD)
		if notional == 0 {
			notional = ep.Quantity * ep.MarkPrice
		}
		switch ep.Side {
		case "LONG", "BUY":
			exchLong += notional
		case "SHORT", "SELL":
			exchShort += notional
		default:
			if ep.Quantity > 0 {
				exchLong += notional
			} else {
				exchShort += math.Abs(notional)
			}
		}
	}
	exchGross := exchLong + exchShort
	exchNet := math.Abs(exchLong - exchShort)

	if driftPct := absDrift(exchGross, omsSnapshot.GrossNotional); driftPct > 0.1 {
		mismatches = append(mismatches, Mismatch{
			ID:             newMismatchID(),
			Domain:         DomainExposure,
			Type:           "gross_notional_drift",
			Severity:       classifySeverity(driftPct),
			ExchangeVal:    fmt.Sprintf("%.2f", exchGross),
			InternalVal:    fmt.Sprintf("%.2f", omsSnapshot.GrossNotional),
			DriftPct:       driftPct,
			Message:        fmt.Sprintf("gross notional drift %.4f%%: exchange=%.2f OMS=%.2f", driftPct, exchGross, omsSnapshot.GrossNotional),
			AutoRepairable: driftPct < 0.5,
			RepairHint:     "projection_rebuild",
			DetectedAt:     now,
		})
	}

	if driftPct := absDrift(exchNet, omsSnapshot.NetNotional); driftPct > 0.1 {
		mismatches = append(mismatches, Mismatch{
			ID:             newMismatchID(),
			Domain:         DomainExposure,
			Type:           "net_notional_drift",
			Severity:       classifySeverity(driftPct),
			ExchangeVal:    fmt.Sprintf("%.2f", exchNet),
			InternalVal:    fmt.Sprintf("%.2f", omsSnapshot.NetNotional),
			DriftPct:       driftPct,
			Message:        fmt.Sprintf("net notional drift %.4f%%: exchange=%.2f OMS=%.2f", driftPct, exchNet, omsSnapshot.NetNotional),
			AutoRepairable: driftPct < 0.5,
			RepairHint:     "projection_rebuild",
			DetectedAt:     now,
		})
	}

	// Cross-check against risk engine's reported gross exposure.
	if riskEngineGross > 0 {
		if driftPct := absDrift(exchGross, riskEngineGross); driftPct > 0.5 {
			mismatches = append(mismatches, Mismatch{
				ID:             newMismatchID(),
				Domain:         DomainExposure,
				Type:           "risk_engine_exposure_drift",
				Severity:       classifySeverity(driftPct),
				ExchangeVal:    fmt.Sprintf("%.2f", exchGross),
				InternalVal:    fmt.Sprintf("%.2f (risk engine)", riskEngineGross),
				DriftPct:       driftPct,
				Message:        fmt.Sprintf("risk engine gross exposure drift %.4f%%", driftPct),
				AutoRepairable: false,
				RepairHint:     "manual_investigation",
				DetectedAt:     now,
			})
		}
	}

	return mismatches
}

// ─── Drift Severity Scorer ────────────────────────────────────────────────────

// DriftSeverityScore aggregates all mismatches into a single numeric score [0, 100].
// 0 = clean, 100 = maximum drift.
func DriftSeverityScore(mismatches []Mismatch) float64 {
	if len(mismatches) == 0 {
		return 0
	}
	weights := map[DriftSeverity]float64{
		SeverityLow:      1,
		SeverityMedium:   5,
		SeverityHigh:     20,
		SeverityCritical: 50,
	}
	total := 0.0
	for _, m := range mismatches {
		total += weights[m.Severity]
	}
	if total > 100 {
		total = 100
	}
	return total
}
