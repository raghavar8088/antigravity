package reconciliationv2

import (
	"context"
	"log"

	"antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/positions"
)

const defaultReconAccountID = "btc-paper-1"

// WiredAuthorities holds production reconciliation authorities started by WireProduction.
type WiredAuthorities struct {
	Runtime *ExchangeReconciliationAuthority
	Delta   *ExchangeReconciliationAuthority
}

// WireProductionConfig holds optional production wiring parameters.
type WireProductionConfig struct {
	InitialBalanceUSD float64
	MarkPriceUSD      func() float64
}

// WireProduction boots reconciliationv2 authorities:
//   - Runtime authority: ledger OMS vs live position manager (always)
//   - Delta authority: ledger OMS vs Delta REST snapshots (when credentials present)
func WireProduction(
	ctx context.Context,
	store ledger.Store,
	posMgr *positions.Manager,
	equityUSD func() float64,
	ks *killswitch.Service,
	accountID string,
	cfg *WireProductionConfig,
) (*WiredAuthorities, error) {
	if accountID == "" {
		accountID = defaultReconAccountID
	}

	readerCfg := LedgerOMSReaderConfig{InitialBalanceUSD: defaultPaperInitialBalanceUSD}
	if cfg != nil {
		if cfg.InitialBalanceUSD > 0 {
			readerCfg.InitialBalanceUSD = cfg.InitialBalanceUSD
		}
		if cfg.MarkPriceUSD != nil {
			readerCfg.MarkPriceUSD = cfg.MarkPriceUSD
		}
	}
	omsReader := NewLedgerOMSStateReader(store, accountID, readerCfg)
	repairTarget := NewLedgerRepairTarget(store)
	schedCfg := DefaultScheduleConfig()
	ksHook := CriticalDriftKillSwitchHook(ks)

	out := &WiredAuthorities{}

	runtimeAdapter := NewPositionManagerExchangeAdapter(posMgr, equityUSD, accountID, readerCfg.MarkPriceUSD)
	out.Runtime = NewExchangeReconciliationAuthority(
		runtimeAdapter,
		omsReader,
		store,
		repairTarget,
		accountID,
		&schedCfg,
	)
	out.Runtime.SetCycleHook(ksHook)
	if err := out.Runtime.Start(ctx); err != nil {
		return nil, err
	}
	log.Printf("[RECON-V2] ✅ Runtime reconciliation started (ledger vs position manager) account=%s", accountID)

	if deltaAdapter, err := NewDeltaReconciliationAdapterFromEnv(); err == nil {
		out.Delta = NewExchangeReconciliationAuthority(
			deltaAdapter,
			omsReader,
			store,
			repairTarget,
			accountID,
			&schedCfg,
		)
		out.Delta.SetCycleHook(ksHook)
		if err := out.Delta.Start(ctx); err != nil {
			log.Printf("[RECON-V2] ⚠️  Delta reconciliation failed to start: %v", err)
		} else {
			log.Printf("[RECON-V2] ✅ Delta broker reconciliation started (real REST snapshots) account=%s", accountID)
		}
	} else {
		log.Printf("[RECON-V2] Delta reconciliation skipped (%v)", err)
	}

	// Post-boot validation: one immediate full audit per started authority.
	go func() {
		if entry, err := out.Runtime.RunNow(ctx); err != nil {
			log.Printf("[RECON-V2] startup runtime audit failed: %v", err)
		} else {
			log.Printf("[RECON-V2] startup runtime audit: mismatches=%d drift=%.2f", entry.MismatchCount, entry.DriftScore)
		}
		if out.Delta != nil {
			if entry, err := out.Delta.RunNow(ctx); err != nil {
				log.Printf("[RECON-V2] startup delta audit failed: %v", err)
			} else {
				log.Printf("[RECON-V2] startup delta audit: mismatches=%d drift=%.2f", entry.MismatchCount, entry.DriftScore)
			}
		}
	}()

	return out, nil
}
