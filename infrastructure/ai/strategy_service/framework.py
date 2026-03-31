from __future__ import annotations

import logging
from abc import ABC, abstractmethod
from dataclasses import asdict, dataclass, fields as dataclass_fields
from datetime import datetime, timezone
from pathlib import Path

import numpy as np
import yaml

from .library import evaluate_all_strategies
from .schemas import (
    ExecutionPlan,
    PerformanceMetrics,
    StrategyConfig,
    StrategyCycleRequest,
    StrategyCycleResponse,
    StrategyEvaluationRequest,
    StrategySignal,
)


DEFAULT_CONFIG_PATH = Path(__file__).resolve().parents[1] / "config.yaml"


def get_logger(name: str = "ApexScalp") -> logging.Logger:
    logger = logging.getLogger(name)
    if logger.handlers:
        return logger

    handler = logging.StreamHandler()
    handler.setFormatter(logging.Formatter("%(asctime)s - %(name)s - %(levelname)s - %(message)s"))
    logger.addHandler(handler)
    logger.setLevel(logging.INFO)
    logger.propagate = False
    return logger


logger = get_logger()


@dataclass(slots=True)
class ApexScalpConfig:
    btc_symbol: str = "BTC/USDT"
    timeframe: str = "1m"
    risk_per_trade: float = 0.01
    daily_loss_limit: float = -0.03
    max_daily_trades: int = 50
    three_strikes_limit: int = 3
    maker_fee: float = 0.0002
    taker_fee: float = 0.0005
    slippage_per_btc: float = 10.0
    preferred_order_type: str = "LIMIT"
    mechanical_session_timezone: str = "Asia/Kolkata"
    mechanical_session_start: str = "05:30"
    ny_timezone: str = "America/New_York"
    ema_gap_threshold: float = 5.0
    liquidity_swing_lookback: int = 5
    structure_lookback: int = 12
    breakout_reentry_lookback: int = 24
    enable_volatility_envelopes: bool = True
    enable_stochastic_squeeze: bool = True
    stochastic_k_period: int = 7
    stochastic_smooth_k: int = 3
    stochastic_smooth_d: int = 10
    stochastic_time_stop_candles: int = 5
    stochastic_oversold_threshold: float = 20.0
    stochastic_overbought_threshold: float = 80.0
    strategy_priority: list[str] | None = None
    config_source: str | None = None

    @classmethod
    def load(cls, path: str | None = None) -> ApexScalpConfig:
        config_path = Path(path) if path else DEFAULT_CONFIG_PATH
        payload: dict[str, object] = {}

        if config_path.exists():
            with config_path.open("r", encoding="utf-8") as handle:
                payload = yaml.safe_load(handle) or {}
            logger.info("Loaded ApexScalp config from %s", config_path)
        elif path:
            logger.warning("Config path %s not found. Falling back to defaults.", config_path)

        valid_fields = {item.name for item in dataclass_fields(cls)}
        filtered = {key: value for key, value in payload.items() if key in valid_fields}
        loaded = cls(**filtered)
        loaded.config_source = str(config_path)
        return loaded

    def to_strategy_config(self) -> StrategyConfig:
        return StrategyConfig(
            mechanical_session_timezone=self.mechanical_session_timezone,
            mechanical_session_start=self.mechanical_session_start,
            ny_timezone=self.ny_timezone,
            ema_gap_threshold=self.ema_gap_threshold,
            liquidity_swing_lookback=self.liquidity_swing_lookback,
            structure_lookback=self.structure_lookback,
            breakout_reentry_lookback=self.breakout_reentry_lookback,
            enable_volatility_envelopes=self.enable_volatility_envelopes,
            enable_stochastic_squeeze=self.enable_stochastic_squeeze,
            stochastic_k_period=self.stochastic_k_period,
            stochastic_smooth_k=self.stochastic_smooth_k,
            stochastic_smooth_d=self.stochastic_smooth_d,
            stochastic_time_stop_candles=self.stochastic_time_stop_candles,
            stochastic_oversold_threshold=self.stochastic_oversold_threshold,
            stochastic_overbought_threshold=self.stochastic_overbought_threshold,
        )

    def as_dict(self) -> dict[str, object]:
        payload = asdict(self)
        return payload


class RiskManager:
    def __init__(self, config: ApexScalpConfig, daily_trades: int = 0, consecutive_losses: int = 0) -> None:
        self.config = config
        self.daily_trades = daily_trades
        self.consecutive_losses = consecutive_losses

    def validate_trade(
        self,
        balance: float,
        daily_pnl: float,
        daily_pnl_fraction: float | None,
        signal: StrategySignal | None,
    ) -> tuple[bool, str | None]:
        if signal is None or signal.state not in {"LONG", "SHORT"}:
            return False, "No actionable strategy signal is available."

        if self.consecutive_losses >= self.config.three_strikes_limit:
            return False, f"Three-strikes rule hit ({self.consecutive_losses} consecutive losses)."

        if self.daily_trades >= self.config.max_daily_trades:
            return False, f"Daily trade limit hit ({self.daily_trades}/{self.config.max_daily_trades})."

        pnl_fraction = daily_pnl_fraction
        if pnl_fraction is None and balance > 0:
            pnl_fraction = daily_pnl / balance
        if pnl_fraction is not None and pnl_fraction <= self.config.daily_loss_limit:
            return False, f"Daily loss limit hit ({pnl_fraction:.2%} <= {self.config.daily_loss_limit:.2%})."

        if signal.entry_price is None or signal.stop_loss is None:
            return False, f"{signal.strategy} did not provide a valid stop-loss for sizing."

        if abs(signal.entry_price - signal.stop_loss) <= 0:
            return False, "Selected signal has zero stop distance."

        return True, None

    def calculate_position_size(self, balance: float, entry: float, stop_loss: float) -> float:
        risk_amount = balance * self.config.risk_per_trade
        stop_distance = abs(entry - stop_loss) + self.config.slippage_per_btc
        if stop_distance <= 0:
            return 0.0
        return risk_amount / stop_distance


class ExecutionEngine(ABC):
    def __init__(self, config: ApexScalpConfig) -> None:
        self.config = config

    @abstractmethod
    def build_order_plan(
        self,
        *,
        symbol: str,
        signal: StrategySignal,
        quantity: float,
        order_type: str,
        mode: str,
    ) -> ExecutionPlan:
        raise NotImplementedError


class PaperExecutionEngine(ExecutionEngine):
    def build_order_plan(
        self,
        *,
        symbol: str,
        signal: StrategySignal,
        quantity: float,
        order_type: str,
        mode: str,
    ) -> ExecutionPlan:
        entry = float(signal.entry_price or 0.0)
        normalized_order_type = order_type.upper()
        fee_rate = self.config.maker_fee if normalized_order_type == "LIMIT" else self.config.taker_fee
        notional = entry * quantity
        estimated_fee = notional * fee_rate
        slippage_buffer = quantity * self.config.slippage_per_btc
        notes = [
            "Maker-style LIMIT orders are preferred to reduce fee drag.",
            f"Execution mode is {mode}; this plan is safe to preview before wiring to an exchange client.",
        ]
        if signal.metadata.get("time_stop_candles"):
            notes.append(f"Time stop: {signal.metadata['time_stop_candles']} candles.")

        return ExecutionPlan(
            symbol=symbol,
            strategy=signal.strategy,
            side=signal.state,
            quantity=quantity,
            order_type=normalized_order_type,
            entry_price=entry,
            stop_loss=signal.stop_loss,
            take_profit=signal.take_profit,
            maker_fee_rate=self.config.maker_fee,
            taker_fee_rate=self.config.taker_fee,
            estimated_fee=estimated_fee,
            slippage_buffer=slippage_buffer,
            notional=notional,
            mode="live_stub" if mode == "live_stub" else "paper",
            notes=notes,
        )


class PerformanceTracker:
    def __init__(self, trades: list) -> None:
        self.trades = trades

    def get_metrics(self) -> PerformanceMetrics:
        if not self.trades:
            return PerformanceMetrics()

        pnl_values = np.asarray([trade.realized_pnl for trade in self.trades], dtype=float)
        wins = float(np.sum(pnl_values > 0))
        total = len(pnl_values)
        average_pnl = float(pnl_values.mean())
        net_pnl = float(pnl_values.sum())
        win_rate = wins / total if total else 0.0

        sharpe_ratio = 0.0
        if total > 1:
            std = float(pnl_values.std(ddof=1))
            if std > 0:
                sharpe_ratio = float((pnl_values.mean() / std) * np.sqrt(total))

        holding_periods = [trade.holding_minutes for trade in self.trades if trade.holding_minutes is not None]
        average_holding = float(np.mean(holding_periods)) if holding_periods else None
        fee_drag = float(sum(trade.fees_paid for trade in self.trades))
        slippage_drag = float(sum(trade.slippage_paid for trade in self.trades))

        return PerformanceMetrics(
            total_trades=total,
            win_rate=win_rate,
            net_pnl=net_pnl,
            average_pnl=average_pnl,
            sharpe_ratio=sharpe_ratio,
            average_holding_minutes=average_holding,
            fee_drag=fee_drag,
            slippage_drag=slippage_drag,
        )


class StrategyManager:
    def __init__(self, config: ApexScalpConfig, risk: RiskManager, execution: ExecutionEngine) -> None:
        self.config = config
        self.risk = risk
        self.execution = execution

    def _build_strategy_request(self, request: StrategyCycleRequest) -> StrategyEvaluationRequest:
        base_config = self.config.to_strategy_config().model_dump()
        overrides = request.strategy_overrides.model_dump(exclude_none=True)
        merged_config = StrategyConfig(**(base_config | overrides))
        return StrategyEvaluationRequest(
            symbol=request.symbol,
            candles_1m=request.candles_1m,
            candles_5m=request.candles_5m,
            candles_30m=request.candles_30m,
            candles_1h=request.candles_1h,
            candles_4h=request.candles_4h,
            support_resistance=request.support_resistance,
            config=merged_config,
        )

    def _select_signal(
        self,
        evaluations: list[StrategySignal],
        allowed_strategies: list[str],
    ) -> StrategySignal | None:
        actionable = [item for item in evaluations if item.state in {"LONG", "SHORT"}]
        if allowed_strategies:
            allowed = set(allowed_strategies)
            actionable = [item for item in actionable if item.strategy in allowed]

        if not actionable:
            return None

        priority = self.config.strategy_priority or [item.strategy for item in actionable]
        priority_rank = {name: index for index, name in enumerate(priority)}
        actionable.sort(key=lambda item: priority_rank.get(item.strategy, len(priority_rank)))
        return actionable[0]

    def run_cycle(self, request: StrategyCycleRequest) -> StrategyCycleResponse:
        strategy_request = self._build_strategy_request(request)
        evaluations = evaluate_all_strategies(strategy_request)
        performance = PerformanceTracker(request.trade_history).get_metrics()
        selected_signal = self._select_signal(evaluations, request.allowed_strategies)
        risk_gate_passed, blocked_reason = self.risk.validate_trade(
            request.balance,
            request.daily_pnl,
            request.daily_pnl_fraction,
            selected_signal,
        )

        execution_plan = None
        if risk_gate_passed and selected_signal is not None:
            quantity = self.risk.calculate_position_size(
                request.balance,
                float(selected_signal.entry_price or 0.0),
                float(selected_signal.stop_loss or 0.0),
            )
            if quantity <= 0:
                risk_gate_passed = False
                blocked_reason = "Risk model produced a zero-sized position."
            else:
                execution_plan = self.execution.build_order_plan(
                    symbol=request.symbol,
                    signal=selected_signal,
                    quantity=quantity,
                    order_type=request.order_type or self.config.preferred_order_type,
                    mode=request.execution_mode,
                )

        loaded_config = self.config.as_dict()
        loaded_config["strategy_config"] = strategy_request.config.model_dump()
        return StrategyCycleResponse(
            symbol=request.symbol,
            generated_at=datetime.now(timezone.utc),
            loaded_config=loaded_config,
            evaluations=evaluations,
            selected_signal=selected_signal,
            risk_gate_passed=risk_gate_passed,
            blocked_reason=blocked_reason,
            execution_plan=execution_plan,
            performance=performance,
        )


class ApexScalpFramework:
    def __init__(self, config: ApexScalpConfig) -> None:
        self.config = config

    @classmethod
    def load(cls, config_path: str | None = None) -> ApexScalpFramework:
        return cls(ApexScalpConfig.load(config_path))

    def run_cycle(self, request: StrategyCycleRequest) -> StrategyCycleResponse:
        risk = RiskManager(
            self.config,
            daily_trades=request.daily_trades,
            consecutive_losses=request.consecutive_losses,
        )
        execution = PaperExecutionEngine(self.config)
        manager = StrategyManager(self.config, risk, execution)
        return manager.run_cycle(request)
