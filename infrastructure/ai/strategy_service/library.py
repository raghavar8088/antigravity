from __future__ import annotations

from datetime import datetime

import pandas as pd

from .schemas import StrategyDescriptor, StrategyEvaluationRequest, StrategySignal
from .utils import (
    atr,
    bollinger_bands,
    candle_body_engulfs,
    candles_to_frame,
    crosses_above,
    crosses_below,
    detect_structure_bias,
    detect_williams_fractals,
    ema,
    find_recent_fvg,
    keltner_channel,
    previous_swing_high,
    previous_swing_low,
    price_inside_zone,
    rsi,
    session_vwap,
    slope,
    stochastic_oscillator,
)


STRATEGY_DESCRIPTORS: list[StrategyDescriptor] = [
    StrategyDescriptor(
        name="mechanical_1m_btc_vwap_ema_rsi",
        description="Weekday-only 1m BTC setup using a 5:30 session VWAP, EMA 9/21 trend alignment, and RSI trigger.",
        required_timeframes=["1m"],
    ),
    StrategyDescriptor(
        name="four_hour_range_reentry",
        description="Uses the first New York 4H candle as the day range and trades failed breakouts back inside the range on 5m.",
        required_timeframes=["4h", "5m"],
    ),
    StrategyDescriptor(
        name="three_ema_williams_fractal",
        description="Trend-following EMA 20/50/100 fan with Williams fractal trigger and 2R target.",
        required_timeframes=["1m or 5m"],
    ),
    StrategyDescriptor(
        name="smc_fvg_liquidity_grab",
        description="30m structure plus fair value gap scan with 5m liquidity-grab confirmation.",
        required_timeframes=["30m", "5m"],
    ),
    StrategyDescriptor(
        name="ny_open_session_range_breakout",
        description="Combines 1H New York bias with a 9:30-9:45 session range breakout and retest confirmation.",
        required_timeframes=["1h", "5m"],
    ),
    StrategyDescriptor(
        name="sr_ema_crossover_sniper",
        description="Immediate EMA 9/20 crossover entry at supplied support or resistance level.",
        required_timeframes=["1m or 5m"],
    ),
    StrategyDescriptor(
        name="stochastic_momentum_squeeze",
        description="Momentum squeeze setup using a 7-period stochastic with a five-candle time stop.",
        required_timeframes=["1m or 5m"],
    ),
    StrategyDescriptor(
        name="bollinger_band_bounce",
        description="Mean-reversion volatility envelope strategy using Bollinger Bands and RSI extremes.",
        required_timeframes=["1m or 5m"],
    ),
    StrategyDescriptor(
        name="keltner_channel_breakout",
        description="Momentum breakout strategy requiring consecutive closes outside the Keltner Channel.",
        required_timeframes=["1m or 5m"],
    ),
]


def _wait(name: str, reason: str, **metadata: object) -> StrategySignal:
    return StrategySignal(strategy=name, state="WAIT", reason=reason, metadata=metadata)


def _no_trade(name: str, reason: str, **metadata: object) -> StrategySignal:
    return StrategySignal(strategy=name, state="NO_TRADE", reason=reason, metadata=metadata)


def _rr_target(entry: float, stop: float, multiple: float, direction: str) -> float:
    risk = abs(entry - stop)
    if direction == "LONG":
        return entry + risk * multiple
    return entry - risk * multiple


def mechanical_1m_btc_strategy(request: StrategyEvaluationRequest) -> StrategySignal:
    name = "mechanical_1m_btc_vwap_ema_rsi"
    frame = candles_to_frame(request.candles_1m)
    if len(frame) < 30:
        return _wait(name, "Need at least 30 one-minute candles.")

    local_now = frame.index[-1].tz_convert(request.config.mechanical_session_timezone)
    if local_now.weekday() >= 5:
        return _no_trade(name, "Weekend filter blocks trades.", local_day=str(local_now.date()))

    frame = frame.copy()
    frame["vwap"] = session_vwap(
        frame,
        timezone_name=request.config.mechanical_session_timezone,
        session_start=request.config.mechanical_session_start,
    )
    frame["ema_9"] = ema(frame["close"], 9)
    frame["ema_21"] = ema(frame["close"], 21)
    frame["rsi"] = rsi(frame["close"], 14)

    last = frame.iloc[-1]
    long_ready = (
        last["close"] > last["vwap"]
        and last["ema_9"] > last["ema_21"]
        and slope(frame["ema_9"]) > 0
        and slope(frame["ema_21"]) > 0
        and crosses_above(frame["rsi"], 55)
        and last["rsi"] < 70
    )
    short_ready = (
        last["close"] < last["vwap"]
        and last["ema_9"] < last["ema_21"]
        and slope(frame["ema_9"]) < 0
        and slope(frame["ema_21"]) < 0
        and crosses_below(frame["rsi"], 45)
        and last["rsi"] > 30
    )

    if not long_ready and not short_ready:
        return _wait(
            name,
            "Trend, VWAP, or RSI trigger not aligned.",
            close=float(last["close"]),
            vwap=float(last["vwap"]),
            ema_9=float(last["ema_9"]),
            ema_21=float(last["ema_21"]),
            rsi=float(last["rsi"]),
        )

    direction = "LONG" if long_ready else "SHORT"
    entry = float(last["close"])
    ema_stop = float(last["ema_21"])
    swing_stop = previous_swing_low(frame) if long_ready else previous_swing_high(frame)
    if long_ready:
        raw_stop = min(value for value in [ema_stop, swing_stop] if value is not None)
        if raw_stop >= entry:
            raw_stop = entry - float(atr(frame).iloc[-1])
    else:
        raw_stop = max(value for value in [ema_stop, swing_stop] if value is not None)
        if raw_stop <= entry:
            raw_stop = entry + float(atr(frame).iloc[-1])

    tp1 = _rr_target(entry, raw_stop, 1.5, direction)
    return StrategySignal(
        strategy=name,
        state=direction,
        entry_price=entry,
        stop_loss=raw_stop,
        take_profit=tp1,
        partial_take_profit=tp1,
        partial_size_fraction=0.5,
        reason=f"{direction} signal aligned with session VWAP, EMA slopes, and RSI trigger.",
        metadata={
            "trail_rule": "Trail remaining 50% on EMA_21 and exit when a 1m candle closes on the opposite side.",
            "vwap": float(last["vwap"]),
            "ema_9": float(last["ema_9"]),
            "ema_21": float(last["ema_21"]),
            "rsi": float(last["rsi"]),
        },
    )


def four_hour_range_reentry_strategy(request: StrategyEvaluationRequest) -> StrategySignal:
    name = "four_hour_range_reentry"
    frame_5m = candles_to_frame(request.candles_5m)
    frame_4h = candles_to_frame(request.candles_4h)
    if len(frame_5m) < 10 or len(frame_4h) < 1:
        return _wait(name, "Need both 5m and 4h candles.")

    ny_index_4h = frame_4h.index.tz_convert(request.config.ny_timezone)
    ny_index_5m = frame_5m.index.tz_convert(request.config.ny_timezone)
    current_date = ny_index_5m[-1].date()

    day_4h = frame_4h[(ny_index_4h.date == current_date)]
    if day_4h.empty:
        return _wait(name, "No 4H candle found for the current New York date.")

    first_candle = day_4h.iloc[0]
    range_high = float(first_candle["high"])
    range_low = float(first_candle["low"])

    post_range = frame_5m[(ny_index_5m.date == current_date) & (ny_index_5m.hour >= 4)]
    if len(post_range) < 2:
        return _wait(name, "Not enough 5m candles after the opening 4H range.")

    closes = post_range["close"]
    above_mask = closes > range_high
    below_mask = closes < range_low
    latest_close = float(closes.iloc[-1])
    previous_close = float(closes.iloc[-2])

    if previous_close > range_high and latest_close < range_high:
        breakout_segment = post_range[above_mask | (post_range.index == post_range.index[-1])]
        stop = float(breakout_segment["high"].max())
        entry = latest_close
        return StrategySignal(
            strategy=name,
            state="SHORT",
            entry_price=entry,
            stop_loss=stop,
            take_profit=entry - 2 * abs(stop - entry),
            reason="5m close re-entered below the 4H range high after a failed breakout.",
            metadata={"range_high": range_high, "range_low": range_low},
        )

    if previous_close < range_low and latest_close > range_low:
        breakout_segment = post_range[below_mask | (post_range.index == post_range.index[-1])]
        stop = float(breakout_segment["low"].min())
        entry = latest_close
        return StrategySignal(
            strategy=name,
            state="LONG",
            entry_price=entry,
            stop_loss=stop,
            take_profit=entry + 2 * abs(entry - stop),
            reason="5m close re-entered above the 4H range low after a failed breakdown.",
            metadata={"range_high": range_high, "range_low": range_low},
        )

    return _wait(name, "No failed-breakout re-entry detected yet.", range_high=range_high, range_low=range_low)


def three_ema_fractal_strategy(request: StrategyEvaluationRequest) -> StrategySignal:
    name = "three_ema_williams_fractal"
    candles = request.candles_5m or request.candles_1m
    frame = candles_to_frame(candles)
    if len(frame) < 120:
        return _wait(name, "Need enough candles to stabilize EMA 100 and fractals.")

    frame = frame.copy()
    frame["ema_20"] = ema(frame["close"], 20)
    frame["ema_50"] = ema(frame["close"], 50)
    frame["ema_100"] = ema(frame["close"], 100)
    fractal_up, fractal_down = detect_williams_fractals(frame)

    last = frame.iloc[-1]
    gap_20_50 = abs(float(last["ema_20"] - last["ema_50"]))
    gap_50_100 = abs(float(last["ema_50"] - last["ema_100"]))
    gap_ok = gap_20_50 > request.config.ema_gap_threshold and gap_50_100 > request.config.ema_gap_threshold

    bullish = gap_ok and last["ema_20"] > last["ema_50"] > last["ema_100"] and last["close"] > last["ema_20"] and bool(fractal_up.iloc[-1])
    bearish = gap_ok and last["ema_20"] < last["ema_50"] < last["ema_100"] and last["close"] < last["ema_20"] and bool(fractal_down.iloc[-1])

    if not bullish and not bearish:
        return _wait(
            name,
            "EMA fan, gap, or fractal trigger not aligned.",
            ema_20=float(last["ema_20"]),
            ema_50=float(last["ema_50"]),
            ema_100=float(last["ema_100"]),
            gap_20_50=gap_20_50,
            gap_50_100=gap_50_100,
        )

    direction = "LONG" if bullish else "SHORT"
    entry = float(last["close"])
    ema_stop = float(last["ema_20"])
    swing_stop = previous_swing_low(frame, 8) if bullish else previous_swing_high(frame, 8)
    stop = min(ema_stop, swing_stop) if bullish and swing_stop is not None else max(ema_stop, swing_stop) if swing_stop is not None else ema_stop
    if bullish and stop >= entry:
        stop = entry - float(atr(frame).iloc[-1])
    if bearish and stop <= entry:
        stop = entry + float(atr(frame).iloc[-1])

    return StrategySignal(
        strategy=name,
        state=direction,
        entry_price=entry,
        stop_loss=stop,
        take_profit=_rr_target(entry, stop, 2.0, direction),
        reason=f"{direction} EMA fan is clean and a Williams fractal trigger is confirmed.",
        metadata={"ema_gap_threshold": request.config.ema_gap_threshold},
    )


def smc_fvg_liquidity_grab_strategy(request: StrategyEvaluationRequest) -> StrategySignal:
    name = "smc_fvg_liquidity_grab"
    frame_30m = candles_to_frame(request.candles_30m)
    frame_5m = candles_to_frame(request.candles_5m)
    if len(frame_30m) < 20 or len(frame_5m) < 10:
        return _wait(name, "Need enough 30m and 5m candles.")

    structure_bias = detect_structure_bias(frame_30m, request.config.structure_lookback)
    if structure_bias == "neutral":
        return _wait(name, "No clear 30m BOS/CHoCH bias detected.")

    fvg = find_recent_fvg(frame_30m, direction="bullish" if structure_bias == "bullish" else "bearish")
    if fvg is None:
        return _wait(name, "No matching 30m fair value gap found.")

    latest = frame_5m.iloc[-1]
    prior_swing_low = previous_swing_low(frame_5m, request.config.liquidity_swing_lookback)
    prior_swing_high = previous_swing_high(frame_5m, request.config.liquidity_swing_lookback)

    if structure_bias == "bullish":
        entered_zone = latest["low"] <= fvg.upper and latest["high"] >= fvg.lower
        liquidity_grab = (
            entered_zone
            and prior_swing_low is not None
            and latest["low"] < prior_swing_low
            and latest["close"] > prior_swing_low
        )
        if not liquidity_grab:
            return _wait(
                name,
                "Bullish FVG found, waiting for 5m liquidity grab confirmation.",
                fvg={"lower": fvg.lower, "upper": fvg.upper},
                structure_bias=structure_bias,
            )

        target = float(frame_30m["high"].iloc[-request.config.structure_lookback :].max())
        return StrategySignal(
            strategy=name,
            state="LONG",
            entry_price=float(latest["close"]),
            stop_loss=float(latest["low"]),
            take_profit=target,
            reason="Price entered a bullish 30m FVG and printed a 5m liquidity grab reclaim.",
            metadata={"fvg": {"lower": fvg.lower, "upper": fvg.upper}, "structure_bias": structure_bias},
        )

    entered_zone = latest["high"] >= fvg.lower and latest["low"] <= fvg.upper
    liquidity_grab = (
        entered_zone
        and prior_swing_high is not None
        and latest["high"] > prior_swing_high
        and latest["close"] < prior_swing_high
    )
    if not liquidity_grab:
        return _wait(
            name,
            "Bearish FVG found, waiting for 5m liquidity grab confirmation.",
            fvg={"lower": fvg.lower, "upper": fvg.upper},
            structure_bias=structure_bias,
        )

    target = float(frame_30m["low"].iloc[-request.config.structure_lookback :].min())
    return StrategySignal(
        strategy=name,
        state="SHORT",
        entry_price=float(latest["close"]),
        stop_loss=float(latest["high"]),
        take_profit=target,
        reason="Price entered a bearish 30m FVG and printed a 5m liquidity sweep rejection.",
        metadata={"fvg": {"lower": fvg.lower, "upper": fvg.upper}, "structure_bias": structure_bias},
    )


def ny_open_session_range_strategy(request: StrategyEvaluationRequest) -> StrategySignal:
    name = "ny_open_session_range_breakout"
    frame_1h = candles_to_frame(request.candles_1h)
    frame_5m = candles_to_frame(request.candles_5m)
    if len(frame_1h) < 3 or len(frame_5m) < 20:
        return _wait(name, "Need both 1H and 5m New York session candles.")

    ny_zone = request.config.ny_timezone
    index_1h = frame_1h.index.tz_convert(ny_zone)
    index_5m = frame_5m.index.tz_convert(ny_zone)
    trading_day = index_5m[-1].date()

    candle_7 = frame_1h[(index_1h.date == trading_day) & (index_1h.hour == 7)]
    candle_8 = frame_1h[(index_1h.date == trading_day) & (index_1h.hour == 8)]
    if candle_7.empty or candle_8.empty:
        return _wait(name, "Missing 7:00 or 8:00 New York hourly candles.")

    h7 = candle_7.iloc[-1]
    h8 = candle_8.iloc[-1]
    bullish_bias = h8["close"] > h8["open"] and h8["high"] >= h7["high"] and h8["low"] <= h7["low"]
    bearish_bias = h8["close"] < h8["open"] and h8["high"] >= h7["high"] and h8["low"] <= h7["low"]
    if not bullish_bias and not bearish_bias:
        return _wait(name, "No clean 7:00/8:00 engulfing bias.")

    range_slice = frame_5m[(index_5m.date == trading_day) & (index_5m.hour == 9) & (index_5m.minute >= 30) & (index_5m.minute < 45)]
    if range_slice.empty:
        return _wait(name, "Missing 9:30-9:45 New York opening range candles.")

    session_high = float(range_slice["high"].max())
    session_low = float(range_slice["low"].min())
    after_range = frame_5m[(index_5m.date == trading_day) & ((index_5m.hour > 9) | ((index_5m.hour == 9) & (index_5m.minute >= 45)))]
    if after_range.empty:
        return _wait(name, "Waiting for post-opening-range action.")

    breakout_candle = None
    if bullish_bias:
        breakout = after_range[after_range["close"] > session_high]
        if breakout.empty:
            return _wait(name, "Bullish bias present, waiting for breakout above session range.", session_high=session_high, session_low=session_low)
        breakout_candle = breakout.iloc[0]
        zone_low = min(float(breakout_candle["open"]), float(breakout_candle["close"]))
        zone_high = max(float(breakout_candle["open"]), float(breakout_candle["close"]))
        latest = after_range.iloc[-1]
        tapped = latest["low"] <= zone_high and latest["high"] >= zone_low
        if not tapped or not candle_body_engulfs(after_range.loc[: after_range.index[-1]].tail(2), bullish=True):
            return _wait(name, "Breakout confirmed, waiting for retest into demand/FVG zone with engulfing confirmation.", zone_low=zone_low, zone_high=zone_high)

        entry = float(latest["close"])
        stop = min(float(latest["low"]), zone_low)
        return StrategySignal(
            strategy=name,
            state="LONG",
            entry_price=entry,
            stop_loss=stop,
            take_profit=_rr_target(entry, stop, 2.0, "LONG"),
            reason="New York bullish bias confirmed, breakout retest tapped demand and printed bullish engulfing.",
            metadata={"session_high": session_high, "session_low": session_low, "zone_low": zone_low, "zone_high": zone_high},
        )

    breakout = after_range[after_range["close"] < session_low]
    if breakout.empty:
        return _wait(name, "Bearish bias present, waiting for breakout below session range.", session_high=session_high, session_low=session_low)
    breakout_candle = breakout.iloc[0]
    zone_low = min(float(breakout_candle["open"]), float(breakout_candle["close"]))
    zone_high = max(float(breakout_candle["open"]), float(breakout_candle["close"]))
    latest = after_range.iloc[-1]
    tapped = latest["high"] >= zone_low and latest["low"] <= zone_high
    if not tapped or not candle_body_engulfs(after_range.loc[: after_range.index[-1]].tail(2), bullish=False):
        return _wait(name, "Breakdown confirmed, waiting for retest into supply/FVG zone with bearish engulfing.", zone_low=zone_low, zone_high=zone_high)

    entry = float(latest["close"])
    stop = max(float(latest["high"]), zone_high)
    return StrategySignal(
        strategy=name,
        state="SHORT",
        entry_price=entry,
        stop_loss=stop,
        take_profit=_rr_target(entry, stop, 2.0, "SHORT"),
        reason="New York bearish bias confirmed, breakout retest tapped supply and printed bearish engulfing.",
        metadata={"session_high": session_high, "session_low": session_low, "zone_low": zone_low, "zone_high": zone_high},
    )


def sr_ema_crossover_strategy(request: StrategyEvaluationRequest) -> StrategySignal:
    name = "sr_ema_crossover_sniper"
    candles = request.candles_1m or request.candles_5m
    frame = candles_to_frame(candles)
    if len(frame) < 25:
        return _wait(name, "Need enough candles for EMA 9/20 crossover.")

    support = request.support_resistance.support
    resistance = request.support_resistance.resistance
    if support is None and resistance is None:
        return _wait(name, "Support/resistance levels are required for the sniper strategy.")

    frame = frame.copy()
    frame["ema_9"] = ema(frame["close"], 9)
    frame["ema_20"] = ema(frame["close"], 20)
    last = frame.iloc[-1]
    tolerance = float(last["close"]) * 0.0015

    if support is not None and crosses_above(frame["ema_9"] - frame["ema_20"], 0):
        near_support = float(last["low"]) <= support + tolerance
        if near_support:
            stop = min(float(last["low"]), support - tolerance)
            entry = float(last["close"])
            return StrategySignal(
                strategy=name,
                state="LONG",
                entry_price=entry,
                stop_loss=stop,
                take_profit=_rr_target(entry, stop, 2.0, "LONG"),
                reason="EMA 9 crossed above EMA 20 at support for an immediate sniper long.",
                metadata={"support": support},
            )

    if resistance is not None and crosses_below(frame["ema_9"] - frame["ema_20"], 0):
        near_resistance = float(last["high"]) >= resistance - tolerance
        if near_resistance:
            stop = max(float(last["high"]), resistance + tolerance)
            entry = float(last["close"])
            return StrategySignal(
                strategy=name,
                state="SHORT",
                entry_price=entry,
                stop_loss=stop,
                take_profit=_rr_target(entry, stop, 2.0, "SHORT"),
                reason="EMA 9 crossed below EMA 20 at resistance for an immediate sniper short.",
                metadata={"resistance": resistance},
            )

    return _wait(name, "No EMA crossover occurred at the supplied support/resistance level.")


def stochastic_momentum_squeeze_strategy(request: StrategyEvaluationRequest) -> StrategySignal:
    name = "stochastic_momentum_squeeze"
    candles = request.candles_1m or request.candles_5m
    frame = candles_to_frame(candles)
    if len(frame) < 30:
        return _wait(name, "Need enough candles for the stochastic squeeze setup.")

    frame = frame.copy()
    frame["k"], frame["d"] = stochastic_oscillator(
        frame,
        k_period=request.config.stochastic_k_period,
        smooth_k=request.config.stochastic_smooth_k,
        smooth_d=request.config.stochastic_smooth_d,
    )

    recent_k = frame["k"].iloc[-3:]
    recent_d = frame["d"].iloc[-3:]
    last = frame.iloc[-1]

    hook_up = recent_k.iloc[-2] <= recent_k.iloc[-3] and recent_k.iloc[-1] > recent_k.iloc[-2]
    hook_down = recent_k.iloc[-2] >= recent_k.iloc[-3] and recent_k.iloc[-1] < recent_k.iloc[-2]
    d_rising = recent_d.iloc[-1] > recent_d.iloc[-2]
    d_falling = recent_d.iloc[-1] < recent_d.iloc[-2]
    recently_oversold = float(frame["k"].iloc[-5:].min()) <= request.config.stochastic_oversold_threshold
    recently_overbought = float(frame["k"].iloc[-5:].max()) >= request.config.stochastic_overbought_threshold

    if hook_up and d_rising and float(last["k"]) > float(last["d"]) and recently_oversold:
        stop = previous_swing_low(frame, 5) or float(last["low"])
        return StrategySignal(
            strategy=name,
            state="LONG",
            entry_price=float(last["close"]),
            stop_loss=float(stop),
            take_profit=None,
            reason="Stochastic K hooked up from a recent dip while D continued rising.",
            metadata={
                "k": float(last["k"]),
                "d": float(last["d"]),
                "time_stop_candles": request.config.stochastic_time_stop_candles,
                "exit_rule": "Close after the configured time stop if momentum does not expand.",
            },
        )

    if hook_down and d_falling and float(last["k"]) < float(last["d"]) and recently_overbought:
        stop = previous_swing_high(frame, 5) or float(last["high"])
        return StrategySignal(
            strategy=name,
            state="SHORT",
            entry_price=float(last["close"]),
            stop_loss=float(stop),
            take_profit=None,
            reason="Stochastic K hooked down from a recent spike while D continued falling.",
            metadata={
                "k": float(last["k"]),
                "d": float(last["d"]),
                "time_stop_candles": request.config.stochastic_time_stop_candles,
                "exit_rule": "Close after the configured time stop if momentum does not expand.",
            },
        )

    return _wait(name, "No stochastic squeeze hook is active.")


def bollinger_band_bounce_strategy(request: StrategyEvaluationRequest) -> StrategySignal:
    name = "bollinger_band_bounce"
    candles = request.candles_1m or request.candles_5m
    frame = candles_to_frame(candles)
    if len(frame) < 25:
        return _wait(name, "Need enough candles for Bollinger Bands.")

    lower, middle, upper = bollinger_bands(frame["close"], 20, 2.0)
    frame = frame.copy()
    frame["rsi"] = rsi(frame["close"], 14)
    last = frame.iloc[-1]

    if float(last["close"]) < float(lower.iloc[-1]) and float(last["rsi"]) < 20:
        stop = previous_swing_low(frame, 6) or float(last["low"])
        return StrategySignal(
            strategy=name,
            state="LONG",
            entry_price=float(last["close"]),
            stop_loss=float(stop),
            take_profit=float(middle.iloc[-1]),
            reason="Price stretched below the lower Bollinger Band with RSI exhaustion.",
            metadata={"lower_band": float(lower.iloc[-1]), "middle_band": float(middle.iloc[-1]), "rsi": float(last["rsi"])},
        )

    if float(last["close"]) > float(upper.iloc[-1]) and float(last["rsi"]) > 80:
        stop = previous_swing_high(frame, 6) or float(last["high"])
        return StrategySignal(
            strategy=name,
            state="SHORT",
            entry_price=float(last["close"]),
            stop_loss=float(stop),
            take_profit=float(middle.iloc[-1]),
            reason="Price stretched above the upper Bollinger Band with RSI exhaustion.",
            metadata={"upper_band": float(upper.iloc[-1]), "middle_band": float(middle.iloc[-1]), "rsi": float(last["rsi"])},
        )

    return _wait(name, "No Bollinger/RSI mean-reversion extreme detected.")


def keltner_channel_breakout_strategy(request: StrategyEvaluationRequest) -> StrategySignal:
    name = "keltner_channel_breakout"
    candles = request.candles_1m or request.candles_5m
    frame = candles_to_frame(candles)
    if len(frame) < 25:
        return _wait(name, "Need enough candles for the Keltner Channel.")

    lower, middle, upper = keltner_channel(frame, 20, 2.0)
    frame = frame.copy()
    frame["rsi"] = rsi(frame["close"], 14)
    last = frame.iloc[-1]
    previous = frame.iloc[-2]

    long_break = previous["close"] > upper.iloc[-2] and last["close"] > upper.iloc[-1] and last["rsi"] > 50
    short_break = previous["close"] < lower.iloc[-2] and last["close"] < lower.iloc[-1] and last["rsi"] < 50
    if long_break:
        stop = float(middle.iloc[-1])
        entry = float(last["close"])
        return StrategySignal(
            strategy=name,
            state="LONG",
            entry_price=entry,
            stop_loss=stop,
            take_profit=_rr_target(entry, stop, 2.0, "LONG"),
            reason="Two consecutive closes above the upper Keltner Channel with bullish RSI support.",
            metadata={"upper_channel": float(upper.iloc[-1]), "middle_channel": stop},
        )

    if short_break:
        stop = float(middle.iloc[-1])
        entry = float(last["close"])
        return StrategySignal(
            strategy=name,
            state="SHORT",
            entry_price=entry,
            stop_loss=stop,
            take_profit=_rr_target(entry, stop, 2.0, "SHORT"),
            reason="Two consecutive closes below the lower Keltner Channel with bearish RSI support.",
            metadata={"lower_channel": float(lower.iloc[-1]), "middle_channel": stop},
        )

    return _wait(name, "No Keltner breakout sequence detected.")


def evaluate_all_strategies(request: StrategyEvaluationRequest) -> list[StrategySignal]:
    evaluations = [
        mechanical_1m_btc_strategy(request),
        four_hour_range_reentry_strategy(request),
        three_ema_fractal_strategy(request),
        smc_fvg_liquidity_grab_strategy(request),
        ny_open_session_range_strategy(request),
        sr_ema_crossover_strategy(request),
    ]

    if request.config.enable_stochastic_squeeze:
        evaluations.append(stochastic_momentum_squeeze_strategy(request))

    if request.config.enable_volatility_envelopes:
        evaluations.extend(
            [
                bollinger_band_bounce_strategy(request),
                keltner_channel_breakout_strategy(request),
            ]
        )

    return evaluations
