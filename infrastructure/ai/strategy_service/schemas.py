from __future__ import annotations

from datetime import datetime, timezone
from typing import Any, Literal

from pydantic import AliasChoices, BaseModel, Field, field_validator


def _coerce_timestamp(value: Any) -> datetime:
    if isinstance(value, datetime):
        if value.tzinfo is None:
            return value.replace(tzinfo=timezone.utc)
        return value.astimezone(timezone.utc)

    if isinstance(value, (int, float)):
        numeric = float(value)
        if numeric > 1_000_000_000_000:
            numeric /= 1000.0
        return datetime.fromtimestamp(numeric, tz=timezone.utc)

    if isinstance(value, str):
        try:
            parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
        except ValueError as exc:
            raise ValueError(f"Unsupported timestamp format: {value}") from exc
        if parsed.tzinfo is None:
            return parsed.replace(tzinfo=timezone.utc)
        return parsed.astimezone(timezone.utc)

    raise TypeError(f"Unsupported timestamp type: {type(value)!r}")


class Candle(BaseModel):
    timestamp: datetime = Field(validation_alias=AliasChoices("timestamp", "time"))
    open: float
    high: float
    low: float
    close: float
    volume: float = 0.0

    @field_validator("timestamp", mode="before")
    @classmethod
    def validate_timestamp(cls, value: Any) -> datetime:
        return _coerce_timestamp(value)


class SupportResistanceInput(BaseModel):
    support: float | None = None
    resistance: float | None = None
    level_type: Literal["Support", "Resistance", "AUTO"] = "AUTO"


class StrategyConfig(BaseModel):
    market_timezone: str = "UTC"
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


class StrategyConfigPatch(BaseModel):
    market_timezone: str | None = None
    mechanical_session_timezone: str | None = None
    mechanical_session_start: str | None = None
    ny_timezone: str | None = None
    ema_gap_threshold: float | None = None
    liquidity_swing_lookback: int | None = None
    structure_lookback: int | None = None
    breakout_reentry_lookback: int | None = None
    enable_volatility_envelopes: bool | None = None
    enable_stochastic_squeeze: bool | None = None
    stochastic_k_period: int | None = None
    stochastic_smooth_k: int | None = None
    stochastic_smooth_d: int | None = None
    stochastic_time_stop_candles: int | None = None
    stochastic_oversold_threshold: float | None = None
    stochastic_overbought_threshold: float | None = None


class StrategyEvaluationRequest(BaseModel):
    symbol: str = "BTCUSDT"
    candles_1m: list[Candle] = Field(default_factory=list)
    candles_5m: list[Candle] = Field(default_factory=list)
    candles_30m: list[Candle] = Field(default_factory=list)
    candles_1h: list[Candle] = Field(default_factory=list)
    candles_4h: list[Candle] = Field(default_factory=list)
    support_resistance: SupportResistanceInput = Field(default_factory=SupportResistanceInput)
    config: StrategyConfig = Field(default_factory=StrategyConfig)


class StrategySignal(BaseModel):
    strategy: str
    state: Literal["LONG", "SHORT", "WAIT", "NO_TRADE"]
    entry_price: float | None = None
    stop_loss: float | None = None
    take_profit: float | None = None
    take_profit_2: float | None = None
    partial_take_profit: float | None = None
    partial_size_fraction: float | None = None
    reason: str
    metadata: dict[str, Any] = Field(default_factory=dict)


class StrategyDescriptor(BaseModel):
    name: str
    description: str
    required_timeframes: list[str]


class StrategyEvaluationResponse(BaseModel):
    symbol: str
    generated_at: datetime
    evaluations: list[StrategySignal]


class ClosedTrade(BaseModel):
    strategy: str = "unknown"
    side: Literal["LONG", "SHORT"] = "LONG"
    realized_pnl: float
    holding_minutes: float | None = None
    fees_paid: float = 0.0
    slippage_paid: float = 0.0
    opened_at: datetime | None = None
    closed_at: datetime | None = None

    @field_validator("opened_at", "closed_at", mode="before")
    @classmethod
    def validate_optional_timestamp(cls, value: Any) -> datetime | None:
        if value is None:
            return None
        return _coerce_timestamp(value)


class ExecutionPlan(BaseModel):
    symbol: str
    strategy: str
    side: Literal["LONG", "SHORT"]
    quantity: float
    order_type: Literal["LIMIT", "MARKET"]
    entry_price: float
    stop_loss: float | None = None
    take_profit: float | None = None
    maker_fee_rate: float
    taker_fee_rate: float
    estimated_fee: float
    slippage_buffer: float
    notional: float
    mode: Literal["paper", "live_stub"] = "paper"
    notes: list[str] = Field(default_factory=list)


class PerformanceMetrics(BaseModel):
    total_trades: int = 0
    win_rate: float = 0.0
    net_pnl: float = 0.0
    average_pnl: float = 0.0
    sharpe_ratio: float = 0.0
    average_holding_minutes: float | None = None
    fee_drag: float = 0.0
    slippage_drag: float = 0.0


class StrategyCycleRequest(BaseModel):
    symbol: str = "BTCUSDT"
    candles_1m: list[Candle] = Field(default_factory=list)
    candles_5m: list[Candle] = Field(default_factory=list)
    candles_30m: list[Candle] = Field(default_factory=list)
    candles_1h: list[Candle] = Field(default_factory=list)
    candles_4h: list[Candle] = Field(default_factory=list)
    support_resistance: SupportResistanceInput = Field(default_factory=SupportResistanceInput)
    strategy_overrides: StrategyConfigPatch = Field(default_factory=StrategyConfigPatch)
    balance: float = Field(default=10_000.0, gt=0)
    daily_pnl: float = 0.0
    daily_pnl_fraction: float | None = None
    daily_trades: int = 0
    consecutive_losses: int = 0
    trade_history: list[ClosedTrade] = Field(default_factory=list)
    config_path: str | None = None
    order_type: Literal["LIMIT", "MARKET"] | None = None
    allowed_strategies: list[str] = Field(default_factory=list)
    execution_mode: Literal["paper", "live_stub"] = "paper"


class StrategyCycleResponse(BaseModel):
    symbol: str
    generated_at: datetime
    loaded_config: dict[str, Any]
    evaluations: list[StrategySignal]
    selected_signal: StrategySignal | None = None
    risk_gate_passed: bool
    blocked_reason: str | None = None
    execution_plan: ExecutionPlan | None = None
    performance: PerformanceMetrics
