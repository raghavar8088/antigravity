from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from .schemas import Candle, StrategyCycleRequest, SupportResistanceInput


def ohlcv_to_candles(rows: list[dict[str, float]]) -> list[Candle]:
    """
    Converts list of OHLCV dictionaries (from CCXT) to a list of Candle objects.
    """
    candles: list[Candle] = []
    for row in rows:
        # CCXT timestamp is in milliseconds.
        ts = datetime.fromtimestamp(row["timestamp"] / 1000, tz=timezone.utc)
        candles.append(
            Candle(
                timestamp=ts,
                open=row.get("open", 0.0),
                high=row.get("high", 0.0),
                low=row.get("low", 0.0),
                close=row.get("close", 0.0),
                volume=row.get("volume", 0.0),
            )
        )
    return candles


def build_cycle_request(
    symbol: str,
    ohlcv_data: dict[str, list[dict[str, Any]]],
    balance: float = 10000.0,
    daily_pnl: float = 0.0,
) -> StrategyCycleRequest:
    """
    Builds a StrategyCycleRequest from raw dictionary-based OHLCV data.
    `ohlcv_data` expects keys like '1m', '5m', '15m', '30m', '1h', '4h'.
    """
    return StrategyCycleRequest(
        symbol=symbol,
        candles_1m=ohlcv_to_candles(ohlcv_data.get("1m", [])),
        candles_5m=ohlcv_to_candles(ohlcv_data.get("5m", [])),
        candles_15m=ohlcv_to_candles(ohlcv_data.get("15m", [])),
        candles_30m=ohlcv_to_candles(ohlcv_data.get("30m", [])),
        candles_1h=ohlcv_to_candles(ohlcv_data.get("1h", [])),
        candles_4h=ohlcv_to_candles(ohlcv_data.get("4h", [])),
        balance=balance,
        daily_pnl=daily_pnl,
        support_resistance=SupportResistanceInput(level_type="AUTO"),
    )
