"""Library v6 for the automatic October search: more mean-reversion anchors.

FSH  Fisher transform extremes, fade
CRS  Connors RSI (RSI3 + streak RSI2 + 100-bar percent rank) extremes, fade
MFI  Money Flow Index extremes, fade
DOF  fade a stretch from the 00:00 UTC open beyond k x daily ATR
PCF  fade a stretch from yesterday's close beyond k x daily ATR
KZH  1h Keltner fade inside an hour window, optional VWAP side
"""
import itertools

import numpy as np
import pandas as pd

from os_families2 import OCT0, OCT1, atr_bars, rsi
from os_library import SESS, bar_orders, window
from sl_core import DAY, H


def space6():
    g = {}
    g["FSH"] = list(itertools.product((15, 30, 60, 240), (9, 14), (1.5, 2.0, 2.5), (1.0, 2.0), (4, 8), SESS))
    g["CRS"] = list(itertools.product((15, 30, 60, 240), (5, 10, 15), (50, 70), (1.0, 2.0), (4, 8), SESS))
    g["MFI"] = list(itertools.product((15, 30, 60, 240), (14, 21), (10, 20, 30), (1.0, 2.0), (4, 8), SESS))
    g["DOF"] = list(itertools.product((0.5, 0.75, 1.0, 1.25, 1.5), (15, 60), ("anchor", "half"), (0.5, 1.0), (4, 8, 0), ("all", "us", "eu")))
    g["PCF"] = list(itertools.product((0.5, 0.75, 1.0, 1.25, 1.5), (15, 60), ("anchor", "half"), (0.5, 1.0), (4, 8, 0), ("all", "us", "eu")))
    g["KZH"] = list(itertools.product((1.5, 2.0, 2.5), (14, 16, 18, 20, 22), (8, 12), ("any", "against"), (1.5, 2.5), (4, 8)))
    return g


def f_fsh(cx, tf, n, thr, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    med = (b["h"] + b["l"]) / 2
    hh, ll = med.rolling(n).max(), med.rolling(n).min()
    x = (2 * (med - ll) / (hh - ll).replace(0, np.nan) - 1).clip(-0.999, 0.999)
    v = x.ewm(alpha=0.5, adjust=False).mean().clip(-0.999, 0.999)
    fish = (0.5 * np.log((1 + v) / (1 - v))).ewm(alpha=0.5, adjust=False).mean()
    return bar_orders(cx, tf, (fish < -thr).to_numpy(), (fish > thr).to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold)


def connors(c):
    r3 = rsi(c, 3)
    d = np.sign(c.diff()).fillna(0).to_numpy()
    streak = np.zeros(len(d))
    for i in range(1, len(d)):
        streak[i] = (streak[i - 1] + 1 if streak[i - 1] >= 0 else 1) if d[i] > 0 else ((streak[i - 1] - 1 if streak[i - 1] <= 0 else -1) if d[i] < 0 else 0)
    rs = rsi(pd.Series(streak, index=c.index), 2)
    roc = c.pct_change()
    pr = roc.rolling(100).apply(lambda w: (w[:-1] < w[-1]).mean() * 100, raw=True)
    return (r3 + rs + pr) / 3


def f_crs(cx, tf, lo, ex, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    cr = connors(c)
    tc = b["tc"].to_numpy()
    return bar_orders(cx, tf, (cr < lo).to_numpy(), (cr > 100 - lo).to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold,
                      exits_l=tc[(cr >= ex).to_numpy()], exits_s=tc[(cr <= 100 - ex).to_numpy()])


def f_mfi(cx, tf, n, lo, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    tp = (b["h"] + b["l"] + b["c"]) / 3
    mf = tp * b["v"]
    pos = mf.where(tp > tp.shift(1), 0.0).rolling(n).sum()
    neg = mf.where(tp < tp.shift(1), 0.0).rolling(n).sum()
    mfi = 100 - 100 / (1 + pos / neg.replace(0, np.nan))
    tc = b["tc"].to_numpy()
    return bar_orders(cx, tf, (mfi < lo).to_numpy(), (mfi > 100 - lo).to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold,
                      exits_l=tc[(mfi >= 50).to_numpy()], exits_s=tc[(mfi <= 50).to_numpy()])


def anchor_fade(cx, anchor_fn, k, tf, tgt, st, hold, sess):
    b = cx.bars(tf)
    tc = b["tc"].to_numpy()
    c = b["c"].to_numpy()
    out, used = [], set()
    for i in range(len(b)):
        t = int(tc[i])
        d = t // DAY * DAY
        if not (OCT0 <= d < OCT1):
            continue
        w = window(cx, d, sess)
        if w is None or not (w[0] <= t < w[1] - 1800):
            continue
        kd = cx.kday[d]
        atr_d = cx.f.atr[kd]
        if np.isnan(atr_d):
            continue
        anc = anchor_fn(kd)
        stretch = (c[i] - anc) / atr_d
        side = -1 if stretch > k else (1 if stretch < -k else 0)
        if side == 0 or (d, side) in used:
            continue
        used.add((d, side))
        target = anc if tgt == "anchor" else (anc + c[i]) / 2
        end = int(w[1]) if hold == 0 else int(min(t + hold * H, w[1]))
        out.append({"ts": t, "side": side, "entry": "market", "target": float(target),
                    "stop": float(c[i] - side * st * atr_d), "exit_by": end})
    return out


def f_dof(cx, k, tf, tgt, st, hold, sess):
    return anchor_fade(cx, lambda kd: cx.f.dopen[kd], k, tf, tgt, st, hold, sess)


def f_pcf(cx, k, tf, tgt, st, hold, sess):
    return anchor_fade(cx, lambda kd: cx.f.mk.daily["c"].iat[kd - 1], k, tf, tgt, st, hold, sess)


def f_kzh(cx, k, h0, span, vw, st, hold):
    b = cx.bars(60)
    c, a = b["c"], atr_bars(b, 14)
    e = c.ewm(span=20, adjust=False).mean()
    tc = b["tc"].to_numpy()
    inwin = (((tc % DAY) // H - h0) % 24) < span
    up, dn = (c > e + k * a).to_numpy() & inwin, (c < e - k * a).to_numpy() & inwin
    if vw == "against":
        vwap = cx.vwap(tc)
        cv = c.to_numpy()
        up &= cv > vwap
        dn &= cv < vwap
    return bar_orders(cx, 60, dn, up, (c - st * a).to_numpy(), (c + st * a).to_numpy(), "all", hold, tgt_l=e.to_numpy(), tgt_s=e.to_numpy())


FN6 = {"FSH": f_fsh, "CRS": f_crs, "MFI": f_mfi, "DOF": f_dof, "PCF": f_pcf, "KZH": f_kzh}
NAMES6 = {"FSH": "Fisher transform", "CRS": "Connors RSI", "MFI": "Money Flow Index", "DOF": "Fade from the day's open",
          "PCF": "Fade from yesterday's close", "KZH": "Keltner 1h by hour"}


def describe6(fam, p):
    S = {"all": "all day", "us": "US hours", "asia": "Asian hours", "eu": "European hours"}
    if fam == "FSH":
        tf, n, thr, st, hold, sess = p
        return (f"Fisher({n}) {tf}m fade beyond {thr}, {S[sess]}", f"Fade a {tf}-min Fisher transform beyond +/-{thr}; stop {st} ATR, up to {hold} h.")
    if fam == "CRS":
        tf, lo, ex, st, hold, sess = p
        return (f"Connors RSI {tf}m <{lo}/>{100 - lo}, exit {ex}, {S[sess]}", f"Buy Connors RSI under {lo} (short over {100 - lo}); exit at {ex}, after {hold} h or at a {st} ATR stop.")
    if fam == "MFI":
        tf, n, lo, st, hold, sess = p
        return (f"MFI({n}) {tf}m <{lo}/>{100 - lo}, {S[sess]}", f"Buy MFI({n}) under {lo} (short over {100 - lo}); exit at 50, after {hold} h or at a {st} ATR stop.")
    if fam in ("DOF", "PCF"):
        k, tf, tgt, st, hold, sess = p
        anc = "the 00:00 UTC open" if fam == "DOF" else "yesterday's close"
        return (f"Fade {k} ATR from {anc} ({tf}m check), target {'the anchor' if tgt == 'anchor' else 'halfway'}, {S[sess]}",
                f"When a {tf}-min close is more than {k} daily ATR from {anc}, trade back toward it; stop {st} ATR beyond, "
                + (f"up to {hold} h." if hold else "to the end of the window."))
    if fam == "KZH":
        k, h0, span, vw, st, hold = p
        return (f"Keltner {k} ATR 1h fade from {h0:02d}:00 UTC for {span} h" + (", far side of VWAP" if vw == "against" else ""),
                f"Between {h0:02d}:00 and {(h0 + span) % 24:02d}:00 UTC fade a 1h close beyond EMA20 +/- {k} ATR; target the EMA, stop {st} ATR, up to {hold} h.")
    return fam, ""
