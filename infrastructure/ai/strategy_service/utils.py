from __future__ import annotations

from dataclasses import dataclass
from zoneinfo import ZoneInfo

import numpy as np
import pandas as pd

from .schemas import Candle


@dataclass(slots=True)
class FairValueGap:
    direction: str
    lower: float
    upper: float
    created_at: pd.Timestamp


def candles_to_frame(candles: list[Candle]) -> pd.DataFrame:
    frame = pd.DataFrame([c.model_dump() for c in candles])
    if frame.empty:
        return frame

    frame = frame.sort_values("timestamp").drop_duplicates(subset=["timestamp"], keep="last")
    frame["timestamp"] = pd.to_datetime(frame["timestamp"], utc=True)
    frame = frame.set_index("timestamp")
    return frame.astype(float)


def ema(series: pd.Series, period: int) -> pd.Series:
    return series.ewm(span=period, adjust=False).mean()


def rsi(series: pd.Series, period: int = 14) -> pd.Series:
    delta = series.diff()
    gains = delta.clip(lower=0)
    losses = -delta.clip(upper=0)
    average_gain = gains.ewm(alpha=1 / period, adjust=False).mean()
    average_loss = losses.ewm(alpha=1 / period, adjust=False).mean()
    rs = average_gain / average_loss.replace(0, np.nan)
    result = 100 - (100 / (1 + rs))
    return result.fillna(50)


def slope(series: pd.Series, lookback: int = 1) -> float:
    if len(series) <= lookback:
        return 0.0
    return float(series.iloc[-1] - series.iloc[-1 - lookback])


def crosses_above(series: pd.Series, level: float) -> bool:
    if len(series) < 2:
        return False
    return bool(series.iloc[-2] < level <= series.iloc[-1])


def crosses_below(series: pd.Series, level: float) -> bool:
    if len(series) < 2:
        return False
    return bool(series.iloc[-2] > level >= series.iloc[-1])


def previous_swing_low(frame: pd.DataFrame, lookback: int = 5) -> float | None:
    if len(frame) <= lookback:
        return None
    return float(frame["low"].iloc[-lookback - 1 : -1].min())


def previous_swing_high(frame: pd.DataFrame, lookback: int = 5) -> float | None:
    if len(frame) <= lookback:
        return None
    return float(frame["high"].iloc[-lookback - 1 : -1].max())


def true_range(frame: pd.DataFrame) -> pd.Series:
    prev_close = frame["close"].shift(1)
    parts = pd.concat(
        [
            frame["high"] - frame["low"],
            (frame["high"] - prev_close).abs(),
            (frame["low"] - prev_close).abs(),
        ],
        axis=1,
    )
    return parts.max(axis=1)


def atr(frame: pd.DataFrame, period: int = 14) -> pd.Series:
    return true_range(frame).ewm(alpha=1 / period, adjust=False).mean()


def session_vwap(frame: pd.DataFrame, timezone_name: str, session_start: str) -> pd.Series:
    if frame.empty:
        return pd.Series(dtype=float)

    tz = ZoneInfo(timezone_name)
    local_index = frame.index.tz_convert(tz)
    hour, minute = map(int, session_start.split(":"))

    local_series = pd.Series(local_index, index=frame.index)
    session_anchor = local_series.dt.floor("D")
    before_session = (local_series.dt.hour < hour) | (
        (local_series.dt.hour == hour) & (local_series.dt.minute < minute)
    )
    session_anchor = session_anchor.mask(before_session, session_anchor - pd.Timedelta(days=1))

    typical_price = (frame["high"] + frame["low"] + frame["close"]) / 3.0
    weighted_price = typical_price * frame["volume"]

    cumulative_wp = weighted_price.groupby(session_anchor).cumsum()
    cumulative_volume = frame["volume"].groupby(session_anchor).cumsum()
    return cumulative_wp / cumulative_volume.replace(0, np.nan)


def detect_williams_fractals(frame: pd.DataFrame) -> tuple[pd.Series, pd.Series]:
    if frame.empty:
        empty = pd.Series(dtype=bool)
        return empty, empty

    highs = frame["high"]
    lows = frame["low"]

    up_center = (
        (highs.shift(2) > highs.shift(1))
        & (highs.shift(2) > highs)
        & (highs.shift(2) > highs.shift(3))
        & (highs.shift(2) > highs.shift(4))
    )
    down_center = (
        (lows.shift(2) < lows.shift(1))
        & (lows.shift(2) < lows)
        & (lows.shift(2) < lows.shift(3))
        & (lows.shift(2) < lows.shift(4))
    )

    return up_center.fillna(False), down_center.fillna(False)


def candle_body_engulfs(frame: pd.DataFrame, bullish: bool) -> bool:
    if len(frame) < 2:
        return False

    previous = frame.iloc[-2]
    current = frame.iloc[-1]

    prev_body_low = min(previous["open"], previous["close"])
    prev_body_high = max(previous["open"], previous["close"])
    current_body_low = min(current["open"], current["close"])
    current_body_high = max(current["open"], current["close"])

    if bullish:
        return (
            current["close"] > current["open"]
            and previous["close"] < previous["open"]
            and current_body_low <= prev_body_low
            and current_body_high >= prev_body_high
        )

    return (
        current["close"] < current["open"]
        and previous["close"] > previous["open"]
        and current_body_low <= prev_body_low
        and current_body_high >= prev_body_high
    )


def find_recent_fvg(frame: pd.DataFrame, direction: str, limit: int = 20) -> FairValueGap | None:
    if len(frame) < 3:
        return None

    window = frame.iloc[-max(limit, 3) :]
    for position in range(len(window) - 1, 1, -1):
        candle1 = window.iloc[position - 2]
        candle3 = window.iloc[position]

        if direction == "bullish" and candle1["low"] > candle3["high"]:
            return FairValueGap(
                direction=direction,
                lower=float(candle3["high"]),
                upper=float(candle1["low"]),
                created_at=window.index[position],
            )
        if direction == "bearish" and candle1["high"] < candle3["low"]:
            return FairValueGap(
                direction=direction,
                lower=float(candle1["high"]),
                upper=float(candle3["low"]),
                created_at=window.index[position],
            )
    return None


def detect_structure_bias(frame: pd.DataFrame, lookback: int = 12) -> str:
    if len(frame) < lookback + 2:
        return "neutral"

    reference = frame.iloc[-lookback - 1 : -1]
    latest_close = float(frame["close"].iloc[-1])
    previous_close = float(frame["close"].iloc[-2])
    swing_high = float(reference["high"].max())
    swing_low = float(reference["low"].min())

    if latest_close > swing_high and previous_close <= swing_high:
        return "bullish"
    if latest_close < swing_low and previous_close >= swing_low:
        return "bearish"

    if latest_close > reference["close"].mean():
        return "bullish"
    if latest_close < reference["close"].mean():
        return "bearish"
    return "neutral"


def price_inside_zone(price: float, lower: float, upper: float) -> bool:
    low, high = sorted((lower, upper))
    return low <= price <= high


def bollinger_bands(series: pd.Series, period: int = 20, multiplier: float = 2.0) -> tuple[pd.Series, pd.Series, pd.Series]:
    middle = series.rolling(period).mean()
    deviation = series.rolling(period).std(ddof=0)
    upper = middle + deviation * multiplier
    lower = middle - deviation * multiplier
    return lower, middle, upper


def keltner_channel(frame: pd.DataFrame, period: int = 20, multiplier: float = 2.0) -> tuple[pd.Series, pd.Series, pd.Series]:
    middle = ema(frame["close"], period)
    width = atr(frame, period) * multiplier
    return middle - width, middle, middle + width


def stochastic_oscillator(
    frame: pd.DataFrame,
    k_period: int = 7,
    smooth_k: int = 3,
    smooth_d: int = 10,
) -> tuple[pd.Series, pd.Series]:
    lowest_low = frame["low"].rolling(k_period).min()
    highest_high = frame["high"].rolling(k_period).max()
    spread = (highest_high - lowest_low).replace(0, np.nan)
    raw_k = ((frame["close"] - lowest_low) / spread) * 100.0
    k_line = raw_k.rolling(smooth_k).mean()
    d_line = k_line.rolling(smooth_d).mean()
    return k_line.fillna(50.0), d_line.fillna(50.0)
