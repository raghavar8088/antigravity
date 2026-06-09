package trading

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"antigravity-engine/internal/ai"
	"antigravity-engine/internal/delta"
	"antigravity-engine/internal/execintel"
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/observability"
	"antigravity-engine/internal/omsv3"
	"antigravity-engine/internal/paperpersist"
	"antigravity-engine/internal/pms"
	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/risk"
	riskgate "antigravity-engine/internal/risk/gate"
	riskv2 "antigravity-engine/internal/risk/v2"
	"antigravity-engine/internal/strategy"
)

const (
	minExecutionSizeBTC       = 0.01
	sizeChangeEpsilonBTC      = 1e-9
	futuresInitialCapitalUSD  = 1000000.0
	futuresPositionCapitalPct = 0.01 // 1% of paper capital per futures entry (BTC Equity)
	fixedTradeCapitalUSD      = futuresInitialCapitalUSD * futuresPositionCapitalPct

	// btcPaperAccountID is the canonical account identifier written into every
	// OMS v3 ledger event so all order and position events for the BTC paper
	// account can be replayed together via ledger.Store.ReplayAccount.
	btcPaperAccountID = "btc-paper-1"

	minExecutableConfidence     = 0.68 // Phase 22A unlock: 0.74→0.68; aligns with ScoringConfig.MinConfidence=0.65 while keeping a safety buffer
	minBridgeApprovalConfidence = 0.65 // Minimum ChatGPT confidence to honour a bridge approval
	minRewardToRiskRatio        = 2.40 // Stronger edge requirement for scalping signals
	minSignalTakeProfitPct      = 0.50 // Wider TP — avoid noise-driven exits
	maxSignalStopLossPct        = 0.20 // Allow slightly wider SL for more stable execution
	defaultSignalStopLossPct    = 0.18 // Safer default SL — reduce micro noise losses

	minExecutionWeightToTrade = 0.50 // Require stronger strategy quality before execution
	marketHistoryMaxSamples   = 320

	marketRegimeUnknown  = "UNKNOWN"
	marketRegimeTrend    = "TREND"
	marketRegimeRange    = "RANGE"
	marketRegimeVolatile = "VOLATILE"
	marketRegimeMixed    = "MIXED"
)

// signalMaxAge returns the maximum age a signal may reach before execution is
// skipped.  Signals older than their timeframe's expiry window are stale — the
// market has moved and the entry thesis no longer applies.
func signalMaxAge(timeframe string) time.Duration {
	switch timeframe {
	case "1m":
		return 90 * time.Second
	case "3m":
		return 4 * time.Minute
	case "5m":
		return 7 * time.Minute
	case "15m":
		return 20 * time.Minute
	case "1h":
		return 75 * time.Minute
	default: // "tick" or unset
		return 500 * time.Millisecond
	}
}

// Orchestrator is the multi-strategy parallel trading engine.
// It correctly separates tick-based strategies from candle-based strategies,
// ensuring each strategy type receives the data it was designed for.
type Orchestrator struct {
	client      marketdata.MarketDataClient
	strategies  []strategy.RegistryEntry
	groups      strategy.StrategyGroups
	risk        *risk.RiskEngine
	exec        *execution.PaperClient
	aggregator  *SignalAggregator
	posMgr      *positions.Manager
	tracker     *risk.StrategyTracker
	journal     *execution.TradeJournal
	candleAgg   *marketdata.CandleAggregator
	eventLedger ledger.Store

	// execIntel is the Phase 22D execution-intelligence layer: per-signal
	// lifecycle tracking, latency percentiles, slippage, missed-entry
	// classification, TP-override audit, trade conversion, and quality score.
	execIntel *execintel.Tracker

	// AI multi-agent layer (nil when ANTHROPIC_API_KEY not set)
	aiAgent    *ai.MultiAgentOrchestrator
	aiCandleCh chan marketdata.Candle

	// Candle history for AI context (last 20 × 5m candles)
	candleHistory []ai.CandleSummary
	candleHistMu  sync.Mutex

	priceWindow  []float64
	volumeWindow []float64

	// Internal state
	lastPrice  float64
	lastRegime string // Last classified market regime — kept so executeThroughInstitutionalPath can pass live regime to Risk V2
	m15Counter int    // Counts 5m candles to simulate 15m (every 3rd 5m candle)
	h1Counter  int    // Counts 5m candles to simulate 1h (every 12th)

	// Heartbeat for automated bridge failover
	lastBridgeHeartbeat time.Time
	bridgeHeartbeatMu   sync.RWMutex

	// Interactive AI: Pending signals waiting for UI submission
	pendingSignals map[string]PendingSignal
	pendingMu      sync.RWMutex

	// Replay protection for browser verdict submissions
	processedBridgeSignals map[string]time.Time
	processedBridgeMu      sync.Mutex

	lastBridgeEvent   string
	lastBridgeEventAt time.Time
	lastBridgeError   string
	lastBridgeErrorAt time.Time
	bridgeStateMu     sync.RWMutex

	// positionToOrderID links a positions.Manager position ID to the OMS v3
	// ClientOrderID so that EventPositionOpened / EventPositionClosed can be
	// correlated with the originating order aggregate in the ledger.
	positionToOrderID map[string]string
	positionToOrderMu sync.RWMutex

	// signalIDByOrder links an OMS v3 ClientOrderID to the execintel lifecycle id
	// so the close-event handler can finalize the lifecycle (PositionClosed +
	// TP/SL outcome + realized PnL) for the originating signal.
	signalIDByOrder map[string]string
	signalIDMu      sync.Mutex

	// pmsBudget is the portfolio-level risk budget gate (P3-A).
	// When set, CheckPortfolioRisk runs before the PreTradeRiskPipeline so
	// portfolio-level heat, VaR, drawdown, and daily-loss limits can veto trades
	// that would individually pass per-strategy risk checks.
	// Nil until SetPMSBudget() is called from main.go.
	pmsBudget *pms.PortfolioRiskBudget

	// killSvc is the institutional kill switch service (internal/killswitch).
	killSvc riskgate.KillSwitch

	// deltaBroker is the Delta live bridge — broker fills for delta venue route here only.
	deltaBroker *delta.Bridge

	// ppPersist is the Phase 31B MongoDB persistence bundle.
	// Nil until SetPaperPersist() is called from main.go.
	ppPersist *PaperPersistBundle

	// portfolioLedger mirrors MongoDB closed-trade accounting in-process.
	portfolioLedger *paperpersist.PortfolioLedger

	// sessionStart records when this engine process started. Used by the
	// AccountStateProvider to populate the session_start field in paper_state.
	sessionStart time.Time

	mu sync.RWMutex
}

// PendingSignal represents a strategy signal waiting for AI/User approval.
type PendingSignal struct {
	ID           string           `json:"id"`
	Signal       strategy.Signal  `json:"signal"`
	StrategyName string           `json:"strategyName"`
	Category     string           `json:"category"`
	Context      ai.MarketContext `json:"context"`
	AutoPrompt   string           `json:"autoPrompt"`
	CreatedAt    time.Time        `json:"createdAt"`
}

// BridgeDecision is the structured verdict returned by the browser bridge.
type BridgeDecision struct {
	Approved   bool    `json:"approved"`
	Action     string  `json:"action"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
	RawReply   string  `json:"rawReply"`
}

type BridgeStatus struct {
	Online              bool      `json:"online"`
	LastHeartbeat       time.Time `json:"lastHeartbeat"`
	SecondsSinceBeat    int       `json:"secondsSinceBeat"`
	PendingSignals      int       `json:"pendingSignals"`
	ProcessedSignalKeys int       `json:"processedSignalKeys"`
	LastEvent           string    `json:"lastEvent"`
	LastEventAt         time.Time `json:"lastEventAt"`
	LastError           string    `json:"lastError"`
	LastErrorAt         time.Time `json:"lastErrorAt"`
}

func NewOrchestrator(
	c marketdata.MarketDataClient,
	strats []strategy.RegistryEntry,
	r *risk.RiskEngine,
	e *execution.PaperClient,
	agg *SignalAggregator,
	pm *positions.Manager,
	tracker *risk.StrategyTracker,
	journal *execution.TradeJournal,
	candleAgg *marketdata.CandleAggregator,
) *Orchestrator {
	groups := strategy.GroupByTimeframe(strats)
	log.Printf("[ORCHESTRATOR] Strategy groups: %d tick, %d 1m, %d 5m, %d 15m, %d 1h",
		len(groups.Tick), len(groups.M1), len(groups.M5), len(groups.M15), len(groups.H1))

	return &Orchestrator{
		client:                 c,
		strategies:             strats,
		groups:                 groups,
		risk:                   r,
		exec:                   e,
		aggregator:             agg,
		posMgr:                 pm,
		tracker:                tracker,
		journal:                journal,
		candleAgg:              candleAgg,
		eventLedger:            ledger.NewMemoryStore(),
		execIntel:              execintel.New(),
		priceWindow:            make([]float64, 0, marketHistoryMaxSamples),
		volumeWindow:           make([]float64, 0, marketHistoryMaxSamples),
		pendingSignals:         make(map[string]PendingSignal),
		processedBridgeSignals: make(map[string]time.Time),
		positionToOrderID:      make(map[string]string),
		signalIDByOrder:        make(map[string]string),
		// Zero until the browser bridge explicitly heartbeats — avoids treating
		// "fresh boot" as bridge-online and parking every signal for 15s with no approver.
		lastBridgeHeartbeat: time.Time{},
		sessionStart:        time.Now(),
		portfolioLedger:     paperpersist.NewPortfolioLedger(futuresInitialCapitalUSD),
	}
}

// PortfolioLedger exposes the in-process accounting mirror (Mongo-authoritative on bootstrap).
func (o *Orchestrator) PortfolioLedger() *paperpersist.PortfolioLedger {
	return o.portfolioLedger
}

func (o *Orchestrator) SetEventLedger(store ledger.Store) {
	if store == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.eventLedger = store
}

// SetKillSwitch injects the institutional kill switch service. Call this from
// main.go after constructing the Orchestrator. Once set, every new order
// submission is gated through killSvc.IsActive() inside PreTradeRiskPipeline —
// the engine process stays alive, only new order flow is blocked.
// Passing nil is safe and disables kill-switch gating.
func (o *Orchestrator) SetKillSwitch(svc *killswitch.Service) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if svc == nil {
		o.killSvc = nil
		return
	}
	o.killSvc = svc
}

// SetPMSBudget injects the portfolio-level risk budget manager (P3-A).
// When set, every new order submission passes through CheckPortfolioRisk before
// reaching the per-strategy PreTradeRiskPipeline.
func (o *Orchestrator) SetPMSBudget(budget *pms.PortfolioRiskBudget) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pmsBudget = budget
}

func (o *Orchestrator) SetDeltaBroker(b *delta.Bridge) {
	o.deltaBroker = b
}

// brokerFillFunc performs the broker-specific submission after institutional gates pass.
type brokerFillFunc func(ctx context.Context, sig strategy.Signal, clientOrderID string) (execution.FillResult, error)

// InstitutionalPathOpts configures optional behaviour for institutional execution.
type InstitutionalPathOpts struct {
	// EmergencyFlatten skips PMS and sizing gates but still records OMS/ledger events.
	EmergencyFlatten bool
}

func (o *Orchestrator) executeThroughInstitutionalPath(ctx context.Context, sig strategy.Signal, strategyName string, currentPrice float64, mode execution.OrderMode) (execution.FillResult, error) {
	return o.executeThroughInstitutionalPathWithFill(ctx, sig, strategyName, currentPrice, mode, func(_ context.Context, s strategy.Signal, clientOrderID string) (execution.FillResult, error) {
		fill, err := o.exec.ExecuteSignal(s, mode)
		if err != nil {
			return execution.FillResult{}, err
		}
		fill.ClientOrderID = clientOrderID
		return fill, nil
	})
}

// ExecuteEmergencyFlatten flattens exposure through the institutional ledger/OMS path.
// Sizing gates are bypassed; audit, risk, and OMS transitions are still recorded.
func (o *Orchestrator) ExecuteEmergencyFlatten(ctx context.Context, sig strategy.Signal, reason string) error {
	o.mu.RLock()
	price := o.lastPrice
	o.mu.RUnlock()
	if price <= 0 {
		price = o.exec.GetLastPrice()
	}
	if price <= 0 {
		return fmt.Errorf("no market price for emergency flatten")
	}
	if sig.StopLossPct <= 0 {
		sig.StopLossPct = defaultSignalStopLossPct
	}
	if sig.TakeProfitPct <= 0 {
		sig.TakeProfitPct = minSignalTakeProfitPct
	}
	strategyName := "KILL_SWITCH_FLATTEN"
	if reason != "" {
		strategyName = "KILL_SWITCH_" + strings.ToUpper(strings.ReplaceAll(reason, " ", "_"))
	}
	_, err := o.executeThroughInstitutionalPathWithFill(ctx, sig, strategyName, price, execution.OrderModeMarket,
		func(_ context.Context, s strategy.Signal, clientOrderID string) (execution.FillResult, error) {
			fill, execErr := o.exec.ExecuteSignal(s, execution.OrderModeMarket)
			if execErr != nil {
				return execution.FillResult{}, execErr
			}
			fill.ClientOrderID = clientOrderID
			return fill, nil
		},
		InstitutionalPathOpts{EmergencyFlatten: true},
	)
	return err
}

func (o *Orchestrator) executeThroughInstitutionalPathWithFill(ctx context.Context, sig strategy.Signal, strategyName string, currentPrice float64, mode execution.OrderMode, fillFn brokerFillFunc, opts ...InstitutionalPathOpts) (execution.FillResult, error) {
	var pathOpts InstitutionalPathOpts
	if len(opts) > 0 {
		pathOpts = opts[0]
	}
	// P2-D: refresh equity immediately before Kelly sizing so Risk V2 never uses
	// the stale boot-time seed (covers 1m/5m candle paths that skip tick pipeline).
	o.risk.SyncEquity(o.exec.GetEquityUSD())

	clientOrderID := fmt.Sprintf("AG-PAPER-%s-%d", strings.ReplaceAll(sig.Symbol, "-", ""), time.Now().UTC().UnixNano())
	store := o.eventLedger
	if store == nil {
		store = ledger.NewMemoryStore()
		o.eventLedger = store
	}

	appendOrderEvent := func(eventType ledger.EventType, payload ledger.OrderPayload) (ledger.Event, error) {
		event, err := ledger.NewEvent(ledger.NewEventInput{
			AggregateType:  ledger.AggregateOrder,
			AggregateID:    clientOrderID,
			EventType:      eventType,
			StrategyID:     strategyName,
			Symbol:         sig.Symbol,
			IdempotencyKey: idempotencyKeyForOrder(clientOrderID, eventType),
			Payload:        payload,
			Source:         "trading-orchestrator",
		})
		if err != nil {
			return ledger.Event{}, err
		}
		return store.Append(ctx, event)
	}
	orderPayload := ledger.OrderPayload{
		ClientOrderID: clientOrderID,
		Symbol:        sig.Symbol,
		Side:          string(sig.Action),
		Quantity:      sig.TargetSize,
	}
	if _, err := appendOrderEvent(ledger.EventOrderCreated, orderPayload); err != nil {
		return execution.FillResult{}, err
	}
	// Phase 31B: record OMS NEW transition to MongoDB.
	o.persistOMSTransition(ctx, paperpersist.OrderTransition{
		OrderID:        clientOrderID,
		StrategyID:     strategyName,
		Symbol:         sig.Symbol,
		Side:           string(sig.Action),
		RequestedSize:  sig.TargetSize,
		TransitionFrom: "",
		TransitionTo:   paperpersist.OMSNew,
		TransitionAt:   time.Now(),
	})
	events, err := store.Replay(ctx, ledger.AggregateOrder, clientOrderID)
	if err != nil {
		return execution.FillResult{}, err
	}
	if _, err := omsv3.Replay(events); err != nil {
		return execution.FillResult{}, err
	}
	if _, err := appendOrderEvent(ledger.EventOrderValidated, orderPayload); err != nil {
		return execution.FillResult{}, err
	}

	if pathOpts.EmergencyFlatten {
		o.persistOMSTransition(ctx, paperpersist.OrderTransition{
			OrderID:        clientOrderID,
			StrategyID:     strategyName,
			Symbol:         sig.Symbol,
			Side:           string(sig.Action),
			RequestedSize:  sig.TargetSize,
			TransitionFrom: paperpersist.OMSNew,
			TransitionTo:   paperpersist.OMSRiskChecked,
			TransitionAt:   time.Now(),
			RiskApproved:   true,
			Reason:         "emergency_flatten",
		})
		return o.submitInstitutionalOrder(ctx, sig, strategyName, clientOrderID, orderPayload, appendOrderEvent, fillFn, 1.0, "emergency_flatten")
	}

	// P2-C: authoritative strategy metadata with live family for concentration checks.
	stratCategory := ""
	if stats, ok := o.tracker.GetStats(strategyName); ok {
		stratCategory = stats.Category
	}
	stratMeta := strategy.MetadataFromCategory(strategyName, stratCategory)

	// P3-A: Portfolio-level risk gate (PMS). Runs before the per-strategy pipeline
	// so portfolio-wide heat, VaR, drawdown, and daily-loss caps can block trades
	// that would individually pass the per-strategy institutional check.
	if o.pmsBudget != nil {
		equityForPMS := o.exec.GetEquityUSD()
		// Estimate dollar risk as stop-loss distance × proposed size.
		proposedDollarRisk := riskv2.PositionRiskUSD(riskv2.TradeRequest{
			EntryPrice:       currentPrice,
			StopLossPrice:    stopLossFromSignal(sig, currentPrice),
			RequestedSizeBTC: sig.TargetSize,
		}, sig.TargetSize)
		pmsBudgetConfig := pms.RiskBudget{
			MaxHeatPct:      8,
			MaxVaR95Pct:     6,
			MaxCVaR95Pct:    9,
			MaxDrawdownPct:  10,
			MaxDailyLossPct: 3,
			MaxGrossExpPct:  250,
			MaxNetExpPct:    150,
		}
		if violations := o.pmsBudget.CheckPortfolioRisk(ctx, btcPaperAccountID, pmsBudgetConfig, proposedDollarRisk, equityForPMS); len(violations) > 0 {
			pmsReason := fmt.Sprintf("PMS portfolio gate blocked: %s", violations[0].Message)
			log.Printf("[PMS GATE] %s: %s", strategyName, pmsReason)
			logDecisionFunnel(strategyName, stratCategory, stratMeta.Family, o.lastRegime, 0, 0, 0, 0, "pms_portfolio_gate: "+violations[0].Message)
			pmsEvent, newEventErr := ledger.NewEvent(ledger.NewEventInput{
				AggregateType: ledger.AggregateOrder,
				AggregateID:   clientOrderID,
				EventType:     ledger.EventRiskBlocked,
				StrategyID:    strategyName,
				Symbol:        sig.Symbol,
				Payload:       map[string]string{"reason": pmsReason, "violations": fmt.Sprintf("%d", len(violations))},
				Source:        "pms-portfolio-gate",
			})
			if newEventErr == nil {
				_, _ = store.Append(ctx, pmsEvent)
			}
			o.persistOMSTransition(ctx, paperpersist.OrderTransition{
				OrderID:        clientOrderID,
				StrategyID:     strategyName,
				Symbol:         sig.Symbol,
				Side:           string(sig.Action),
				RequestedSize:  sig.TargetSize,
				TransitionFrom: paperpersist.OMSNew,
				TransitionTo:   paperpersist.OMSRejected,
				TransitionAt:   time.Now(),
				Reason:         pmsReason,
				RiskApproved:   false,
			})
			return execution.FillResult{}, fmt.Errorf("%s", pmsReason)
		}
	}

	pipeline := riskgate.NewPreTradeRiskPipeline(o.risk.V2(), o.killSvc)
	riskDecision := pipeline.Check(ctx, riskgate.Input{
		Request: riskv2.TradeRequest{
			Symbol:            sig.Symbol,
			Strategy:          strategyName,
			Family:            stratMeta.Family,
			Side:              riskSideFromAction(sig.Action),
			EntryPrice:        currentPrice,
			StopLossPrice:     stopLossFromSignal(sig, currentPrice),
			RequestedSizeBTC:  sig.TargetSize,
			RequestedLeverage: 1,
			Confidence:        sig.Confidence,
			Exchange:          "paper",
		},
		Market: o.buildRiskV2MarketState(),
		Metrics: o.tracker.BuildRiskMetrics(strategyName),
	})
	// Phase 31B: record RISK_CHECKED transition.
	riskCheckedAt := time.Now()
	if riskDecision.Status == riskgate.DecisionBlocked {
		logDecisionFunnel(
			strategyName, stratCategory, stratMeta.Family, o.lastRegime,
			riskDecision.RiskDecision.Kelly.SelectedFraction,
			riskDecision.RiskDecision.Kelly.RecommendedSizeBTC,
			riskDecision.RiskDecision.DynamicSizing.Multiplier,
			riskDecision.RiskDecision.DynamicSizing.RecommendedSizeBTC,
			"risk_pipeline: "+riskDecision.Reason,
		)
		event, newEventErr := ledger.NewEvent(ledger.NewEventInput{
			AggregateType: ledger.AggregateOrder,
			AggregateID:   clientOrderID,
			EventType:     ledger.EventRiskBlocked,
			StrategyID:    strategyName,
			Symbol:        sig.Symbol,
			Payload:       map[string]string{"reason": riskDecision.Reason},
			Source:        "pre-trade-risk-pipeline",
		})
		if newEventErr == nil {
			_, _ = store.Append(ctx, event)
		}
		o.persistOMSTransition(ctx, paperpersist.OrderTransition{
			OrderID:        clientOrderID,
			StrategyID:     strategyName,
			Symbol:         sig.Symbol,
			Side:           string(sig.Action),
			RequestedSize:  sig.TargetSize,
			TransitionFrom: paperpersist.OMSNew,
			TransitionTo:   paperpersist.OMSRejected,
			TransitionAt:   riskCheckedAt,
			Reason:         riskDecision.Reason,
			RiskApproved:   false,
		})
		return execution.FillResult{}, riskDecision.Error()
	}
	// Risk approved — record RISK_CHECKED → ACCEPTED.
	o.persistOMSTransition(ctx, paperpersist.OrderTransition{
		OrderID:        clientOrderID,
		StrategyID:     strategyName,
		Symbol:         sig.Symbol,
		Side:           string(sig.Action),
		RequestedSize:  sig.TargetSize,
		TransitionFrom: paperpersist.OMSNew,
		TransitionTo:   paperpersist.OMSRiskChecked,
		TransitionAt:   riskCheckedAt,
		RiskApproved:   true,
		KellyFraction:  riskDecision.RiskDecision.Kelly.SelectedFraction,
	})
	// Elite-only drawdown regime enforcement (P1-C).
	if riskDecision.RiskDecision.Drawdown.OnlyEliteStrategies {
		if err := risk.EvaluateDrawdownExecution(stratMeta, riskDecision.RiskDecision.Drawdown); err != nil {
			eliteRej := err.Error()
			log.Printf("[ELITE GATE] %s: %s", strategyName, eliteRej)
			logDecisionFunnel(
				strategyName, stratCategory, stratMeta.Family, o.lastRegime,
				riskDecision.RiskDecision.Kelly.SelectedFraction,
				riskDecision.RiskDecision.Kelly.RecommendedSizeBTC,
				riskDecision.RiskDecision.DynamicSizing.Multiplier,
				riskDecision.RiskDecision.DynamicSizing.RecommendedSizeBTC,
				"elite_drawdown_gate: "+eliteRej,
			)
			eliteEvent, newEventErr := ledger.NewEvent(ledger.NewEventInput{
				AggregateType: ledger.AggregateOrder,
				AggregateID:   clientOrderID,
				EventType:     ledger.EventRiskBlocked,
				StrategyID:    strategyName,
				Symbol:        sig.Symbol,
				Payload:       map[string]string{"reason": eliteRej},
				Source:        "elite-drawdown-gate",
			})
			if newEventErr == nil {
				_, _ = store.Append(ctx, eliteEvent)
			}
			o.persistOMSTransition(ctx, paperpersist.OrderTransition{
				OrderID:        clientOrderID,
				StrategyID:     strategyName,
				Symbol:         sig.Symbol,
				Side:           string(sig.Action),
				RequestedSize:  sig.TargetSize,
				TransitionFrom: paperpersist.OMSRiskChecked,
				TransitionTo:   paperpersist.OMSRejected,
				TransitionAt:   time.Now(),
				Reason:         eliteRej,
				RiskApproved:   false,
			})
			return execution.FillResult{}, err
		}
	}

	// Risk V2 sizing floor — authoritative rejection via sizing.go (P1-A).
	rec := riskDecision.RiskDecision.RecommendedSizeBTC
	if _, err := riskv2.EnforceExecutionFloor(
		strategyName,
		rec,
		riskDecision.RiskDecision.Kelly.SelectedFraction,
		riskDecision.RiskDecision.DynamicSizing.Multiplier,
	); err != nil {
		rejReason := err.Error()
		logDecisionFunnel(
			strategyName, stratCategory, stratMeta.Family, o.lastRegime,
			riskDecision.RiskDecision.Kelly.SelectedFraction,
			riskDecision.RiskDecision.Kelly.RecommendedSizeBTC,
			riskDecision.RiskDecision.DynamicSizing.Multiplier,
			riskDecision.RiskDecision.DynamicSizing.RecommendedSizeBTC,
			"risk_floor_gate: "+rejReason,
		)
		rejEvent, newEventErr := ledger.NewEvent(ledger.NewEventInput{
			AggregateType: ledger.AggregateOrder,
			AggregateID:   clientOrderID,
			EventType:     ledger.EventRiskBlocked,
			StrategyID:    strategyName,
			Symbol:        sig.Symbol,
			Payload:       map[string]string{"reason": rejReason},
			Source:        "risk-floor-gate",
		})
		if newEventErr == nil {
			_, _ = store.Append(ctx, rejEvent)
		}
		o.persistOMSTransition(ctx, paperpersist.OrderTransition{
			OrderID:        clientOrderID,
			StrategyID:     strategyName,
			Symbol:         sig.Symbol,
			Side:           string(sig.Action),
			RequestedSize:  sig.TargetSize,
			TransitionFrom: paperpersist.OMSRiskChecked,
			TransitionTo:   paperpersist.OMSRejected,
			TransitionAt:   time.Now(),
			Reason:         rejReason,
			RiskApproved:   false,
		})
		return execution.FillResult{}, fmt.Errorf("%s", rejReason)
	}
	sig.TargetSize = rec
	orderPayload.Quantity = rec
	return o.submitInstitutionalOrder(ctx, sig, strategyName, clientOrderID, orderPayload, appendOrderEvent, fillFn, riskDecision.RiskDecision.Kelly.SelectedFraction, "institutional_path")
}

func (o *Orchestrator) submitInstitutionalOrder(
	ctx context.Context,
	sig strategy.Signal,
	strategyName string,
	clientOrderID string,
	orderPayload ledger.OrderPayload,
	appendOrderEvent func(ledger.EventType, ledger.OrderPayload) (ledger.Event, error),
	fillFn brokerFillFunc,
	kellyFraction float64,
	source string,
) (execution.FillResult, error) {
	if _, err := appendOrderEvent(ledger.EventRiskApproved, orderPayload); err != nil {
		return execution.FillResult{}, err
	}
	o.persistOMSTransition(ctx, paperpersist.OrderTransition{
		OrderID:         clientOrderID,
		StrategyID:      strategyName,
		Symbol:          sig.Symbol,
		Side:            string(sig.Action),
		RequestedSize:   sig.TargetSize,
		TransitionFrom:  paperpersist.OMSRiskChecked,
		TransitionTo:    paperpersist.OMSAccepted,
		TransitionAt:    time.Now(),
		RiskApproved:    true,
		KellyFraction:   kellyFraction,
		ApprovedSizeBTC: sig.TargetSize,
		Reason:          source,
	})
	if _, err := appendOrderEvent(ledger.EventOrderSubmitted, orderPayload); err != nil {
		return execution.FillResult{}, err
	}
	ackPayload := orderPayload
	ackPayload.ExchangeOrderID = "paper-" + clientOrderID
	if _, err := appendOrderEvent(ledger.EventOrderAcked, ackPayload); err != nil {
		return execution.FillResult{}, err
	}

	fill, err := fillFn(ctx, sig, clientOrderID)
	if err != nil {
		rejectEvent, newEventErr := ledger.NewEvent(ledger.NewEventInput{
			AggregateType: ledger.AggregateOrder,
			AggregateID:   clientOrderID,
			EventType:     ledger.EventOrderRejected,
			StrategyID:    strategyName,
			Symbol:        sig.Symbol,
			Payload:       map[string]string{"reason": err.Error()},
			Source:        source,
		})
		if newEventErr == nil {
			store := o.eventLedger
			if store != nil {
				_, _ = store.Append(ctx, rejectEvent)
			}
		}
		o.persistOMSTransition(ctx, paperpersist.OrderTransition{
			OrderID:        clientOrderID,
			StrategyID:     strategyName,
			Symbol:         sig.Symbol,
			Side:           string(sig.Action),
			RequestedSize:  sig.TargetSize,
			TransitionFrom: paperpersist.OMSAccepted,
			TransitionTo:   paperpersist.OMSCancelled,
			TransitionAt:   time.Now(),
			Reason:         err.Error(),
		})
		return execution.FillResult{}, err
	}
	fillPayload := ackPayload
	fillPayload.FillQuantity = sig.TargetSize
	fillPayload.FillPrice = fill.ExecPrice
	if _, err := appendOrderEvent(ledger.EventOrderFilled, fillPayload); err != nil {
		return execution.FillResult{}, err
	}
	o.persistOMSTransition(ctx, paperpersist.OrderTransition{
		OrderID:        clientOrderID,
		StrategyID:     strategyName,
		Symbol:         sig.Symbol,
		Side:           string(sig.Action),
		RequestedSize:  sig.TargetSize,
		TransitionFrom: paperpersist.OMSAccepted,
		TransitionTo:   paperpersist.OMSSimulatedFill,
		TransitionAt:   time.Now(),
		FillPrice:      fill.ExecPrice,
		FillSize:       sig.TargetSize,
	})
	fill.ClientOrderID = clientOrderID
	return fill, nil
}

func idempotencyKeyForOrder(clientOrderID string, eventType ledger.EventType) string {
	return clientOrderID + ":" + string(eventType)
}

// rememberSignalForPosition links a ClientOrderID to its execintel lifecycle id.
func (o *Orchestrator) rememberSignalForPosition(clientOrderID, signalID string) {
	if clientOrderID == "" || signalID == "" {
		return
	}
	o.signalIDMu.Lock()
	o.signalIDByOrder[clientOrderID] = signalID
	o.signalIDMu.Unlock()
}

// takeSignalForPosition retrieves and removes the execintel lifecycle id for a
// ClientOrderID. Returns "" when none is mapped.
func (o *Orchestrator) takeSignalForPosition(clientOrderID string) string {
	if clientOrderID == "" {
		return ""
	}
	o.signalIDMu.Lock()
	defer o.signalIDMu.Unlock()
	id := o.signalIDByOrder[clientOrderID]
	delete(o.signalIDByOrder, clientOrderID)
	return id
}

// ExecIntelSnapshot returns the current Phase 22D execution-intelligence report.
func (o *Orchestrator) ExecIntelSnapshot() execintel.Snapshot {
	return o.execIntel.Snapshot()
}

// finalizeExecIntelClose records the terminal lifecycle states and realized
// outcome for a closed position into the execution-intelligence layer.
func (o *Orchestrator) finalizeExecIntelClose(event positions.CloseEvent, netPnL float64) {
	o.positionToOrderMu.RLock()
	clientOrderID := o.positionToOrderID[event.Position.ID]
	o.positionToOrderMu.RUnlock()

	exitReason := string(event.Reason)
	// Always record the realized PnL (conversion stats) and TP-override outcome,
	// keyed by strategy, even if no lifecycle id is mapped (e.g. AI/bridge trades).
	o.execIntel.RecordTradeResult(netPnL)
	o.execIntel.RecordTPOutcome(event.Position.StrategyName, netPnL, exitReason)

	sigID := o.takeSignalForPosition(clientOrderID)
	if sigID == "" {
		return
	}
	switch event.Reason {
	case positions.ReasonTakeProfit:
		o.execIntel.Record(sigID, execintel.StateTPTriggered, exitReason)
	case positions.ReasonStopLoss:
		o.execIntel.Record(sigID, execintel.StateSLTriggered, exitReason)
	}
	o.execIntel.Record(sigID, execintel.StatePositionClosed,
		fmt.Sprintf("pnl=%.4f reason=%s", netPnL, exitReason))
}

func riskSideFromAction(action strategy.Action) riskv2.Side {
	if action == strategy.ActionSell {
		return riskv2.SideShort
	}
	return riskv2.SideLong
}

func stopLossFromSignal(sig strategy.Signal, entry float64) float64 {
	if entry <= 0 {
		return 0
	}
	stopPct := sig.StopLossPct
	if stopPct <= 0 {
		stopPct = defaultSignalStopLossPct
	}
	if sig.Action == strategy.ActionSell {
		return entry * (1 + stopPct/100)
	}
	return entry * (1 - stopPct/100)
}

// ── OMS v3 position event helpers ─────────────────────────────────────────────

// openAndTrackPosition opens a position in the positions.Manager, registers the
// positionID → clientOrderID mapping, and asynchronously emits EventPositionOpened
// to the OMS v3 ledger. This is the single call-site for all position opens so
// the mapping is never missed regardless of execution path (strategy, AI, bridge).
func (o *Orchestrator) openAndTrackPosition(ctx context.Context, sig strategy.Signal, fill execution.FillResult, stratName string) {
	pos, err := o.posMgr.OpenPosition(sig, fill.ExecPrice, stratName)
	if err != nil {
		log.Printf("[POSITION OPEN REJECTED] %s: %v", stratName, err)
		return
	}
	if pos == nil || fill.ClientOrderID == "" {
		return
	}
	o.positionToOrderMu.Lock()
	o.positionToOrderID[pos.ID] = fill.ClientOrderID
	o.positionToOrderMu.Unlock()

	go o.emitPositionOpened(ctx, pos, fill, sig)

	// Phase 31B: persist to paper_positions + record POSITION_OPENED transition.
	o.persistPositionOpen(ctx, pos, fill, sig)
}

// emitPositionOpened appends an EventPositionOpened event to the OMS v3 ledger.
// Runs asynchronously so it never blocks the execution hot-path.
func (o *Orchestrator) emitPositionOpened(ctx context.Context, pos *positions.Position, fill execution.FillResult, sig strategy.Signal) {
	notional := sig.TargetSize * fill.ExecPrice
	payload := omsv3.PositionOpenedPayload{
		ClientOrderID: fill.ClientOrderID,
		PositionID:    pos.ID,
		Symbol:        pos.Symbol,
		Side:          string(pos.Side),
		EntryPrice:    fill.ExecPrice,
		Quantity:      sig.TargetSize,
		NotionalUSD:   notional,
		StopLoss:      pos.StopLoss,
		TakeProfit:    pos.TakeProfit,
		StopLossPct:   pos.StopLossPct,
		TakeProfitPct: pos.TakeProfitPct,
		StrategyName:  pos.StrategyName,
	}
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregatePosition,
		AggregateID:   pos.ID,
		EventType:     ledger.EventPositionOpened,
		AccountID:     btcPaperAccountID,
		StrategyID:    pos.StrategyName,
		Symbol:        pos.Symbol,
		CorrelationID: fill.ClientOrderID,
		Payload:       payload,
		Source:        "trading-orchestrator",
	})
	if err != nil {
		log.Printf("[OMS V3] emitPositionOpened: build event: %v", err)
		return
	}
	if _, err := o.eventLedger.Append(ctx, event); err != nil {
		log.Printf("[OMS V3] emitPositionOpened: append event: %v", err)
	}
}

// emitPositionClosed appends an EventPositionClosed event to the OMS v3 ledger.
// Called from processCloseEvents after the close is recorded in the trade journal.
func (o *Orchestrator) emitPositionClosed(ctx context.Context, closeEvt positions.CloseEvent, clientOrderID string, netPnL float64) {
	pos := closeEvt.Position
	notional := pos.Size * pos.EntryPrice
	holdMin := time.Since(pos.OpenedAt).Minutes()
	feesUSD := notional * execution.BinanceFuturesTakerFeePct * 2

	payload := omsv3.PositionClosedPayload{
		ClientOrderID: clientOrderID,
		PositionID:    pos.ID,
		Symbol:        pos.Symbol,
		Side:          string(pos.Side),
		EntryPrice:    pos.EntryPrice,
		ExitPrice:     closeEvt.ExitPrice,
		Quantity:      pos.Size,
		NotionalUSD:   notional,
		GrossPnLUSD:   closeEvt.PnL,
		NetPnLUSD:     netPnL,
		FeesUSD:       feesUSD,
		ExitReason:    string(closeEvt.Reason),
		StrategyName:  pos.StrategyName,
		HoldMinutes:   holdMin,
	}
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregatePosition,
		AggregateID:   pos.ID,
		EventType:     ledger.EventPositionClosed,
		AccountID:     btcPaperAccountID,
		StrategyID:    pos.StrategyName,
		Symbol:        pos.Symbol,
		CorrelationID: clientOrderID,
		Payload:       payload,
		Source:        "trading-orchestrator",
	})
	if err != nil {
		log.Printf("[OMS V3] emitPositionClosed: build event: %v", err)
		return
	}
	if _, err := o.eventLedger.Append(ctx, event); err != nil {
		log.Printf("[OMS V3] emitPositionClosed: append event: %v", err)
	}
}

// SetAIOrchestrator attaches the multi-agent AI system to the orchestrator.
// Called after construction so the constructor signature stays unchanged.
func (o *Orchestrator) SetAIOrchestrator(agent *ai.MultiAgentOrchestrator) {
	o.aiAgent = agent
	o.aiCandleCh = make(chan marketdata.Candle, 10)
	log.Println("[AI] Multi-agent orchestrator attached — Claude will trade on every 5m candle")
}

// WarmupStrategies pre-fills strategy price buffers with historical candle data.
// This eliminates the warmup delay on cold start / Render restart.
func (o *Orchestrator) WarmupStrategies(warmup *marketdata.WarmupData) {
	if warmup == nil {
		log.Println("[WARMUP] No warmup data provided, strategies will warm up from live data")
		return
	}

	log.Printf("[WARMUP] Feeding %d historical 1m candles to %d strategies...",
		len(warmup.Candles1m), len(o.groups.M1))

	// Feed 1m candles to 1m strategies
	for _, candle := range warmup.Candles1m {
		tick := candle.ToTick()
		for _, entry := range o.groups.M1 {
			entry.Strategy.OnTick(tick)
		}
	}

	log.Printf("[WARMUP] Feeding %d historical 5m candles to %d 5m / %d 15m / %d 1h strategies...",
		len(warmup.Candles5m), len(o.groups.M5), len(o.groups.M15), len(o.groups.H1))

	// Feed 5m candles to 5m strategies and simulate 15m / 1h closes.
	for idx, candle := range warmup.Candles5m {
		tick := candle.ToTick()
		for _, entry := range o.groups.M5 {
			entry.Strategy.OnTick(tick)
		}
		if (idx+1)%3 == 0 {
			for _, entry := range o.groups.M15 {
				entry.Strategy.OnTick(tick)
			}
		}
		if (idx+1)%12 == 0 {
			for _, entry := range o.groups.H1 {
				entry.Strategy.OnTick(tick)
			}
		}
	}

	log.Println("[WARMUP] ✅ All strategy buffers pre-filled. Ready for live trading.")
}

// Run is the infinite heartbeat of RAIG Autonomous Trading.
// It processes ticks and candles through their respective strategy groups.
func (o *Orchestrator) Run(ctx context.Context) {
	log.Printf("[RAIG MASTER LOOP] 🛰️  Booting Protocols with %d active strategies...", len(o.strategies))
	ticks := o.client.GetTickChannel()

	// Background: process position close events (SL/TP/trailing)
	go o.processCloseEvents(ctx)

	// Background: re-enable cooled-down strategies every minute
	go o.strategyCooldownChecker(ctx)

	// Background: process 1m candle closes
	go o.process1mCandles(ctx)

	// Background: process 5m candle closes
	go o.process5mCandles(ctx)

	// Background: AI multi-agent decisions (only when API key is set)
	if o.aiAgent != nil && o.aiAgent.IsAvailable() {
		go o.processAIDecisions(ctx)
		log.Println("[AI] 🤖 Claude multi-agent trading loop ACTIVE")
	}

	// Background: Auto-fallback monitor for bridge failover
	go o.autoFallbackMonitor(ctx)

	// Background: publish Phase 22D execution-intelligence metrics to Prometheus.
	go o.publishExecIntel(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[MASTER LOOP] Gracefully halting execution loop...")
			return
		case t := <-ticks:
			o.processTickPipeline(ctx, t)
		}
	}
}

// processTickPipeline handles every raw tick:
// 1. Updates market state + position SL/TP
// 2. Feeds tick to candle aggregator (which emits candles on channels)
// 3. Runs ONLY tick-timeframe strategies on the raw tick
func (o *Orchestrator) processTickPipeline(ctx context.Context, t marketdata.Tick) {
	// 1. Update market state
	o.exec.UpdateMarketState(t.Price)
	o.mu.Lock()
	o.lastPrice = t.Price
	o.priceWindow = append(o.priceWindow, t.Price)
	if len(o.priceWindow) > marketHistoryMaxSamples {
		o.priceWindow = o.priceWindow[len(o.priceWindow)-marketHistoryMaxSamples:]
	}
	vol := t.Quantity
	if vol <= 0 {
		vol = 1
	}
	o.volumeWindow = append(o.volumeWindow, vol)
	if len(o.volumeWindow) > marketHistoryMaxSamples {
		o.volumeWindow = o.volumeWindow[len(o.volumeWindow)-marketHistoryMaxSamples:]
	}
	o.mu.Unlock()

	// P2-D: sync live equity into Risk V2 engine on every tick so Kelly sizing
	// and all percentage-based risk calculations use current account value.
	o.risk.SyncEquity(o.exec.GetEquityUSD())

	// 2. Check SL/TP/trailing on all open positions
	o.posMgr.CheckStopLossAndTakeProfit(t.Price)
	o.posMgr.CheckExpiredPositions(t.Price)

	// 3. Feed tick to candle aggregator (it emits 1m/5m candles on channels)
	o.candleAgg.Feed(t)

	// 4. Run ONLY tick-based strategies (OrderFlow, TickVelocity, VolumeSpike, GapFill)
	if len(o.groups.Tick) == 0 {
		return
	}
	o.processStrategyGroup(ctx, o.groups.Tick, t, "tick")
}

// process1mCandles listens for closed 1-minute candles and runs all 1m strategies.
func (o *Orchestrator) process1mCandles(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case candle := <-o.candleAgg.Candles1m:
			tick := candle.ToTick()
			log.Printf("[CANDLE 1m] Closed: O=%.2f H=%.2f L=%.2f C=%.2f Vol=%.4f Trades=%d",
				candle.Open, candle.High, candle.Low, candle.Close, candle.Volume, candle.Trades)
			o.processStrategyGroup(ctx, o.groups.M1, tick, "1m")
		}
	}
}

// process5mCandles listens for closed 5-minute candles and runs all 5m, 15m, and 1h strategies.
func (o *Orchestrator) process5mCandles(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case candle := <-o.candleAgg.Candles5m:
			tick := candle.ToTick()
			log.Printf("[CANDLE 5m] Closed: O=%.2f H=%.2f L=%.2f C=%.2f Vol=%.4f",
				candle.Open, candle.High, candle.Low, candle.Close, candle.Volume)
			o.processStrategyGroup(ctx, o.groups.M5, tick, "5m")

			// Simulate 15m candle: run 15m strategies every 3rd 5m candle.
			o.m15Counter++
			if o.m15Counter >= 3 {
				o.m15Counter = 0
				log.Println("[CANDLE 15m] Simulated 15m close — running 15m strategies")
				o.processStrategyGroup(ctx, o.groups.M15, tick, "15m")
			}

			// Simulate 1h candle: run 1h strategies every 12th 5m candle
			o.h1Counter++
			if o.h1Counter >= 12 {
				o.h1Counter = 0
				log.Println("[CANDLE 1h] Simulated 1h close — running hourly strategies")
				o.processStrategyGroup(ctx, o.groups.H1, tick, "1h")
			}

			// Record candle in history for AI context
			o.recordCandleHistory(candle)

			// Forward to AI channel (non-blocking — drop if AI is busy)
			if o.aiCandleCh != nil {
				select {
				case o.aiCandleCh <- candle:
				default:
					log.Println("[AI] Candle dropped — AI agent still processing previous candle")
				}
			}
		}
	}
}

// recordCandleHistory stores the last 20 × 5m candles for AI context.
func (o *Orchestrator) recordCandleHistory(candle marketdata.Candle) {
	o.candleHistMu.Lock()
	defer o.candleHistMu.Unlock()
	o.candleHistory = append(o.candleHistory, ai.CandleSummary{
		Open:   candle.Open,
		High:   candle.High,
		Low:    candle.Low,
		Close:  candle.Close,
		Volume: candle.Volume,
	})
	if len(o.candleHistory) > 20 {
		o.candleHistory = o.candleHistory[len(o.candleHistory)-20:]
	}
}

// processAIDecisions runs the Claude multi-agent debate on every 5m candle.
// This goroutine runs independently so it never blocks the main trading loop.
func (o *Orchestrator) processAIDecisions(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.aiCandleCh:
			o.runAIDecision(ctx)
		}
	}
}

// runAIDecision builds market context and calls the multi-agent orchestrator.
func (o *Orchestrator) runAIDecision(ctx context.Context) {
	o.mu.RLock()
	price := o.lastPrice
	prices := append([]float64(nil), o.priceWindow...)
	volumes := append([]float64(nil), o.volumeWindow...)
	o.mu.RUnlock()

	if price <= 0 || len(prices) < 20 {
		return
	}

	o.candleHistMu.Lock()
	candles := append([]ai.CandleSummary(nil), o.candleHistory...)
	o.candleHistMu.Unlock()

	// Compute indicators for AI context
	regime := o.classifyMarketRegime()
	rsi := computeRSI(prices, 14)
	atr := strategy.ATR(prices, 14)
	vwap := strategy.RollingVWAP(prices, volumes, 55)
	adx := strategy.ADX(prices, 14)
	emaFast := strategy.EMA(prices, 9)
	emaSlow := strategy.EMA(prices, 21)

	// Count open positions by direction
	openPos := o.posMgr.GetOpenPositions()
	longs, shorts := 0, 0
	for _, p := range openPos {
		if string(p.Side) == "BUY" {
			longs++
		} else {
			shorts++
		}
	}

	// Get account stats
	equityUSD := o.exec.GetEquityUSD()
	dailyPnL := o.risk.GetDailyPnL()

	market := ai.MarketContext{
		Symbol:         "BTC-USD",
		Price:          price,
		Regime:         regime,
		RSI:            rsi,
		ATR:            atr,
		VWAP:           vwap,
		ADX:            adx,
		EMAFast:        emaFast,
		EMASlow:        emaSlow,
		RecentCandles:  candles,
		OpenPositions:  len(openPos),
		LongPositions:  longs,
		ShortPositions: shorts,
		Balance:        equityUSD,
		DailyPnL:       dailyPnL,
	}

	decision := o.aiAgent.Decide(ctx, market)

	if !decision.RiskVerdict.Approved || decision.FinalAction == "HOLD" {
		log.Printf("[AI] 🤖 %s → HOLD | %s", decision.ID, decision.RiskVerdict.Reasoning)
		return
	}

	// Build a strategy.Signal from the AI decision and execute it
	riskSig := decision.RiskVerdict
	var activeSig AgentSignalForExec
	if decision.FinalAction == "BUY" {
		activeSig = AgentSignalForExec{
			action:        strategy.ActionBuy,
			size:          riskSig.AdjustedSize,
			stopLossPct:   decision.BullSignal.StopLossPct,
			takeProfitPct: decision.BullSignal.TakeProfitPct,
			confidence:    decision.BullSignal.Confidence,
		}
	} else {
		activeSig = AgentSignalForExec{
			action:        strategy.ActionSell,
			size:          riskSig.AdjustedSize,
			stopLossPct:   decision.BearSignal.StopLossPct,
			takeProfitPct: decision.BearSignal.TakeProfitPct,
			confidence:    decision.BearSignal.Confidence,
		}
	}

	if normalizedSize := targetSizeForCapital(price); normalizedSize > 0 {
		activeSig.size = normalizedSize
	}

	// Sanitize size — reject sub-floor AI signals; never bump to minExecutionSizeBTC.
	if activeSig.size < minExecutionSizeBTC {
		log.Printf("[RISK REJECTION] AI_%s: size %.6f BTC below execution floor %.6f BTC — skipping",
			decision.ID, activeSig.size, minExecutionSizeBTC)
		return
	}
	if activeSig.size > 0.5 {
		activeSig.size = 0.5
	}

	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        activeSig.action,
		TargetSize:    activeSig.size,
		Confidence:    activeSig.confidence,
		StopLossPct:   activeSig.stopLossPct,
		TakeProfitPct: activeSig.takeProfitPct,
	}

	// Sanitize SL/TP
	sanitized, sanitizeReason, ok := sanitizeSignalForProfit(sig)
	if !ok {
		log.Printf("[AI] Signal sanitization failed — skipping: %s", sanitizeReason)
		return
	}
	sig = sanitized

	// Risk engine validation
	if err := o.risk.Validate(sig, price); err != nil {
		log.Printf("[AI] Risk engine rejected AI signal: %v", err)
		return
	}

	fill, err := o.executeThroughInstitutionalPath(ctx, sig, "AI_"+decision.ID, price, execution.OrderModeIOC)
	if err != nil {
		log.Printf("[AI] Execution failed: %v", err)
		return
	}

	o.risk.NotifyFill(sig)
	o.openAndTrackPosition(ctx, sig, fill, fmt.Sprintf("AI_%s", decision.ID))

	// Mark this decision as executed
	decision.Executed = true
	o.aiAgent.GetInsights().Add(decision)

	log.Printf("[AI] ✅ %s EXECUTED %s %.4f BTC @ $%.2f | Bull: %s",
		decision.ID, decision.FinalAction, sig.TargetSize, fill.ExecPrice,
		truncate(decision.BullSignal.Thesis, 80))
}

// AgentSignalForExec holds execution parameters derived from the winning agent.
type AgentSignalForExec struct {
	action        strategy.Action
	size          float64
	stopLossPct   float64
	takeProfitPct float64
	confidence    float64
}

// computeRSI calculates RSI(n) using Wilder's smoothed average method.
// Requires at least period+1 prices; returns neutral 50 when insufficient data.
func computeRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50.0
	}

	// Seed: simple average of first `period` deltas
	avgGain, avgLoss := 0.0, 0.0
	for i := 1; i <= period; i++ {
		delta := prices[i] - prices[i-1]
		if delta > 0 {
			avgGain += delta
		} else {
			avgLoss -= delta
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// Wilder smoothing over remaining bars
	for i := period + 1; i < len(prices); i++ {
		delta := prices[i] - prices[i-1]
		if delta > 0 {
			avgGain = (avgGain*float64(period-1) + delta) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-delta)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100.0
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// processStrategyGroup runs a group of strategies against a tick/candle and
// processes any resulting signals through aggregation, risk, and execution.
func (o *Orchestrator) processStrategyGroup(ctx context.Context, entries []strategy.RegistryEntry, t marketdata.Tick, timeframe string) {
	// Anchor pipeline timer at tick arrival; use exchange timestamp when available.
	tickAt := time.Now()
	if t.TimeMs > 0 {
		tickAt = time.UnixMilli(t.TimeMs)
	}
	pt := observability.NewPipelineTimerAt("paper", t.Symbol, tickAt)
	ctx = observability.WithPipelineTimer(ctx, pt)
	stageStart := tickAt

	var wg sync.WaitGroup
	var sigMu sync.Mutex
	var rawSignals []AggregatedSignal

	for _, entry := range entries {
		wg.Add(1)
		go func(e strategy.RegistryEntry) {
			defer wg.Done()

			// Skip disabled strategies
			if !o.tracker.IsEnabled(e.Strategy.Name()) {
				return
			}

			signals := e.Strategy.OnTick(t)
			normalizedCategory := strategy.NormalizeCategory(e.Category, e.Strategy.Name())
			executionWeight := o.tracker.GetExecutionWeight(e.Strategy.Name())
			totalTrades := 0
			winRate := 0.5
			totalPnL := 0.0
			if stats, ok := o.tracker.GetStats(e.Strategy.Name()); ok {
				totalTrades = stats.TotalTrades
				totalPnL = stats.TotalPnL
				if stats.TotalTrades > 0 {
					winRate = float64(stats.Wins) / float64(stats.TotalTrades)
				}
			}

			now := time.Now()
			for _, sig := range signals {
				if sig.Action == strategy.ActionHold {
					continue
				}
				// Stamp provenance for expiry gating and latency attribution.
				sig.CreatedAt = now
				sig.Timeframe = timeframe
				sigMu.Lock()
				rawSignals = append(rawSignals, AggregatedSignal{
					Signal:          sig,
					StrategyName:    e.Strategy.Name(),
					Category:        normalizedCategory,
					ExecutionWeight: executionWeight,
					TotalTrades:     totalTrades,
					WinRate:         winRate,
					TotalPnL:        totalPnL,
				})
				sigMu.Unlock()
			}
		}(entry)
	}
	wg.Wait()

	// Stage 1: Tick → Strategy evaluation complete.
	stageStart = observability.RecordPipelineStage(ctx, observability.StageTickToStrategy, stageStart)

	// Aggregate and filter signals
	if len(rawSignals) == 0 {
		return
	}
	approved := o.aggregator.FilterSignalsSelective(rawSignals)
	regime := o.classifyMarketRegime()

	// Persist the live regime so executeThroughInstitutionalPath can pass it to
	// the Risk V2 MarketState (P2-B: eliminates hardcoded RegimeUnknown).
	o.mu.Lock()
	o.lastRegime = regime
	o.mu.Unlock()

	// Execute approved signals
	for _, aggSig := range approved {
		sig := aggSig.Signal

		// Phase 22D: open an execution-intelligence lifecycle for this approved
		// signal. The signal has cleared aggregation, so it begins life as both
		// Generated and Approved; every downstream rejection or fill is recorded
		// against this id.
		sigID := fmt.Sprintf("%s-%d", aggSig.StrategyName, time.Now().UnixNano())
		o.execIntel.Begin(execintel.Meta{
			SignalID:    sigID,
			Strategy:    aggSig.StrategyName,
			Category:    aggSig.Category,
			AlphaSource: aggSig.StrategyName,
			Symbol:      sig.Symbol,
			Direction:   string(sig.Action),
			Price:       o.lastPrice,
			Size:        sig.TargetSize,
			Regime:      regime,
			Timeframe:   sig.Timeframe,
		})
		o.execIntel.Record(sigID, execintel.StateSignalApproved, "passed selective aggregator")

		// Stage 2: Strategy → Risk gate entry.
		stageStart = observability.RecordPipelineStage(ctx, observability.StageStrategyToRisk, stageStart)

		// Record signal in tracker
		o.tracker.RecordSignal(aggSig.StrategyName)

		// ── Stale signal guard ─────────────────────────────────────────────
		// Reject signals whose entry thesis expired before reaching execution.
		// Two ceilings apply, and an expired signal MUST never execute:
		//   1. signalMaxAge — the strict operational guard (fires first).
		//   2. execintel.HardExpiry — the Phase 22D hard ceiling (outer bound).
		if sig.CreatedAt.IsZero() {
			sig.CreatedAt = time.Now()
		}
		signalAge := time.Since(sig.CreatedAt)
		if maxAge := signalMaxAge(sig.Timeframe); signalAge > maxAge || execintel.IsExpired(sig.Timeframe, signalAge) {
			log.Printf("[STALE SIGNAL] %s dropped: age %v > %s max %v (hard ceiling %v)",
				aggSig.StrategyName, signalAge.Round(time.Millisecond), sig.Timeframe, maxAge,
				execintel.HardExpiry(sig.Timeframe))
			o.aggregator.RecordSignalFlowStage(SignalStageExecution, 1, 0)
			o.aggregator.RecordSignalFlowRejection(SignalStageExecution, "stale_signal_expired", aggSig.Category)
			o.execIntel.RecordRejection(sigID, "stale_signal_expired")
			continue
		}

		// Position limit check: prevent stacking too many positions per strategy
		if !o.posMgr.CanOpenPosition(aggSig.StrategyName) {
			log.Printf("[POSITION LIMIT] %s already at max positions — skipping", aggSig.StrategyName)
			o.aggregator.RecordSignalFlowStage(SignalStageExecution, 1, 0)
			o.aggregator.RecordSignalFlowRejection(SignalStageExecution, "position_limit", aggSig.Category)
			o.execIntel.RecordRejection(sigID, "position_limit")
			continue
		}

		// Risk validation
		o.mu.RLock()
		currentPrice := o.lastPrice
		o.mu.RUnlock()

		if !isCategoryAlignedWithRegime(aggSig.Category, regime) {
			log.Printf("[REGIME FILTER] %s skipped in %s regime (%s category)",
				aggSig.StrategyName, regime, aggSig.Category)
			o.aggregator.RecordSignalFlowStage(SignalStageRegimeFilter, 1, 0)
			o.aggregator.RecordSignalFlowRejection(SignalStageRegimeFilter, "category_not_aligned_with_regime", aggSig.Category)
			o.execIntel.RecordRejection(sigID, "category_not_aligned_with_regime")
			continue
		}
		o.aggregator.RecordSignalFlowStage(SignalStageRegimeFilter, 1, 1)

		// Enforce a fixed 1% capital budget per trade for the futures engine.
		originalSize := sig.TargetSize
		baseSize := originalSize
		if normalizedSize := targetSizeForCapital(currentPrice); normalizedSize > 0 {
			baseSize = normalizedSize
		}
		executionWeight := o.tracker.GetExecutionWeight(aggSig.StrategyName)
		if executionWeight < minExecutionWeightToTrade {
			log.Printf("[QUALITY FILTER] %s skipped due to weak execution weight %.2f",
				aggSig.StrategyName, executionWeight)
			o.aggregator.RecordSignalFlowStage(SignalStageExecutionWeightFilter, 1, 0)
			o.aggregator.RecordSignalFlowRejection(SignalStageExecutionWeightFilter, "execution_weight_below_floor", aggSig.Category)
			o.execIntel.RecordRejection(sigID, "execution_weight_below_floor")
			continue
		}
		o.aggregator.RecordSignalFlowStage(SignalStageExecutionWeightFilter, 1, 1)
		sig.TargetSize = baseSize
		sig.Confidence = adjustConfidenceByExecutionWeight(sig.Confidence, executionWeight)

		if sig.TargetSize < minExecutionSizeBTC {
			log.Printf("[SIZE ENGINE] %s size too small after scaling (%.6f BTC) — skipping",
				aggSig.StrategyName, sig.TargetSize)
			o.aggregator.RecordSignalFlowStage(SignalStageExecution, 1, 0)
			o.aggregator.RecordSignalFlowRejection(SignalStageExecution, "size_below_minimum", aggSig.Category)
			o.execIntel.RecordRejection(sigID, "size_below_minimum")
			continue
		}

		if sig.TargetSize-originalSize > sizeChangeEpsilonBTC || originalSize-sig.TargetSize > sizeChangeEpsilonBTC {
			log.Printf("[SIZE ENGINE] %s normalized %.4f -> %.4f BTC to the fixed 1%% capital rule",
				aggSig.StrategyName, originalSize, sig.TargetSize)
		}

		baseStopLossPct := sig.StopLossPct
		baseTakeProfitPct := sig.TakeProfitPct
		sanitizedSig, sanitizeReason, allowed := sanitizeSignalForProfit(sig)
		if !allowed {
			log.Printf("[PROFIT FILTER] %s dropped: %s",
				aggSig.StrategyName, sanitizeReason)
			o.aggregator.RecordSignalFlowStage(SignalStageConfidenceFilter, 1, 0)
			o.aggregator.RecordSignalFlowRejection(SignalStageConfidenceFilter, sanitizeReason, aggSig.Category)
			o.execIntel.RecordRejection(sigID, sanitizeReason)
			continue
		}
		o.aggregator.RecordSignalFlowStage(SignalStageConfidenceFilter, 1, 1)
		sig = sanitizedSig
		if sig.StopLossPct != baseStopLossPct || sig.TakeProfitPct != baseTakeProfitPct {
			log.Printf("[GEOMETRY] %s SL/TP %.2f%%/%.2f%% -> %.2f%%/%.2f%% (R:R %.2f)",
				aggSig.StrategyName, baseStopLossPct, baseTakeProfitPct,
				sig.StopLossPct, sig.TakeProfitPct, sig.TakeProfitPct/sig.StopLossPct)
		}
		// Phase 22D: record the TP-override applied by the profit sanitizer so its
		// realized impact can be audited once the trade closes.
		if sig.TakeProfitPct != baseTakeProfitPct {
			o.execIntel.RecordTPOverride(execintel.TPOverrideSample{
				Strategy:   aggSig.StrategyName,
				Source:     "sanitize",
				OriginalTP: baseTakeProfitPct,
				AdjustedTP: sig.TakeProfitPct,
			})
		}

		err := o.risk.Validate(sig, currentPrice)
		if err != nil {
			log.Printf("[RISK DROPPED] %s from %s: %s", sig.Action, aggSig.StrategyName, err.Error())
			o.aggregator.RecordSignalFlowStage(SignalStageRiskFilter, 1, 0)
			o.aggregator.RecordSignalFlowRejection(SignalStageRiskFilter, err.Error(), aggSig.Category)
			o.execIntel.RecordRejection(sigID, "risk: "+err.Error())
			continue
		}
		o.aggregator.RecordSignalFlowStage(SignalStageRiskFilter, 1, 1)
		o.execIntel.Record(sigID, execintel.StateRiskApproved, "pre-trade risk validated")

		// Stage 3: Risk approved → OMS submission.
		stageStart = observability.RecordPipelineStage(ctx, observability.StageRiskToOMS, stageStart)

		orderMode := execution.RouteModeForCategory(aggSig.Category, regime)

		// ══════════════════════════════════════════════════════════════════════
		// AI SIGNAL AUDIT — Command Center bridge parking
		// Signals are parked ONLY when a human has the Command Center open
		// (bridge heartbeat < 15s). When the bridge is offline, all signals
		// that pass regime/risk/confidence checks execute directly.
		// The AI multi-agent loop (processAIDecisions) runs independently
		// and places its own trades without touching this execution path.
		// ══════════════════════════════════════════════════════════════════════
		if o.IsBridgeOnline() && !isTrustedStrategy(aggSig.StrategyName, sig.Confidence) {
			// Build market context for the parked signal card
			o.mu.RLock()
			prices := append([]float64(nil), o.priceWindow...)
			volumes := append([]float64(nil), o.volumeWindow...)
			o.mu.RUnlock()

			o.candleHistMu.Lock()
			candles := append([]ai.CandleSummary(nil), o.candleHistory...)
			o.candleHistMu.Unlock()

			market := ai.MarketContext{
				Symbol:        sig.Symbol,
				Price:         currentPrice,
				Regime:        regime,
				RSI:           computeRSI(prices, 14),
				ATR:           strategy.ATR(prices, 14),
				VWAP:          strategy.RollingVWAP(prices, volumes, 55),
				ADX:           strategy.ADX(prices, 14),
				EMAFast:       strategy.EMA(prices, 9),
				EMASlow:       strategy.EMA(prices, 21),
				RecentCandles: candles,
				OpenPositions: len(o.posMgr.GetOpenPositions()),
				Balance:       o.exec.GetEquityUSD(),
				DailyPnL:      o.risk.GetDailyPnL(),
			}

			pendingID := fmt.Sprintf("SIG-%d", time.Now().UnixNano()/1e6)

			o.pendingMu.Lock()
			o.pendingSignals[pendingID] = PendingSignal{
				ID:           pendingID,
				Signal:       sig,
				StrategyName: aggSig.StrategyName,
				Category:     aggSig.Category,
				Context:      market,
				AutoPrompt:   generateAutoPrompt(market, aggSig.StrategyName, string(sig.Action)),
				CreatedAt:    time.Now(),
			}
			o.pendingMu.Unlock()

			log.Printf("[COMMAND CENTER] 🛰️  Signal Parked: %s %s [%s] -> Bridge online, waiting for UI",
				aggSig.StrategyName, sig.Action, pendingID)
			o.aggregator.RecordSignalFlowStage(SignalStageExecution, 1, 0)
			o.aggregator.RecordSignalFlowRejection(SignalStageExecution, "parked_for_command_center_bridge", aggSig.Category)
			o.execIntel.RecordRejection(sigID, "parked_for_command_center_bridge")
			continue
		}

		// Bridge is offline — execute directly (or bridge is online but this is a trusted strategy)
		if isTrustedStrategy(aggSig.StrategyName, sig.Confidence) {
			log.Printf("[TRUSTED BYPASS] %s → executing directly (conf=%.2f)",
				aggSig.StrategyName, sig.Confidence)
		}

		// ══════════════════════════════════════════════════════════════════════
		// 13. EXECUTION — Fill via Coinbase Advanced Trad (Live/Paper)
		// ══════════════════════════════════════════════════════════════════════
		o.execIntel.Record(sigID, execintel.StateOrderSubmitted, string(orderMode))
		fill, err := o.executeThroughInstitutionalPath(ctx, sig, aggSig.StrategyName, currentPrice, orderMode)
		if err != nil {
			log.Printf("[EXECUTION FAILED] %s from %s: %s", sig.Action, aggSig.StrategyName, err.Error())
			o.aggregator.RecordSignalFlowStage(SignalStageExecution, 1, 0)
			o.aggregator.RecordSignalFlowRejection(SignalStageExecution, err.Error(), aggSig.Category)
			o.execIntel.RecordRejection(sigID, "oms: "+err.Error())
			continue
		}
		o.aggregator.RecordSignalFlowStage(SignalStageExecution, 1, 1)
		o.aggregator.RecordStrategyExecution(aggSig.StrategyName)
		execPrice := fill.ExecPrice

		// Phase 22D lifecycle: order acknowledged + filled.
		o.execIntel.Record(sigID, execintel.StateOrderAcknowledged, fill.ClientOrderID)
		o.execIntel.Record(sigID, execintel.StateOrderFilled, fmt.Sprintf("%.2f", execPrice))

		// Stage 4-6: OMS dispatch → Exchange ack → Fill → Ledger write.
		stageStart = observability.RecordPipelineStage(ctx, observability.StageOMSToExchange, stageStart)
		stageStart = observability.RecordPipelineStage(ctx, observability.StageExchangeToFill, stageStart)
		_ = observability.RecordPipelineStage(ctx, observability.StageFillToLedger, stageStart)
		pt.Finalise()

		// Phase 22D: record real slippage from the fill (decision price → fill price),
		// attributed by strategy, alpha source, session, regime, and direction.
		refPrice := fill.RequestedPrice
		if refPrice <= 0 {
			refPrice = currentPrice
		}
		if refPrice > 0 && execPrice > 0 {
			o.execIntel.RecordSlippage(execintel.SlippageSample{
				Strategy:    aggSig.StrategyName,
				AlphaSource: aggSig.StrategyName,
				Category:    aggSig.Category,
				Session:     execintel.SessionForUTC(time.Now().UTC().Hour()),
				Regime:      regime,
				Direction:   string(sig.Action),
				SignalPrice: refPrice,
				FilledPrice: execPrice,
				Size:        sig.TargetSize,
			})
			log.Printf("[SLIPPAGE] %s %s entry slippage %.2f bps (expected $%.2f, filled $%.2f)",
				aggSig.StrategyName, sig.Timeframe, fill.SlippageBps, refPrice, execPrice)
		}

		// Notify risk engine
		o.risk.NotifyFill(sig)

		// P3-A: update PMS portfolio state after every fill so the next pre-trade
		// check sees current heat and drawdown levels.
		o.syncPMSState(ctx)

		// Open tracked position with SL/TP and emit OMS v3 EventPositionOpened
		o.openAndTrackPosition(ctx, sig, fill, aggSig.StrategyName)
		o.execIntel.Record(sigID, execintel.StatePositionOpened, aggSig.StrategyName)
		// Remember the lifecycle id so the close-event handler can finalize it.
		o.rememberSignalForPosition(fill.ClientOrderID, sigID)

		log.Printf("[EXECUTION ROUTE] %s used %s in %s regime", aggSig.StrategyName, fill.OrderMode, regime)

		log.Printf("[✅ TRADE EXECUTED] %s | %s %.4f BTC @ $%.2f | Strategy: %s | Age: %v",
			sig.Action, sig.Symbol, sig.TargetSize, execPrice, aggSig.StrategyName,
			time.Since(sig.CreatedAt).Round(time.Millisecond))
	}
}

// processCloseEvents listens for position close events (SL/TP hits) and records them.
func (o *Orchestrator) processCloseEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-o.posMgr.CloseEvents():
			// ═══ FIX: Settle paper balance (credit USD back) ═══
			// Without this, every BUY drains the balance permanently
			// because no SELL ever executes to return the USD.
			o.exec.SettlePosition(event.Position.Side, event.Position.Size, event.ExitPrice)
			feeBreakdown := execution.CanonicalTradeFees(
				event.Position.EntryPrice,
				event.ExitPrice,
				event.Position.Size,
			)
			netPnL := execution.CanonicalNetPnL(event.PnL, feeBreakdown)

			// Record in trade journal (cache — same values persisted to MongoDB)
			entry := execution.JournalEntry{
				ID:           event.Position.ID,
				StrategyName: event.Position.StrategyName,
				Side:         string(event.Position.Side),
				EntryPrice:   event.Position.EntryPrice,
				ExitPrice:    event.ExitPrice,
				Size:         event.Position.Size,
				GrossPnL:     event.PnL,
				Fees:         feeBreakdown.TotalFee,
				NetPnL:       netPnL,
				Reason:       string(event.Reason),
				EntryTime:    event.Position.OpenedAt,
				ExitTime:     time.Now(),
			}
			o.journal.RecordTrade(entry)

			if o.portfolioLedger != nil {
				o.portfolioLedger.RecordClose(
					event.PnL,
					feeBreakdown.EntryFee,
					feeBreakdown.ExitFee,
					netPnL,
					o.exec.GetBalanceUSD(),
				)
			}

			// Update strategy tracker
			o.tracker.RecordTradeResult(event.Position.StrategyName, netPnL)

			// Phase 22D: finalize the execution-intelligence lifecycle for this
			// position — record the terminal TP/SL state, realized PnL for
			// conversion stats, and attribute the TP-override outcome.
			o.finalizeExecIntelClose(event, netPnL)

			// Update risk engine daily PnL tracker
			o.risk.RecordPnL(netPnL)

			// Update risk engine (reduce exposure)
			closeSig := strategy.Signal{
				Symbol:     event.Position.Symbol,
				Action:     strategy.ActionSell,
				TargetSize: event.Position.Size,
			}
			if event.Position.Side == strategy.ActionSell {
				closeSig.Action = strategy.ActionBuy
			}
			o.risk.NotifyFill(closeSig)

			// Emit OMS v3 EventPositionClosed so the ledger becomes the
			// authoritative history of every closed position.
			o.positionToOrderMu.RLock()
			clientOrderID := o.positionToOrderID[event.Position.ID]
			o.positionToOrderMu.RUnlock()
			if clientOrderID != "" {
				go o.emitPositionClosed(ctx, event, clientOrderID, netPnL)
				o.positionToOrderMu.Lock()
				delete(o.positionToOrderID, event.Position.ID)
				o.positionToOrderMu.Unlock()
			}

			// Phase 31B: persist closed position + trade record to MongoDB.
			o.persistPositionClose(ctx, event, netPnL, clientOrderID)
			o.persistClosedTrade(ctx, event, netPnL)

			log.Printf("[✅ TRADE CLOSED] %s | %s | Entry: $%.2f → Exit: $%.2f | PnL: $%.4f | Reason: %s",
				event.Position.StrategyName, event.Position.Side,
				event.Position.EntryPrice, event.ExitPrice, event.PnL, event.Reason)
		}
	}
}

// publishExecIntel mirrors the execution-intelligence snapshot into Prometheus
// gauges every 15 seconds so dashboards and alerts can track conversion,
// latency, slippage, and quality without scraping the JSON endpoint.
func (o *Orchestrator) publishExecIntel(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			execintel.PublishPrometheus(o.execIntel.Snapshot())
		}
	}
}

// strategyCooldownChecker periodically re-enables strategies that have cooled down.
func (o *Orchestrator) strategyCooldownChecker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.tracker.ReEnableExpired()
		}
	}
}

func (o *Orchestrator) RecordBridgeHeartbeat() {
	o.bridgeHeartbeatMu.Lock()
	defer o.bridgeHeartbeatMu.Unlock()
	o.lastBridgeHeartbeat = time.Now()
}

func (o *Orchestrator) RecordBridgeEvent(event, level string) {
	now := time.Now()
	o.bridgeStateMu.Lock()
	defer o.bridgeStateMu.Unlock()
	o.lastBridgeEvent = strings.TrimSpace(event)
	o.lastBridgeEventAt = now
	if strings.EqualFold(level, "error") {
		o.lastBridgeError = strings.TrimSpace(event)
		o.lastBridgeErrorAt = now
	}
}

func (o *Orchestrator) autoFallbackMonitor(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.bridgeHeartbeatMu.RLock()
			bridgeOffline := time.Since(o.lastBridgeHeartbeat) > 15*time.Second
			o.bridgeHeartbeatMu.RUnlock()

			if bridgeOffline {
				o.pendingMu.RLock()
				// Collect IDs to process to avoid holding lock during AI call
				var toProcess []string
				now := time.Now()
				for id, p := range o.pendingSignals {
					if now.Sub(p.CreatedAt) > 45*time.Second {
						toProcess = append(toProcess, id)
					}
				}
				o.pendingMu.RUnlock()

				if len(toProcess) > 0 && !o.hasBackendAIFallback() {
					age := o.bridgeAge().Round(time.Second)
					msg := fmt.Sprintf("Bridge offline for %s and no backend AI fallback is configured; keeping %d parked signal(s) queued", age, len(toProcess))
					log.Printf("[FAILOVER] %s", msg)
					o.RecordBridgeEvent(msg, "warn")
					continue
				}

				for _, id := range toProcess {
					log.Printf("[FAILOVER] 🔄 Bridge Offline (Last seen %s ago). Triggering Auto-Cloud Fallback for %s",
						o.bridgeAge().Round(time.Second), id)
					go func(sigID string) {
						if err := o.ConfirmSignal(ctx, sigID, "AUTOMATIC_CLOUD_FALLBACK"); err != nil {
							log.Printf("[FAILOVER] Auto fallback failed for %s: %v", sigID, err)
						}
					}(id)
				}
			}
		}
	}
}

// GetLastPrice returns the latest BTC price (for API endpoints).
// GetPendingSignals returns the list of signals currently parked in the Command Center.
func (o *Orchestrator) GetPendingSignals() []PendingSignal {
	now := time.Now()

	// Collect stale IDs under read lock first
	o.pendingMu.RLock()
	var stale []string
	res := make([]PendingSignal, 0, len(o.pendingSignals))
	for id, p := range o.pendingSignals {
		if now.Sub(p.CreatedAt) > 5*time.Minute {
			stale = append(stale, id)
		} else {
			res = append(res, p)
		}
	}
	o.pendingMu.RUnlock()

	// Promote to write lock only when there is something to delete
	if len(stale) > 0 {
		o.pendingMu.Lock()
		for _, id := range stale {
			delete(o.pendingSignals, id)
		}
		o.pendingMu.Unlock()
	}

	return res
}

// AddTestSignal - INJECTS a fake signal for local testing of the Robot
func (o *Orchestrator) AddTestSignal() {
	o.pendingMu.Lock()
	defer o.pendingMu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("test_%d", now.UnixNano())
	ctx := ai.MarketContext{
		Symbol:        "BTC-USD",
		Price:         70000,
		Regime:        marketRegimeTrend,
		RSI:           61.5,
		ADX:           28.0,
		VWAP:          69880,
		ATR:           245,
		RecentCandles: []ai.CandleSummary{{High: 70120, Low: 69780, Close: 70040, Volume: 182.4}},
	}
	o.pendingSignals[id] = PendingSignal{
		ID:           id,
		StrategyName: "RAIG_COMBAT_SIMULATOR",
		Signal: strategy.Signal{
			Symbol:        "BTC-USD",
			Action:        strategy.ActionBuy,
			TargetSize:    targetSizeForCapital(ctx.Price),
			Confidence:    0.92,
			StopLossPct:   0.45,
			TakeProfitPct: 0.90,
		},
		Context:    ctx,
		CreatedAt:  now,
		AutoPrompt: generateAutoPrompt(ctx, "RAIG_COMBAT_SIMULATOR", string(strategy.ActionBuy)),
	}
	log.Println("[RAIG] TEST SIGNAL INJECTED INTO COMMAND CENTER.")
}

// ConfirmSignal triggers the AI Audit for a parked signal and executes if approved.
func (o *Orchestrator) ConfirmSignal(ctx context.Context, pendingID, userPrompt string) error {
	if !o.hasBackendAIFallback() {
		return fmt.Errorf("backend AI fallback unavailable: configure an AI provider or keep the browser bridge online")
	}

	o.pendingMu.Lock()
	p, ok := o.pendingSignals[pendingID]
	if !ok {
		o.pendingMu.Unlock()
		return fmt.Errorf("signal %s not found or expired", pendingID)
	}
	delete(o.pendingSignals, pendingID)
	o.pendingMu.Unlock()

	// 1. Final AI Audit via Supreme Court (including User Feedback)
	log.Printf("[COMMAND CENTER] 🧠 Submitting Signal %s to ChatGPT for final audit (Note: %s)", pendingID, userPrompt)

	// We pass the userPrompt as the human feedback to the AI
	approved, reason, _, provider := o.aiAgent.AuditSignalWithFallback(ctx, p.Context, p.StrategyName, string(p.Signal.Action), userPrompt)

	if !approved {
		log.Printf("[COMMAND CENTER] ⛔ ChatGPT REJECTED signal: %s", reason)
		return fmt.Errorf("AI Rejection: %s", reason)
	}

	// 2. Execution
	log.Printf("[COMMAND CENTER] ✅ ChatGPT APPROVED signal! Executing...")

	p.Signal.AIDecisionID = provider
	p.Signal.AIReasoning = fmt.Sprintf("[Human Input: %s] %s", userPrompt, reason)
	if normalizedSize := targetSizeForCapital(p.Context.Price); normalizedSize > 0 {
		p.Signal.TargetSize = normalizedSize
	}

	fill, err := o.executeThroughInstitutionalPath(ctx, p.Signal, p.StrategyName, p.Context.Price, execution.OrderModeIOC)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	// 3. Notify sub-systems
	o.risk.NotifyFill(p.Signal)
	o.openAndTrackPosition(ctx, p.Signal, fill, p.StrategyName)

	log.Printf("[✅ TRADE EXECUTED] %s %s APPROVED via Command Center!", p.StrategyName, p.Signal.Action)
	return nil
}

// ConfirmSignalFromBridge executes a parked signal based on a browser-automation verdict.
func (o *Orchestrator) ConfirmSignalFromBridge(ctx context.Context, pendingID string, decision BridgeDecision) error {
	if err := validateBridgeDecision(decision); err != nil {
		return fmt.Errorf("invalid bridge decision: %w", err)
	}
	if !o.markBridgeSignalProcessing(pendingID) {
		return fmt.Errorf("duplicate bridge result for %s", pendingID)
	}

	o.pendingMu.Lock()
	p, ok := o.pendingSignals[pendingID]
	if !ok {
		o.pendingMu.Unlock()
		return fmt.Errorf("signal %s not found or expired", pendingID)
	}
	delete(o.pendingSignals, pendingID)
	o.pendingMu.Unlock()

	if !decision.Approved {
		log.Printf("[COMMAND CENTER] Browser bridge rejected %s: %s", pendingID, decision.Reason)
		return fmt.Errorf("bridge rejected signal: %s", decision.Reason)
	}

	action := strings.ToUpper(strings.TrimSpace(decision.Action))
	if action != string(strategy.ActionBuy) && action != string(strategy.ActionSell) {
		action = string(p.Signal.Action)
	}
	if action != string(p.Signal.Action) {
		log.Printf("[COMMAND CENTER] Browser bridge action mismatch for %s: wanted %s got %s",
			pendingID, p.Signal.Action, action)
		return fmt.Errorf("bridge action mismatch: expected %s got %s", p.Signal.Action, action)
	}

	if decision.Confidence > 0 {
		if decision.Confidence < 0.70 {
			log.Printf("[COMMAND CENTER] ⛔ Bridge signal %s blocked: ChatGPT confidence %.2f below minimum %.2f",
				pendingID, decision.Confidence, 0.70)
			return fmt.Errorf("bridge confidence %.2f below required %.2f", decision.Confidence, 0.70)
		}
		p.Signal.Confidence = decision.Confidence
	}
	if normalizedSize := targetSizeForCapital(p.Context.Price); normalizedSize > 0 {
		p.Signal.TargetSize = normalizedSize
	}
	sanitized, reason, allowed := sanitizeSignalForProfit(p.Signal)
	if !allowed {
		log.Printf("[COMMAND CENTER] ⛔ Bridge signal %s blocked by profit filter: %s (conf=%.2f)",
			pendingID, reason, p.Signal.Confidence)
		return fmt.Errorf("bridge signal blocked: %s", reason)
	}
	p.Signal = sanitized

	if err := o.risk.Validate(p.Signal, p.Context.Price); err != nil {
		return fmt.Errorf("risk rejected bridge signal: %w", err)
	}

	p.Signal.AIDecisionID = "browser-bridge"
	p.Signal.AIReasoning = strings.TrimSpace(fmt.Sprintf("[Browser Bridge] %s", decision.Reason))

	fill, err := o.executeThroughInstitutionalPath(ctx, p.Signal, p.StrategyName, p.Context.Price, execution.OrderModeIOC)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	o.risk.NotifyFill(p.Signal)
	o.openAndTrackPosition(ctx, p.Signal, fill, p.StrategyName)

	log.Printf("[✅ TRADE EXECUTED] %s %s APPROVED via Browser Bridge | conf=%.2f | reason=%s",
		p.StrategyName, p.Signal.Action, p.Signal.Confidence, truncate(decision.Reason, 120))
	return nil
}

func validateBridgeDecision(decision BridgeDecision) error {
	if strings.TrimSpace(decision.Reason) == "" {
		return fmt.Errorf("missing reason")
	}
	if decision.Confidence < 0 || decision.Confidence > 1 {
		return fmt.Errorf("confidence %.4f out of range", decision.Confidence)
	}
	// Only validate action when the bridge is actually approving a trade
	if decision.Approved {
		action := strings.ToUpper(strings.TrimSpace(decision.Action))
		if action != string(strategy.ActionBuy) && action != string(strategy.ActionSell) {
			return fmt.Errorf("unsupported action %q", decision.Action)
		}
	}
	return nil
}

func (o *Orchestrator) markBridgeSignalProcessing(signalID string) bool {
	o.processedBridgeMu.Lock()
	defer o.processedBridgeMu.Unlock()

	now := time.Now()
	for id, ts := range o.processedBridgeSignals {
		if now.Sub(ts) > 10*time.Minute {
			delete(o.processedBridgeSignals, id)
		}
	}
	if _, exists := o.processedBridgeSignals[signalID]; exists {
		return false
	}
	o.processedBridgeSignals[signalID] = now
	return true
}

func (o *Orchestrator) hasBackendAIFallback() bool {
	return o.aiAgent != nil && o.aiAgent.IsAvailable()
}

func (o *Orchestrator) bridgeAge() time.Duration {
	o.bridgeHeartbeatMu.RLock()
	defer o.bridgeHeartbeatMu.RUnlock()
	return time.Since(o.lastBridgeHeartbeat)
}

func (o *Orchestrator) IsBridgeOnline() bool {
	o.bridgeHeartbeatMu.RLock()
	defer o.bridgeHeartbeatMu.RUnlock()
	return time.Since(o.lastBridgeHeartbeat) < 15*time.Second
}

func (o *Orchestrator) GetBridgeStatus() BridgeStatus {
	o.bridgeHeartbeatMu.RLock()
	lastHeartbeat := o.lastBridgeHeartbeat
	secondsSinceBeat := int(time.Since(lastHeartbeat).Seconds())
	online := time.Since(lastHeartbeat) < 15*time.Second
	o.bridgeHeartbeatMu.RUnlock()

	o.pendingMu.RLock()
	pendingCount := len(o.pendingSignals)
	o.pendingMu.RUnlock()

	o.processedBridgeMu.Lock()
	processedCount := len(o.processedBridgeSignals)
	o.processedBridgeMu.Unlock()

	o.bridgeStateMu.RLock()
	lastEvent := o.lastBridgeEvent
	lastEventAt := o.lastBridgeEventAt
	lastError := o.lastBridgeError
	lastErrorAt := o.lastBridgeErrorAt
	o.bridgeStateMu.RUnlock()

	if secondsSinceBeat < 0 {
		secondsSinceBeat = 0
	}

	return BridgeStatus{
		Online:              online,
		LastHeartbeat:       lastHeartbeat,
		SecondsSinceBeat:    secondsSinceBeat,
		PendingSignals:      pendingCount,
		ProcessedSignalKeys: processedCount,
		LastEvent:           lastEvent,
		LastEventAt:         lastEventAt,
		LastError:           lastError,
		LastErrorAt:         lastErrorAt,
	}
}

func (o *Orchestrator) GetLastPrice() float64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.lastPrice
}

func sanitizeSignalForProfit(sig strategy.Signal) (strategy.Signal, string, bool) {
	adjusted := sig

	if adjusted.Confidence < 0 {
		return adjusted, fmt.Sprintf("confidence %.2f is invalid (must be >= 0)", adjusted.Confidence), false
	}
	if adjusted.Confidence == 0 {
		adjusted.Confidence = 1.0
	}
	if adjusted.Confidence < minExecutableConfidence {
		return adjusted, fmt.Sprintf("confidence %.2f below minimum %.2f", adjusted.Confidence, minExecutableConfidence), false
	}

	if adjusted.StopLossPct <= 0 {
		adjusted.StopLossPct = 0.10
	}
	if adjusted.StopLossPct > maxSignalStopLossPct {
		adjusted.StopLossPct = maxSignalStopLossPct
	}

	// Whether the strategy explicitly provided a TP. When TP is explicitly set
	// we honour the strategy's trade geometry and skip R:R inflation — the
	// strategy's own edge calculation already bakes in the expected return.
	tpExplicit := adjusted.TakeProfitPct > 0

	if !tpExplicit {
		// No strategy-defined TP: apply the minimum floor so we always have a
		// viable exit and a usable R:R baseline.
		adjusted.TakeProfitPct = minSignalTakeProfitPct

		minTakeProfitByRR := adjusted.StopLossPct * minRewardToRiskRatio
		if adjusted.TakeProfitPct < minTakeProfitByRR {
			adjusted.TakeProfitPct = minTakeProfitByRR
		}
	}

	// Always enforce an absolute floor — even on explicit TPs — to avoid
	// exits so tight that fees erase the entire gain.
	const absoluteTPFloor = 0.10
	if adjusted.TakeProfitPct < absoluteTPFloor {
		adjusted.TakeProfitPct = absoluteTPFloor
	}

	return adjusted, "", true
}

// isTrustedStrategy returns true for proven-winner strategies that have
// demonstrated a positive PnL edge in live trading. These bypass the AI/bridge
// parking queue and execute directly when confidence >= 0.80.
func isTrustedStrategy(name string, confidence float64) bool {
	if confidence < 0.80 {
		return false
	}
	switch name {
	case "TripleFilter_Alpha_Scalp",
		"VolumeWeighted_Trend_Scalp",
		"EMA_Cross_Scalp",
		"ZScoreBand_MeanRev_Scalp",
		"OrderFlow_Pressure_Pro_Scalp",
		"BollingerWalk_Trend_Scalp",
		"Stochastic_Range_Scalp",
		"RSI_BB_Confluence_Scalp",
		"LinReg_Statistical_Scalp",
		"Chart_DoubleTap_Reversal_Scalp",
		"OpeningRange_Breakout_Scalp",
		"VolSqueeze_Explosion_Scalp",
		"TrendMomentum_Score_Scalp":
		return true
	}
	return false
}

func adjustConfidenceByExecutionWeight(confidence, executionWeight float64) float64 {
	if confidence < 0 {
		return 0
	}
	if confidence == 0 {
		confidence = 1.0
	}
	adjusted := confidence

	if executionWeight < 1 {
		adjusted *= 0.80 + 0.20*executionWeight
	} else {
		adjusted *= 1.0 + (executionWeight-1.0)*0.25
	}

	if adjusted > 1.5 {
		return 1.5
	}
	if adjusted < 0 {
		return 0
	}
	return adjusted
}

func (o *Orchestrator) classifyMarketRegime() string {
	o.mu.RLock()
	if len(o.priceWindow) < 80 || len(o.volumeWindow) < 80 {
		o.mu.RUnlock()
		return marketRegimeUnknown
	}
	prices := append([]float64(nil), o.priceWindow...)
	volumes := append([]float64(nil), o.volumeWindow...)
	o.mu.RUnlock()

	latestPrice := prices[len(prices)-1]
	fast := strategy.EMA(prices, 21)
	slow := strategy.EMA(prices, 55)
	adx := strategy.ADX(prices, 14)
	vwap := strategy.RollingVWAP(prices, volumes, 55)
	atrFast := strategy.ATR(prices, 14)
	atrSlow := strategy.ATR(prices, 55)
	if atrSlow <= 0 || vwap <= 0 {
		return marketRegimeUnknown
	}

	trendStrength := math.Abs(fast-slow) / (atrSlow * 3.0)
	volRatio := atrFast / atrSlow
	priceVsVWAPPct := math.Abs((latestPrice - vwap) / vwap * 100)
	trendAlignedWithVWAP := (latestPrice >= vwap && fast >= slow) || (latestPrice <= vwap && fast <= slow)

	switch {
	case adx >= 25 && trendStrength >= 0.55 && trendAlignedWithVWAP:
		return marketRegimeTrend
	case volRatio >= 1.45 && adx < 25 && trendStrength < 0.70:
		return marketRegimeVolatile
	case adx <= 20 && trendStrength <= 0.40 && volRatio <= 1.10 && priceVsVWAPPct <= 0.18:
		return marketRegimeRange
	default:
		return marketRegimeMixed
	}
}

func isCategoryAlignedWithRegime(category, regime string) bool {
	switch regime {
	case marketRegimeUnknown:
		// During warmup or unclassifiable periods, allow all proven strategy categories.
		// Blocking 100% during warmup was causing zero trades after every restart.
		return true
	case marketRegimeMixed:
		// Mixed is the most common BTC state. Allow all proven strategy categories
		// including institutional alpha — all 16 alpha modules are valid in mixed conditions.
		switch category {
		case "Multi-Signal", "Trend", "Trend Elite", "Breakout", "Breakout Elite",
			"Momentum", "Momentum Elite", "Mean Reversion", "Mean Rev Elite", "Statistical",
			"Volatility", "Volatility Elite", "Time-of-Day", "Price Action",
			"Price Action Elite", "Adaptive Elite", "Microstructure", "Intraday",
			"Funding", "Liquidity", "Liquidations", "Structure", "Smart Money", "Market Profile", "Session",
			"Phase 11 Liquidity", "Phase 11 Derivatives", "Phase 11 Order Flow",
			"Phase 11 Liquidations", "Phase 11 Structure", "Phase 11 Smart Money":
			return true
		}
		return false
	case marketRegimeTrend:
		switch category {
		case "Trend", "Trend Elite", "Breakout", "Breakout Elite", "Momentum", "Momentum Elite",
			"Time-of-Day", "Microstructure", "Multi-Signal", "Price Action", "Price Action Elite", "Intraday",
			"Structure", "Smart Money", "Phase 11 Structure", "Phase 11 Smart Money":
			return true
		}
		return false
	case marketRegimeRange:
		switch category {
		case "Mean Reversion", "Mean Rev Elite", "Statistical", "Adaptive", "Adaptive Elite",
			"Oscillator Elite", "Price Action", "Price Action Elite", "Multi-Signal", "Intraday",
			"Liquidity", "Funding", "Market Profile", "Session",
			"Phase 11 Liquidity", "Phase 11 Derivatives":
			return true
		}
		return false
	case marketRegimeVolatile:
		switch category {
		case "Volatility", "Volatility Elite", "Breakout", "Breakout Elite", "Microstructure",
			"Time-of-Day", "Multi-Signal", "Momentum Elite", "Intraday",
			"Liquidity", "Liquidations", "Funding", "Phase 11 Liquidations", "Phase 11 Derivatives", "Phase 11 Liquidity":
			return true
		}
		return false
	default:
		return true
	}
}

func generateAutoPrompt(ctx ai.MarketContext, name, action string) string {
	return fmt.Sprintf(`You are reviewing a BTC trading signal for execution safety.

Return ONLY valid JSON with this schema:
{
  "approved": true,
  "action": "%s",
  "confidence": 0.0,
  "reason": "short reason"
}

Rules:
- Keep "action" exactly "%s" if approved.
- Set "approved" false if this trade should be vetoed.
- Confidence must be a number between 0.0 and 1.0.
- Do not include markdown fences.

### BITCOIN TRADING SIGNAL AUDIT ###
STRATEGY: %s
ACTION: %s
PRICE: $%.2f
RSI: %.1f
ADX: %.1f
VWAP: $%.2f
ATR: $%.2f

### RECENT 5M CANDLES ###
(Oldest to Newest)
%s

### INSTRUCTION ###
Analyze the data above and return the JSON decision now.`,
		action, action, name, action, ctx.Price, ctx.RSI, ctx.ADX, ctx.VWAP, ctx.ATR,
		buildCandleHistoryText(ctx))
}

func buildCandleHistoryText(ctx ai.MarketContext) string {
	var sb strings.Builder
	for i, c := range ctx.RecentCandles {
		sb.WriteString(fmt.Sprintf("[%d] H:%.0f L:%.0f C:%.0f V:%.2f\n", i, c.High, c.Low, c.Close, c.Volume))
	}
	return sb.String()
}

// logDecisionFunnel emits a structured [DECISION FUNNEL] log line at every
// trade rejection point. One log per rejection gives forensic reconstruction:
// strategy → family → regime → Kelly output → DynamicSize output → reason.
// P3-D: every rejected trade must expose the full sizing + rejection context.
func logDecisionFunnel(
	strategyName, category string,
	family riskv2.StrategyFamily,
	regime string,
	kellyFraction, kellySizeBTC float64,
	dynMultiplier, dynSizeBTC float64,
	rejectionCode string,
) {
	log.Printf(
		"[DECISION FUNNEL] REJECTED | strategy=%s category=%s family=%s regime=%s"+
			" kelly_fraction=%.4f kelly_size_btc=%.6f"+
			" dyn_multiplier=%.4f dyn_size_btc=%.6f"+
			" reason=%s",
		strategyName, category, family, regime,
		kellyFraction, kellySizeBTC,
		dynMultiplier, dynSizeBTC,
		rejectionCode,
	)
}

// syncPMSState refreshes the PMS portfolio risk state from the live Risk V2
// engine snapshot. Called after every fill so subsequent CheckPortfolioRisk calls
// see current heat, VaR, and drawdown rather than stale values (P3-A).
func (o *Orchestrator) syncPMSState(ctx context.Context) {
	if o.pmsBudget == nil {
		return
	}
	snap := o.risk.V2().Snapshot()
	equityUSD := o.exec.GetEquityUSD()
	if equityUSD <= 0 {
		equityUSD = 1
	}
	heat := snap.Account.EquityUSD // fallback
	if snap.Account.EquityUSD > 0 {
		heat = snap.Account.EquityUSD
	}
	_ = heat
	// Compute heat % from positions
	heatPct := 0.0
	for _, pos := range snap.Positions {
		heatPct += riskv2.PositionRiskFromPosition(pos) / equityUSD * 100
	}
	drawdownPct := 0.0
	if snap.Account.HighWatermarkUSD > 0 && equityUSD < snap.Account.HighWatermarkUSD {
		drawdownPct = (snap.Account.HighWatermarkUSD-equityUSD) / snap.Account.HighWatermarkUSD * 100
	}
	dailyLossUSD := 0.0
	if snap.Account.DailyPnLUSD < 0 {
		dailyLossUSD = -snap.Account.DailyPnLUSD
	}
	o.pmsBudget.UpdateState(
		btcPaperAccountID,
		heatPct, 0, 0, drawdownPct,
		dailyLossUSD, 0, 0,
		0, 0,
		nil, nil, nil,
	)
}

func targetSizeForCapital(currentPrice float64) float64 {
	if currentPrice <= 0 {
		return 0
	}
	return fixedTradeCapitalUSD / currentPrice
}

// buildRiskV2MarketState assembles live market inputs for Risk V2 from the
// orchestrator's rolling price/volume windows and last classified regime (P2-B).
func (o *Orchestrator) buildRiskV2MarketState() riskv2.MarketState {
	o.mu.RLock()
	regime := o.lastRegime
	prices := append([]float64(nil), o.priceWindow...)
	volumes := append([]float64(nil), o.volumeWindow...)
	o.mu.RUnlock()

	state := riskv2.MarketState{
		Regime:         o.riskV2RegimeFromLive(regime, prices),
		LiquidityScore: computeLiquidityScore(volumes),
	}
	if len(prices) >= 14 {
		atr := strategy.ATR(prices, 14)
		latest := prices[len(prices)-1]
		if latest > 0 {
			state.VolatilityPct = atr / latest * 100
		}
		if len(prices) >= 60 {
			anchor := prices[len(prices)-60]
			if anchor > 0 {
				state.BTCMovePct1m = (latest - anchor) / anchor * 100
			}
		}
		if len(prices) >= 300 {
			anchor5m := prices[len(prices)-300]
			if anchor5m > 0 {
				state.BTCMovePct5m = (latest - anchor5m) / anchor5m * 100
			}
		}
	}
	return state
}

// riskV2RegimeFromLive maps the internal regime classifier output to Risk V2
// regime enums, resolving trend direction from live EMA alignment (P2-B).
func (o *Orchestrator) riskV2RegimeFromLive(regime string, prices []float64) riskv2.MarketRegime {
	switch regime {
	case marketRegimeTrend:
		if len(prices) >= 55 {
			fast := strategy.EMA(prices, 21)
			slow := strategy.EMA(prices, 55)
			if fast >= slow {
				return riskv2.RegimeTrendingBull
			}
			return riskv2.RegimeTrendingBear
		}
		return riskv2.RegimeTrendingBull
	case marketRegimeRange:
		return riskv2.RegimeRange
	case marketRegimeVolatile:
		return riskv2.RegimeHighVol
	case marketRegimeMixed:
		return riskv2.RegimeRange
	default:
		return riskv2.RegimeUnknown
	}
}

// computeLiquidityScore derives a 0–1 liquidity score from recent vs baseline volume.
func computeLiquidityScore(volumes []float64) float64 {
	if len(volumes) < 10 {
		return 0.5
	}
	recent := averageFloat(volumes[len(volumes)-5:])
	baseline := averageFloat(volumes)
	if baseline <= 0 {
		return 0.5
	}
	ratio := recent / baseline
	return math.Max(0.05, math.Min(1.0, 0.35+ratio*0.35))
}

func averageFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
