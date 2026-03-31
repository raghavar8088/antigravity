from __future__ import annotations

from datetime import datetime, timezone

from fastapi import FastAPI

from .framework import ApexScalpFramework
from .library import STRATEGY_DESCRIPTORS, evaluate_all_strategies
from .schemas import (
    StrategyCycleRequest,
    StrategyCycleResponse,
    StrategyEvaluationRequest,
    StrategyEvaluationResponse,
)


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
    framework = ApexScalpFramework.load()
    return framework.config.as_dict()


@app.post("/framework/run-cycle", response_model=StrategyCycleResponse)
def run_framework_cycle(request: StrategyCycleRequest) -> StrategyCycleResponse:
    framework = ApexScalpFramework.load(request.config_path)
    return framework.run_cycle(request)
