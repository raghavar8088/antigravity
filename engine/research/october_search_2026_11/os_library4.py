"""Library v4 for the automatic October search: finer grids around the three
families that lead the list, plus volume filters.

RZ  RSI fade, 30m-2h bars, period 16-28, thresholds 26-34, exits 50-60
BZ  Bollinger fade, 2h-4h bars, 16-40 bars, 1.6-2.4 sd
HZ  hour-of-day windows 13-17 UTC for 3-9 h with seven filters
Volume filter: 'high' / 'low' = the signal bar's volume above / below its
20-bar average; 'none' = either.
"""
import itertools

import numpy as np
import pandas as pd

from os_families2 import atr_bars, rsi
from os_library import bar_orders
from sl_core import DAY, H


def space4():
    g = {}
    g["RZ"] = list(itertools.product((30, 60, 90, 120), (16, 18, 21, 24, 28), (26, 28, 30, 32, 34), (50, 55, 60), (1.5, 2.0, 2.5),
                                     (4, 6, 8), ("none", "high", "low")))
    g["BZ"] = list(itertools.product((120, 180, 240), (16, 20, 24, 30, 40), (1.6, 1.8, 2.0, 2.2, 2.4), (2.0, 3.0, 4.0), (4, 8, 12),
                                     ("none", "high", "low")))
    g["HZ"] = [(hh, d, side, filt, days) for hh in range(13, 18) for d in range(3, 10) if hh + d <= 24 for side in ("short", "long")
               for filt in ("none", "vwap", "rsi1h", "day", "trend10", "highvol", "lowvol") for days in ("wkday", "all")]
    return g


def vol_ok(b, mode):
    if mode == "none":
        return np.ones(len(b), bool)
    avg = b["v"].rolling(20).mean().shift(1)
    return ((b["v"] > avg) if mode == "high" else (b["v"] < avg)).to_numpy()


def f_rz(cx, tf, n, lo, ex, st, hold, vf):
    b = cx.bars(tf)
    r = rsi(b["c"], n)
    a = atr_bars(b, 14)
    c = b["c"]
    tc = b["tc"].to_numpy()
    ok = vol_ok(b, vf)
    return bar_orders(cx, tf, (r < lo).to_numpy() & ok, (r > 100 - lo).to_numpy() & ok, (c - st * a).to_numpy(), (c + st * a).to_numpy(),
                      "all", hold, exits_l=tc[(r >= ex).to_numpy()], exits_s=tc[(r <= 100 - ex).to_numpy()])


def f_bz(cx, tf, n, k, st, hold, vf):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    m, sd = c.rolling(n).mean(), c.rolling(n).std(ddof=0)
    ok = vol_ok(b, vf)
    return bar_orders(cx, tf, (c < m - k * sd).to_numpy() & ok, (c > m + k * sd).to_numpy() & ok, (c - st * a).to_numpy(), (c + st * a).to_numpy(),
                      "all", hold, tgt_l=m.to_numpy(), tgt_s=m.to_numpy())


def f_hz(cx, hh, dur, side, filt, days):
    b1 = cx.bars(60)
    r1 = pd.Series(rsi(b1["c"], 14).to_numpy(), index=b1["tc"].to_numpy())
    v1 = pd.Series(b1["v"].to_numpy(), index=b1["tc"].to_numpy())
    va = pd.Series(b1["v"].rolling(24).mean().shift(1).to_numpy(), index=b1["tc"].to_numpy())
    s = 1 if side == "long" else -1
    out = []
    for d in cx.days:
        if days == "wkday" and cx.f.weekday[cx.kday[d]] >= 5:
            continue
        ts = d + hh * H
        px = cx.last_close(ts)
        if filt == "vwap":
            vw = cx.vwap(np.array([ts]))[0]
            if np.isnan(vw) or np.sign(px - vw) != s:
                continue
        elif filt == "rsi1h":
            rv = r1.get(ts, np.nan)
            if np.isnan(rv) or (s < 0 and rv <= 50) or (s > 0 and rv >= 50):
                continue
        elif filt == "day":
            if np.sign(px - cx.f.dopen[cx.kday[d]]) != s:
                continue
        elif filt == "trend10":
            if cx.trend(d) != s:
                continue
        elif filt in ("highvol", "lowvol"):
            vv, aa = v1.get(ts, np.nan), va.get(ts, np.nan)
            if np.isnan(vv) or np.isnan(aa) or ((vv > aa) != (filt == "highvol")):
                continue
        out.append({"ts": int(ts), "side": s, "entry": "market", "stop": None, "exit_by": int(ts + dur * H)})
    return out


FN4 = {"RZ": f_rz, "BZ": f_bz, "HZ": f_hz}
NAMES4 = {"RZ": "RSI fade", "BZ": "Bollinger fade", "HZ": "Hour-of-day"}


def describe4(fam, p):
    vf = lambda x: {"none": "", "high": ", high-volume bars only", "low": ", low-volume bars only"}[x]
    if fam == "RZ":
        tf, n, lo, ex, st, hold, v = p
        return (f"RSI({n}) {tf}m fade <{lo}/>{100 - lo}, exit {ex}, stop {st} ATR, {hold} h{vf(v)}",
                f"On {tf}-min bars buy RSI({n}) under {lo} (short over {100 - lo}); exit at {ex}, after {hold} h or at a {st} ATR stop{vf(v)}.")
    if fam == "BZ":
        tf, n, k, st, hold, v = p
        return (f"Bollinger({n},{k}) {tf}m fade, stop {st} ATR, {hold} h{vf(v)}",
                f"Buy a {tf}-min close under the lower band (short over the upper); target the middle band, stop {st} ATR, up to {hold} h{vf(v)}.")
    if fam == "HZ":
        hh, d, side, filt, days = p
        fl = {"none": "", "vwap": ", only on that side of VWAP", "rsi1h": ", only against the last hour's RSI", "day": ", only with the day's move",
              "trend10": ", only with the 10-day trend", "highvol": ", only after a busy hour", "lowvol": ", only after a quiet hour"}[filt]
        return (f"{'Sell' if side == 'short' else 'Buy'} {hh:02d}:00 UTC for {d} h{fl} ({'weekdays' if days == 'wkday' else 'every day'})",
                f"{'Weekdays' if days == 'wkday' else 'Every day'} at {hh:02d}:00 UTC {'sell' if side == 'short' else 'buy'}{fl}; close {d} h later.")
    return fam, ""
