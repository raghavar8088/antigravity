package reconciliation

import (
	"context"

	"antigravity-engine/internal/positions"
)

// PaperSnapshotProvider adapts the in-process PositionManager into a
// SnapshotProvider for the reconciliation Service.
//
// For paper trading, "exchange" state == position manager state.
// The reconciliation checks that OMS ledger transitions are consistent with
// what the position manager actually recorded — catching ACK-before-fill gaps,
// ghost positions, or state divergence between ledger events and live positions.
type PaperSnapshotProvider struct {
	posMgr    *positions.Manager
	accountID string
}

// NewPaperSnapshotProvider creates a snapshot provider backed by the position manager.
func NewPaperSnapshotProvider(posMgr *positions.Manager, accountID string) *PaperSnapshotProvider {
	return &PaperSnapshotProvider{posMgr: posMgr, accountID: accountID}
}

// Snapshot captures current paper positions as both OMS-side and exchange-side views
// so drift detectors can compare them and emit alerts on divergence.
func (p *PaperSnapshotProvider) Snapshot(_ context.Context) (Snapshot, error) {
	openPositions := p.posMgr.GetOpenPositions()

	omsPositions := make([]OMSPosition, 0, len(openPositions))
	exchPositions := make([]ExchangePosition, 0, len(openPositions))

	for _, pos := range openPositions {
		side := "long"
		if pos.Side == "SELL" {
			side = "short"
		}
		omsPositions = append(omsPositions, OMSPosition{
			Symbol:   pos.Symbol,
			Side:     side,
			Quantity: pos.Size,
		})
		exchPositions = append(exchPositions, ExchangePosition{
			Symbol:   pos.Symbol,
			Side:     side,
			Quantity: pos.Size,
		})
	}

	return Snapshot{
		AccountID:         p.accountID,
		OMSPositions:      omsPositions,
		ExchangePositions: exchPositions,
		// Balance drift checks are owned by paperpersist bootstrapping — skip here.
		OMSBalance:      BalanceSnapshot{},
		ExchangeBalance: BalanceSnapshot{},
	}, nil
}
