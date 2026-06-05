// Package execintel implements Phase 22D execution intelligence: a per-signal
// lifecycle tracker, latency percentile aggregation, slippage analysis,
// missed-entry classification, TP-override auditing, trade-conversion metrics,
// and an execution quality score.
//
// It is a passive observability layer. It records what the trading orchestrator
// reports at each real execution stage; it never fabricates events. Every counter
// and percentile is derived from RecordTransition / RecordSlippage / RecordTPOverride
// calls made from the live execution path in internal/trading/loop.go.
package execintel

import (
	"sync"
	"time"
)

// State is a single stage in a signal's execution lifecycle.
type State string

const (
	StateSignalGenerated   State = "SignalGenerated"
	StateSignalApproved    State = "SignalApproved"
	StateSignalRejected    State = "SignalRejected"
	StateRiskApproved      State = "RiskApproved"
	StateRiskRejected      State = "RiskRejected"
	StateOrderSubmitted    State = "OrderSubmitted"
	StateOrderRejected     State = "OrderRejected"
	StateOrderAcknowledged State = "OrderAcknowledged"
	StateOrderFilled       State = "OrderFilled"
	StatePositionOpened    State = "PositionOpened"
	StatePositionClosed    State = "PositionClosed"
	StateTPTriggered       State = "TPTriggered"
	StateSLTriggered       State = "SLTriggered"
	StateExpired           State = "Expired"
)

// IsTerminal reports whether the state ends a signal's lifecycle.
func (s State) IsTerminal() bool {
	switch s {
	case StateSignalRejected, StateRiskRejected, StateOrderRejected,
		StatePositionClosed, StateExpired:
		return true
	}
	return false
}

// Transition is one timestamped state change.
type Transition struct {
	State  State     `json:"state"`
	At     time.Time `json:"at"`
	Detail string    `json:"detail,omitempty"`
}

// SignalRecord is the full lifecycle of one signal.
type SignalRecord struct {
	SignalID    string       `json:"signalId"`
	Strategy    string       `json:"strategy"`
	Category    string       `json:"category"`
	AlphaSource string       `json:"alphaSource"`
	Symbol      string       `json:"symbol"`
	Direction   string       `json:"direction"`
	Price       float64      `json:"price"`
	Size        float64      `json:"size"`
	Regime      string       `json:"regime"`
	Timeframe   string       `json:"timeframe"`
	CreatedAt   time.Time    `json:"createdAt"`
	Transitions []Transition `json:"transitions"`
}

// Meta carries the immutable identity of a signal at creation time.
type Meta struct {
	SignalID    string
	Strategy    string
	Category    string
	AlphaSource string
	Symbol      string
	Direction   string
	Price       float64
	Size        float64
	Regime      string
	Timeframe   string
}

// has reports whether a state already appears in the transition list.
func (r *SignalRecord) has(s State) bool {
	for _, t := range r.Transitions {
		if t.State == s {
			return true
		}
	}
	return false
}

// timeOf returns the timestamp of the first occurrence of state s, or zero.
func (r *SignalRecord) timeOf(s State) time.Time {
	for _, t := range r.Transitions {
		if t.State == s {
			return t.At
		}
	}
	return time.Time{}
}

const (
	defaultActiveCap    = 4096 // max in-flight signals retained
	defaultCompletedCap = 8192 // ring buffer of completed lifecycles for stats
)

// Tracker is the central execution-intelligence aggregator. Safe for concurrent use.
type Tracker struct {
	mu sync.RWMutex

	active    map[string]*SignalRecord
	completed []*SignalRecord // ring buffer
	compHead  int
	compCount int

	// Conversion counters (monotonic).
	generated  int64
	approved   int64
	executed   int64 // orders filled
	profitable int64
	losing     int64

	// Missed-entry rejection counts keyed by classified reason.
	rejections map[RejectReason]int64
	rejPnL     map[RejectReason]float64 // opportunity proxy (signal notional)

	latency  *latencyBook
	slippage *slippageBook
	tpAudit  *tpAuditBook

	activeCap    int
	completedCap int
}

// New returns a Tracker with default capacities.
func New() *Tracker {
	return NewWithCap(defaultActiveCap, defaultCompletedCap)
}

// NewWithCap returns a Tracker with explicit ring-buffer capacities.
func NewWithCap(activeCap, completedCap int) *Tracker {
	if activeCap <= 0 {
		activeCap = defaultActiveCap
	}
	if completedCap <= 0 {
		completedCap = defaultCompletedCap
	}
	return &Tracker{
		active:       make(map[string]*SignalRecord, activeCap),
		completed:    make([]*SignalRecord, completedCap),
		rejections:   make(map[RejectReason]int64),
		rejPnL:       make(map[RejectReason]float64),
		latency:      newLatencyBook(),
		slippage:     newSlippageBook(),
		tpAudit:      newTPAuditBook(),
		activeCap:    activeCap,
		completedCap: completedCap,
	}
}

// Begin registers a newly generated signal and records StateSignalGenerated.
// Returns the record so callers may chain, but all mutation goes through the Tracker.
func (t *Tracker) Begin(m Meta) {
	if t == nil || m.SignalID == "" {
		return
	}
	now := time.Now()
	rec := &SignalRecord{
		SignalID:    m.SignalID,
		Strategy:    m.Strategy,
		Category:    m.Category,
		AlphaSource: m.AlphaSource,
		Symbol:      m.Symbol,
		Direction:   m.Direction,
		Price:       m.Price,
		Size:        m.Size,
		Regime:      m.Regime,
		Timeframe:   m.Timeframe,
		CreatedAt:   now,
		Transitions: []Transition{{State: StateSignalGenerated, At: now}},
	}
	t.mu.Lock()
	// Evict an arbitrary active record if we are at capacity (prevents unbounded growth
	// when signals never reach a terminal state, e.g. parked-and-forgotten).
	if len(t.active) >= t.activeCap {
		for k := range t.active {
			delete(t.active, k)
			break
		}
	}
	t.active[m.SignalID] = rec
	t.generated++
	t.mu.Unlock()
}

// Record appends a state transition for the given signal id. Unknown ids are
// ignored (the signal was evicted or never began). Terminal states retire the
// record into the completed ring buffer.
func (t *Tracker) Record(signalID string, state State, detail string) {
	if t == nil || signalID == "" {
		return
	}
	now := time.Now()
	t.mu.Lock()
	rec := t.active[signalID]
	if rec == nil {
		t.mu.Unlock()
		return
	}
	rec.Transitions = append(rec.Transitions, Transition{State: state, At: now, Detail: detail})

	// Conversion + latency side effects.
	switch state {
	case StateSignalApproved:
		t.approved++
	case StateOrderFilled:
		t.executed++
	}
	t.recordLatencyLocked(rec)

	if state.IsTerminal() {
		delete(t.active, signalID)
		t.retireLocked(rec)
	}
	t.mu.Unlock()
}

// RecordRejection classifies a missed entry and retires the signal. The reason is
// mapped to a canonical RejectReason; rawReason is preserved as transition detail.
func (t *Tracker) RecordRejection(signalID string, rawReason string) {
	if t == nil {
		return
	}
	reason := Classify(rawReason)
	state := stateForReject(reason)
	t.mu.Lock()
	rec := t.active[signalID]
	if rec != nil {
		rec.Transitions = append(rec.Transitions, Transition{State: state, At: time.Now(), Detail: rawReason})
		notional := rec.Price * rec.Size
		t.rejections[reason]++
		t.rejPnL[reason] += notional
		delete(t.active, signalID)
		t.retireLocked(rec)
	} else {
		// Signal was never tracked (e.g. rejected before Begin) — still count it.
		t.rejections[reason]++
	}
	t.mu.Unlock()
}

// RecordTradeResult records the realized PnL of a closed position for conversion stats.
func (t *Tracker) RecordTradeResult(netPnL float64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	if netPnL > 0 {
		t.profitable++
	} else if netPnL < 0 {
		t.losing++
	}
	t.mu.Unlock()
}

// RecordSlippage forwards a fill's slippage to the slippage book.
func (t *Tracker) RecordSlippage(s SlippageSample) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.slippage.add(s)
	t.mu.Unlock()
}

// RecordTPOverride forwards a TP adjustment to the TP-override audit book.
func (t *Tracker) RecordTPOverride(o TPOverrideSample) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.tpAudit.add(o)
	t.mu.Unlock()
}

// recordLatencyLocked derives stage spans from the record's transitions and feeds
// them to the latency book. Caller holds t.mu.
func (t *Tracker) recordLatencyLocked(rec *SignalRecord) {
	type span struct {
		name string
		from State
		to   State
	}
	spans := []span{
		{"signal_to_submit", StateSignalGenerated, StateOrderSubmitted},
		{"submit_to_ack", StateOrderSubmitted, StateOrderAcknowledged},
		{"ack_to_fill", StateOrderAcknowledged, StateOrderFilled},
		{"fill_to_position", StateOrderFilled, StatePositionOpened},
		{"position_to_close", StatePositionOpened, StatePositionClosed},
		{"signal_to_fill_e2e", StateSignalGenerated, StateOrderFilled},
	}
	last := rec.Transitions[len(rec.Transitions)-1].State
	for _, sp := range spans {
		if sp.to != last {
			continue
		}
		from := rec.timeOf(sp.from)
		to := rec.timeOf(sp.to)
		if from.IsZero() || to.IsZero() || !to.After(from) {
			continue
		}
		ms := float64(to.Sub(from).Microseconds()) / 1000.0
		t.latency.add(sp.name, rec.Strategy, rec.Regime, ms)
	}
}

// retireLocked moves a completed record into the ring buffer. Caller holds t.mu.
func (t *Tracker) retireLocked(rec *SignalRecord) {
	t.completed[t.compHead] = rec
	t.compHead = (t.compHead + 1) % t.completedCap
	if t.compCount < t.completedCap {
		t.compCount++
	}
}

// ActiveCount returns the number of in-flight signals.
func (t *Tracker) ActiveCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.active)
}

// Lookup returns a deep copy of an active signal record, if present.
func (t *Tracker) Lookup(signalID string) (SignalRecord, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	rec := t.active[signalID]
	if rec == nil {
		return SignalRecord{}, false
	}
	out := *rec
	out.Transitions = append([]Transition(nil), rec.Transitions...)
	return out, true
}
