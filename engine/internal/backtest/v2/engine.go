package v2

import (
	"errors"
	"fmt"

	btExecution "antigravity-engine/internal/backtest/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

type Engine struct {
	execution ExecutionSimulator
}

func NewEngine(fundingRates []btExecution.FundingRate) *Engine {
	return &Engine{execution: NewExecutionSimulator(btExecution.NewFundingModel(fundingRates))}
}

func (e *Engine) Run(input Input) (Result, error) {
	cfg := normalizeConfig(input.Config)
	strategies := input.Strategies
	if input.Strategy != nil {
		strategies = append([]strategy.Strategy{input.Strategy}, strategies...)
	}
	if len(strategies) == 0 {
		return Result{}, errors.New("backtest v2 requires at least one strategy")
	}
	bias := BiasDetector{}.ValidateTicks(input.Ticks)
	if !bias.Passed {
		return Result{Config: cfg, BiasReport: bias}, fmt.Errorf("bias detector failed with %d violations", len(bias.Violations))
	}
	portfolio := NewPortfolio(cfg.InitialCapitalUSD)
	clock := NewSimulationClock(input.Ticks)
	scheduler := &Scheduler{}
	warmup := NewWarmupEnforcer(cfg.WarmupBars)
	regimes := &RegimeClassifier{}
	events := make([]Event, 0)

	for {
		tick, ok := clock.Next()
		if !ok {
			break
		}
		if !scheduler.Accept(tick) {
			events = append(events, Event{Type: EventBiasViolation, Symbol: tick.Symbol, Message: "non-monotonic tick rejected", Timestamp: tickTime(tick)})
			continue
		}
		ts := tickTime(tick)
		warmup.Observe(ts)
		regime := regimes.Classify(tick)

		for _, pos := range portfolio.Positions {
			if shouldClose(pos, tick.Price, ts) {
				exit := e.execution.FillExit(pos, tick, cfg)
				funding := e.execution.Funding(pos, tick)
				portfolio.Close(pos, exit, funding, cfg.FeeBps, "RULE_EXIT")
				events = append(events, Event{Type: EventPositionClosed, Strategy: pos.StrategyName, Symbol: pos.Symbol, Message: "position closed by backtest lifecycle", Timestamp: exit.Timestamp})
			}
		}

		for _, strat := range strategies {
			for _, sig := range signalsFor(cfg.Kind, strat, tick) {
				if sig.Action == strategy.ActionHold {
					continue
				}
				events = append(events, Event{Type: EventSignalGenerated, Strategy: strat.Name(), Symbol: sig.Symbol, Timestamp: ts})
				if !cfg.AllowShorts && sig.Action == strategy.ActionSell {
					events = append(events, Event{Type: EventRiskRejected, Strategy: strat.Name(), Symbol: sig.Symbol, Message: "shorts disabled", Timestamp: ts})
					continue
				}
				if !warmup.CanTrade(ts) {
					events = append(events, Event{Type: EventRiskRejected, Strategy: strat.Name(), Symbol: sig.Symbol, Message: "warmup incomplete", Timestamp: ts})
					continue
				}
				sig = normalizeSignal(sig, tick.Price, cfg)
				if hasOpenPosition(portfolio, strat.Name(), sig.Symbol) {
					continue
				}
				id := fmt.Sprintf("BT2-%s-%d", strat.Name(), len(events)+1)
				fill := e.execution.FillEntry(sig, tick, cfg, id)
				if fill.FilledQuantity <= 0 {
					continue
				}
				pos := portfolio.Open(strat.Name(), sig, fill, regime)
				events = append(events, Event{Type: EventPositionOpened, Strategy: pos.StrategyName, Symbol: pos.Symbol, Message: "realistic fill opened position", Timestamp: fill.Timestamp})
			}
		}
	}

	lastPrice := 0.0
	if len(input.Ticks) > 0 {
		lastPrice = input.Ticks[len(input.Ticks)-1].Price
	}
	result := Result{
		Config:            cfg,
		InitialCapitalUSD: cfg.InitialCapitalUSD,
		FinalCapitalUSD:   portfolio.Equity(lastPrice),
		Trades:            portfolio.Trades,
		Events:            events,
		BiasReport:        bias,
		WarmupReport:      warmup.Report(),
	}
	result.Metrics = CalculateMetrics(result.Trades, cfg.InitialCapitalUSD)
	result.ExecutionQuality = CalculateExecutionQuality(result.Trades)
	result.RegimeStats = RegimeStatistics(result.Trades, cfg.InitialCapitalUSD)
	return result, nil
}

func normalizeConfig(cfg Config) Config {
	if cfg.Mode == "" {
		cfg.Mode = ModeInstitutional
	}
	if cfg.Kind == "" {
		cfg.Kind = SimulationHybrid
	}
	if cfg.InitialCapitalUSD <= 0 {
		cfg.InitialCapitalUSD = 100_000
	}
	if cfg.FeeBps <= 0 {
		cfg.FeeBps = 4
	}
	if cfg.LatencyTier <= 0 {
		cfg.LatencyTier = btExecution.Latency50ms
	}
	if cfg.WarmupBars <= 0 {
		cfg.WarmupBars = 50
	}
	if cfg.RiskPerTradePct <= 0 {
		cfg.RiskPerTradePct = 0.5
	}
	return cfg
}

func signalsFor(kind SimulationKind, strat strategy.Strategy, tick marketdata.Tick) []strategy.Signal {
	switch kind {
	case SimulationTick:
		return strat.OnTick(tick)
	case SimulationCandle:
		return strat.OnCandle(tick)
	default:
		out := append([]strategy.Signal{}, strat.OnTick(tick)...)
		out = append(out, strat.OnCandle(tick)...)
		return out
	}
}

func normalizeSignal(sig strategy.Signal, price float64, cfg Config) strategy.Signal {
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

func hasOpenPosition(portfolio *Portfolio, strategyName, symbol string) bool {
	for _, pos := range portfolio.Positions {
		if pos.StrategyName == strategyName && pos.Symbol == symbol {
			return true
		}
	}
	return false
}
