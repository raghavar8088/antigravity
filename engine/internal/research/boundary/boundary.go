// Package boundary enforces the strict isolation contract between the
// research environment and the production trading environment.
//
// INVARIANTS (never violate):
//  1. Research code NEVER holds broker credentials.
//  2. Research code NEVER calls production OMS write paths.
//  3. Research code NEVER submits orders to any live exchange.
//  4. Strategies enter production ONLY through the Promotion Pipeline.
//  5. Research reads market data read-only; it never writes to production ledger.
package boundary

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ─── Violation types ──────────────────────────────────────────────────────────

// ViolationType classifies what boundary was breached.
type ViolationType string

const (
	ViolationBrokerCredentialAccess   ViolationType = "BROKER_CREDENTIAL_ACCESS"
	ViolationOMSWriteAccess            ViolationType = "OMS_WRITE_ACCESS"
	ViolationOrderSubmission           ViolationType = "ORDER_SUBMISSION"
	ViolationDirectExchangeAccess      ViolationType = "DIRECT_EXCHANGE_ACCESS"
	ViolationProductionLedgerWrite     ViolationType = "PRODUCTION_LEDGER_WRITE"
	ViolationUnauthorizedPromotion     ViolationType = "UNAUTHORIZED_PROMOTION"
)

var (
	ErrBoundaryViolation = errors.New("research/boundary: isolation boundary violated")
)

// Violation records a detected boundary breach.
type Violation struct {
	ID           string
	Type         ViolationType
	Source       string
	Description  string
	OccurredAt   time.Time
}

// ─── Isolation Boundary ───────────────────────────────────────────────────────

// IsolationBoundary is a compile-time and runtime firewall between research
// and production. Embed it in every research service to enforce isolation.
type IsolationBoundary struct {
	mu         sync.RWMutex
	violations []Violation
	strict     bool // if true, violations panic in addition to being logged
}

// NewIsolationBoundary creates an isolation boundary.
// strict=true is recommended for development; false for production audit-only mode.
func NewIsolationBoundary(strict bool) *IsolationBoundary {
	return &IsolationBoundary{strict: strict}
}

// AssertNoOrderSubmission must be called at the top of any function that could
// submit an order. It panics in strict mode if called from research context.
func (b *IsolationBoundary) AssertNoOrderSubmission(ctx context.Context, caller string) error {
	return b.recordViolation(ctx, ViolationOrderSubmission, caller,
		"research component attempted to submit an order — this is never permitted")
}

// AssertNoBrokerCredentialAccess must be called before any credential lookup
// within the research environment.
func (b *IsolationBoundary) AssertNoBrokerCredentialAccess(ctx context.Context, caller string) error {
	return b.recordViolation(ctx, ViolationBrokerCredentialAccess, caller,
		"research component attempted to access broker credentials")
}

// AssertNoOMSWrite must be called before any OMS write operation from research code.
func (b *IsolationBoundary) AssertNoOMSWrite(ctx context.Context, caller string) error {
	return b.recordViolation(ctx, ViolationOMSWriteAccess, caller,
		"research component attempted to write to production OMS")
}

// AssertNoProductionLedgerWrite must be called before any production ledger write.
func (b *IsolationBoundary) AssertNoProductionLedgerWrite(ctx context.Context, caller string) error {
	return b.recordViolation(ctx, ViolationProductionLedgerWrite, caller,
		"research component attempted to write to production event ledger")
}

// AssertPromotionIsApproved must be called by the promotion pipeline before
// advancing a strategy to PRODUCTION state.
func (b *IsolationBoundary) AssertPromotionIsApproved(approvedBy string) error {
	if approvedBy == "" {
		v := Violation{
			Type:        ViolationUnauthorizedPromotion,
			Source:      "promotion-pipeline",
			Description: "strategy promotion attempted without approver identity",
			OccurredAt:  time.Now().UTC(),
		}
		b.mu.Lock()
		b.violations = append(b.violations, v)
		b.mu.Unlock()
		if b.strict {
			panic(fmt.Sprintf("BOUNDARY VIOLATION: %s — %s", v.Type, v.Description))
		}
		return fmt.Errorf("%w: %s", ErrBoundaryViolation, v.Description)
	}
	return nil
}

func (b *IsolationBoundary) recordViolation(ctx context.Context, vType ViolationType, source, desc string) error {
	_ = ctx
	v := Violation{
		Type:        vType,
		Source:      source,
		Description: desc,
		OccurredAt:  time.Now().UTC(),
	}
	b.mu.Lock()
	b.violations = append(b.violations, v)
	b.mu.Unlock()
	if b.strict {
		panic(fmt.Sprintf("BOUNDARY VIOLATION: %s [%s] — %s", vType, source, desc))
	}
	return fmt.Errorf("%w: %s [%s] — %s", ErrBoundaryViolation, vType, source, desc)
}

// Violations returns all recorded boundary violations.
func (b *IsolationBoundary) Violations() []Violation {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Violation, len(b.violations))
	copy(out, b.violations)
	return out
}

// IsClean returns true if no violations have been recorded.
func (b *IsolationBoundary) IsClean() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.violations) == 0
}

// Reset clears recorded violations (for use in testing only).
func (b *IsolationBoundary) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.violations = nil
}

// ─── Read-Only Market Data Contract ──────────────────────────────────────────

// MarketDataReader is the ONLY interface research components may use to access
// market data. It is read-only by construction — no write methods exist.
type MarketDataReader interface {
	// GetCandles returns OHLCV bars for the given symbol and time range.
	GetCandles(ctx context.Context, symbol string, from, to time.Time) ([]Candle, error)
	// GetTrades returns recent public trades for the given symbol.
	GetTrades(ctx context.Context, symbol string, limit int) ([]PublicTrade, error)
	// GetFundingRates returns historical funding rates.
	GetFundingRates(ctx context.Context, symbol string, from, to time.Time) ([]FundingRate, error)
}

// Candle is a read-only OHLCV bar — research use only.
type Candle struct {
	Symbol    string
	OpenTime  time.Time
	CloseTime time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// PublicTrade is a single public market trade — research use only.
type PublicTrade struct {
	Symbol    string
	Price     float64
	Quantity  float64
	Side      string
	Timestamp time.Time
}

// FundingRate is a historical funding rate record — research use only.
type FundingRate struct {
	Symbol      string
	Rate        float64
	IntervalHrs int
	Timestamp   time.Time
}

// ResearchDataSource is a named read-only data source for research pipelines.
// All research data access goes through this wrapper to maintain auditability.
type ResearchDataSource struct {
	Name       string
	boundary   *IsolationBoundary
	reader     MarketDataReader
	accessLog  []DataAccessEntry
	mu         sync.Mutex
}

// DataAccessEntry records a single data access for audit purposes.
type DataAccessEntry struct {
	Source    string
	Symbol    string
	From      time.Time
	To        time.Time
	AccessedAt time.Time
}

// NewResearchDataSource creates an audited, read-only data source for research.
func NewResearchDataSource(name string, reader MarketDataReader, b *IsolationBoundary) *ResearchDataSource {
	return &ResearchDataSource{Name: name, reader: reader, boundary: b}
}

// GetCandles returns OHLCV bars and logs the access for auditability.
func (r *ResearchDataSource) GetCandles(ctx context.Context, symbol string, from, to time.Time) ([]Candle, error) {
	r.mu.Lock()
	r.accessLog = append(r.accessLog, DataAccessEntry{
		Source: r.Name, Symbol: symbol, From: from, To: to, AccessedAt: time.Now().UTC(),
	})
	r.mu.Unlock()
	return r.reader.GetCandles(ctx, symbol, from, to)
}

// AccessLog returns all data access entries for audit purposes.
func (r *ResearchDataSource) AccessLog() []DataAccessEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]DataAccessEntry, len(r.accessLog))
	copy(out, r.accessLog)
	return out
}
