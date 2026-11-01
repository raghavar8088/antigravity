"""Strategy library for the automatic October search (rounds 4+).

12 parametric intraday families; every combination is one variant. All use
closed bars only, fill through sl_core with Delta fees, and are flat by 00:00
UTC. Sessions: all (UTC day), us (NYSE hours, weekdays), asia (00-08 UTC),
eu (London open + 6 h).
"""
import itertools

import numpy as np
import pandas as pd

from os_families2 import OCT0, OCT1, adx, atr_bars, rsi
from sl_core import DAY, H, london_open, us_open

SESS = ("all", "us", "asia", "eu")


def space():
    g = {}
    g["RSI"] = list(itertools.product((15, 30, 60, 120, 240), (7, 14, 21), (15, 20, 25, 30, 35), (40, 50, 60), (1.0, 1.5, 2.0, 3.0),
                                      (2, 4, 6, 12), ("none", "adx20", "adx25"), SESS))
    g["BBF"] = list(itertools.product((15, 30, 60, 240), (20, 30), (2.0, 2.5, 3.0), (1.0, 1.5, 2.0, 3.0), (2, 4, 8), SESS))
    g["BBB"] = list(itertools.product((15, 30, 60, 240), (20, 30), (2.0, 2.5), ("mid", "atr"), (1.5, 3.0, 0.0), ("none", "trend"), SESS))
    g["STO"] = list(itertools.product((15, 30, 60, 240), (14, 21), (10, 20), (1.0, 2.0), (2, 4, 8), SESS))
    g["DON"] = list(itertools.product((15, 30, 60, 240), (10, 20, 40), ("opp", "atr"), (2.0, 0.0), ("none", "trend"), SESS))
    g["ROC"] = list(itertools.product((15, 30, 60), (3, 6, 12), (1.0, 1.5, 2.0, 3.0), (1, 2, 4), ("mom", "rev"), ("none", "trend")))
    g["STK"] = list(itertools.product((15, 30, 60), (4, 5, 6, 7), (1, 2, 4), (1.0, 2.0), SESS))
    g["PIV"] = list(itertools.product(("classic", "camarilla", "fib", "woodie"), (1, 2, 3), ("fade", "break"), ("all", "us", "asia", "eu")))
    g["TOD"] = [(hh, d, rule, days) for hh in range(24) for d in (1, 2, 3, 4, 6) if hh + d <= 24
                for rule in ("long", "short", "vwap_follow", "vwap_fade", "day_follow", "day_fade", "short_below", "long_above")
                for days in ("all", "wkday", "wkend")]
    g["ORB"] = list(itertools.product(("asia0", "hk", "lon", "nypre", "nyse", "uscl"), (15, 30, 60), ("close", "stop", "retest"),
                                      ("far", "mid"), (1.5, 3.0, 0.0), ("none", "trend", "nr4")))
    g["CDL"] = list(itertools.product((60, 240), ("engulf", "pin", "inside"), ("none", "trend"), (1.5, 2.0, 3.0), (4, 8)))
    g["EMA"] = list(itertools.product((15, 60), (20, 50), (1.0, 2.0), (1.5, 3.0), SESS))
    return g


def vid(fam, p):
    return fam + "|" + "|".join(str(x) for x in p)


def window(cx, day, sess):
    if sess == "all":
        return day, day + DAY
    if sess == "us":
        return cx.window(day, "us")
    if sess == "asia":
        return day, day + 8 * H
    lo = london_open(day)
    return lo, lo + 6 * H


def bar_orders(cx, tf, longs, shorts, stop_l, stop_s, sess, hold_h, filt="none", tgt_l=None, tgt_s=None, target_r=None,
               exits_l=None, exits_s=None, entry="market", level_l=None, level_s=None, valid_bars=0):
    b = cx.bars(tf)
    tc = b["tc"].to_numpy()
    sig = np.asarray(longs) | np.asarray(shorts)
    out = []
    for i in np.nonzero(sig)[0]:
        t = int(tc[i])
        d = t // DAY * DAY
        if not (OCT0 <= d < OCT1):
            continue
        w = window(cx, d, sess)
        if w is None or not (w[0] <= t < w[1] - 300):
            continue
        side = 1 if longs[i] else -1
        if filt == "trend" and side != cx.trend(d):
            continue
        stop = stop_l[i] if side > 0 else stop_s[i]
        if stop is None or not np.isfinite(stop):
            continue
        exit_by = min(t + int(hold_h * H), int(w[1]))
        ex = exits_l if side > 0 else exits_s
        if ex is not None:
            k = np.searchsorted(ex, t, side="right")
            if k < len(ex):
                exit_by = min(exit_by, int(ex[k]))
        od = {"ts": t, "side": side, "entry": entry, "stop": float(stop), "exit_by": exit_by}
        if entry == "limit":
            od["level"] = float(level_l[i] if side > 0 else level_s[i])
            od["valid_until"] = t + valid_bars * tf * 60
        tg = (tgt_l[i] if side > 0 else tgt_s[i]) if tgt_l is not None else None
        if tg is not None and np.isfinite(tg):
            od["target"] = float(tg)
        elif target_r:
            od["target_r"] = float(target_r)
        out.append(od)
    return out


def f_rsi(cx, tf, n, lo, ex, st, hold, reg, sess):
    b = cx.bars(tf)
    r = rsi(b["c"], n)
    a = atr_bars(b, 14)
    ok = pd.Series(True, index=b.index) if reg == "none" else adx(b, 14) < (20 if reg == "adx20" else 25)
    c = b["c"]
    tc = b["tc"].to_numpy()
    return bar_orders(cx, tf, ((r < lo) & ok).to_numpy(), ((r > 100 - lo) & ok).to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(),
                      sess, hold, exits_l=tc[(r >= ex).to_numpy()], exits_s=tc[(r <= 100 - ex).to_numpy()])


def f_bbf(cx, tf, n, k, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    m, sd = c.rolling(n).mean(), c.rolling(n).std(ddof=0)
    return bar_orders(cx, tf, (c < m - k * sd).to_numpy(), (c > m + k * sd).to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(),
                      sess, hold, tgt_l=m.to_numpy(), tgt_s=m.to_numpy())


def f_bbb(cx, tf, n, k, stp, tr, filt, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    m, sd = c.rolling(n).mean(), c.rolling(n).std(ddof=0)
    sl = m if stp == "mid" else c - 2 * a
    ss = m if stp == "mid" else c + 2 * a
    return bar_orders(cx, tf, (c > m + k * sd).to_numpy(), (c < m - k * sd).to_numpy(), sl.to_numpy(), ss.to_numpy(), sess, 24, filt,
                      target_r=tr or None)


def f_sto(cx, tf, kp, lo, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    hh, ll = b["h"].rolling(kp).max(), b["l"].rolling(kp).min()
    k = 100 * (c - ll) / (hh - ll).replace(0, np.nan)
    tc = b["tc"].to_numpy()
    return bar_orders(cx, tf, (k < lo).to_numpy(), (k > 100 - lo).to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold,
                      exits_l=tc[(k >= 50).to_numpy()], exits_s=tc[(k <= 50).to_numpy()])


def f_don(cx, tf, n, stp, tr, filt, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    hh, ll = b["h"].rolling(n).max().shift(1), b["l"].rolling(n).min().shift(1)
    sl = ll if stp == "opp" else c - 1.5 * a
    ss = hh if stp == "opp" else c + 1.5 * a
    return bar_orders(cx, tf, (c > hh).to_numpy(), (c < ll).to_numpy(), sl.to_numpy(), ss.to_numpy(), sess, 24, filt, target_r=tr or None)


def f_roc(cx, tf, nb, thr, hold, mode, filt):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    mv = c - c.shift(nb)
    up, dn = (mv > thr * a).to_numpy(), (mv < -thr * a).to_numpy()
    longs, shorts = (up, dn) if mode == "mom" else (dn, up)
    return bar_orders(cx, tf, longs, shorts, (c - 1.5 * a).to_numpy(), (c + 1.5 * a).to_numpy(), "all", hold, filt)


def f_stk(cx, tf, n, hold, st, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    g = np.sign(b["c"] - b["o"])
    ups = g.rolling(n).sum() == n
    dns = g.rolling(n).sum() == -n
    return bar_orders(cx, tf, dns.to_numpy(), ups.to_numpy(), (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold)


def levels(kind, hi, lo, c):
    r = hi - lo
    if kind == "camarilla":
        R = [c + r * 1.1 / 12, c + r * 1.1 / 6, c + r * 1.1 / 4, c + r * 1.1 / 2]
        S = [c - r * 1.1 / 12, c - r * 1.1 / 6, c - r * 1.1 / 4, c - r * 1.1 / 2]
        return c, R, S
    p = (hi + lo + 2 * c) / 4 if kind == "woodie" else (hi + lo + c) / 3
    if kind == "fib":
        R = [p + 0.382 * r, p + 0.618 * r, p + 1.0 * r, p + 1.382 * r]
        S = [p - 0.382 * r, p - 0.618 * r, p - 1.0 * r, p - 1.382 * r]
    else:
        R = [2 * p - lo, p + r, hi + 2 * (p - lo), hi + 3 * (p - lo)]
        S = [2 * p - hi, p - r, lo - 2 * (hi - p), lo - 3 * (hi - p)]
    return p, R, S


def f_piv(cx, kind, lvl, mode, sess):
    mk, f = cx.mk, cx.f
    out = []
    for d in cx.days:
        k = cx.kday[d]
        w = window(cx, d, sess)
        if w is None or k < 1:
            continue
        hi, lo, c = (f.mk.daily[x].iat[k - 1] for x in ("h", "l", "c"))
        p, R, S = levels(kind, hi, lo, c)
        r_, s_ = R[lvl - 1], S[lvl - 1]
        i0, i1 = int(mk.idx(w[0])), int(mk.idx(w[1] - 3600))
        if i1 <= i0:
            continue
        if mode == "fade":
            hit_s, hit_b = mk.h[i0:i1] >= r_ * 1.0001, mk.l[i0:i1] <= s_ * 0.9999
        else:
            hit_s, hit_b = mk.l[i0:i1] <= s_, mk.h[i0:i1] >= r_
        if not (hit_s | hit_b).any():
            continue
        j = int(np.argmax(hit_s | hit_b))
        if hit_s[j] and hit_b[j]:
            continue
        inner_r, inner_s = (p, p) if lvl == 1 else (R[lvl - 2], S[lvl - 2])
        if mode == "fade":
            side = -1 if hit_s[j] else 1
            od = {"entry": "limit", "level": r_ if side < 0 else s_, "stop": R[lvl] if side < 0 else S[lvl],
                  "target": inner_r if side < 0 else inner_s}
        else:
            side = -1 if hit_s[j] else 1
            od = {"entry": "stop", "level": s_ if side < 0 else r_, "stop": inner_s if side < 0 else inner_r,
                  "target": S[lvl] if side < 0 else R[lvl]}
        od.update({"ts": int(w[0]), "side": side, "valid_until": int(w[1] - 3600), "exit_by": int(w[1])})
        out.append(od)
    return out


def f_tod(cx, hh, dur, rule, days):
    out = []
    for d in cx.days:
        wd = cx.f.weekday[cx.kday[d]]
        if (days == "wkday" and wd >= 5) or (days == "wkend" and wd < 5):
            continue
        ts = d + hh * H
        px = cx.last_close(ts) if hh > 0 else cx.mk.o[int(cx.mk.idx(ts))]
        vw = cx.vwap(np.array([ts]))[0] if hh > 0 else np.nan
        dop = cx.f.dopen[cx.kday[d]]
        if rule == "long":
            side = 1
        elif rule == "short":
            side = -1
        elif rule in ("vwap_follow", "vwap_fade", "short_below", "long_above"):
            if np.isnan(vw):
                continue
            s0 = int(np.sign(px - vw))
            side = {"vwap_follow": s0, "vwap_fade": -s0, "short_below": -1 if s0 < 0 else 0, "long_above": 1 if s0 > 0 else 0}[rule]
        else:
            if hh == 0:
                continue
            s0 = int(np.sign(px - dop))
            side = s0 if rule == "day_follow" else -s0
        if side == 0:
            continue
        out.append({"ts": int(ts), "side": side, "entry": "market", "stop": None, "exit_by": int(ts + dur * H)})
    return out


def orb_open(day, which):
    return {"asia0": day, "hk": day + H + 1800, "lon": london_open(day), "nypre": us_open(day) - H, "nyse": us_open(day),
            "uscl": us_open(day) + 6 * H + 1800}[which]


def f_orb(cx, which, rmin, ent, stp, tr, filt):
    mk, f = cx.mk, cx.f
    out = []
    for d in cx.days:
        k = cx.kday[d]
        if which in ("lon", "nypre", "nyse", "uscl") and f.weekday[k] >= 5:
            continue
        if filt == "nr4" and not f.nr4[k]:
            continue
        op = orb_open(d, which)
        end = min(op + 8 * H, d + DAY) if which in ("asia0", "hk", "lon") else (us_open(d) + 6 * H + 1800 if which != "uscl" else d + DAY)
        win = min(op + rmin * 60 + 3 * H, end - 1800)
        i0, i1, iw = (int(x) for x in mk.idx([op, op + rmin * 60, win]))
        if iw <= i1 or iw >= mk.n:
            continue
        rh, rl = mk.h[i0:i1].max(), mk.l[i0:i1].min()
        if rh <= rl:
            continue
        if ent == "stop":
            up, dn = mk.h[i1:iw] >= rh, mk.l[i1:iw] <= rl
            if not (up | dn).any():
                continue
            j = int(np.argmax(up | dn))
            if up[j] and dn[j]:
                continue
            side = 1 if up[j] else -1
            od = {"ts": int(op + rmin * 60), "entry": "stop", "level": rh if side > 0 else rl, "valid_until": int(win)}
        else:
            c = mk.c[i1:iw]
            hit = (c > rh) | (c < rl)
            if not hit.any():
                continue
            j = int(np.argmax(hit))
            side = 1 if c[j] > rh else -1
            ts = int(mk.t[i1 + j]) + 60
            od = ({"ts": ts, "entry": "market"} if ent == "close" else
                  {"ts": ts, "entry": "limit", "level": rh if side > 0 else rl, "valid_until": int(win)})
        if filt == "trend" and side != cx.trend(d):
            continue
        od.update({"side": side, "stop": (rl if side > 0 else rh) if stp == "far" else (rh + rl) / 2, "exit_by": int(end)})
        if tr:
            od["target_r"] = float(tr)
        out.append(od)
    return out


def f_cdl(cx, tf, pat, filt, tr, hold):
    b = cx.bars(tf)
    o, h, l, c = (b[x].to_numpy() for x in ("o", "h", "l", "c"))
    n = len(b)
    longs, shorts = np.zeros(n, bool), np.zeros(n, bool)
    sl_, ss_ = np.full(n, np.nan), np.full(n, np.nan)
    entry, lvl_l, lvl_s = "market", np.full(n, np.nan), np.full(n, np.nan)
    for i in range(1, n):
        body, rng = abs(c[i] - o[i]), h[i] - l[i]
        if rng <= 0:
            continue
        if pat == "engulf":
            if c[i] > o[i] and c[i - 1] < o[i - 1] and c[i] >= o[i - 1] and o[i] <= c[i - 1]:
                longs[i], sl_[i] = True, l[i]
            elif c[i] < o[i] and c[i - 1] > o[i - 1] and c[i] <= o[i - 1] and o[i] >= c[i - 1]:
                shorts[i], ss_[i] = True, h[i]
        elif pat == "pin":
            lw, uw = min(o[i], c[i]) - l[i], h[i] - max(o[i], c[i])
            if lw >= 2 * body and c[i] >= l[i] + 0.66 * rng:
                longs[i], sl_[i] = True, l[i]
            elif uw >= 2 * body and c[i] <= l[i] + 0.34 * rng:
                shorts[i], ss_[i] = True, h[i]
        else:
            if h[i] < h[i - 1] and l[i] > l[i - 1]:
                longs[i] = shorts[i] = False
                lvl_l[i], lvl_s[i] = h[i], l[i]
    if pat == "inside":
        # after an inside bar, the first break of its high or low (within one bar) is the trade
        mk, out = cx.mk, []
        tcs = b["tc"].to_numpy()
        for i in np.nonzero(~np.isnan(lvl_l))[0]:
            t = int(tcs[i])
            d = t // DAY * DAY
            if not (OCT0 <= d < OCT1):
                continue
            hi_, lo_ = lvl_l[i], lvl_s[i]
            a0, a1 = int(mk.idx(t)), int(mk.idx(min(t + tf * 60, d + DAY - 60)))
            if a1 <= a0:
                continue
            up, dn = mk.h[a0:a1] >= hi_, mk.l[a0:a1] <= lo_
            if not (up | dn).any():
                continue
            j = int(np.argmax(up | dn))
            if up[j] and dn[j]:
                continue
            side = 1 if up[j] else -1
            if filt == "trend" and side != cx.trend(d):
                continue
            out.append({"ts": t, "side": side, "entry": "stop", "level": float(hi_ if side > 0 else lo_), "valid_until": t + tf * 60,
                        "stop": float(lo_ if side > 0 else hi_), "target_r": float(tr), "exit_by": int(min(t + hold * H, d + DAY))})
        return out
    return bar_orders(cx, tf, longs, shorts, sl_, ss_, "all", hold, filt, target_r=tr)


def f_ema(cx, tf, n, st, tr, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    e = c.ewm(span=n, adjust=False).mean()
    rising, falling = (e > e.shift(5)) & (c > e), (e < e.shift(5)) & (c < e)
    return bar_orders(cx, tf, rising.to_numpy(), falling.to_numpy(), (e - st * a).to_numpy(), (e + st * a).to_numpy(), sess, 12,
                      target_r=tr, entry="limit", level_l=e.to_numpy(), level_s=e.to_numpy(), valid_bars=4)


FN = {"RSI": f_rsi, "BBF": f_bbf, "BBB": f_bbb, "STO": f_sto, "DON": f_don, "ROC": f_roc, "STK": f_stk, "PIV": f_piv,
      "TOD": f_tod, "ORB": f_orb, "CDL": f_cdl, "EMA": f_ema}
NAMES = {"RSI": "RSI fade", "BBF": "Bollinger fade", "BBB": "Bollinger breakout", "STO": "Stochastic extreme", "DON": "Donchian breakout",
         "ROC": "Sharp-move momentum/reversal", "STK": "Candle-streak fade", "PIV": "Pivot levels", "TOD": "Hour-of-day",
         "ORB": "Session-open breakout", "CDL": "Candle pattern", "EMA": "EMA pullback"}


def orders(cx, fam, p):
    return FN[fam](cx, *p)


def describe(fam, p):
    S = {"all": "all day", "us": "US hours", "asia": "Asian hours", "eu": "European hours"}
    if fam == "RSI":
        tf, n, lo, ex, st, hold, reg, sess = p
        return (f"RSI({n}) {tf}m fade <{lo}/>{100 - lo}, exit {ex}, {S[sess]}" + ("" if reg == "none" else f", ADX<{reg[3:]}"),
                f"On {tf}-min bars buy when RSI({n}) closes under {lo} (short over {100 - lo}); exit when it crosses {ex}, after {hold} h or at a {st} ATR stop.")
    if fam == "BBF":
        tf, n, k, st, hold, sess = p
        return (f"Bollinger({n},{k}) {tf}m fade, {S[sess]}", f"Buy a {tf}-min close under the lower band (short over the upper), target the middle band, stop {st} ATR, up to {hold} h.")
    if fam == "BBB":
        tf, n, k, stp, tr, filt, sess = p
        return (f"Bollinger({n},{k}) {tf}m breakout, {S[sess]}" + (", with trend" if filt == "trend" else ""),
                f"Follow a {tf}-min close outside the band; stop at {'the middle band' if stp == 'mid' else '2 ATR'}" + (f", target {tr}R." if tr else ", hold to the end of the day."))
    if fam == "STO":
        tf, kp, lo, st, hold, sess = p
        return (f"Stochastic({kp}) {tf}m <{lo}/>{100 - lo}, {S[sess]}", f"Buy when %K is under {lo} (short over {100 - lo}); exit at 50, after {hold} h or at a {st} ATR stop.")
    if fam == "DON":
        tf, n, stp, tr, filt, sess = p
        return (f"Donchian({n}) {tf}m breakout, {S[sess]}" + (", with trend" if filt == "trend" else ""),
                f"Follow a {tf}-min close beyond the last {n} bars' high or low; stop at {'the opposite side' if stp == 'opp' else '1.5 ATR'}" + (f", target {tr}R." if tr else "."))
    if fam == "ROC":
        tf, nb, thr, hold, mode, filt = p
        return (f"{'Follow' if mode == 'mom' else 'Fade'} a {thr} ATR move over {nb} x {tf}m" + (", with trend" if filt == "trend" else ""),
                f"When price moves more than {thr} ATR in {nb} {tf}-min bars, {'follow' if mode == 'mom' else 'fade'} it for {hold} h; stop 1.5 ATR.")
    if fam == "STK":
        tf, n, hold, st, sess = p
        return (f"Fade {n} same-colour {tf}m candles, {S[sess]}", f"After {n} rising (falling) {tf}-min candles in a row, trade the other way for {hold} h; stop {st} ATR.")
    if fam == "PIV":
        kind, lvl, mode, sess = p
        return (f"{kind.title()} pivot {mode} at level {lvl}, {S[sess]}", f"{'Rest limit orders at' if mode == 'fade' else 'Rest stop orders at'} yesterday's {kind} R{lvl}/S{lvl}; first touch is the trade.")
    if fam == "TOD":
        hh, d, rule, days = p
        what = {"long": "buy", "short": "sell", "vwap_follow": "trade with price vs VWAP", "vwap_fade": "trade against price vs VWAP",
                "day_follow": "trade with the day's move", "day_fade": "trade against the day's move", "short_below": "short if below VWAP",
                "long_above": "buy if above VWAP"}[rule]
        dd = {"all": "every day", "wkday": "weekdays", "wkend": "weekends"}[days]
        return (f"{hh:02d}:00 UTC for {d} h, {what} ({dd})", f"{dd.capitalize()} at {hh:02d}:00 UTC {what}; close {d} h later.")
    if fam == "ORB":
        which, rmin, ent, stp, tr, filt = p
        nm = {"asia0": "Asia (00:00 UTC)", "hk": "Hong Kong (01:30 UTC)", "lon": "London", "nypre": "US pre-market", "nyse": "NYSE", "uscl": "after US close"}[which]
        return (f"{nm} {rmin}-min breakout, {ent} entry, {'no target' if not tr else f'{tr}R target'}" + ("" if filt == "none" else f", {filt}"),
                f"Range = first {rmin} min from the {nm} open; trade the first break ({ent} entry); stop at the {'far side' if stp == 'far' else 'middle'}.")
    if fam == "CDL":
        tf, pat, filt, tr, hold = p
        return (f"{pat.title()} {tf}m, {tr}R target" + (", with trend" if filt == "trend" else ""), f"Trade a {tf}-min {pat} candle; stop at its extreme, target {tr}R, up to {hold} h.")
    if fam == "EMA":
        tf, n, st, tr, sess = p
        return (f"EMA{n} pullback {tf}m, {tr}R target, {S[sess]}", f"In a rising (falling) EMA{n} on {tf}-min bars, rest a limit at the EMA for 4 bars; stop {st} ATR, target {tr}R.")
    return vid(fam, p), ""
