"""Execution model for the session-strategy lab.

Orders (dicts) carry: ts (when the order goes live, a closed-bar time), side
(+1 long / -1 short), entry ('market' | 'stop' | 'limit'), level (for stop and
limit entries), valid_until (last time a stop/limit entry may fill), stop (an
absolute price), target (absolute) or target_r (multiple of the fill-to-stop
risk), exit_by (time exit), tag.

Fill rules, all on 1m bars:
  market  fills at the open of the minute starting at ts (taker).
  stop    triggers when the minute's range touches the level; fills at the
          level, or at the open if the minute opens beyond it (taker).
  limit   fills only if price trades 1 bp THROUGH the level (queue position);
          at the level, or at the open if it opens beyond it (maker).
  After an intrabar entry fill, if that same minute also reaches the stop the
  stop is taken; a target is never counted in the fill minute.
  Later minutes: an open beyond a level fills there; the stop (touch) is
  checked before the target (trade-through by 1 bp; maker); time exits fill at
  the open of the exit minute (taker). Flat by 00:00 UTC. Days flagged bad
  (missing exchange data, or rebuilt from spot) are not traded.
Costs per side: maker 0.02% + 18% GST = 2.36 bp; taker 0.05% + 18% GST =
5.9 bp plus slippage of 2 bp (BTC, ETH) or 4 bp (SOL, XRP, DOGE, BNB).
Funding: Binance's real prints, signed. $10,000 notional per order.
"""
import os

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
DATA = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "session_lab"))
SYMBOLS = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"]
MAKER = 0.000236
TAKER = 0.00059
SLIP = {"BTCUSDT": 0.0002, "ETHUSDT": 0.0002, "SOLUSDT": 0.0004, "XRPUSDT": 0.0004, "DOGEUSDT": 0.0004, "BNBUSDT": 0.0004}
THROUGH = 0.0001
NOTIONAL = 10_000.0
DAY, H = 86400, 3600


def _nth_sunday(y, month, n):
    d = pd.Timestamp(y, month, 1)
    return d + pd.Timedelta(days=(6 - d.weekday()) % 7) + pd.Timedelta(weeks=n - 1)


def _last_sunday(y, month):
    d = pd.Timestamp(y, month, 1) + pd.offsets.MonthEnd(0)
    return d - pd.Timedelta(days=(d.weekday() - 6) % 7)


def us_open(day):
    d = pd.Timestamp(day, unit="s")
    return day + (13 * H + 1800 if _nth_sunday(d.year, 3, 2) <= d < _nth_sunday(d.year, 11, 1) else 14 * H + 1800)


def london_open(day):
    d = pd.Timestamp(day, unit="s")
    return day + (7 * H if _last_sunday(d.year, 3) <= d < _last_sunday(d.year, 10) else 8 * H)


class Market:
    def __init__(self, sym):
        z = np.load(os.path.join(DATA, f"{sym}.npz"))
        self.sym = sym
        self.t = z["t"]
        self.o, self.h, self.l, self.c, self.v = z["o"], z["h"], z["l"], z["c"], z["v"]
        self.t0, self.n = int(self.t[0]), len(self.t)
        assert self.t0 % DAY == 0 and np.all(np.diff(self.t) == 60)
        self.bad_days = set(int(x) for x in z["bad_day"])
        f = np.load(os.path.join(DATA, f"{sym}_funding.npz"))
        self.fund_t, self.fund_r = f["t"], f["rate"]
        self.slip = SLIP[sym]
        k = 1440
        n = self.n // k * k
        self.daily = pd.DataFrame({"t": self.t[:n:k], "o": self.o[:n:k], "h": self.h[:n].reshape(-1, k).max(1),
                                   "l": self.l[:n].reshape(-1, k).min(1), "c": self.c[k - 1:n:k]})

    def idx(self, ts):
        return (np.asarray(ts, dtype=np.int64) - self.t0) // 60


def fill(mk, od):
    """One order -> trade tuple or None."""
    ts = int(od["ts"])
    day = ts // DAY * DAY
    end = day + DAY
    if day in mk.bad_days:
        return None
    exit_by = min(int(od.get("exit_by") or end), end)
    jmax = min(int(mk.idx(exit_by)), mk.n - 1)
    i0 = int(mk.idx(ts))
    if i0 < 0 or i0 >= jmax:
        return None
    side, kind = od["side"], od.get("entry", "market")
    o, h, l = mk.o, mk.h, mk.l
    intrabar = False
    if kind == "market":
        j, px, ecost = i0, o[i0], TAKER + mk.slip
    else:
        L = od["level"]
        last = min(int(mk.idx(int(od.get("valid_until") or exit_by))), jmax)
        if last <= i0:
            return None
        so, sh, sl_ = o[i0:last], h[i0:last], l[i0:last]
        if kind == "stop":
            hit = (sh >= L) if side > 0 else (sl_ <= L)
        else:
            hit = (sl_ <= L * (1 - THROUGH)) if side > 0 else (sh >= L * (1 + THROUGH))
        if not hit.any():
            return None
        k = int(np.argmax(hit))
        j = i0 + k
        if kind == "stop":
            gapped = (so[k] >= L) if side > 0 else (so[k] <= L)
            ecost = TAKER + mk.slip
        else:
            gapped = (so[k] <= L) if side > 0 else (so[k] >= L)
            ecost = MAKER
        px = so[k] if gapped else L
        intrabar = not gapped
    stop = od.get("stop")
    if stop is not None and side * (px - stop) <= 0:
        return None
    tgt = od.get("target")
    if tgt is None and od.get("target_r") and stop is not None:
        tgt = px + side * od["target_r"] * abs(px - stop)
    if tgt is not None and side * (tgt - px) <= 0:
        return None
    if j >= jmax:
        return None
    # the fill minute
    if intrabar:
        if stop is not None and ((side > 0 and l[j] <= stop) or (side < 0 and h[j] >= stop)):
            return _close(mk, od, ts, j, px, stop, int(mk.t[j]) + 60, "SL", ecost, TAKER + mk.slip)
        start = j + 1
    else:
        start = j
    if start < jmax:
        so, sh, sl_ = o[start:jmax], h[start:jmax], l[start:jmax]
        first = np.zeros(len(so), bool)
        first[0] = start == j  # an open fill at this minute's open: no gap check on it
        if stop is not None:
            gs = (side * (so - stop) <= 0) & ~first
            hs = (sl_ <= stop) if side > 0 else (sh >= stop)
        else:
            gs = hs = np.zeros(len(so), bool)
        if tgt is not None:
            gt = (side * (so - tgt) >= 0) & ~first
            ht = (sh >= tgt * (1 + THROUGH)) if side > 0 else (sl_ <= tgt * (1 - THROUGH))
        else:
            gt = ht = np.zeros(len(so), bool)
        any_ = gs | gt | hs | ht
        if any_.any():
            k = int(np.argmax(any_))
            m = start + k
            if gs[k]:
                return _close(mk, od, ts, j, px, so[k], int(mk.t[m]), "SL", ecost, TAKER + mk.slip)
            if gt[k]:
                return _close(mk, od, ts, j, px, so[k], int(mk.t[m]), "TP", ecost, MAKER)
            if hs[k]:
                return _close(mk, od, ts, j, px, stop, int(mk.t[m]) + 60, "SL", ecost, TAKER + mk.slip)
            return _close(mk, od, ts, j, px, tgt, int(mk.t[m]) + 60, "TP", ecost, MAKER)
    return _close(mk, od, ts, j, px, o[jmax], int(mk.t[jmax]), "TIME", ecost, TAKER + mk.slip)


def _close(mk, od, ts, j, px, xp, xt, why, ecost, xcost):
    side = od["side"]
    et = int(mk.t[j])
    gross = side * (xp - px) / px
    f = (mk.fund_t > et) & (mk.fund_t < xt)
    funding = side * float(mk.fund_r[f].sum())
    net = gross - ecost - xcost - funding
    return (et, xt, side, gross, net, why)


def run_orders(mk, orders):
    """Fill a variant's orders on one coin, one position at a time."""
    out, busy = [], -1
    for od in sorted(orders, key=lambda x: x["ts"]):
        if od["ts"] < busy:
            continue
        r = fill(mk, od)
        if r is None:
            continue
        out.append(r)
        busy = r[1]
    return out
