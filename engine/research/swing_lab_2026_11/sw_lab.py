"""Swing-strategy lab: multi-day crypto strategies under a locked protocol.

Positions are decided at the daily close (00:00 UTC) from finished days only and
held from that day's open to the next: P&L_d = $10,000 x [pos_d x (open_{d+1} /
open_d - 1) - |pos_d - pos_{d-1}| x cost_side - pos_d x funding_d].
cost_side = Delta India taker 0.05% + 18% GST (5.9 bp) plus slippage 2 bp (BTC,
ETH), 4 bp (SOL XRP DOGE BNB) or 5 bp (ADA LTC LINK TRX XLM EOS).
funding_d = the real Binance perpetual funding prints of that day (signed: a
long pays positive funding); before a coin's perpetual existed, 0.01% per 8 h.
Prices: Binance spot daily candles (longest consistent history).

Protocol (fixed before the first run; fingerprint in results/prereg.json):
  discovery 2018-01-01 .. 2022-12-31, every variant
  eligible  >= 30 entries, net profit, daily t >= 2.0, and profit in all but
            at most one of the discovery years in which it held positions
  finalist  the eligible variant with the highest discovery t in each family
  hold-out  2023-01-01 .. 2026-10-31, finalists only. PASS = net profit, daily
            t >= 2.0, profit in both halves (Jan 2023-Nov 2024, Dec 2024-Oct
            2026) and, for single-coin families, profit on >= 4 of 6 coins.
The hold-out includes Oct 2025 - Oct 2026, whose BTC prices earlier studies saw.

Usage: python -I sw_lab.py
"""
import hashlib
import json
import math
import os
import sys

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
DATA = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "swing_lab"))
RES = os.path.join(HERE, "results")
COINS6 = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"]
COINS12 = COINS6 + ["ADAUSDT", "LTCUSDT", "LINKUSDT", "TRXUSDT", "XLMUSDT", "EOSUSDT"]
TAKER = 0.00059
SLIP = {"BTCUSDT": 0.0002, "ETHUSDT": 0.0002, **{c: 0.0004 for c in ["SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"]},
        **{c: 0.0005 for c in ["ADAUSDT", "LTCUSDT", "LINKUSDT", "TRXUSDT", "XLMUSDT", "EOSUSDT"]}}
SYNTH_DAY = 0.0003
N = 10_000.0
D = lambda s: pd.Timestamp(s)
DISC = (D("2018-01-01"), D("2023-01-01"))
HOLD = (D("2023-01-01"), D("2026-11-01"))
HALVES = [(D("2023-01-01"), D("2024-12-01")), (D("2024-12-01"), D("2026-11-01"))]


class Panel:
    def __init__(self, cut=None):
        k = pd.read_pickle(os.path.join(DATA, "daily.pkl"))
        k["date"] = pd.to_datetime(k["day"], unit="s")
        if cut is not None:
            k = k[k["date"] < cut]
        idx = pd.date_range(k["date"].min(), k["date"].max(), freq="D")
        piv = lambda col: k.pivot(index="date", columns="sym", values=col).reindex(index=idx, columns=COINS12)
        self.o, self.h, self.l, self.c = (piv(x) for x in ("o", "h", "l", "c"))
        listed = (self.c.notna().cumsum() > 0) & (self.c.notna()[::-1].cumsum()[::-1] > 0)  # listing to delisting (EOS: May 2025)
        for a in ("o", "h", "l", "c"):  # interior gaps only (an exchange outage day)
            setattr(self, a, getattr(self, a).ffill().where(listed))
        self.ret = (self.o.shift(-1) / self.o - 1)          # open_d -> open_{d+1}
        f = pd.read_pickle(os.path.join(DATA, "funding.pkl"))
        f["date"] = pd.to_datetime((f["t"] - 1) // 86400 * 86400, unit="s")  # a print at 00:00 belongs to the day before
        if cut is not None:
            f = f[f["date"] < cut]
        real = f.pivot_table(index="date", columns="sym", values="rate", aggfunc="sum").reindex(index=idx, columns=COINS12)
        first_real = real.notna().cumsum() > 0
        self.funding_real = real.where(first_real)            # NaN before the perpetual existed
        self.funding = real.fillna(0.0).where(first_real, SYNTH_DAY)
        self.synthetic = ~first_real
        self.cost = pd.Series(SLIP).reindex(COINS12) + TAKER

    def pnl(self, pos):
        """Daily $ P&L per coin for a position frame (columns subset of COINS12)."""
        cols = pos.columns
        pos = pos.reindex(self.c.index).fillna(0.0).where(self.c[cols].notna(), 0.0)
        r = self.ret[cols].fillna(0.0)
        turn = pos.diff().abs().fillna(pos.abs())
        return N * (pos * r - turn * self.cost[cols] - pos * self.funding[cols])


# ── families (each returns a position frame; row d uses data through d-1) ──────
def lo(x, mode):
    return x.clip(lower=0) if mode == "LO" else x


def sma_trend(P, n, mode):
    c = P.c[COINS6]
    return lo(np.sign(c - c.rolling(n).mean()).shift(1), mode)


def ma_cross(P, f, s, mode):
    c = P.c[COINS6]
    return lo(np.sign(c.rolling(f).mean() - c.rolling(s).mean()).shift(1), mode)


def donchian(P, n_in, n_out, mode):
    c, h, l = P.c[COINS6], P.h[COINS6], P.l[COINS6]
    up, dn = h.rolling(n_in).max().shift(1), l.rolling(n_in).min().shift(1)
    xu, xd = l.rolling(n_out).min().shift(1), h.rolling(n_out).max().shift(1)
    out = pd.DataFrame(0.0, index=c.index, columns=COINS6)
    for s in COINS6:
        pos, res = 0, np.zeros(len(c))
        cc, uu, dd, xuu, xdd = (a[s].to_numpy() for a in (c, up, dn, xu, xd))
        for i in range(len(cc)):
            res[i] = pos                                         # held today, decided yesterday
            if np.isnan(cc[i]) or np.isnan(uu[i]):
                continue
            if pos == 1 and cc[i] < xuu[i]:
                pos = 0
            elif pos == -1 and cc[i] > xdd[i]:
                pos = 0
            if pos == 0:
                if cc[i] > uu[i]:
                    pos = 1
                elif cc[i] < dd[i] and mode == "LS":
                    pos = -1
        out[s] = res
    return out


def tsmom(P, L, reb, mode):
    c = P.c[COINS6]
    sig = np.sign(c / c.shift(L) - 1).shift(1)
    if reb == "weekly":
        sig = sig.copy()
        sig.loc[sig.index.dayofweek != 0] = np.nan  # keep Monday decisions, hold them all week
        sig = sig.ffill()
    return lo(sig, mode)


def cross(P, score, k, reb, flip=False, coins=COINS12):
    """Long the k highest scores, short the k lowest (flip = the reverse)."""
    sc = score[coins].shift(1)
    out = pd.DataFrame(0.0, index=sc.index, columns=coins)
    last = None
    for i, d in enumerate(sc.index):
        if reb == "daily" or d.dayofweek == 0 or last is None:
            row = sc.iloc[i].dropna()
            if len(row) >= 2 * k + 1:
                order = row.sort_values(ascending=False, kind="stable").index  # ties keep coin order
                last = {s: 0.0 for s in coins}
                for s in order[:k]:
                    last[s] = -1.0 if flip else 1.0
                for s in order[-k:]:
                    last[s] = 1.0 if flip else -1.0
            else:
                last = None
        if last:
            out.iloc[i] = [last[s] for s in coins]
    return out


def cs_mom(P, L, k):
    return cross(P, P.c / P.c.shift(L) - 1, k, "weekly")


def cs_rev(P, L, k):
    return cross(P, P.c / P.c.shift(L) - 1, k, "daily" if L < 7 else "weekly", flip=True)


def carry(P, days, k, reb):
    fr = P.funding_real.rolling(days, min_periods=days).mean()
    return cross(P, fr, k, reb, flip=True)  # short the highest funding, long the lowest


def rsi2(P, thr, filt, mode):
    c = P.c[COINS6]
    d = c.diff()
    up = d.clip(lower=0).ewm(alpha=0.5, adjust=False).mean()
    dn = (-d.clip(upper=0)).ewm(alpha=0.5, adjust=False).mean()
    r = 100 - 100 / (1 + up / dn.replace(0, np.nan))
    sma = c.rolling(200).mean()
    out = pd.DataFrame(0.0, index=c.index, columns=COINS6)
    for s in COINS6:
        rr, cc, mm = r[s].to_numpy(), c[s].to_numpy(), sma[s].to_numpy()
        pos, age, res = 0, 0, np.zeros(len(cc))
        for i in range(len(cc)):
            res[i] = pos
            if np.isnan(rr[i]) or (filt == "sma200" and np.isnan(mm[i])):
                continue
            if pos != 0:
                age += 1
                if (pos == 1 and rr[i] > 70) or (pos == -1 and rr[i] < 30) or age >= 5:
                    pos, age = 0, 0
            if pos == 0:
                up_ok = filt == "none" or cc[i] > mm[i]
                dn_ok = filt == "none" or cc[i] < mm[i]
                if rr[i] < thr and up_ok:
                    pos, age = 1, 0
                elif rr[i] > 100 - thr and dn_ok and mode == "LS":
                    pos, age = -1, 0
        out[s] = res
    return out


def day1(P, kk, mode):
    c = P.c[COINS6]
    r = c / c.shift(1) - 1
    sd = r.rolling(20).std()
    sig = np.sign(r).where(r.abs() > kk * sd, 0.0)
    return (sig if mode == "mom" else -sig).shift(1)


FAMILIES = {
    "T1 Price vs moving average": [(f"T1|{n}|{m}", sma_trend, (n, m)) for n in (20, 50, 100, 200) for m in ("LO", "LS")],
    "T2 Moving-average crossover": [(f"T2|{f}/{s}|{m}", ma_cross, (f, s, m)) for f, s in ((10, 50), (20, 100), (50, 200)) for m in ("LO", "LS")],
    "T3 Donchian breakout": [(f"T3|{a}/{b}|{m}", donchian, (a, b, m)) for a, b in ((20, 10), (55, 20), (100, 50)) for m in ("LO", "LS")],
    "T4 Time-series momentum": [(f"T4|{L}|{r}|{m}", tsmom, (L, r, m)) for L in (7, 14, 28, 56, 112) for r in ("daily", "weekly") for m in ("LO", "LS")],
    "X1 Cross-coin momentum": [(f"X1|{L}|{k}", cs_mom, (L, k)) for L in (7, 14, 28, 56) for k in (1, 2, 3)],
    "X2 Cross-coin reversal": [(f"X2|{L}|{k}", cs_rev, (L, k)) for L in (1, 3, 7) for k in (1, 2, 3)],
    "X3 Funding carry": [(f"X3|{d}|{k}|{r}", carry, (d, k, r)) for d in (1, 7) for k in (1, 2) for r in ("daily", "weekly")],
    "M1 Daily RSI(2) dip-buying": [(f"M1|{t}|{f}|{m}", rsi2, (t, f, m)) for t in (5, 10) for f in ("sma200", "none") for m in ("LO", "LS")],
    "S1 One-day momentum / reversal": [(f"S1|{k}|{m}", day1, (k, m)) for k in (0.0, 0.5, 1.0) for m in ("mom", "rev")],
}


def capital(vid, pos):
    if vid[0] == "X":
        k = int(vid.split("|")[2])
        return N * 2 * k
    return N * len(pos.columns)


def stats(pnl_frame, w, cap):
    p = pnl_frame[(pnl_frame.index >= w[0]) & (pnl_frame.index < w[1])]
    d = p.sum(axis=1)
    sd = d.std(ddof=1)
    cum = d.cumsum()
    years = len(d) / 365.25
    return {
        "pnl": round(float(d.sum()), 2), "return_pct": round(float(d.sum() / cap * 100), 2),
        "annual_pct": round(float(d.sum() / cap * 100 / years), 2) if years else None,
        "t_daily": round(float(d.mean() / sd * math.sqrt(len(d))), 2) if sd > 0 else None,
        "max_dd_pct": round(float((cum.cummax().clip(lower=0) - cum).max() / cap * 100), 2),
        "by_year_pct": {str(y): round(float(g.sum() / cap * 100), 2) for y, g in d.groupby(d.index.year)},
        "by_coin": {c: round(float(p[c].sum()), 2) for c in p.columns if p[c].abs().sum() > 0},
    }


def entries(pos, w):
    p = pos[(pos.index >= w[0]) & (pos.index < w[1])].fillna(0.0)
    prev = p.shift(1).fillna(0.0)
    return int(((p != 0) & (p != prev)).sum().sum())


def fingerprint():
    return hashlib.sha256(open(os.path.join(HERE, "sw_lab.py"), "rb").read().replace(b"\r\n", b"\n")).hexdigest()[:16]


def norm_cdf(x):
    return 0.5 * math.erfc(-x / math.sqrt(2))


def main():
    os.makedirs(RES, exist_ok=True)
    fp = fingerprint()
    pre = os.path.join(RES, "prereg.json")
    if os.path.exists(pre):
        old = json.load(open(pre, encoding="utf-8"))
        allowed = ([a["to"] for a in old.get("amendments", [])] or [old["fingerprint"]])[-1]
        if allowed != fp:
            sys.exit(f"code changed since pre-registration ({allowed} -> {fp}); log an amendment first")
    else:
        json.dump({"fingerprint": fp, "registered_utc": pd.Timestamp.now(tz="UTC").isoformat(timespec="seconds"),
                   "protocol": __doc__.split("Usage:")[0].strip(),
                   "variants": {f: [v for v, _, _ in vs] for f, vs in FAMILIES.items()}},
                  open(pre, "w", encoding="utf-8"), indent=1)
    P = Panel()
    disc, store = {}, {}
    for fam, vs in FAMILIES.items():
        for vid, fn, args in vs:
            pos = fn(P, *args).fillna(0.0)
            pnl = P.pnl(pos)
            cap = capital(vid, pos)
            st = stats(pnl, DISC, cap)
            yrs = {y: v for y, v in st["by_year_pct"].items()
                   if pos[(pos.index.year == int(y))].abs().sum().sum() > 0}
            losing = sum(v <= 0 for v in yrs.values())
            st["entries"] = entries(pos, DISC)
            st["eligible"] = bool(st["entries"] >= 30 and st["pnl"] > 0 and (st["t_daily"] or 0) >= 2.0 and losing <= 1)
            disc[vid] = (fam, st)
            store[vid] = (pos, pnl, cap)
        print(f"{fam:<34} {len(vs):3d} variants, eligible {sum(disc[v][1]['eligible'] for v, _, _ in vs)}", flush=True)
    finals = []
    for fam, vs in FAMILIES.items():
        el = sorted([v for v, _, _ in vs if disc[v][1]["eligible"]], key=lambda v: -disc[v][1]["t_daily"])
        if el:
            finals.append(el[0])
    out = []
    for v in finals:
        pos, pnl, cap = store[v]
        h = stats(pnl, HOLD, cap)
        halves = [stats(pnl, w, cap)["pnl"] for w in HALVES]
        coins_pos = sum(x > 0 for x in h["by_coin"].values())
        single = v[0] != "X"
        passed = bool(h["pnl"] > 0 and (h["t_daily"] or 0) >= 2.0 and all(x > 0 for x in halves) and (coins_pos >= 4 or not single))
        full = stats(pnl, (DISC[0], HOLD[1]), cap)
        out.append({"variant": v, "family": disc[v][0], "capital": cap, "discovery": disc[v][1], "holdout": h,
                    "holdout_halves": halves, "holdout_coins_positive": coins_pos, "pass": passed, "full": full,
                    "holdout_entries": entries(pos, HOLD),
                    "equity": {str(k.date()): round(float(x), 2) for k, x in pnl.sum(axis=1)[pnl.index >= DISC[0]].cumsum()[::7].items()}})
        print(f"{v:<22} disc {disc[v][1]['annual_pct']:6.1f}%/yr t {disc[v][1]['t_daily']:5.2f} | hold {h['annual_pct']:6.1f}%/yr "
              f"t {h['t_daily']} halves {halves} coins+ {coins_pos} {'PASS' if passed else 'fail'} | DD {h['max_dd_pct']}%", flush=True)
    # benchmarks: always long, same costs and funding
    bench = {}
    for name, cols in (("BTC buy and hold", ["BTCUSDT"]), ("6-coin buy and hold", COINS6)):
        pos = pd.DataFrame(1.0, index=P.c.index, columns=cols).where(P.c[cols].notna(), 0.0)
        pnl = P.pnl(pos)
        bench[name] = {"discovery": stats(pnl, DISC, N * len(cols)), "holdout": stats(pnl, HOLD, N * len(cols))}
    fams = {f: {"variants": len(vs), "disc_positive": sum(disc[v][1]["pnl"] > 0 for v, _, _ in vs),
                "eligible": sum(disc[v][1]["eligible"] for v, _, _ in vs)} for f, vs in FAMILIES.items()}
    allv = [{"variant": v, "family": f, **{k: st[k] for k in ("pnl", "annual_pct", "t_daily", "max_dd_pct", "entries", "eligible", "by_year_pct")}}
            for v, (f, st) in disc.items()]
    json.dump({"fingerprint": fp, "families": fams, "finalists": out, "benchmarks": bench, "discovery_all": allv,
               "synthetic_funding_share_disc": round(float(P.synthetic[COINS6][(P.synthetic.index >= DISC[0]) & (P.synthetic.index < DISC[1])].mean().mean()), 3)},
              open(os.path.join(RES, "summary.json"), "w", encoding="utf-8"), indent=1, default=str)
    print(json.dumps(fams))
    for k, v in bench.items():
        print(k, "disc", v["discovery"]["annual_pct"], "%/yr | hold", v["holdout"]["annual_pct"], "%/yr DD", v["holdout"]["max_dd_pct"])


if __name__ == "__main__":
    sys.exit(main())
