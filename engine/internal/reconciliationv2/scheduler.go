package reconciliationv2

import (
	"context"
	"log"
	"sync"
	"time"
)

// ScheduleConfig controls how frequently each domain is reconciled.
// All intervals are configurable; the defaults match institutional requirements.
type ScheduleConfig struct {
	OrdersInterval   time.Duration // critical: exchange orders
	PositionsInterval time.Duration
	BalancesInterval  time.Duration
	ExposureInterval  time.Duration
	FullAuditInterval time.Duration
}

// DefaultScheduleConfig returns the Phase 15E default intervals.
func DefaultScheduleConfig() ScheduleConfig {
	return ScheduleConfig{
		OrdersInterval:    2 * time.Second,
		PositionsInterval: 5 * time.Second,
		BalancesInterval:  10 * time.Second,
		ExposureInterval:  10 * time.Second,
		FullAuditInterval: 60 * time.Second,
	}
}

// ReconciliationScheduler drives periodic reconciliation cycles.
// Each domain runs on its own independent ticker in a separate goroutine.
// All cycles are non-blocking relative to each other — a slow full audit
// does not delay the 2-second order check.
type ReconciliationScheduler struct {
	cfg     ScheduleConfig
	engine  *ReconciliationEngine
	metrics *Metrics
	wg      sync.WaitGroup
}

// NewReconciliationScheduler creates a scheduler wired to the given engine.
func NewReconciliationScheduler(engine *ReconciliationEngine, metrics *Metrics, cfg ScheduleConfig) *ReconciliationScheduler {
	return &ReconciliationScheduler{
		cfg:     cfg,
		engine:  engine,
		metrics: metrics,
	}
}

// Start launches all reconciliation goroutines. Blocks until ctx is cancelled,
// then waits for all in-flight cycles to finish before returning.
func (s *ReconciliationScheduler) Start(ctx context.Context) {
	s.wg.Add(5)
	go s.runTicker(ctx, "orders",   DomainOrder,    s.cfg.OrdersInterval)
	go s.runTicker(ctx, "positions",DomainPosition, s.cfg.PositionsInterval)
	go s.runTicker(ctx, "balances", DomainBalance,  s.cfg.BalancesInterval)
	go s.runTicker(ctx, "exposure", DomainExposure, s.cfg.ExposureInterval)
	go s.runTicker(ctx, "full",     DomainFull,     s.cfg.FullAuditInterval)

	<-ctx.Done()
	s.wg.Wait()
	log.Println("[RECON-V2] scheduler shutdown complete")
}

func (s *ReconciliationScheduler) runTicker(ctx context.Context, name string, domain MismatchDomain, interval time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[RECON-V2] scheduler started: %s every %v", name, interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runCycle(ctx, domain)
		}
	}
}

func (s *ReconciliationScheduler) runCycle(ctx context.Context, domain MismatchDomain) {
	// Use a timeout per cycle to prevent a single stuck cycle from blocking the ticker slot.
	timeout := 8 * time.Second
	if domain == DomainFull {
		timeout = 30 * time.Second
	}

	cycleCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	entry, err := s.engine.RunDomain(cycleCtx, domain)
	if err != nil {
		log.Printf("[RECON-V2] cycle error domain=%s: %v", domain, err)
		return
	}

	if s.metrics != nil {
		s.metrics.RecordCycle(s.engine.exchangeName, domain, entry)
	}

	if entry.DriftScore > 0 {
		log.Printf("[RECON-V2] cycle done domain=%s mismatches=%d repairs=%d escalations=%d score=%.2f duration=%dms",
			domain, entry.MismatchCount, entry.RepairCount, entry.EscalateCount, entry.DriftScore, entry.DurationMs)
	}
}
