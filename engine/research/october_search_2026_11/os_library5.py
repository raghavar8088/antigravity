"""Library v5 for the automatic October search: grids around the newest leaders.

KZ  Keltner fade on 2h-4h bars (EMA 10/20/30, 1.0-2.0 ATR), target EMA or none
CZ  1h RSI fade restricted to an hour window and a VWAP side (window 14-22 UTC start)
MT2 4h band extreme (Bollinger or Keltner) triggered by a 15m/30m RSI extreme
"""
import itertools

import numpy as np
import pandas as pd

from os_families2 import atr_bars, rsi
from os_library import bar_orders
from sl_core import DAY, H


def space5():
    g = {}
    g["KZ"] = list(itertools.product((120, 180, 240), (10, 20, 30), (1.0, 1.25, 1.5, 1.75, 2.0), (1.0, 2.0, 3.0), (4, 8, 12), ("ema", "none")))
    g["CZ"] = list(itertools.product((14, 16, 18, 20, 22), (8, 10, 12, 14), ("any", "with", "against"), (14, 21, 28), (25, 30, 35), (50, 60)))
    g["MT2"] = list(itertools.product(("bb", "kel"), (1.5, 2.0), (15, 30), (14, 21), (25, 30), (1.0, 2.0), (4, 8)))
    return g


def f_kz(cx, tf, n, k, st, hold, tgt):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    e = c.ewm(span=n, adjust=False).mean()
    up, dn = (c > e + k * a).to_numpy(), (c < e - k * a).to_numpy()
    t = e.to_numpy() if tgt == "ema" else None
    return bar_orders(cx, tf, dn, up, (c - st * a).to_numpy(), (c + st * a).to_numpy(), "all", hold, tgt_l=t, tgt_s=t)


def f_cz(cx, h0, span, vw, n, lo, ex):
    b = cx.bars(60)
    r = rsi(b["c"], n)
    a = atr_bars(b, 14)
    c = b["c"]
    tc = b["tc"].to_numpy()
    inwin = (((tc % DAY) // H - h0) % 24) < span
    vwap = cx.vwap(tc)
    cv = c.to_numpy()
    longs = (r < lo).to_numpy() & inwin
    shorts = (r > 100 - lo).to_numpy() & inwin
    if vw == "with":
        longs &= cv > vwap
        shorts &= cv < vwap
    elif vw == "against":
        longs &= cv < vwap
        shorts &= cv > vwap
    return bar_orders(cx, 60, longs, shorts, (c - 2.0 * a).to_numpy(), (c + 2.0 * a).to_numpy(), "all", 6,
                      exits_l=tc[(r >= ex).to_numpy()], exits_s=tc[(r <= 100 - ex).to_numpy()])


def f_mt2(cx, band, k, ttf, n, thr, st, hold):
    b4 = cx.bars(240)
    c4 = b4["c"]
    if band == "bb":
        m, sd = c4.rolling(20).mean(), c4.rolling(20).std(ddof=0)
        lower, upper = m - k * sd, m + k * sd
    else:
        e, a4 = c4.ewm(span=20, adjust=False).mean(), atr_bars(b4, 14)
        lower, upper = e - k * a4, e + k * a4
    low_at = pd.Series((c4 < lower).to_numpy(), index=b4["tc"].to_numpy())
    high_at = pd.Series((c4 > upper).to_numpy(), index=b4["tc"].to_numpy())
    bt = cx.bars(ttf)
    tct = bt["tc"].to_numpy()
    lo_ok = low_at.reindex(tct, method="ffill").fillna(False).to_numpy(bool)
    hi_ok = high_at.reindex(tct, method="ffill").fillna(False).to_numpy(bool)
    r = rsi(bt["c"], n).to_numpy()
    c, a = bt["c"], atr_bars(bt, 14)
    return bar_orders(cx, ttf, lo_ok & (r < thr), hi_ok & (r > 100 - thr), (c - st * a).to_numpy(), (c + st * a).to_numpy(), "all", hold)


FN5 = {"KZ": f_kz, "CZ": f_cz, "MT2": f_mt2}
NAMES5 = {"KZ": "Keltner channel", "CZ": "RSI fade by hour and VWAP", "MT2": "4h band + fast RSI"}


def describe5(fam, p):
    if fam == "KZ":
        tf, n, k, st, hold, tgt = p
        return (f"Keltner(EMA{n}, {k} ATR) fade on {tf}m, stop {st} ATR, {hold} h" + (", target EMA" if tgt == "ema" else ""),
                f"Fade a {tf}-min close beyond EMA{n} +/- {k} ATR; stop {st} ATR, up to {hold} h" + (", target the EMA." if tgt == "ema" else "."))
    if fam == "CZ":
        h0, span, vw, n, lo, ex = p
        return (f"RSI({n}) 1h fade {lo}/{100 - lo} from {h0:02d}:00 UTC for {span} h, VWAP {vw}, exit {ex}",
                f"Between {h0:02d}:00 and {(h0 + span) % 24:02d}:00 UTC fade 1h RSI({n}) under {lo} / over {100 - lo}" +
                ("" if vw == "any" else f", only {'with' if vw == 'with' else 'against'} price's side of VWAP") + f"; exit at {ex}, 6 h or a 2 ATR stop.")
    if fam == "MT2":
        band, k, ttf, n, thr, st, hold = p
        return (f"4h {'Bollinger' if band == 'bb' else 'Keltner'}({k}) extreme + {ttf}m RSI({n}) {thr}",
                f"When the last 4h close is beyond its {'Bollinger' if band == 'bb' else 'Keltner'} {k} band, fade a {ttf}-min RSI({n}) extreme ({thr}/{100 - thr}); stop {st} ATR, up to {hold} h.")
    return fam, ""
