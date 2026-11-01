"""Strategy families and the full parameter grid, fixed before any run.

A  Session opening-range breakout (Crabel 1990; Zarattini, Barbon & Aziz 2024;
   the US-open gross edge found in the previous study). Sessions: US (NYSE
   open, weekdays), LDN (London 08:00 local, weekdays), UTC (00:00, every day).
   Range = first 15/30/60 min. Entry: 'close' (market after a 1m close outside
   the range), 'stop' (resting stop orders at both edges; first touch), 'retest'
   (after a close outside, a limit back at the broken edge; maker). Stop at the
   far edge or the range middle; target 1.5R, 3R or none; filters: none,
   'trend' (with the sign of the 10-day return), 'nr4' (yesterday was the
   narrowest of 4 days), 'tight' (opening range below its 20-day median).
   Entries until half the session has passed; exit at the session close.
B  Trend-day continuation (intraday momentum on large early moves, Gao et al.
   2018; Caporale & Plastun 2019). At 04/08/12 UTC or US open + 1 h, if the
   day's move is at least k x ATR(14 days), follow it to 00:00. Stop at the day
   open or halfway back; optional 10-day trend filter.
C  Cross-sectional intraday (Liu, Tsyvinski & Wu 2022 on crypto momentum, in a
   day-trading form). At 04/08/12 UTC rank the 6 coins by return since 00:00
   or over 24 h; long the top 1-2 and short the bottom 1-2 (or the reverse);
   flat at 00:00. Market-neutral.
F  Volatility breakout (Williams 1999) with a stop-order entry option and
   optional trend filter; stop at the day open or none; exit at 00:00.
G  Time-of-day drift: long or short from hour h for 1, 2 or 4 h, every day.
"""
import itertools

import numpy as np

from sl_core import DAY, H, london_open, us_open

GRID_A = list(itertools.product(["US", "LDN", "UTC"], [15, 30, 60], ["close", "stop", "retest"], ["far", "mid"],
                                [1.5, 3.0, None], ["none", "trend", "nr4", "tight"]))
GRID_B = list(itertools.product(["04", "08", "12", "US1h"], [0.25, 0.5, 0.75], ["none", "trend"], ["open", "half"]))
GRID_C = list(itertools.product(["04", "08", "12"], ["day", "24h"], ["mom", "rev"], [1, 2]))
GRID_F = list(itertools.product([0.3, 0.5, 0.7], ["close", "stop"], ["none", "trend"], ["open", "none"]))
GRID_G = [(hh, d, s) for d in (1, 2, 4) for hh in range(24) if hh + d <= 24 for s in (1, -1)]


def variant_ids():
    ids = ["A|%s|%d|%s|%s|%s|%s" % (s, r, e, st, "none" if t is None else t, f) for s, r, e, st, t, f in GRID_A]
    ids += ["B|%s|%s|%s|%s" % g for g in GRID_B]
    ids += ["C|%s|%s|%s|%d" % g for g in GRID_C]
    ids += ["F|%s|%s|%s|%s" % g for g in GRID_F]
    ids += ["G|%02d|%d|%s" % (hh, d, "long" if s > 0 else "short") for hh, d, s in GRID_G]
    return ids


class Features:
    """Per-coin, per-day inputs computed once (all from data before their use)."""

    def __init__(self, mk):
        self.mk = mk
        d = mk.daily
        self.days = d["t"].to_numpy()
        c, hi, lo, op = (d[k].to_numpy() for k in ("c", "h", "l", "o"))
        self.dopen = op
        n = len(d)
        self.trend = np.zeros(n)
        self.trend[11:] = np.sign(c[10:-1] - c[:-11])          # close(d-1) vs close(d-11)
        rng = hi - lo
        self.prev_rng = np.concatenate([[np.nan], rng[:-1]])
        self.nr4 = np.zeros(n, bool)
        for k in range(4, n):
            self.nr4[k] = rng[k - 1] <= rng[k - 4:k].min()
        pc = np.concatenate([[np.nan], c[:-1]])
        tr = np.maximum(rng, np.maximum(np.abs(hi - pc), np.abs(lo - pc)))
        self.atr = np.full(n, np.nan)
        for k in range(15, n):
            self.atr[k] = tr[k - 14:k].mean()                  # days d-14 .. d-1
        self.weekday = ((self.days // DAY) + 3) % 7             # 1970-01-01 was a Thursday
        self.orb = {}

    def session(self, k, s):
        day = int(self.days[k])
        if s == "US":
            op = us_open(day)
            return op, op + 6 * H + 1800, op + 3 * H + 900
        if s == "LDN":
            op = london_open(day)
            return op, op + 8 * H + 1800, op + 4 * H + 900
        return day, day + DAY, day + 12 * H

    def orb_table(self, s, rmin):
        """Opening range and breakout events for one session and range length."""
        key = (s, rmin)
        if key in self.orb:
            return self.orb[key]
        mk = self.mk
        rows = []
        for k in range(len(self.days)):
            if s in ("US", "LDN") and self.weekday[k] >= 5:
                rows.append(None)
                continue
            op, close, win = self.session(k, s)
            i0, i1, iw = (int(x) for x in mk.idx([op, op + rmin * 60, win]))
            if iw >= mk.n:
                rows.append(None)
                continue
            rh, rl = mk.h[i0:i1].max(), mk.l[i0:i1].min()
            seg_c = mk.c[i1:iw]
            hit = (seg_c > rh) | (seg_c < rl)
            brk = None
            if hit.any():
                j = int(np.argmax(hit))
                brk = (int(mk.t[i1 + j]) + 60, 1 if seg_c[j] > rh else -1)
            sh, sl_ = mk.h[i1:iw], mk.l[i1:iw]
            up, dn = sh >= rh, sl_ <= rl
            touch = None
            if (up | dn).any():
                j = int(np.argmax(up | dn))
                if not (up[j] and dn[j]):
                    touch = 1 if up[j] else -1
            rows.append({"op": op, "close": close, "win": win, "start": op + rmin * 60, "rh": rh, "rl": rl,
                         "height": (rh - rl) / rl, "brk": brk, "touch": touch})
        hts = np.array([r["height"] if r else np.nan for r in rows])
        for k, r in enumerate(rows):
            if r:
                past = hts[max(0, k - 20):k]
                past = past[~np.isnan(past)]
                r["tight"] = len(past) >= 10 and r["height"] < np.median(past)
        self.orb[key] = rows
        return rows


def orders_A(f, s, rmin, entry, stopk, tgt, filt):
    out = []
    for k, r in enumerate(f.orb_table(s, rmin)):
        if r is None:
            continue
        if filt == "nr4" and not f.nr4[k]:
            continue
        if filt == "tight" and not r["tight"]:
            continue
        if entry == "stop":
            if r["touch"] is None:
                continue
            side = r["touch"]
            od = {"ts": r["start"], "entry": "stop", "level": r["rh"] if side > 0 else r["rl"], "valid_until": r["win"]}
        else:
            if r["brk"] is None:
                continue
            ts, side = r["brk"]
            if entry == "close":
                od = {"ts": ts, "entry": "market"}
            else:
                od = {"ts": ts, "entry": "limit", "level": r["rh"] if side > 0 else r["rl"], "valid_until": r["win"]}
        if filt == "trend" and f.trend[k] != side:
            continue
        stop = (r["rl"] if side > 0 else r["rh"]) if stopk == "far" else (r["rh"] + r["rl"]) / 2
        od.update({"side": side, "stop": stop, "target_r": tgt, "exit_by": r["close"]})
        out.append(od)
    return out


def orders_B(f, chk, kk, filt, stopk):
    mk, out = f.mk, []
    for k in range(15, len(f.days)):
        day = int(f.days[k])
        if chk == "US1h":
            if f.weekday[k] >= 5:
                continue
            ts = us_open(day) + H
        else:
            ts = day + int(chk) * H
        i = int(mk.idx(ts)) - 1
        if i >= mk.n or np.isnan(f.atr[k]):
            continue
        move = mk.c[i] - f.dopen[k]
        side = 1 if move >= kk * f.atr[k] else (-1 if move <= -kk * f.atr[k] else 0)
        if side == 0 or (filt == "trend" and f.trend[k] != side):
            continue
        stop = f.dopen[k] if stopk == "open" else (f.dopen[k] + mk.c[i]) / 2
        out.append({"ts": ts, "side": side, "entry": "market", "stop": stop, "exit_by": day + DAY})
    return out


def orders_F(f, kk, entry, filt, stopk):
    mk, out = f.mk, []
    for k in range(15, len(f.days)):
        day = int(f.days[k])
        o_, rng = f.dopen[k], f.prev_rng[k]
        if np.isnan(rng) or rng <= 0:
            continue
        up, dn = o_ + kk * rng, o_ - kk * rng
        i0, i1 = int(mk.idx(day)), int(mk.idx(day + DAY - 60))
        if i1 >= mk.n:
            continue
        if entry == "close":
            c = mk.c[i0:i1]
            hit = (c > up) | (c < dn)
            if not hit.any():
                continue
            j = int(np.argmax(hit))
            side = 1 if c[j] > up else -1
            od = {"ts": int(mk.t[i0 + j]) + 60, "entry": "market"}
        else:
            uh, dl = mk.h[i0:i1] >= up, mk.l[i0:i1] <= dn
            if not (uh | dl).any():
                continue
            j = int(np.argmax(uh | dl))
            if uh[j] and dl[j]:
                continue
            side = 1 if uh[j] else -1
            od = {"ts": day, "entry": "stop", "level": up if side > 0 else dn, "valid_until": day + DAY - 60}
        if filt == "trend" and f.trend[k] != side:
            continue
        od.update({"side": side, "stop": o_ if stopk == "open" else None, "exit_by": day + DAY})
        out.append(od)
    return out


def orders_G(f, hh, dur, side):
    return [{"ts": int(d) + hh * H, "side": side, "entry": "market", "stop": None, "exit_by": int(d) + (hh + dur) * H}
            for d in f.days[15:]]


def orders(f, vid):
    p = vid.split("|")
    if p[0] == "A":
        return orders_A(f, p[1], int(p[2]), p[3], p[4], None if p[5] == "none" else float(p[5]), p[6])
    if p[0] == "B":
        return orders_B(f, p[1], float(p[2]), p[3], p[4])
    if p[0] == "F":
        return orders_F(f, float(p[1]), p[2], p[3], p[4])
    if p[0] == "G":
        return orders_G(f, int(p[1]), int(p[2]), 1 if p[3] == "long" else -1)
    raise ValueError(vid)


def orders_C(mkts, feats, x, basis, mode, legs):
    """Cross-sectional baskets: {sym: [orders]}."""
    syms = list(mkts)
    days = feats[syms[0]].days
    out = {s: [] for s in syms}
    for k in range(15, len(days)):
        day = int(days[k])
        ts = day + int(x) * H
        rets = {}
        for s in syms:
            mk = mkts[s]
            i = int(mk.idx(ts)) - 1
            if i >= mk.n or day in mk.bad_days:
                break
            ref = mk.o[int(mk.idx(day))] if basis == "day" else mk.c[i - 1440]
            rets[s] = mk.c[i] / ref - 1
        if len(rets) < len(syms):
            continue
        order = sorted(syms, key=lambda s: rets[s], reverse=True)
        top, bottom = order[:legs], order[-legs:]
        longs, shorts = (top, bottom) if mode == "mom" else (bottom, top)
        for s in longs:
            out[s].append({"ts": ts, "side": 1, "entry": "market", "stop": None, "exit_by": day + DAY})
        for s in shorts:
            out[s].append({"ts": ts, "side": -1, "entry": "market", "stop": None, "exit_by": day + DAY})
    return out
