"""Library v3 for the automatic October search.

DOW  day-of-week x hour windows (long or short), every weekday separately
XS   cross-coin intraday ranking: at hour h rank the coins by their return over
     a look-back; long the strongest k and short the weakest k (or the reverse)
     for a fixed hold. Ranked within whichever coin set is being tested.
ATX  range-expansion bar (range over m x ATR): follow or fade
ICH  Ichimoku Tenkan/Kijun cross, optionally only on the cloud's side
SAR  Parabolic SAR flips
DMI  +DI/-DI cross with an ADX floor
CMB  RSI(21) 1h fade restricted to an hour window and a VWAP side
"""
import itertools

import numpy as np
import pandas as pd

from os_families2 import OCT0, OCT1, adx, atr_bars, rsi
from os_library import SESS, bar_orders, window
from sl_core import DAY, H


def space3():
    g = {}
    g["DOW"] = [(wd, hh, d, side) for wd in range(7) for hh in range(0, 24, 2) for d in (2, 4, 6) if hh + d <= 24 for side in ("long", "short")]
    g["XS"] = list(itertools.product(range(2, 21, 2), ("day", "4h", "1h"), ("mom", "rev"), (1, 2), (2, 4, 8, 0)))
    g["ATX"] = list(itertools.product((15, 60), (1.5, 2.0, 2.5, 3.0), ("follow", "fade"), (1, 2, 4), ("none", "trend")))
    g["ICH"] = list(itertools.product((15, 60), ("any", "cloud"), ("all", "us", "eu", "asia"), ("none", "trend")))
    g["SAR"] = list(itertools.product((15, 60), (0.02, 0.01), ("all", "us", "eu", "asia"), ("none", "trend")))
    g["DMI"] = list(itertools.product((15, 60), (14,), (15, 20, 25), ("all", "us", "eu", "asia"), ("none", "trend")))
    g["CMB"] = list(itertools.product((0, 6, 12, 18), (6, 12), ("any", "with", "against"), (25, 30), (50, 60)))
    return g


def f_dow(cx, wd, hh, dur, side):
    s = 1 if side == "long" else -1
    return [{"ts": int(d + hh * H), "side": s, "entry": "market", "stop": None, "exit_by": int(d + (hh + dur) * H)}
            for d in cx.days if cx.f.weekday[cx.kday[d]] == wd]


def xs_orders(ctxs, hh, basis, mode, k, hold):
    """{sym: orders} for a coin set; ranks are taken within that set."""
    syms = list(ctxs)
    out = {s: [] for s in syms}
    days = ctxs[syms[0]].days
    look = {"day": None, "4h": 4 * H, "1h": H}[basis]
    for d in days:
        ts = d + hh * H
        rets = {}
        for s in syms:
            mk = ctxs[s].mk
            if d in mk.bad_days:
                break
            i = int(mk.idx(ts)) - 1
            ref = mk.o[int(mk.idx(d))] if look is None else mk.c[int(mk.idx(ts - look)) - 1]
            rets[s] = mk.c[i] / ref - 1
        if len(rets) < len(syms) or len(syms) < 2 * k + 1:
            continue
        order = sorted(syms, key=lambda s: -rets[s])
        top, bot = order[:k], order[-k:]
        longs, shorts = (top, bot) if mode == "mom" else (bot, top)
        end = d + DAY if hold == 0 else min(ts + hold * H, d + DAY)
        for s in longs:
            out[s].append({"ts": int(ts), "side": 1, "entry": "market", "stop": None, "exit_by": int(end)})
        for s in shorts:
            out[s].append({"ts": int(ts), "side": -1, "entry": "market", "stop": None, "exit_by": int(end)})
    return out


def f_atx(cx, tf, m, mode, hold, filt):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14).shift(1)
    big = ((b["h"] - b["l"]) > m * a)
    up, dn = (big & (b["c"] > b["o"])).to_numpy(), (big & (b["c"] < b["o"])).to_numpy()
    longs, shorts = (up, dn) if mode == "follow" else (dn, up)
    lo_, hi_ = b["l"].to_numpy(), b["h"].to_numpy()
    sl = lo_ if mode == "follow" else (c - 1.5 * a).to_numpy()
    ss = hi_ if mode == "follow" else (c + 1.5 * a).to_numpy()
    return bar_orders(cx, tf, longs, shorts, sl, ss, "all", hold, filt)


def segments_from_side(cx, tf, side, sess, filt):
    """Always-in-the-window positions from a per-bar side (+1/-1/0), flat outside the window."""
    b = cx.bars(tf)
    tc = b["tc"].to_numpy()
    out = []
    for d in cx.days:
        w = window(cx, d, sess)
        if w is None:
            continue
        m = (tc >= w[0]) & (tc < w[1] - 60)
        cur = 0
        tr = cx.trend(d)
        for t, s in zip(tc[m], side[m]):
            s = int(s)
            if filt == "trend" and s != tr:
                s = 0
            if s != cur:
                if out and out[-1]["exit_by"] is None:
                    out[-1]["exit_by"] = int(t)
                if s != 0:
                    out.append({"ts": int(t), "side": s, "entry": "market", "stop": None, "exit_by": None})
                cur = s
        if out and out[-1]["exit_by"] is None:
            out[-1]["exit_by"] = int(w[1])
    return out


def f_ich(cx, tf, mode, sess, filt):
    b = cx.bars(tf)
    h, l, c = b["h"], b["l"], b["c"]
    ten = (h.rolling(9).max() + l.rolling(9).min()) / 2
    kij = (h.rolling(26).max() + l.rolling(26).min()) / 2
    spa = ((ten + kij) / 2).shift(26)
    spb = ((h.rolling(52).max() + l.rolling(52).min()) / 2).shift(26)
    side = np.sign(ten - kij).fillna(0).to_numpy()
    if mode == "cloud":
        top, bot = np.maximum(spa, spb).to_numpy(), np.minimum(spa, spb).to_numpy()
        cv = c.to_numpy()
        side = np.where((side > 0) & (cv > top), 1, np.where((side < 0) & (cv < bot), -1, 0))
    return segments_from_side(cx, tf, side, sess, filt)


def psar(b, step):
    h, l = b["h"].to_numpy(), b["l"].to_numpy()
    n = len(h)
    d = np.zeros(n)
    up, sar, ep, af = True, l[0], h[0], step
    for i in range(1, n):
        sar = sar + af * (ep - sar)
        if up:
            sar = min(sar, l[i - 1], l[i - 2] if i > 1 else l[i - 1])
            if l[i] < sar:
                up, sar, ep, af = False, ep, l[i], step
            elif h[i] > ep:
                ep, af = h[i], min(af + step, 0.2)
        else:
            sar = max(sar, h[i - 1], h[i - 2] if i > 1 else h[i - 1])
            if h[i] > sar:
                up, sar, ep, af = True, ep, h[i], step
            elif l[i] < ep:
                ep, af = l[i], min(af + step, 0.2)
        d[i] = 1 if up else -1
    return d


def f_sar(cx, tf, step, sess, filt):
    return segments_from_side(cx, tf, psar(cx.bars(tf), step), sess, filt)


def f_dmi(cx, tf, n, floor, sess, filt):
    b = cx.bars(tf)
    up, dn = b["h"].diff(), -b["l"].diff()
    pdm = pd.Series(np.where((up > dn) & (up > 0), up, 0.0))
    mdm = pd.Series(np.where((dn > up) & (dn > 0), dn, 0.0))
    pc = b["c"].shift(1)
    tr = np.maximum(b["h"] - b["l"], np.maximum((b["h"] - pc).abs(), (b["l"] - pc).abs()))
    a = tr.ewm(alpha=1 / n, adjust=False).mean()
    pdi = 100 * pdm.ewm(alpha=1 / n, adjust=False).mean() / a
    mdi = 100 * mdm.ewm(alpha=1 / n, adjust=False).mean() / a
    ax = adx(b, n)
    side = np.where(ax > floor, np.sign(pdi - mdi), 0)
    return segments_from_side(cx, tf, np.nan_to_num(side), sess, filt)


def f_cmb(cx, h0, span, vw, lo, ex):
    b = cx.bars(60)
    r = rsi(b["c"], 21)
    a = atr_bars(b, 14)
    c = b["c"]
    tc = b["tc"].to_numpy()
    hour = (tc % DAY) // H
    inwin = ((hour - h0) % 24) < span
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


FN3 = {"DOW": f_dow, "ATX": f_atx, "ICH": f_ich, "SAR": f_sar, "DMI": f_dmi, "CMB": f_cmb}
NAMES3 = {"DOW": "Day-of-week window", "XS": "Cross-coin intraday", "ATX": "Range-expansion bar", "ICH": "Ichimoku",
          "SAR": "Parabolic SAR", "DMI": "DMI cross", "CMB": "RSI fade by hour and VWAP"}
DAYN = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"]


def describe3(fam, p):
    S = {"all": "all day", "us": "US hours", "asia": "Asian hours", "eu": "European hours"}
    if fam == "DOW":
        wd, hh, d, side = p
        return (f"{DAYN[wd]} {hh:02d}:00 UTC for {d} h, {side}", f"Every {DAYN[wd]} {'buy' if side == 'long' else 'sell'} at {hh:02d}:00 UTC and close {d} h later.")
    if fam == "XS":
        hh, basis, mode, k, hold = p
        lb = {"day": "since 00:00 UTC", "4h": "over the last 4 h", "1h": "over the last hour"}[basis]
        return (f"Cross-coin {'momentum' if mode == 'mom' else 'reversal'} at {hh:02d}:00 UTC, {k} each side, " + (f"{hold} h" if hold else "to 00:00"),
                f"At {hh:02d}:00 UTC rank the coins by return {lb}; {'buy the strongest' if mode == 'mom' else 'buy the weakest'} {k} and "
                f"{'sell the weakest' if mode == 'mom' else 'sell the strongest'} {k}; close " + (f"{hold} h later." if hold else "at 00:00 UTC."))
    if fam == "ATX":
        tf, m, mode, hold, filt = p
        return (f"{mode.title()} a {m} ATR {tf}m bar" + (", with trend" if filt == "trend" else ""), f"{'Follow' if mode == 'follow' else 'Fade'} a {tf}-min bar whose range is over {m} ATR; hold {hold} h.")
    if fam == "ICH":
        tf, mode, sess, filt = p
        return (f"Ichimoku TK cross {tf}m" + (" beyond the cloud" if mode == "cloud" else "") + f", {S[sess]}" + (", with trend" if filt == "trend" else ""),
                "Long while Tenkan is above Kijun, short while below" + (" and price is beyond the cloud." if mode == "cloud" else "."))
    if fam == "SAR":
        tf, step, sess, filt = p
        return (f"Parabolic SAR({step}) {tf}m, {S[sess]}" + (", with trend" if filt == "trend" else ""), "Follow the Parabolic SAR side; flip on each SAR reversal.")
    if fam == "DMI":
        tf, n, floor, sess, filt = p
        return (f"DMI({n}) cross {tf}m, ADX>{floor}, {S[sess]}" + (", with trend" if filt == "trend" else ""), f"Long while +DI is above -DI and ADX is over {floor}, short the mirror.")
    if fam == "CMB":
        h0, span, vw, lo, ex = p
        return (f"RSI(21) 1h fade {lo}/{100 - lo} from {h0:02d}:00 UTC for {span} h, VWAP {vw}, exit {ex}",
                f"Between {h0:02d}:00 and {(h0 + span) % 24:02d}:00 UTC, fade 1h RSI(21) under {lo} / over {100 - lo}" +
                ("" if vw == "any" else f" only {'with' if vw == 'with' else 'against'} price's side of VWAP") + f"; exit at RSI {ex}, 6 h or a 2 ATR stop.")
    return fam, ""
