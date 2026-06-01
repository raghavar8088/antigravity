// Package v3 implements the Phase 15F institutional backtest engine.
// It produces realistic fills by layering: order book simulation, queue position,
// latency jitter, slippage curves, market impact, funding, and exchange-specific
// commissions. All fills are optionally emitted through OMS v3 via OMSBridge.
package v3

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

// Engine is the V3 institutional backtest engine.
type Engine struct {
	fill       btExecution.FillModel
	queue      *QueuePositionEngine
	commission *CommissionEngine
	funding    *FundingSimulator
	liquidity  *LiquidityEngine
	stress     *StressEngine
	omsBridge  *OMSBridge
	midHistory *MidPriceHistory
}

// NewEngine constructs a V3 engine from a Config.
// Pass a non-nil OMSBridge to emit ledger events during simulation.
func NewEngine(cfg Config, fundingRates []btExecution.FundingRate, bridge *OMSBridge) *Engine {
	return &Engine{
		fill:       btExecution.NewFillModel(),
		queue:      NewQueuePositionEngine(),
		commission: NewCommissionEngine(cfg.Exchange, cfg.CommissionTier),
		funding:    NewFundingSimulator(cfg.Exchange, fundingRates),
		liquidity:  NewLiquidityEngine(cfg.LiquidityScenario),
		stress:     NewStressEngine(cfg.StressScenarios, time.Now()),
		omsBridge:  bridge,
		midHistory: NewMidPriceHistory(60),
	}
}

// Run executes the V3 backtest simulation over the provided ticks.
func (e *Engine) Run(input V3Input) (V3Result, error) {
	cfg := normalizeConfig(input.Config)
	strategies := input.Strategies
	if input.Strategy != nil {
		strategies = append([]strategy.Strategy{input.Strategy}, strategies...)
	}
	if len(strategies) == 0 {
		return V3Result{}, errors.New("v3 engine requires at least one strategy")
	}
	if len(input.Ticks) == 0 {
		return V3Result{}, errors.New("v3 engine requires at least one tick")
	}

	ctx := context.Background()
	portfolio := newV3Portfolio(cfg.InitialCapitalUSD)
	events := make([]V3Event, 0, len(input.Ticks)/10)

	for _, tick := range input.Ticks {
		ts := tickTimestamp(tick)
		e.midHistory.Add(tick.Price)

		// Build realistic order book from tick + rolling vol.
		vol := e.midHistory.RealizedVolPct()
		book := NewOrderBook(tick.Price, vol/100, 0.65, ts)

		// Apply liquidity scenario.
		e.liquidity.AdjustBook(book)

		// Apply any active stress scenario.
		spreadMult, depthMult := e.stress.CompositeMultipliers(ts)
		if spreadMult > 1 || depthMult < 1 {
			book.ApplyStress(spreadMult, depthMult)
			events = append(events, V3Event{Type: EventV3StressInjected, Message: "stress overlay applied", Timestamp: ts})
		}

		// Price shock from crash scenarios.
		priceAdj := e.stress.PriceAdjustment(ts)
		effectivePrice := tick.Price * (1 + priceAdj/100)
		if effectivePrice <= 0 {
			effectivePrice = tick.Price
		}

		// Check SL/TP on open positions.
		for _, pos := range portfolio.openPositions {
			if shouldCloseV3(pos, effectivePrice, ts) {
				e.closePosition(ctx, pos, tick, effectivePrice, book, cfg, portfolio, &events)
			}
		}

		// Exchange outage blocks new orders.
		if e.stress.IsOrderBlocked(ts) {
			continue
		}

		// Run strategies and process signals.
		warmupOK := len(e.midHistory.prices) >= cfg.WarmupBars
		if !warmupOK {
			continue
		}

		for _, strat := range strategies {
			sigs := allSignals(strat, tick)
			for _, sig := range sigs {
				if sig.Action == strategy.ActionHold {
					continue
				}
				if !cfg.AllowShorts && sig.Action == strategy.ActionSell {
					continue
				}
				if portfolio.hasOpen(strat.Name(), sig.Symbol) {
					continue
				}
				if portfolio.openCount() >= cfg.MaxOpenPositions {
					continue
				}
				sig = normalizeSignalV3(sig, effectivePrice, cfg)
				orderID := fmt.Sprintf("V3-%s-%d", strat.Name(), len(events)+1)
				trade, ok := e.executeEntry(ctx, orderID, sig, tick, effectivePrice, book, cfg, strat.Name(), &events, ts)
				if !ok {
					continue
				}
				portfolio.openPosition(trade)
				events = append(events, V3Event{Type: EventV3PositionOpened, Strategy: strat.Name(), Symbol: sig.Symbol, Timestamp: ts})
			}
		}
	}

	// Force-close all remaining positions at last price.
	if len(input.Ticks) > 0 {
		lastTick := input.Ticks[len(input.Ticks)-1]
		lastTs := tickTimestamp(lastTick)
		book := NewOrderBook(lastTick.Price, 0.20, 0.65, lastTs)
		e.liquidity.AdjustBook(book)
		for _, pos := range portfolio.openPositions {
			e.closePosition(ctx, pos, lastTick, lastTick.Price, book, cfg, portfolio, &events)
		}
	}

	finalEquity := portfolio.equity(lastPrice(input.Ticks))
	analytics := BuildExecutionAnalytics(portfolio.trades)
	mc := RunMonteCarloV3(portfolio.trades, cfg.MonteCarloConfig)
	stressReport := BuildStressReport(portfolio.trades, e.stress.events)

	return V3Result{
		Config:            cfg,
		InitialCapitalUSD: cfg.InitialCapitalUSD,
		FinalCapitalUSD:   finalEquity,
		Trades:            portfolio.trades,
		Analytics:         analytics,
		MonteCarloReport:  mc,
		StressReport:      stressReport,
		Events:            events,
	}, nil
}

// executeEntry processes an entry signal and returns a V3Trade if filled.
func (e *Engine) executeEntry(
	ctx context.Context,
	orderID string,
	sig strategy.Signal,
	tick marketdata.Tick,
	effectivePrice float64,
	book *OrderBook,
	cfg Config,
	strategyName string,
	events *[]V3Event,
	ts time.Time,
) (V3Trade, bool) {
	side := btExecution.SideBuy
	if sig.Action == strategy.ActionSell {
		side = btExecution.SideSell
	}

	// OMS event: order created.
	if e.omsBridge != nil {
		_ = e.omsBridge.RecordOrderCreated(ctx, orderID, sig.Symbol, string(side), strategyName, sig.TargetSize, ts)
	}
	*events = append(*events, V3Event{Type: EventV3OrderCreated, Strategy: strategyName, Symbol: sig.Symbol, Timestamp: ts})

	// Compute rolling volatility.
	vol := clampF(e.midHistory.RealizedVolPct()/100, 0.001, 5)

	// Build market context with dynamic vol and book-derived liquidity.
	liqScore := book.LiquidityScore()
	adv := effectivePrice * 5000
	bookDepth := book.TotalAskDepthUSD(5)
	if sig.Action == strategy.ActionSell {
		bookDepth = book.TotalBidDepthUSD(5)
	}

	// Latency with optional jitter.
	latencyTier := cfg.LatencyTier
	latInput := btExecution.LatencyInput{
		Tier:                 latencyTier,
		SignalEdgeBps:        8,
		MomentumPctPerSecond: 0.01,
	}
	if cfg.LatencyJitter {
		jitterFactor := 1 + gaussianJitter(e.midHistory, 0.3)
		latInput.ExecutionDelay = time.Duration(float64(time.Duration(latencyTier)) * jitterFactor)
	}

	mktCtx := btExecution.MarketContext{
		MidPrice:       effectivePrice,
		VolatilityPct:  vol * 100,
		LiquidityScore: liqScore,
		ADVNotionalUSD: adv,
		BookDepthUSD:   bookDepth,
		MomentumPct:    momentumPct(e.midHistory),
		Timestamp:      ts,
		Latency:        latInput,
	}

	// Additional slippage from liquidity scenario.
	slippageAdder := e.liquidity.SlippageAdder()

	order := btExecution.Order{
		ID:          orderID,
		Symbol:      sig.Symbol,
		Side:        side,
		Type:        btExecution.OrderMarket,
		Quantity:    sig.TargetSize,
		SignalPrice: effectivePrice,
		GeneratedAt: ts,
	}

	fill := e.fill.Execute(order, mktCtx)
	if fill.FilledQuantity <= 0 {
		if e.omsBridge != nil {
			_ = e.omsBridge.RecordOrderRejected(ctx, orderID, sig.Symbol, strategyName, "zero fill", ts)
		}
		return V3Trade{}, false
	}

	// Adjust fill price for scenario slippage adder.
	if slippageAdder > 0 {
		if side == btExecution.SideBuy {
			fill.FillPrice *= 1 + slippageAdder/10_000
		} else {
			fill.FillPrice *= 1 - slippageAdder/10_000
		}
		fill.SlippageCostUSD += fill.FilledQuantity * effectivePrice * slippageAdder / 10_000
	}

	// Queue position analysis for limit order classification.
	queueOrder := QueueOrder{
		OrderID:     orderID,
		Side:        string(side),
		Type:        "MARKET",
		LimitPrice:  fill.FillPrice,
		Quantity:    sig.TargetSize,
		SubmittedAt: ts,
	}
	queueResult := e.queue.Estimate(queueOrder, book, effectivePrice*2)

	// Book walk for market impact (how much book we consumed).
	var bookFill BookFillResult
	if side == btExecution.SideBuy {
		bookFill = book.WalkAsks(fill.FilledQuantity)
	} else {
		bookFill = book.WalkBids(fill.FilledQuantity)
	}

	// Commission.
	notional := fill.FilledQuantity * fill.FillPrice
	commResult := e.commission.RoundTrip(notional, queueResult.Class, ClassTaker)

	// Emit partial fills to OMS if applicable.
	for _, pf := range queueResult.PartialFills {
		if e.omsBridge != nil {
			_ = e.omsBridge.RecordOrderPartialFill(ctx, orderID, sig.Symbol, strategyName, pf)
		}
	}

	// Emit order filled.
	if e.omsBridge != nil {
		slipBps := fill.SlippageCostUSD / notional * 10_000
		_ = e.omsBridge.RecordOrderFilled(ctx, orderID, sig.Symbol, strategyName, fill.FillPrice, fill.FilledQuantity, commResult.FeeUSD, slipBps, fill.Timestamp)
	}
	*events = append(*events, V3Event{Type: EventV3OrderFilled, Strategy: strategyName, Symbol: sig.Symbol, Timestamp: fill.Timestamp})

	trade := V3Trade{
		ID:           orderID,
		StrategyName: strategyName,
		Symbol:       sig.Symbol,
		Side:         sig.Action,
		EntryPrice:   fill.FillPrice,
		Size:         fill.FilledQuantity,
		OpenedAt:     fill.Timestamp,
		FillRatio:    fill.FillRatio,
		EntryFill:    fill,
		QueueMetrics: queueResult,
		PartialFillEvents:  queueResult.PartialFills,
		SimulatedLatencyMs: float64(fill.Latency.EntryLatency.Milliseconds()),
		SpreadCostUSD:  fill.SpreadCostUSD,
		SlippageCostUSD: fill.SlippageCostUSD,
		ImpactCostUSD:  fill.ImpactCostUSD + bookFill.PriceImpactBps*notional/10_000,
		CommissionUSD:  commResult.FeeUSD,
		// StopLoss/TakeProfit tracked in portfolio for exit trigger.
		ExitReason: "",
	}

	// Store TP/SL for position management (packed into trade for portfolio).
	trade.GrossPnL = 0 // set on close

	// OMS bridge: position opened.
	if e.omsBridge != nil {
		_ = e.omsBridge.RecordPositionOpened(ctx, orderID, orderID, sig.Symbol, string(side), strategyName,
			fill.FillPrice, fill.FilledQuantity, sig.StopLossPct, sig.TakeProfitPct, fill.Timestamp)
	}

	return trade, true
}

// closePosition processes a position exit and records the closed trade.
func (e *Engine) closePosition(
	ctx context.Context,
	pos *v3Position,
	tick marketdata.Tick,
	effectivePrice float64,
	book *OrderBook,
	cfg Config,
	portfolio *v3Portfolio,
	events *[]V3Event,
) {
	exitSide := btExecution.SideSell
	if pos.trade.Side == strategy.ActionSell {
		exitSide = btExecution.SideBuy
	}

	vol := clampF(e.midHistory.RealizedVolPct()/100, 0.001, 5)
	liqScore := book.LiquidityScore()
	exitLatency := btExecution.LatencyInput{
		Tier:          cfg.LatencyTier,
		SignalEdgeBps: 5,
	}
	mktCtx := btExecution.MarketContext{
		MidPrice:       effectivePrice,
		VolatilityPct:  vol * 100,
		LiquidityScore: liqScore,
		ADVNotionalUSD: effectivePrice * 5000,
		BookDepthUSD:   book.TotalBidDepthUSD(5),
		MomentumPct:    momentumPct(e.midHistory),
		Timestamp:      tickTimestamp(tick),
		Latency:        exitLatency,
	}

	exitOrder := btExecution.Order{
		ID:          pos.trade.ID + "-EXIT",
		Symbol:      pos.trade.Symbol,
		Side:        exitSide,
		Type:        btExecution.OrderMarket,
		Quantity:    pos.trade.Size,
		SignalPrice:  effectivePrice,
		GeneratedAt: tickTimestamp(tick),
	}
	exitFill := e.fill.Execute(exitOrder, mktCtx)

	// Funding costs over hold period.
	fundingSide := "BUY"
	if pos.trade.Side == strategy.ActionSell {
		fundingSide = "SELL"
	}
	notional := pos.trade.EntryPrice * pos.trade.Size
	fundingResult := e.funding.Apply(fundingSide, notional, pos.trade.OpenedAt, tickTimestamp(tick))

	// Exit commission.
	exitNotional := exitFill.FilledQuantity * exitFill.FillPrice
	exitComm := e.commission.Calculate(exitNotional, ClassTaker)

	// Gross P&L.
	grossPnL := 0.0
	if pos.trade.Side == strategy.ActionSell {
		grossPnL = (pos.trade.EntryPrice - exitFill.FillPrice) * pos.trade.Size
	} else {
		grossPnL = (exitFill.FillPrice - pos.trade.EntryPrice) * pos.trade.Size
	}

	// Net P&L = gross - all costs.
	totalCosts := pos.trade.SpreadCostUSD +
		pos.trade.SlippageCostUSD +
		pos.trade.ImpactCostUSD +
		pos.trade.CommissionUSD +
		exitFill.SpreadCostUSD +
		exitFill.SlippageCostUSD +
		exitFill.ImpactCostUSD +
		exitComm.FeeUSD +
		math.Abs(fundingResult.FundingPaidUSD-fundingResult.FundingReceivedUSD)

	openedAt := pos.trade.OpenedAt
	closedAt := exitFill.Timestamp
	holdMin := closedAt.Sub(openedAt).Minutes()
	if holdMin < 0 {
		holdMin = 0
	}

	// Finalize trade record.
	t := pos.trade
	t.ExitPrice = exitFill.FillPrice
	t.ExitFill = exitFill
	t.ClosedAt = closedAt
	t.HoldMinutes = holdMin
	t.GrossPnL = grossPnL
	t.SpreadCostUSD += exitFill.SpreadCostUSD
	t.SlippageCostUSD += exitFill.SlippageCostUSD
	t.ImpactCostUSD += exitFill.ImpactCostUSD
	t.FundingCostUSD = math.Abs(fundingResult.FundingPaidUSD - fundingResult.FundingReceivedUSD)
	t.CommissionUSD += exitComm.FeeUSD
	t.LatencyCostUSD += float64(exitFill.Latency.EntryLatency.Milliseconds()) * effectivePrice * pos.trade.Size * 0.0001 / 1000
	t.NetPnL = grossPnL - totalCosts
	t.ExitReason = pos.exitReason

	if e.omsBridge != nil {
		_ = e.omsBridge.RecordPositionClosed(ctx, t)
	}
	*events = append(*events, V3Event{Type: EventV3PositionClosed, Strategy: t.StrategyName, Symbol: t.Symbol, Timestamp: t.ClosedAt})

	portfolio.closePosition(pos, t)
}

// ── Portfolio helpers ────────────────────────────────────────────────────────

type v3Position struct {
	trade      V3Trade
	stopLossPct  float64
	takeProfitPct float64
	exitReason  string
}

type v3Portfolio struct {
	initialCapital float64
	balance        float64
	openPositions  []*v3Position
	trades         []V3Trade
}

func newV3Portfolio(capital float64) *v3Portfolio {
	return &v3Portfolio{initialCapital: capital, balance: capital}
}

func (p *v3Portfolio) openPosition(t V3Trade) {
	p.balance -= t.EntryPrice * t.Size
	p.openPositions = append(p.openPositions, &v3Position{
		trade:         t,
		stopLossPct:   0.35,
		takeProfitPct: 0.70,
	})
}

func (p *v3Portfolio) closePosition(pos *v3Position, t V3Trade) {
	p.balance += t.ExitPrice * t.Size
	p.balance += t.NetPnL - (t.ExitPrice-pos.trade.EntryPrice)*t.Size // re-settle net
	p.trades = append(p.trades, t)
	// Remove from open.
	for i, op := range p.openPositions {
		if op == pos {
			p.openPositions = append(p.openPositions[:i], p.openPositions[i+1:]...)
			return
		}
	}
}

func (p *v3Portfolio) hasOpen(strategyName, symbol string) bool {
	for _, pos := range p.openPositions {
		if pos.trade.StrategyName == strategyName && pos.trade.Symbol == symbol {
			return true
		}
	}
	return false
}

func (p *v3Portfolio) openCount() int { return len(p.openPositions) }

func (p *v3Portfolio) equity(mark float64) float64 {
	eq := p.balance
	for _, pos := range p.openPositions {
		if pos.trade.Side == strategy.ActionSell {
			eq += (pos.trade.EntryPrice - mark) * pos.trade.Size
		} else {
			eq += (mark - pos.trade.EntryPrice) * pos.trade.Size
		}
	}
	return eq
}

// ── Utility helpers ──────────────────────────────────────────────────────────

func shouldCloseV3(pos *v3Position, currentPrice float64, ts time.Time) bool {
	entry := pos.trade.EntryPrice
	sl := pos.stopLossPct / 100
	tp := pos.takeProfitPct / 100

	if pos.trade.Side == strategy.ActionSell {
		if currentPrice >= entry*(1+sl) {
			pos.exitReason = "SL"
			return true
		}
		if currentPrice <= entry*(1-tp) {
			pos.exitReason = "TP"
			return true
		}
	} else {
		if currentPrice <= entry*(1-sl) {
			pos.exitReason = "SL"
			return true
		}
		if currentPrice >= entry*(1+tp) {
			pos.exitReason = "TP"
			return true
		}
	}

	// Maximum hold: 45 minutes.
	if ts.Sub(pos.trade.OpenedAt) >= 45*time.Minute {
		pos.exitReason = "TIME"
		return true
	}
	return false
}

func normalizeConfig(cfg Config) Config {
	if cfg.Mode == "" {
		cfg.Mode = SimModeInstitutional
	}
	if cfg.Exchange == "" {
		cfg.Exchange = ExchangeBinance
	}
	if cfg.InitialCapitalUSD <= 0 {
		cfg.InitialCapitalUSD = 1_000_000
	}
	if cfg.RiskPerTradePct <= 0 {
		cfg.RiskPerTradePct = 0.5
	}
	if cfg.LatencyTier <= 0 {
		cfg.LatencyTier = btExecution.Latency50ms
	}
	if cfg.WarmupBars <= 0 {
		cfg.WarmupBars = 50
	}
	if cfg.MaxOpenPositions <= 0 {
		cfg.MaxOpenPositions = 10
	}
	if cfg.CommissionTier == "" {
		cfg.CommissionTier = TierStandard
	}
	if cfg.LiquidityScenario == "" {
		cfg.LiquidityScenario = ScenarioNormal
	}
	if cfg.MonteCarloConfig.Paths <= 0 {
		cfg.MonteCarloConfig.Paths = 5000
	}
	if cfg.MonteCarloConfig.DegreesOfFreedom <= 0 {
		cfg.MonteCarloConfig.DegreesOfFreedom = 4
	}
	if cfg.ValidationConfig.MaxSimErrorPct <= 0 {
		cfg.ValidationConfig.MaxSimErrorPct = 10
	}
	return cfg
}

func normalizeSignalV3(sig strategy.Signal, price float64, cfg Config) strategy.Signal {
	if sig.Symbol == "" {
		sig.Symbol = "BTCUSD"
	}
	if sig.TargetSize <= 0 && price > 0 {
		sig.TargetSize = cfg.InitialCapitalUSD * cfg.RiskPerTradePct / 100 / price
	}
	if sig.StopLossPct <= 0 {
		sig.StopLossPct = 0.35
	}
	if sig.TakeProfitPct <= 0 {
		sig.TakeProfitPct = 0.70
	}
	return sig
}

func allSignals(strat strategy.Strategy, tick marketdata.Tick) []strategy.Signal {
	sigs := strat.OnTick(tick)
	sigs = append(sigs, strat.OnCandle(tick)...)
	return sigs
}

func tickTimestamp(tick marketdata.Tick) time.Time {
	if tick.TimeMs > 0 {
		return time.UnixMilli(tick.TimeMs)
	}
	return time.Now()
}

func lastPrice(ticks []marketdata.Tick) float64 {
	if len(ticks) == 0 {
		return 0
	}
	return ticks[len(ticks)-1].Price
}

// momentumPct estimates short-term momentum from recent mid prices.
func momentumPct(h *MidPriceHistory) float64 {
	n := len(h.prices)
	if n < 5 {
		return 0
	}
	recent := h.prices[n-1]
	older := h.prices[n-5]
	if older <= 0 {
		return 0
	}
	return (recent - older) / older * 100
}

// gaussianJitter returns a small Gaussian noise sample for latency jitter.
func gaussianJitter(h *MidPriceHistory, sigma float64) float64 {
	rng := newSimpleRNG(int64(len(h.prices)))
	return rng.NormFloat64() * sigma
}
