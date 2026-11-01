"""Library v2 for the automatic October search: 8 more intraday families.

KEL  Keltner channel (EMA20 +/- k ATR) fade or breakout
WPR  Williams %R extremes, exit at -50
CCI  CCI extremes, fade or follow
ZEM  z-score of price against its EMA, exit when it crosses back to 0
MTF  4h Bollinger %b extreme confirmed by a 1h RSI extreme
TD2  finer hour-of-day windows 12-18 UTC with filters
RS2  RSI fade on 45/90/180-minute bars
BB2  Bollinger fade on 45/90/120/180-minute bars
Same conventions as os_library (closed bars, sl_core fills, flat by 00:00 UTC).
"""
import itertools

import numpy as np
import pandas as pd

from os_families2 import atr_bars, rsi
from os_library import SESS, bar_orders
from sl_core import DAY, H


def space2():
    g = {}
    g["KEL"] = list(itertools.product((15, 30, 60, 240), (1.5, 2.0, 2.5, 3.0), ("fade", "break"), (1.0, 2.0), (4, 8), SESS))
    g["WPR"] = list(itertools.product((15, 30, 60, 240), (14, 21), (5, 10, 20), (1.0, 2.0), (4, 8), SESS))
    g["CCI"] = list(itertools.product((15, 30, 60, 240), (14, 20), (100, 150, 200), ("fade", "follow"), (4, 8), SESS))
    g["ZEM"] = list(itertools.product((15, 30, 60, 240), (20, 50), (2.0, 2.5, 3.0), (1.0, 2.0), (4, 8, 12), SESS))
    g["MTF"] = list(itertools.product((1.5, 2.0), (25, 30, 35), (14, 21), (1.0, 2.0), (6, 12)))
    g["TD2"] = [(hh, d, side, filt, days) for hh in range(12, 19) for d in range(2, 9) if hh + d <= 24 for side in ("short", "long")
                for filt in ("none", "vwap", "rsi1h", "day", "trend10") for days in ("wkday", "all")]
    g["RS2"] = list(itertools.product((45, 90, 180), (14, 21, 28), (25, 30, 35), (50, 60, 70), (1.0, 2.0, 3.0), (6, 12)))
    g["BB2"] = list(itertools.product((45, 90, 120, 180), (14, 20, 30), (1.5, 2.0, 2.5), (2.0, 3.0), (4, 8, 12)))
    return g


def f_kel(cx, tf, k, mode, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    e = c.ewm(span=20, adjust=False).mean()
    up, dn = (c > e + k * a).to_numpy(), (c < e - k * a).to_numpy()
    if mode == "fade":
        return bar_orders(cx, tf, dn, up, (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold, tgt_l=e.to_numpy(), tgt_s=e.to_numpy())
    return bar_orders(cx, tf, up, dn, (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold)


def f_wpr(cx, tf, n, lo, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    hh, ll = b["h"].rolling(n).max(), b["l"].rolling(n).min()
    w = -100 * (hh - c) / (hh - ll).replace(0, np.nan)
    tc = b["tc"].to_numpy()
    return bar_orders(cx, tf, (w < -100 + lo).to_numpy(), (w > -lo).to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold,
                      exits_l=tc[(w >= -50).to_numpy()], exits_s=tc[(w <= -50).to_numpy()])


def f_cci(cx, tf, n, thr, mode, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    tp = (b["h"] + b["l"] + b["c"]) / 3
    sma = tp.rolling(n).mean()
    md = tp.rolling(n).apply(lambda x: np.mean(np.abs(x - x.mean())), raw=True)
    cci = (tp - sma) / (0.015 * md.replace(0, np.nan))
    hi, lo = (cci > thr).to_numpy(), (cci < -thr).to_numpy()
    longs, shorts = (lo, hi) if mode == "fade" else (hi, lo)
    return bar_orders(cx, tf, longs, shorts, (c - 1.5 * a).to_numpy(), (c + 1.5 * a).to_numpy(), sess, hold)


def f_zem(cx, tf, n, z, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    e = c.ewm(span=n, adjust=False).mean()
    dev = c - e
    zz = dev / dev.rolling(n).std(ddof=0).replace(0, np.nan)
    tc = b["tc"].to_numpy()
    return bar_orders(cx, tf, (zz < -z).to_numpy(), (zz > z).to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold,
                      exits_l=tc[(zz >= 0).to_numpy()], exits_s=tc[(zz <= 0).to_numpy()])


def f_mtf(cx, k, thr, n, st, hold):
    b4, b1 = cx.bars(240), cx.bars(60)
    c4 = b4["c"]
    m, sd = c4.rolling(20).mean(), c4.rolling(20).std(ddof=0)
    pb = ((c4 - (m - k * sd)) / (2 * k * sd).replace(0, np.nan))
    pb_at = pd.Series(pb.to_numpy(), index=b4["tc"].to_numpy()).reindex(b1["tc"].to_numpy(), method="ffill").to_numpy()
    r = rsi(b1["c"], n).to_numpy()
    c1, a1 = b1["c"], atr_bars(b1, 14)
    longs = (pb_at < 0.1) & (r < thr)
    shorts = (pb_at > 0.9) & (r > 100 - thr)
    return bar_orders(cx, 60, longs, shorts, (c1 - st * a1).to_numpy(), (c1 + st * a1).to_numpy(), "all", hold)


def f_td2(cx, hh, dur, side, filt, days):
    out = []
    b1 = cx.bars(60)
    r1 = pd.Series(rsi(b1["c"], 14).to_numpy(), index=b1["tc"].to_numpy())
    for d in cx.days:
        wd = cx.f.weekday[cx.kday[d]]
        if days == "wkday" and wd >= 5:
            continue
        ts = d + hh * H
        s = 1 if side == "long" else -1
        px = cx.last_close(ts)
        if filt == "vwap":
            vw = cx.vwap(np.array([ts]))[0]
            if np.isnan(vw) or np.sign(px - vw) != s:
                continue
        elif filt == "rsi1h":
            rv = r1.get(ts, np.nan)
            if np.isnan(rv) or (s < 0 and rv <= 50) or (s > 0 and rv >= 50):
                continue  # sell only after an up-move (RSI over 50), buy only after a down-move
        elif filt == "day":
            if np.sign(px - cx.f.dopen[cx.kday[d]]) != s:
                continue
        elif filt == "trend10":
            if cx.trend(d) != s:
                continue
        out.append({"ts": int(ts), "side": s, "entry": "market", "stop": None, "exit_by": int(ts + dur * H)})
    return out


def f_rs2(cx, tf, n, lo, ex, st, hold):
    b = cx.bars(tf)
    r = rsi(b["c"], n)
    a = atr_bars(b, 14)
    c = b["c"]
    tc = b["tc"].to_numpy()
    return bar_orders(cx, tf, (r < lo).to_numpy(), (r > 100 - lo).to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(), "all", hold,
                      exits_l=tc[(r >= ex).to_numpy()], exits_s=tc[(r <= 100 - ex).to_numpy()])


def f_bb2(cx, tf, n, k, st, hold):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    m, sd = c.rolling(n).mean(), c.rolling(n).std(ddof=0)
    return bar_orders(cx, tf, (c < m - k * sd).to_numpy(), (c > m + k * sd).to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(),
                      "all", hold, tgt_l=m.to_numpy(), tgt_s=m.to_numpy())


FN2 = {"KEL": f_kel, "WPR": f_wpr, "CCI": f_cci, "ZEM": f_zem, "MTF": f_mtf, "TD2": f_td2, "RS2": f_rs2, "BB2": f_bb2}
NAMES2 = {"KEL": "Keltner channel", "WPR": "Williams %R", "CCI": "CCI extreme", "ZEM": "EMA z-score fade", "MTF": "4h Bollinger + 1h RSI",
          "TD2": "Hour-of-day (filtered)", "RS2": "RSI fade", "BB2": "Bollinger fade"}


def describe2(fam, p):
    S = {"all": "all day", "us": "US hours", "asia": "Asian hours", "eu": "European hours"}
    if fam == "KEL":
        tf, k, mode, st, hold, sess = p
        return (f"Keltner {k} ATR {mode} on {tf}m, {S[sess]}", f"{'Fade' if mode == 'fade' else 'Follow'} a {tf}-min close beyond EMA20 +/- {k} ATR; stop {st} ATR, up to {hold} h" + (", target EMA20." if mode == "fade" else "."))
    if fam == "WPR":
        tf, n, lo, st, hold, sess = p
        return (f"Williams %R({n}) {tf}m beyond -{100 - lo}/-{lo}, {S[sess]}", f"Buy when %R is under -{100 - lo} (short over -{lo}); exit at -50, after {hold} h or at a {st} ATR stop.")
    if fam == "CCI":
        tf, n, thr, mode, hold, sess = p
        return (f"CCI({n}) {tf}m {mode} at {thr}, {S[sess]}", f"{'Fade' if mode == 'fade' else 'Follow'} CCI beyond +/-{thr}; stop 1.5 ATR, up to {hold} h.")
    if fam == "ZEM":
        tf, n, z, st, hold, sess = p
        return (f"EMA{n} z-score {z} fade on {tf}m, {S[sess]}", f"Fade price {z} standard deviations from its EMA{n}; exit when it crosses back, after {hold} h or at a {st} ATR stop.")
    if fam == "MTF":
        k, thr, n, st, hold = p
        return (f"4h Bollinger({k}) extreme + 1h RSI({n}) {thr}", f"Buy when the 4h close is near its lower band and 1h RSI({n}) is under {thr} (mirror for shorts); stop {st} ATR, up to {hold} h.")
    if fam == "TD2":
        hh, d, side, filt, days = p
        fl = {"none": "", "vwap": ", only if price is on that side of VWAP", "rsi1h": ", only against the last hour's RSI",
              "day": ", only with the day's move", "trend10": ", only with the 10-day trend"}[filt]
        return (f"{'Sell' if side == 'short' else 'Buy'} {hh:02d}:00 UTC for {d} h{fl} ({'weekdays' if days == 'wkday' else 'every day'})",
                f"{'Weekdays' if days == 'wkday' else 'Every day'} at {hh:02d}:00 UTC {'sell' if side == 'short' else 'buy'}{fl}; close {d} h later.")
    if fam == "RS2":
        tf, n, lo, ex, st, hold = p
        return (f"RSI({n}) {tf}m fade <{lo}/>{100 - lo}, exit {ex}", f"On {tf}-min bars buy RSI({n}) under {lo} (short over {100 - lo}); exit at {ex}, after {hold} h or at a {st} ATR stop.")
    if fam == "BB2":
        tf, n, k, st, hold = p
        return (f"Bollinger({n},{k}) {tf}m fade", f"Buy a {tf}-min close under the lower band (short over the upper); target the middle band, stop {st} ATR, up to {hold} h.")
    return fam, ""
