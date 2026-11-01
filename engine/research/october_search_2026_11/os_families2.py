"""Round 3 families for the October search: 332 more intraday variants.

All use closed bars only, fill through sl_core (market / stop / limit, Delta
fees), and are flat by 00:00 UTC. 'US hours' = the NYSE session, weekdays.

N1  EMA crossover (5m/15m/1h; 9/21, 12/26, 20/50), always in within the window
N2  Supertrend (15m/1h; ATR 10x2, 10x3, 14x3)
N3  MACD 12/26/9 signal cross (15m/1h), optional zero-line agreement
N4  VWAP pullback in an intraday trend: limit order at VWAP (maker)
N5  VWAP extreme fade at 2/2.5/3 standard deviations, target VWAP
N6  Previous-day high/low breakout
N7  CPR / floor pivots: narrow-CPR breakout; R1/S1 and R2/S2 limit fades
N8  1h high-volume, full-body momentum candle
N9  1h RSI(14) extremes back to 50
N10 US pre-market range breakout (range = 08:00-08:30 New York, trade from 08:30)
N11 Conditional US-afternoon trades (side from the day's move or VWAP at 13-16 UTC)
N12 Heikin-Ashi trend with EMA(50) agreement (15m/1h)
Every variant takes either no trend filter or 'trend' (only with the sign of the
10-day move), where listed.
"""
import itertools

import numpy as np
import pandas as pd

from sl_core import DAY, H, us_open

OCT0 = int(pd.Timestamp("2026-10-01", tz="UTC").timestamp())
OCT1 = int(pd.Timestamp("2026-11-01", tz="UTC").timestamp())

GRID = []
GRID += [("N1", tf, f, s, sess, filt) for tf in (5, 15, 60) for f, s in ((9, 21), (12, 26), (20, 50)) for sess in ("all", "us") for filt in ("none", "trend")]
GRID += [("N2", tf, n, m, sess, filt) for tf in (15, 60) for n, m in ((10, 2.0), (10, 3.0), (14, 3.0)) for sess in ("all", "us") for filt in ("none", "trend")]
GRID += [("N3", tf, zero, sess, filt) for tf in (15, 60) for zero in ("any", "zero") for sess in ("all", "us") for filt in ("none", "trend")]
GRID += [("N4", x, tgt, filt, sess) for x in (0.5, 1.0) for tgt in ("1R", "2R", "eod") for filt in ("none", "trend") for sess in ("all", "us")]
GRID += [("N5", k, sess, filt) for k in (2.0, 2.5, 3.0) for sess in ("all", "us") for filt in ("none", "counter")]
GRID += [("N6", ent, stp, tgt, filt) for ent in ("close", "stop") for stp in ("pdc", "open", "atr") for tgt in ("1.5R", "3R", "eod") for filt in ("none", "trend")]
GRID += [("N7b", width, stp, tgt, filt) for width in ("narrow", "abs", "any") for stp in ("pivot", "cpr") for tgt in ("r1", "2R") for filt in ("none", "trend")]
GRID += [("N7f", lvl, filt, sess) for lvl in (1, 2) for filt in ("none", "counter") for sess in ("all", "us")]
GRID += [("N8", k, hold, filt) for k in (1.5, 2.0, 3.0) for hold in (1, 2, 4) for filt in ("none", "trend")]
GRID += [("N9", thr, reg) for thr in (20, 25, 30) for reg in ("none", "adx")]
GRID += [("N10", ent, stp, tgt, filt) for ent in ("close", "stop", "retest") for stp in ("far", "mid") for tgt in ("1.5R", "3R", "eod") for filt in ("none", "trend", "nr4", "tight")]
GRID += [("N11", rule, hh, d) for rule in ("day", "vwap", "short_below", "long_above") for hh in (13, 14, 15, 16) for d in (2, 3, 4)]
GRID += [("N12", tf, sess, filt) for tf in (15, 60) for sess in ("all", "us") for filt in ("none", "trend")]


def vid(g):
    return "|".join(str(x) for x in g)


# ── shared helpers ───────────────────────────────────────────────────────────
class Ctx:
    """Per-coin caches: bars, VWAP, daily levels, sessions."""

    def __init__(self, mk, feats):
        self.mk, self.f = mk, feats
        tp = (mk.h + mk.l + mk.c) / 3
        self.cpv = np.concatenate([[0.0], np.cumsum(tp * mk.v)])
        self.cv = np.concatenate([[0.0], np.cumsum(mk.v)])
        self._bars = {}
        self.days = [int(d) for d in feats.days if OCT0 <= d < OCT1]
        self.kday = {int(d): k for k, d in enumerate(feats.days)}

    def bars(self, k):
        if k not in self._bars:
            mk = self.mk
            n = mk.n // k * k
            b = pd.DataFrame({"t": mk.t[:n:k], "o": mk.o[:n:k], "h": mk.h[:n].reshape(-1, k).max(1),
                              "l": mk.l[:n].reshape(-1, k).min(1), "c": mk.c[k - 1:n:k], "v": mk.v[:n].reshape(-1, k).sum(1)})
            b["tc"] = b["t"] + 60 * k
            self._bars[k] = b
        return self._bars[k]

    def vwap(self, ts):
        ts = np.asarray(ts, dtype=np.int64)
        i, d = self.mk.idx(ts), self.mk.idx(ts // DAY * DAY)
        vol = self.cv[i] - self.cv[d]
        with np.errstate(invalid="ignore", divide="ignore"):
            return np.where(vol > 0, (self.cpv[i] - self.cpv[d]) / vol, np.nan)

    def window(self, day, sess):
        """(start, end) for 'all' (the UTC day) or 'us' (NYSE session, weekdays; None at weekends)."""
        if sess == "all":
            return day, day + DAY
        if self.f.weekday[self.kday[day]] >= 5:
            return None
        op = us_open(day)
        return op, op + 6 * H + 1800

    def trend(self, day):
        return self.f.trend[self.kday[day]]

    def last_close(self, ts):
        return self.mk.c[int(self.mk.idx(ts)) - 1]


def atr_bars(b, n=14):
    pc = b["c"].shift(1)
    tr = np.maximum(b["h"] - b["l"], np.maximum((b["h"] - pc).abs(), (b["l"] - pc).abs()))
    return tr.rolling(n).mean()


def segments(cx, sides_by_time, day, sess, filt):
    """Always-in-the-window orders from a desired side at each bar close."""
    w = cx.window(day, sess)
    if w is None:
        return []
    start, end = w
    tr = cx.trend(day)
    pts = [(t, s) for t, s in sides_by_time if start <= t < end - 60]
    out, cur = [], 0
    for t, s in pts:
        if filt == "trend" and s != tr:
            s = 0
        if s != cur:
            if out and out[-1]["exit_by"] is None:
                out[-1]["exit_by"] = t
            if s != 0:
                out.append({"ts": int(t), "side": int(s), "entry": "market", "stop": None, "exit_by": None})
            cur = s
    if out and out[-1]["exit_by"] is None:
        out[-1]["exit_by"] = int(end)
    return out


def bar_sides(cx, k, series_side):
    b = cx.bars(k)
    return list(zip(b["tc"].to_numpy().tolist(), series_side.tolist()))


def per_day(cx, sides, sess, filt):
    out = []
    by_day = {}
    for t, s in sides:
        d = int(t - 1) // DAY * DAY
        if OCT0 <= d < OCT1:
            by_day.setdefault(d, []).append((t, s))
    for d in cx.days:
        out += segments(cx, by_day.get(d, []), d, sess, filt)
    return out


# ── families ─────────────────────────────────────────────────────────────────
def n1(cx, tf, f, s, sess, filt):
    c = cx.bars(tf)["c"]
    side = np.sign(c.ewm(span=f, adjust=False).mean() - c.ewm(span=s, adjust=False).mean()).fillna(0)
    return per_day(cx, bar_sides(cx, tf, side), sess, filt)


def supertrend_dir(b, n, m):
    a = atr_bars(b, n).to_numpy()
    hl2 = ((b["h"] + b["l"]) / 2).to_numpy()
    c = b["c"].to_numpy()
    up, dn = hl2 - m * a, hl2 + m * a
    d = np.zeros(len(c))
    fu, fd = np.nan, np.nan
    cur = 1
    for i in range(len(c)):
        if np.isnan(a[i]):
            continue
        fu = up[i] if np.isnan(fu) or c[i - 1] < fu else max(up[i], fu)
        fd = dn[i] if np.isnan(fd) or c[i - 1] > fd else min(dn[i], fd)
        if cur == 1 and c[i] < fu:
            cur = -1
        elif cur == -1 and c[i] > fd:
            cur = 1
        d[i] = cur
    return pd.Series(d)


def n2(cx, tf, n, m, sess, filt):
    return per_day(cx, bar_sides(cx, tf, supertrend_dir(cx.bars(tf), n, m)), sess, filt)


def n3(cx, tf, zero, sess, filt):
    c = cx.bars(tf)["c"]
    macd = c.ewm(span=12, adjust=False).mean() - c.ewm(span=26, adjust=False).mean()
    sig = macd.ewm(span=9, adjust=False).mean()
    side = np.sign(macd - sig)
    if zero == "zero":
        side = side.where(np.sign(macd) == side, 0)
    return per_day(cx, bar_sides(cx, tf, side.fillna(0)), sess, filt)


def n4(cx, x, tgt, filt, sess):
    b = cx.bars(15)
    a = atr_bars(b, 14).to_numpy()
    tc = b["tc"].to_numpy()
    vw = cx.vwap(tc)
    c = b["c"].to_numpy()
    out = []
    for i in range(2, len(b)):
        d = int(tc[i] - 1) // DAY * DAY
        if not (OCT0 <= d < OCT1) or np.isnan(vw[i]) or np.isnan(a[i]):
            continue
        w = cx.window(d, sess)
        if w is None or not (w[0] + 3600 <= tc[i] < w[1] - 3600):
            continue
        above = all(c[j] > vw[j] for j in (i - 2, i - 1, i))
        below = all(c[j] < vw[j] for j in (i - 2, i - 1, i))
        side = 1 if above else (-1 if below else 0)
        if side == 0 or (filt == "trend" and side != cx.trend(d)):
            continue
        lvl = vw[i]
        od = {"ts": int(tc[i]), "side": side, "entry": "limit", "level": lvl, "valid_until": int(tc[i] + 3600),
              "stop": lvl - side * x * a[i], "exit_by": int(w[1])}
        if tgt != "eod":
            od["target_r"] = float(tgt[0])
        out.append(od)
    return out


def n5(cx, k, sess, filt):
    mk = cx.mk
    b = cx.bars(15)
    tc = b["tc"].to_numpy()
    out = []
    for d in cx.days:
        w = cx.window(d, sess)
        if w is None:
            continue
        i0 = int(mk.idx(d))
        for t in tc[(tc >= w[0] + 3600) & (tc < w[1] - 1800)]:
            i = int(mk.idx(t))
            seg = slice(i0, i)
            vol = cx.cv[i] - cx.cv[i0]
            if vol <= 0:
                continue
            vw = (cx.cpv[i] - cx.cpv[i0]) / vol
            dev = mk.c[seg] - vw
            sd = dev.std()
            if not sd > 0:
                continue
            z = (mk.c[i - 1] - vw) / sd
            side = -1 if z > k else (1 if z < -k else 0)
            if side == 0:
                continue
            if filt == "counter" and side != cx.trend(d):
                continue  # fade only stretches against the 10-day trend, so the trade goes with it
            out.append({"ts": int(t), "side": side, "entry": "market", "target": vw, "stop": mk.c[i - 1] - side * sd,
                        "exit_by": int(min(t + 2 * H, w[1]))})
    return out


def n6(cx, ent, stp, tgt, filt):
    mk, f = cx.mk, cx.f
    out = []
    for d in cx.days:
        k = cx.kday[d]
        if k < 1:
            continue
        pdh, pdl, pdc = f.mk.daily["h"].iat[k - 1], f.mk.daily["l"].iat[k - 1], f.mk.daily["c"].iat[k - 1]
        i0, i1 = int(mk.idx(d)), int(mk.idx(d + 18 * H))
        if ent == "close":
            c = mk.c[i0:i1]
            hit = (c > pdh) | (c < pdl)
            if not hit.any():
                continue
            j = int(np.argmax(hit))
            side = 1 if c[j] > pdh else -1
            od = {"ts": int(mk.t[i0 + j]) + 60, "entry": "market"}
        else:
            up, dn = mk.h[i0:i1] >= pdh, mk.l[i0:i1] <= pdl
            if not (up | dn).any():
                continue
            j = int(np.argmax(up | dn))
            if up[j] and dn[j]:
                continue
            side = 1 if up[j] else -1
            od = {"ts": d, "entry": "stop", "level": pdh if side > 0 else pdl, "valid_until": d + 18 * H}
        if filt == "trend" and side != cx.trend(d):
            continue
        lvl = pdh if side > 0 else pdl
        if stp == "pdc":
            stop = pdc
        elif stp == "open":
            stop = f.dopen[k]
        else:
            if np.isnan(f.atr[k]):
                continue
            stop = lvl - side * f.atr[k]
        od.update({"side": side, "stop": stop, "exit_by": d + DAY})
        if tgt != "eod":
            od["target_r"] = float(tgt[:-1])
        out.append(od)
    return out


def pivots(f, k):
    hi, lo, c = f.mk.daily["h"].iat[k - 1], f.mk.daily["l"].iat[k - 1], f.mk.daily["c"].iat[k - 1]
    p = (hi + lo + c) / 3
    bc = (hi + lo) / 2
    tc = 2 * p - bc
    tc, bc = max(tc, bc), min(tc, bc)
    return {"P": p, "TC": tc, "BC": bc, "R1": 2 * p - lo, "S1": 2 * p - hi, "R2": p + (hi - lo), "S2": p - (hi - lo),
            "R3": hi + 2 * (p - lo), "S3": lo - 2 * (hi - p), "width": (tc - bc) / p}


def n7b(cx, width, stp, tgt, filt):
    mk, f = cx.mk, cx.f
    widths = {}
    for k in range(1, len(f.days)):
        widths[int(f.days[k])] = pivots(f, k)["width"]
    out = []
    for d in cx.days:
        k = cx.kday[d]
        pv = pivots(f, k)
        past = [widths[int(x)] for x in f.days[max(1, k - 20):k]]
        if width == "narrow" and not (len(past) >= 10 and pv["width"] < np.median(past)):
            continue
        if width == "abs" and not pv["width"] < 0.002:
            continue
        i0, i1 = int(mk.idx(d)), int(mk.idx(d + 16 * H))
        c = mk.c[i0:i1]
        hit = (c > pv["TC"]) | (c < pv["BC"])
        if not hit.any():
            continue
        j = int(np.argmax(hit))
        side = 1 if c[j] > pv["TC"] else -1
        if filt == "trend" and side != cx.trend(d):
            continue
        stop = pv["P"] if stp == "pivot" else (pv["BC"] if side > 0 else pv["TC"])
        od = {"ts": int(mk.t[i0 + j]) + 60, "side": side, "entry": "market", "stop": stop, "exit_by": d + DAY}
        if tgt == "r1":
            od["target"] = pv["R1"] if side > 0 else pv["S1"]
        else:
            od["target_r"] = 2.0
        out.append(od)
    return out


def n7f(cx, lvl, filt, sess):
    mk, f = cx.mk, cx.f
    out = []
    for d in cx.days:
        w = cx.window(d, sess)
        if w is None:
            continue
        pv = pivots(f, cx.kday[d])
        sell_at, buy_at = (pv["R1"], pv["S1"]) if lvl == 1 else (pv["R2"], pv["S2"])
        stop_s, stop_b = (pv["R2"], pv["S2"]) if lvl == 1 else (pv["R3"], pv["S3"])
        tgt_s, tgt_b = (pv["P"], pv["P"]) if lvl == 1 else (pv["R1"], pv["S1"])
        i0, i1 = int(mk.idx(w[0])), int(mk.idx(w[1] - 3600))
        hs = mk.h[i0:i1] >= sell_at * 1.0001
        lb = mk.l[i0:i1] <= buy_at * 0.9999
        if not (hs | lb).any():
            continue
        j = int(np.argmax(hs | lb))
        if hs[j] and lb[j]:
            continue
        side = -1 if hs[j] else 1
        if filt == "counter" and side != cx.trend(d):
            continue  # fade only stretches against the 10-day trend, so the trade goes with it
        out.append({"ts": int(w[0]), "side": side, "entry": "limit", "level": sell_at if side < 0 else buy_at,
                    "valid_until": int(w[1] - 3600), "stop": stop_s if side < 0 else stop_b,
                    "target": tgt_s if side < 0 else tgt_b, "exit_by": int(w[1])})
    return out


def n8(cx, k, hold, filt):
    b = cx.bars(60)
    va = b["v"].rolling(20).mean().shift(1)
    out = []
    for i in range(len(b)):
        tc = int(b["tc"].iat[i])
        d = (tc - 1) // DAY * DAY
        if not (OCT0 <= d < OCT1) or np.isnan(va.iat[i]):
            continue
        o, hi, lo, c, v = (b[x].iat[i] for x in ("o", "h", "l", "c", "v"))
        if hi <= lo or v < k * va.iat[i] or abs(c - o) < 0.6 * (hi - lo):
            continue
        side = 1 if c > o else -1
        if filt == "trend" and side != cx.trend(d):
            continue
        out.append({"ts": tc, "side": side, "entry": "market", "stop": lo if side > 0 else hi, "exit_by": tc + hold * H})
    return out


def rsi(c, n):
    dlt = c.diff()
    up = dlt.clip(lower=0).ewm(alpha=1 / n, adjust=False).mean()
    dn = (-dlt.clip(upper=0)).ewm(alpha=1 / n, adjust=False).mean()
    return 100 - 100 / (1 + up / dn.replace(0, np.nan))


def adx(b, n=14):
    up, dn = b["h"].diff(), -b["l"].diff()
    pdm = pd.Series(np.where((up > dn) & (up > 0), up, 0.0))
    mdm = pd.Series(np.where((dn > up) & (dn > 0), dn, 0.0))
    pc = b["c"].shift(1)
    tr = np.maximum(b["h"] - b["l"], np.maximum((b["h"] - pc).abs(), (b["l"] - pc).abs()))
    a = tr.ewm(alpha=1 / n, adjust=False).mean()
    pdi, mdi = 100 * pdm.ewm(alpha=1 / n, adjust=False).mean() / a, 100 * mdm.ewm(alpha=1 / n, adjust=False).mean() / a
    return (100 * (pdi - mdi).abs() / (pdi + mdi).replace(0, np.nan)).ewm(alpha=1 / n, adjust=False).mean()


def n9(cx, thr, reg):
    b = cx.bars(60)
    r = rsi(b["c"], 14)
    ax = adx(b, 14)
    a = atr_bars(b, 14)
    tc = b["tc"].to_numpy()
    above50, below50 = tc[(r >= 50).to_numpy()], tc[(r <= 50).to_numpy()]
    out = []
    for i in range(len(b)):
        d = int(tc[i] - 1) // DAY * DAY
        if not (OCT0 <= d < OCT1) or np.isnan(r.iat[i]) or np.isnan(a.iat[i]):
            continue
        if reg == "adx" and not ax.iat[i] < 25:
            continue
        side = 1 if r.iat[i] < thr else (-1 if r.iat[i] > 100 - thr else 0)
        if side == 0:
            continue
        ex = above50 if side > 0 else below50
        k = np.searchsorted(ex, tc[i], side="right")
        exit_by = min(int(ex[k]) if k < len(ex) else int(tc[i]) + 6 * H, int(tc[i]) + 6 * H)
        out.append({"ts": int(tc[i]), "side": side, "entry": "market", "stop": b["c"].iat[i] - side * 1.5 * a.iat[i], "exit_by": exit_by})
    return out


def n10(cx, ent, stp, tgt, filt):
    mk, f = cx.mk, cx.f
    out = []
    for d in cx.days:
        k = cx.kday[d]
        if f.weekday[k] >= 5:
            continue
        rel = us_open(d) - H                     # 08:30 New York
        i0, i1, iw = (int(x) for x in mk.idx([rel - 1800, rel, rel + 2 * H + 1800]))
        rh, rl = mk.h[i0:i1].max(), mk.l[i0:i1].min()
        if rh <= rl:
            continue
        if filt == "nr4" and not f.nr4[k]:
            continue
        if filt == "tight":
            hts = []
            for kk in range(max(0, k - 20), k):
                dd = int(f.days[kk])
                if f.weekday[kk] >= 5:
                    continue
                rr = us_open(dd) - H
                a0, a1 = (int(x) for x in mk.idx([rr - 1800, rr]))
                hts.append((mk.h[a0:a1].max() - mk.l[a0:a1].min()) / mk.l[a0:a1].min())
            if len(hts) < 10 or not (rh - rl) / rl < np.median(hts):
                continue
        if ent == "stop":
            up, dn = mk.h[i1:iw] >= rh, mk.l[i1:iw] <= rl
            if not (up | dn).any():
                continue
            j = int(np.argmax(up | dn))
            if up[j] and dn[j]:
                continue
            side = 1 if up[j] else -1
            od = {"ts": rel, "entry": "stop", "level": rh if side > 0 else rl, "valid_until": int(mk.t[iw])}
        else:
            c = mk.c[i1:iw]
            hit = (c > rh) | (c < rl)
            if not hit.any():
                continue
            j = int(np.argmax(hit))
            side = 1 if c[j] > rh else -1
            ts = int(mk.t[i1 + j]) + 60
            od = ({"ts": ts, "entry": "market"} if ent == "close" else
                  {"ts": ts, "entry": "limit", "level": rh if side > 0 else rl, "valid_until": int(mk.t[iw])})
        if filt == "trend" and side != cx.trend(d):
            continue
        stop = (rl if side > 0 else rh) if stp == "far" else (rh + rl) / 2
        od.update({"side": side, "stop": stop, "exit_by": us_open(d) + 6 * H + 1800})
        if tgt != "eod":
            od["target_r"] = float(tgt[:-1])
        out.append(od)
    return out


def n11(cx, rule, hh, dur):
    out = []
    for d in cx.days:
        ts = d + hh * H
        px = cx.last_close(ts)
        vw = cx.vwap(np.array([ts]))[0]
        if np.isnan(vw):
            continue
        if rule == "day":
            side = int(np.sign(px - cx.f.dopen[cx.kday[d]]))
        elif rule == "vwap":
            side = int(np.sign(px - vw))
        elif rule == "short_below":
            side = -1 if px < vw else 0
        else:
            side = 1 if px > vw else 0
        if side == 0:
            continue
        out.append({"ts": int(ts), "side": side, "entry": "market", "stop": None, "exit_by": int(ts + dur * H)})
    return out


def n12(cx, tf, sess, filt):
    b = cx.bars(tf)
    ha_c = (b["o"] + b["h"] + b["l"] + b["c"]) / 4
    ha_o = np.zeros(len(b))
    ha_o[0] = (b["o"].iat[0] + b["c"].iat[0]) / 2
    hc = ha_c.to_numpy()
    for i in range(1, len(b)):
        ha_o[i] = (ha_o[i - 1] + hc[i - 1]) / 2
    color = np.sign(hc - ha_o)
    ema50 = b["c"].ewm(span=50, adjust=False).mean().to_numpy()
    side = np.where((color > 0) & (b["c"].to_numpy() > ema50), 1, np.where((color < 0) & (b["c"].to_numpy() < ema50), -1, 0))
    return per_day(cx, bar_sides(cx, tf, pd.Series(side)), sess, filt)


FN = {"N1": n1, "N2": n2, "N3": n3, "N4": n4, "N5": n5, "N6": n6, "N7b": n7b, "N7f": n7f, "N8": n8, "N9": n9,
      "N10": n10, "N11": n11, "N12": n12}


def orders(cx, g):
    return FN[g[0]](cx, *g[1:])


def describe(g):
    fam = g[0]
    sess = lambda s: "US hours" if s == "us" else "all day"
    tr = lambda f: ", with 10-day trend" if f == "trend" else ""
    if fam == "N1":
        return f"EMA {g[2]}/{g[3]} cross on {g[1]}m, {sess(g[4])}{tr(g[5])}", f"Hold long while EMA{g[2]} is above EMA{g[3]} on {g[1]}-minute bars, short while below; flat outside the window and at 00:00 UTC."
    if fam == "N2":
        return f"Supertrend {g[2]}x{g[3]} on {g[1]}m, {sess(g[4])}{tr(g[5])}", f"Follow the Supertrend (ATR {g[2]}, multiplier {g[3]}) direction on {g[1]}-minute bars; flat outside the window."
    if fam == "N3":
        return f"MACD cross on {g[1]}m{', zero-line agreement' if g[2] == 'zero' else ''}, {sess(g[3])}{tr(g[4])}", "Long while MACD(12,26) is above its 9-period signal, short while below."
    if fam == "N4":
        return f"VWAP pullback, stop {g[1]} ATR, target {g[2]}{tr(g[3])}, {sess(g[4])}", "After three 15m closes on one side of VWAP, rest a limit order at VWAP for an hour in that direction."
    if fam == "N5":
        return f"VWAP {g[1]}-sd fade, {sess(g[2])}{', only fading moves against the 10-day trend' if g[3] == 'counter' else ''}", f"When price is {g[1]} standard deviations from VWAP, trade back toward VWAP for up to 2 h."
    if fam == "N6":
        return f"Previous-day high/low breakout, {g[1]} entry, stop {g[2]}, target {g[3]}{tr(g[4])}", "Trade the first break of yesterday's high or low before 18:00 UTC; exit at 00:00."
    if fam == "N7b":
        return f"CPR breakout ({g[1]} CPR), stop {g[2]}, target {g[3]}{tr(g[4])}", "Floor-pivot CPR from yesterday; trade the first close above TC or below BC before 16:00 UTC."
    if fam == "N7f":
        return f"Pivot fade at {'R1/S1' if g[1] == 1 else 'R2/S2'}{', only fading moves against the 10-day trend' if g[2] == 'counter' else ''}, {sess(g[3])}", "Rest limit orders at the pivot levels; target the pivot (or R1/S1), stop one level further."
    if fam == "N8":
        return f"1h volume candle ({g[1]}x volume), hold {g[2]} h{tr(g[3])}", "Follow an hourly candle with heavy volume and a full body; stop at its other end."
    if fam == "N9":
        return f"1h RSI(14) under {g[1]} / over {100 - g[1]}{', ADX under 25' if g[2] == 'adx' else ''}", "Fade hourly RSI extremes until RSI crosses 50 or 6 h pass; stop 1.5 ATR."
    if fam == "N10":
        return f"US pre-market breakout, {g[1]} entry, stop {g[2]}, target {g[3]}{tr(g[4]) if g[4] == 'trend' else (', ' + g[4] if g[4] != 'none' else '')}", "Range = 08:00-08:30 New York; trade the break from 08:30 for 2.5 h; exit at the NYSE close."
    if fam == "N11":
        what = {"day": "with the day's move", "vwap": "with price vs VWAP", "short_below": "short if below VWAP", "long_above": "long if above VWAP"}[g[1]]
        return f"{g[2]}:00 UTC for {g[3]} h, {what}", f"Every day at {g[2]}:00 UTC take the position ({what}) and close {g[3]} hours later."
    if fam == "N12":
        return f"Heikin-Ashi {g[1]}m with EMA50, {sess(g[2])}{tr(g[3])}", "Long on green Heikin-Ashi candles above EMA50, short on red below it."
    return vid(g), ""
