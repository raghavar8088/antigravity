"""Shared machinery for the 20-strategy intraday crypto study.

Conventions every strategy follows:
  - The session is the UTC day. Every position is flat by 00:00 UTC.
  - A signal uses only bars that have closed. Its time `ts` is the close of the
    bar it was read from; the entry fills at the open of the 1m bar starting at ts.
  - The open position is walked minute by minute: a minute that opens beyond the
    stop or target fills at that open; a minute whose range reaches both levels
    takes the stop. Time exits fill at the open of the minute they fall on.
  - One position per strategy per coin; a signal during a position is skipped.
  - No trades on a day rebuilt from spot bars (2026-10-28: Binance never published it).
  - Costs per leg: Delta India taker fee 0.05% + 18% GST = 5.9 bp, plus spread and
    slippage of 2 bp (BTC, ETH) or 4 bp (SOL, XRP, DOGE, BNB). Funding: Binance's
    real prints, signed (a long pays positive funding), charged for every print
    strictly inside the holding period.
  - $10,000 notional per trade, no leverage, no compounding.
"""
import os

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
DATA = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "intraday20"))
SYMBOLS = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"]
FEE_LEG = 0.00059
SLIP_LEG = {"BTCUSDT": 0.0002, "ETHUSDT": 0.0002, "SOLUSDT": 0.0004, "XRPUSDT": 0.0004,
            "DOGEUSDT": 0.0004, "BNBUSDT": 0.0004}
NOTIONAL = 10_000.0
DAY = 86400
H = 3600


# ── sessions ────────────────────────────────────────────────────────────────
def _nth_sunday(y, month, n):
    d = pd.Timestamp(y, month, 1)
    first = d + pd.Timedelta(days=(6 - d.weekday()) % 7)
    return first + pd.Timedelta(weeks=n - 1)


def _last_sunday(y, month):
    d = pd.Timestamp(y, month, 1) + pd.offsets.MonthEnd(0)
    return d - pd.Timedelta(days=(d.weekday() - 6) % 7)


def us_open_utc(day_ts):
    """NYSE open (09:30 New York) in unix seconds for the UTC day starting at day_ts."""
    d = pd.Timestamp(day_ts, unit="s")
    dst = _nth_sunday(d.year, 3, 2) <= d < _nth_sunday(d.year, 11, 1)
    return day_ts + (13 * H + 1800 if dst else 14 * H + 1800)


def london_open_utc(day_ts):
    """London 08:00 local in unix seconds for the UTC day starting at day_ts."""
    d = pd.Timestamp(day_ts, unit="s")
    bst = _last_sunday(d.year, 3) <= d < _last_sunday(d.year, 10)
    return day_ts + (7 * H if bst else 8 * H)


# ── data ────────────────────────────────────────────────────────────────────
class Market:
    """One coin on a complete UTC minute grid, plus resampled bars and metrics."""

    def __init__(self, sym):
        df = pd.read_pickle(os.path.join(DATA, f"{sym}.pkl"))
        self.sym = sym
        self.t = df.index.to_numpy(np.int64)
        assert self.t[0] % DAY == 0 and np.all(np.diff(self.t) == 60), "minute grid must be complete and day-aligned"
        self.t0 = int(self.t[0])
        self.o, self.h, self.l, self.c = (df[k].to_numpy(float) for k in ("o", "h", "l", "c"))
        self.v, self.tbv, self.prem = (df[k].to_numpy(float) for k in ("v", "tbv", "prem"))
        # days rebuilt from spot bars (Binance never published the perpetual file): no trading
        self.rebuilt_days = set(int(x) for x in np.unique(self.t[df["rebuilt"].to_numpy(bool)] // DAY * DAY))
        self.n = len(self.t)
        tp = (self.h + self.l + self.c) / 3
        self.cpv = np.concatenate([[0.0], np.cumsum(tp * self.v)])
        self.cv = np.concatenate([[0.0], np.cumsum(self.v)])
        self.ctb = np.concatenate([[0.0], np.cumsum(self.tbv)])
        f = pd.read_pickle(os.path.join(DATA, f"{sym}_funding.pkl"))
        self.fund_t, self.fund_r = f["t"].to_numpy(np.int64), f["rate"].to_numpy(float)
        m = pd.read_pickle(os.path.join(DATA, f"{sym}_metrics.pkl"))
        # a 5m metrics row stamped X is treated as known only at X + 5 min
        self.metrics = {}
        for col in m.columns[1:]:
            s = pd.Series(m[col].to_numpy(float), index=m["t"].to_numpy(np.int64) + 300)
            s = s[~s.index.duplicated()].sort_index()
            self.metrics[col] = s.reindex(self.t, method="ffill").to_numpy()
        self._bars = {}

    def idx(self, ts):
        return (np.asarray(ts, dtype=np.int64) - self.t0) // 60

    def bars(self, minutes):
        """UTC-aligned bars of `minutes`; column tc is the close time (when the bar is known)."""
        if minutes not in self._bars:
            k = minutes
            n = self.n // k * k
            b = pd.DataFrame({
                "t": self.t[:n:k], "o": self.o[:n:k], "h": self.h[:n].reshape(-1, k).max(1),
                "l": self.l[:n].reshape(-1, k).min(1), "c": self.c[k - 1:n:k],
                "v": self.v[:n].reshape(-1, k).sum(1), "tbv": self.tbv[:n].reshape(-1, k).sum(1)})
            b["tc"] = b["t"] + 60 * k
            self._bars[k] = b
        return self._bars[minutes]

    def vwap_before(self, ts):
        """Session VWAP over the minutes from 00:00 UTC up to (not including) ts."""
        ts = np.asarray(ts, dtype=np.int64)
        i, d = self.idx(ts), self.idx(ts // DAY * DAY)
        vol = self.cv[i] - self.cv[d]
        with np.errstate(invalid="ignore", divide="ignore"):
            return np.where(vol > 0, (self.cpv[i] - self.cpv[d]) / vol, np.nan)

    def flow(self, ts, minutes):
        """Taker imbalance, volume and return over the `minutes` before ts."""
        i = self.idx(ts)
        a = i - minutes
        vol = self.cv[i] - self.cv[a]
        buy = self.ctb[i] - self.ctb[a]
        with np.errstate(invalid="ignore", divide="ignore"):
            imb = np.where(vol > 0, (2 * buy - vol) / vol, 0.0)
        ret = self.c[i - 1] / self.o[a] - 1
        return imb, vol, ret


def atr(b, n=14):
    """Simple-average true range; the value on a bar includes that (closed) bar."""
    pc = b["c"].shift(1)
    tr = np.maximum(b["h"] - b["l"], np.maximum((b["h"] - pc).abs(), (b["l"] - pc).abs()))
    return tr.rolling(n).mean()


def rsi(c, n):
    d = c.diff()
    up = d.clip(lower=0).ewm(alpha=1 / n, adjust=False).mean()
    dn = (-d.clip(upper=0)).ewm(alpha=1 / n, adjust=False).mean()
    return 100 - 100 / (1 + up / dn.replace(0, np.nan))


def adx(b, n=14):
    up, dn = b["h"].diff(), -b["l"].diff()
    pdm = np.where((up > dn) & (up > 0), up, 0.0)
    mdm = np.where((dn > up) & (dn > 0), dn, 0.0)
    pc = b["c"].shift(1)
    tr = np.maximum(b["h"] - b["l"], np.maximum((b["h"] - pc).abs(), (b["l"] - pc).abs()))
    a = 1 / n
    atr_ = tr.ewm(alpha=a, adjust=False).mean()
    pdi = 100 * pd.Series(pdm, index=b.index).ewm(alpha=a, adjust=False).mean() / atr_
    mdi = 100 * pd.Series(mdm, index=b.index).ewm(alpha=a, adjust=False).mean() / atr_
    dx = 100 * (pdi - mdi).abs() / (pdi + mdi).replace(0, np.nan)
    return dx.ewm(alpha=a, adjust=False).mean()


def round_step(price):
    """The largest 1-2-5 step that is at most 1% of price: the round levels traders watch."""
    target = 0.01 * price
    e = np.floor(np.log10(target))
    for m in (5, 2, 1):
        if m * 10 ** e <= target:
            return m * 10 ** e
    return 10 ** e


# ── simulator ───────────────────────────────────────────────────────────────
def simulate(mkt, entries):
    """Fill a strategy's entries on one coin. Each entry is a dict with
    ts, side (+1 long / -1 short) and optionally stop / target (absolute prices),
    stop_dist / target_dist (distance from the fill), exit_by (latest exit time)
    and exits (sorted array of signal times that close the position)."""
    out = []
    busy = -1
    cost_leg = FEE_LEG + SLIP_LEG[mkt.sym]
    o, h, l, t = mkt.o, mkt.h, mkt.l, mkt.t
    for e in sorted(entries, key=lambda x: x["ts"]):
        ts = int(e["ts"])
        if ts < busy:
            continue
        i = int(mkt.idx(ts))
        end = ts // DAY * DAY + DAY
        if i < 0 or i >= mkt.n - 1 or ts >= end - 60 or (end - DAY) in mkt.rebuilt_days:
            continue
        side = e["side"]
        px = o[i]
        stop = e.get("stop")
        if stop is None and e.get("stop_dist") is not None:
            stop = px - side * e["stop_dist"]
        tgt = e.get("target")
        if tgt is None and e.get("target_dist") is not None:
            tgt = px + side * e["target_dist"]
        if (stop is not None and side * (px - stop) <= 0) or (tgt is not None and side * (tgt - px) <= 0):
            continue  # the market opened beyond a level: no valid trade
        exit_by = min(int(e.get("exit_by") or end), end)
        ex = e.get("exits")
        if ex is not None and len(ex):
            k = np.searchsorted(ex, ts, side="right")
            if k < len(ex):
                exit_by = min(exit_by, int(ex[k]))
        jmax = min(int(mkt.idx(exit_by)), mkt.n - 1)
        if jmax <= i:
            continue
        so, sh, sl = o[i:jmax], h[i:jmax], l[i:jmax]
        never = np.zeros(len(so), bool)
        if stop is not None:
            gs = (side * (so - stop) <= 0)
            gs[0] = False
            hs = (sl <= stop) if side > 0 else (sh >= stop)
        else:
            gs, hs = never, never
        if tgt is not None:
            gt = (side * (so - tgt) >= 0)
            gt[0] = False
            ht = (sh >= tgt) if side > 0 else (sl <= tgt)
        else:
            gt, ht = never, never
        hit = gs | gt | hs | ht
        if hit.any():
            k = int(np.argmax(hit))
            j = i + k
            if gs[k]:
                xp, xt, why = so[k], t[j], "SL"
            elif gt[k]:
                xp, xt, why = so[k], t[j], "TP"
            elif hs[k]:
                xp, xt, why = stop, t[j] + 60, "SL"
            else:
                xp, xt, why = tgt, t[j] + 60, "TP"
        else:
            xp, xt, why = o[jmax], t[jmax], ("EOD" if exit_by == end else "TIME")
        gross = side * (xp - px) / px
        f = (mkt.fund_t > ts) & (mkt.fund_t < xt)
        funding = side * float(mkt.fund_r[f].sum())
        net = gross - 2 * cost_leg - funding
        out.append({"sym": mkt.sym, "side": side, "entry_t": ts, "exit_t": int(xt), "entry": px, "exit": xp,
                    "reason": why, "gross": gross, "funding": funding, "net": net, "pnl": net * NOTIONAL,
                    "tag": e.get("tag", "")})
        busy = int(xt)
    return out
