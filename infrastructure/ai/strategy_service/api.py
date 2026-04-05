from __future__ import annotations

from datetime import datetime, timezone

from fastapi import FastAPI

from .framework import ApexScalpFramework
from .library import STRATEGY_DESCRIPTORS, evaluate_all_strategies
from .schemas import (
    StrategyCycleResponse,
    StrategyEvaluationRequest,
    StrategyEvaluationResponse,
    BacktestReport,
    DashboardStatus,
    EventWindow,
    MonteCarloReport,
)
from .management import (
    MonteCarloRiskTester,
)
from .backtest import Backtester


# Persistent framework instance
framework_instance = ApexScalpFramework.load()
# Add the new persistent layers to the instance
from .management import NewsEventProtectionLayer, TradeJournal
framework_instance.event_layer = NewsEventProtectionLayer()
framework_instance.journal = TradeJournal()


app = FastAPI(
    title="Antigravity Python Strategy Service",
    version="1.0.0",
    description="Python implementations of BTC scalping strategies for the autonomous trading application.",
)


@app.get("/")
def root() -> dict[str, object]:
    return {
        "message": "Antigravity Python strategy service is running",
        "health_url": "/health",
        "strategies_url": "/strategies",
        "evaluate_url": "/strategies/evaluate",
        "framework_config_url": "/framework/config",
        "framework_run_cycle_url": "/framework/run-cycle",
    }


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/strategies")
def list_strategies() -> list[dict[str, object]]:
    return [descriptor.model_dump() for descriptor in STRATEGY_DESCRIPTORS]


@app.post("/strategies/evaluate", response_model=StrategyEvaluationResponse)
def evaluate_strategies(request: StrategyEvaluationRequest) -> StrategyEvaluationResponse:
    evaluations = evaluate_all_strategies(request)
    return StrategyEvaluationResponse(
        symbol=request.symbol,
        generated_at=datetime.now(timezone.utc),
        evaluations=evaluations,
    )


@app.get("/framework/config")
def framework_config() -> dict[str, object]:
    return framework_instance.config.as_dict()


@app.get("/framework/status", response_model=DashboardStatus | None)
def framework_status() -> DashboardStatus | None:
    return framework_instance.last_status


@app.post("/framework/run-cycle", response_model=StrategyCycleResponse)
def run_framework_cycle(request: StrategyCycleRequest) -> StrategyCycleResponse:
    # Use the persistent instance
    return framework_instance.run_cycle(request)


@app.post("/framework/backtest", response_model=BacktestReport)
def run_backtest(snapshots: list[StrategyCycleRequest]) -> BacktestReport:
    backtester = Backtester(framework_instance) # Simplified for example
    return backtester.run(snapshots)


@app.post("/framework/events")
def add_news_event(event: EventWindow) -> dict[str, str]:
    framework_instance.event_layer.add_event(event)
    return {"status": "event added", "name": event.name}


@app.get("/framework/journal")
def get_journal_summary() -> dict[str, Any]:
    return framework_instance.journal.summarize()


@app.post("/framework/monte-carlo", response_model=MonteCarloReport)
def run_monte_carlo(simulations: int = 500) -> MonteCarloReport:
    # Use real trade history from journal
    pnls = [e.pnl for e in framework_instance.journal.entries]
    balance = framework_instance.config.starting_equity if hasattr(framework_instance.config, 'starting_equity') else 10000.0
    tester = MonteCarloRiskTester()
    return tester.run(pnls, balance, simulations)
