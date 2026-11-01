"""Walk-forward lab: multi-day (daily-bar) crypto strategies, chosen on January-June 2026 and then tested,
unchanged, on July-October 2026. Protocol fixed before the first run (fingerprint in results/prereg.json).

Market: the 16 USDT perpetuals of the October search (BTC ETH SOL XRP DOGE BNB ADA LINK AVAX LTC DOT TRX BCH
NEAR SUI APT). Positions are decided at the daily close (00:00 UTC) from finished days only and held from that
day's open to the next:
  P&L_d = $10,000 x [pos_d x (open_{d+1} / open_d - 1) - |pos_d - pos_{d-1}| x cost_side - pos_d x funding_d]
cost_side = Delta India taker 0.05% + 18% GST (5.9 bp) plus slippage 2 bp (BTC, ETH), 4 bp (SOL XRP DOGE BNB)
or 5 bp (the other 10); funding_d = that day's real Binance funding prints (a long pays positive funding).
No leverage: $10,000 per coin (capital $160,000), or $10,000 per position for the cross-coin families
(capital 2 x k x $10,000). Indicators warm up on July 2024 - December 2025.

Choose (2026-01-01 .. 2026-06-30), every variant: CHOSEN = net profit, profit in at least 4 of the 6 months,
at least 6 entries, worst drawdown at most 25% of capital.
Finalists: in each family, the chosen variant with the highest daily t-statistic over January-June.
Test (2026-07-01 .. 2026-10-31), finalists only, rules unchanged: PASS = net profit over July-October and
profit in at least 3 of the 4 months. Also reported: the test result of every chosen variant against the
variants that were not chosen, and buying and holding all 16 coins with the same costs and funding.

Usage: python -I wf_lab.py
"""
import hashlib
import json
import math
import os
import sys

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
DATA = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "walkforward"))
RES = os.path.join(HERE, "results")
MAIN = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"]
EXTRA = ["ADAUSDT", "LINKUSDT", "AVAXUSDT", "LTCUSDT", "DOTUSDT", "TRXUSDT", "BCHUSDT", "NEARUSDT", "SUIUSDT", "APTUSDT"]
COINS = MAIN + EXTRA
TAKER = 0.00059
SLIP = {"BTCUSDT": 0.0002, "ETHUSDT": 0.0002, **{c: 0.0004 for c in MAIN[2:]}, **{c: 0.0005 for c in EXTRA}}
N = 10_000.0
D = pd.Timestamp
CHOOSE = (D("2026-01-01"), D("2026-07-01"))
TEST = (D("2026-07-01"), D("2026-11-01"))


class Panel:
    def __init__(self, cut=None):
        k = pd.read_pickle(os.path.join(DATA, "daily.pkl"))
        k["date"] = pd.to_datetime(k["day"], unit="s")
        if cut is not None:
            k = k[k["date"] < cut]
        idx = pd.date_range(k["date"].min(), k["date"].max(), freq="D")
        piv = lambda col: k.pivot(index="date", columns="sym", values=col).reindex(index=idx, columns=COINS)
        self.o, self.h, self.l, self.c, self.v = (piv(x) for x in ("o", "h", "l", "c", "v"))
        self.ret = self.o.shift(-1) / self.o - 1                  # open_d -> open_{d+1}
        self.ret.iloc[-1] = self.c.iloc[-1] / self.o.iloc[-1] - 1  # last day: to its close
        f = pd.read_pickle(os.path.join(DATA, "funding.pkl"))
        f["date"] = pd.to_datetime((f["t"] - 1) // 86400 * 86400, unit="s")  # a print at 00:00 belongs to the day before
        if cut is not None:
            f = f[f["date"] < cut]
        self.funding_real = f.pivot_table(index="date", columns="sym", values="rate", aggfunc="sum").reindex(index=idx, columns=COINS)
        self.funding = self.funding_real.fillna(0.0)
        self.cost = pd.Series(SLIP).reindex(COINS) + TAKER

    def pnl(self, pos):
        """Daily $ P&L per coin for a position frame."""
        cols = pos.columns
        pos = pos.reindex(self.c.index).fillna(0.0)
        turn = pos.diff().abs().fillna(pos.abs())
        return N * (pos * self.ret[cols].fillna(0.0) - turn * self.cost[cols] - pos * self.funding[cols])


# ── families (each returns a position frame; row d uses data through d-1) ──────
def mode_of(x, mode):
    return x.clip(lower=0) if mode == "LO" else (x.clip(upper=0) if mode == "SO" else x)


def sma_trend(P, n, mode):
    c = P.c
    return mode_of(np.sign(c - c.rolling(n).mean()).shift(1), mode)


def ma_cross(P, f, s, mode):
    c = P.c
    return mode_of(np.sign(c.rolling(f).mean() - c.rolling(s).mean()).shift(1), mode)


def ema_cross(P, f, s, mode):
    c = P.c
    return mode_of(np.sign(c.ewm(span=f, adjust=False).mean() - c.ewm(span=s, adjust=False).mean()).shift(1), mode)


def donchian(P, n_in, n_out, mode):
    c, h, l = P.c, P.h, P.l
    up, dn = h.rolling(n_in).max().shift(1), l.rolling(n_in).min().shift(1)
    xu, xd = l.rolling(n_out).min().shift(1), h.rolling(n_out).max().shift(1)
    out = pd.DataFrame(0.0, index=c.index, columns=COINS)
    for s in COINS:
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
                if cc[i] > uu[i] and mode != "SO":
                    pos = 1
                elif cc[i] < dd[i] and mode != "LO":
                    pos = -1
        out[s] = res
    return out


def tsmom(P, L, reb, mode):
    c = P.c
    sig = np.sign(c / c.shift(L) - 1).shift(1)
    if reb == "weekly":
        sig = sig.copy()
        sig.loc[sig.index.dayofweek != 0] = np.nan  # Monday decisions, held all week
        sig = sig.ffill()
    return mode_of(sig, mode)


def rsi_of(c, n):
    d = c.diff()
    up = d.clip(lower=0).ewm(alpha=1 / n, adjust=False).mean()
    dn = (-d.clip(upper=0)).ewm(alpha=1 / n, adjust=False).mean()
    return 100 - 100 / (1 + up / dn.replace(0, np.nan))


def state_machine(P, enter_l, enter_s, exit_l, exit_s, max_age, mode):
    """Positions from per-day entry/exit conditions (decided at each close, held from the next day)."""
    out = pd.DataFrame(0.0, index=P.c.index, columns=COINS)
    for s in COINS:
        el, es, xl, xs = (a[s].to_numpy() for a in (enter_l, enter_s, exit_l, exit_s))
        pos, age, res = 0, 0, np.zeros(len(el))
        for i in range(len(el)):
            res[i] = pos
            if pos != 0:
                age += 1
                if (pos == 1 and xl[i]) or (pos == -1 and xs[i]) or age >= max_age:
                    pos, age = 0, 0
            if pos == 0:
                if el[i] and mode != "SO":
                    pos, age = 1, 0
                elif es[i] and mode == "LS":
                    pos, age = -1, 0
        out[s] = res
    return out


def rsi_rev(P, n, thr, ex, max_age, filt, mode):
    c = P.c
    r = rsi_of(c, n)
    sma = c.rolling(200).mean()
    up_ok = (c > sma) if filt == "sma200" else pd.DataFrame(True, index=c.index, columns=COINS)
    dn_ok = (c < sma) if filt == "sma200" else pd.DataFrame(True, index=c.index, columns=COINS)
    return state_machine(P, (r < thr) & up_ok, (r > 100 - thr) & dn_ok, r > ex, r < 100 - ex, max_age, mode)


def bb_rev(P, n, k, max_age, mode):
    c = P.c
    m, sd = c.rolling(n).mean(), c.rolling(n).std(ddof=0)
    return state_machine(P, c < m - k * sd, c > m + k * sd, c >= m, c <= m, max_age, mode)


def nday_rev(P, n, hold, mode):
    c = P.c
    lo_, hi_ = c <= c.rolling(n).min(), c >= c.rolling(n).max()
    never = pd.DataFrame(False, index=c.index, columns=COINS)
    return state_machine(P, lo_, hi_, never, never, hold, mode)


def cross(P, score, k, reb, flip=False):
    """Long the k highest scores, short the k lowest (flip = the reverse)."""
    sc = score[COINS].shift(1)
    out = pd.DataFrame(0.0, index=sc.index, columns=COINS)
    last = None
    for i, d in enumerate(sc.index):
        if reb == "daily" or d.dayofweek == 0 or last is None:
            row = sc.iloc[i].dropna()
            if len(row) >= 2 * k + 1:
                order = row.sort_values(ascending=False, kind="stable").index  # ties keep coin order
                last = {s: 0.0 for s in COINS}
                for s in order[:k]:
                    last[s] = -1.0 if flip else 1.0
                for s in order[-k:]:
                    last[s] = 1.0 if flip else -1.0
            else:
                last = None
        if last:
            out.iloc[i] = [last[s] for s in COINS]
    return out


def cs_mom(P, L, k, reb):
    return cross(P, P.c / P.c.shift(L) - 1, k, reb)


def cs_rev(P, L, k, reb):
    return cross(P, P.c / P.c.shift(L) - 1, k, reb, flip=True)


def carry(P, days, k, reb):
    return cross(P, P.funding_real.rolling(days, min_periods=days).mean(), k, reb, flip=True)  # short the highest funding


def day1(P, kk, mode):
    c = P.c
    r = c / c.shift(1) - 1
    sig = np.sign(r).where(r.abs() > kk * r.rolling(20).std(), 0.0)
    return (sig if mode == "mom" else -sig).shift(1)


def btc_regime(P, n, mode):
    b = P.c["BTCUSDT"]
    sig = np.sign(b - b.rolling(n).mean()).shift(1)
    return mode_of(pd.DataFrame({s: sig for s in COINS}), mode)


def vol_trend(P, n, mode, vol):
    c = P.c
    rv = np.log(c).diff().rolling(20).std()
    med = rv.rolling(180, min_periods=120).median()
    cond = ((rv < med) if vol == "low" else (rv > med)).shift(1, fill_value=False).astype(bool)
    return sma_trend(P, n, mode).where(cond, 0.0)


def weekday(P, wd, side):
    on = pd.Series(P.c.index.dayofweek == wd, index=P.c.index).astype(float) * (1.0 if side == "long" else -1.0)
    return pd.DataFrame({s: on for s in COINS})


MODES = ("LO", "LS", "SO")
FAMILIES = {
    "T1 Price vs moving average": [(f"T1|{n}|{m}", sma_trend, (n, m)) for n in (5, 10, 15, 20, 30, 50, 75, 100, 150, 200) for m in MODES],
    "T2 Moving-average crossover": [(f"T2|{f}/{s}|{m}", ma_cross, (f, s, m)) for f, s in ((5, 20), (5, 50), (10, 30), (10, 50), (20, 50),
                                                                                      (20, 100), (50, 100), (50, 200)) for m in MODES],
    "T3 EMA crossover": [(f"T3|{f}/{s}|{m}", ema_cross, (f, s, m)) for f, s in ((8, 21), (9, 30), (12, 26), (20, 50)) for m in MODES],
    "T4 Donchian breakout": [(f"T4|{a}/{b}|{m}", donchian, (a, b, m)) for a, b in ((10, 5), (20, 10), (30, 15), (55, 20), (100, 50)) for m in MODES],
    "T5 Time-series momentum": [(f"T5|{L}|{r}|{m}", tsmom, (L, r, m)) for L in (3, 5, 7, 10, 14, 21, 28, 56, 90) for r in ("daily", "weekly")
                                for m in MODES],
    "M1 RSI dip-buying": [(f"M1|{n}|{t}|{x}|{f}|{m}", rsi_rev, (n, t, x, 5, f, m)) for n, t in ((2, 10), (2, 20), (3, 15), (5, 25), (14, 30))
                          for x in (50, 70) for f in ("none", "sma200") for m in ("LO", "LS")],
    "M2 Bollinger mean reversion": [(f"M2|{n}|{k}|{a}|{m}", bb_rev, (n, k, a, m)) for n in (10, 20) for k in (1.5, 2.0, 2.5) for a in (5, 10)
                                    for m in ("LO", "LS")],
    "M3 N-day low/high reversal": [(f"M3|{n}|{h}|{m}", nday_rev, (n, h, m)) for n in (3, 5, 10, 20) for h in (1, 2, 3, 5) for m in ("LO", "LS")],
    "X1 Cross-coin momentum": [(f"X1|{L}|{k}|{r}", cs_mom, (L, k, r)) for L in (3, 7, 14, 28, 56) for k in (1, 2, 3, 4) for r in ("daily", "weekly")],
    "X2 Cross-coin reversal": [(f"X2|{L}|{k}|{r}", cs_rev, (L, k, r)) for L in (1, 2, 3, 5, 7) for k in (1, 2, 3, 4) for r in ("daily", "weekly")],
    "X3 Funding carry": [(f"X3|{d}|{k}|{r}", carry, (d, k, r)) for d in (1, 3, 7, 14) for k in (1, 2, 3, 4) for r in ("daily", "weekly")],
    "S1 One-day momentum / reversal": [(f"S1|{k}|{m}", day1, (k, m)) for k in (0.0, 0.5, 1.0, 1.5, 2.0) for m in ("mom", "rev")],
    "B1 BTC trend steers all coins": [(f"B1|{n}|{m}", btc_regime, (n, m)) for n in (10, 20, 50, 100, 200) for m in MODES],
    "V1 Trend with a volatility filter": [(f"V1|{n}|{m}|{v}", vol_trend, (n, m, v)) for n in (20, 50) for m in ("LO", "LS") for v in ("low", "high")],
    "D1 Day of the week": [(f"D1|{w}|{s}", weekday, (w, s)) for w in range(7) for s in ("long", "short")],
}


def capital(vid, pos):
    if vid[0] == "X":
        return N * 2 * int(vid.split("|")[2])
    return N * len(pos.columns)


def stats(pnl_frame, w, cap):
    p = pnl_frame[(pnl_frame.index >= w[0]) & (pnl_frame.index < w[1])]
    d = p.sum(axis=1)
    sd = d.std(ddof=1)
    cum = d.cumsum()
    months = {str(m): round(float(g.sum()), 2) for m, g in d.groupby(d.index.to_period("M"))}
    return {"pnl": round(float(d.sum()), 2), "return_pct": round(float(d.sum() / cap * 100), 2),
            "t_daily": round(float(d.mean() / sd * math.sqrt(len(d))), 2) if sd > 0 else None,
            "max_dd_pct": round(float((cum.cummax().clip(lower=0) - cum).max() / cap * 100), 2),
            "months": months, "months_up": int(sum(v > 0 for v in months.values())),
            "by_coin": {c: round(float(p[c].sum()), 2) for c in p.columns if p[c].abs().sum() > 0}}


def entries(pos, w):
    p = pos[(pos.index >= w[0]) & (pos.index < w[1])].fillna(0.0)
    prev = pos.shift(1).fillna(0.0)[(pos.index >= w[0]) & (pos.index < w[1])]
    return int(((p != 0) & (p != prev)).sum().sum())


def fingerprint():
    return hashlib.sha256(open(os.path.join(HERE, "wf_lab.py"), "rb").read().replace(b"\r\n", b"\n")).hexdigest()[:16]


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
    rows = {}
    for fam, vs in FAMILIES.items():
        for vid, fn, args in vs:
            pos = fn(P, *args).fillna(0.0)
            pnl = P.pnl(pos)
            cap = capital(vid, pos)
            ch, te = stats(pnl, CHOOSE, cap), stats(pnl, TEST, cap)
            ch["entries"], te["entries"] = entries(pos, CHOOSE), entries(pos, TEST)
            chosen = bool(ch["pnl"] > 0 and ch["months_up"] >= 4 and ch["entries"] >= 6 and ch["max_dd_pct"] <= 25)
            rows[vid] = {"variant": vid, "family": fam, "capital": cap, "choose": ch, "test": te, "chosen": chosen}
        n_ch = sum(rows[v]["chosen"] for v, _, _ in vs)
        print(f"{fam:<36} {len(vs):3d} variants, chosen {n_ch}", flush=True)
    finals = []
    for fam, vs in FAMILIES.items():
        ch = sorted([v for v, _, _ in vs if rows[v]["chosen"]], key=lambda v: -(rows[v]["choose"]["t_daily"] or 0))
        if ch:
            finals.append(ch[0])
    for v in finals:
        te = rows[v]["test"]
        rows[v]["finalist"] = True
        rows[v]["pass"] = bool(te["pnl"] > 0 and te["months_up"] >= 3)
        print(f"{v:<26} choose {rows[v]['choose']['return_pct']:+7.2f}% t {rows[v]['choose']['t_daily']} up {rows[v]['choose']['months_up']}/6 | "
              f"test {te['return_pct']:+7.2f}% up {te['months_up']}/4 {'PASS' if rows[v]['pass'] else 'fail'}", flush=True)
    bench = {}
    pos = pd.DataFrame(1.0, index=P.c.index, columns=COINS)
    pnl = P.pnl(pos)
    bench["Buy and hold all 16 coins"] = {"choose": stats(pnl, CHOOSE, N * 16), "test": stats(pnl, TEST, N * 16)}
    pos = pd.DataFrame(1.0, index=P.c.index, columns=["BTCUSDT"])
    pnl = P.pnl(pos)
    bench["Buy and hold BTC"] = {"choose": stats(pnl, CHOOSE, N), "test": stats(pnl, TEST, N)}
    chosen = [r for r in rows.values() if r["chosen"]]
    others = [r for r in rows.values() if not r["chosen"]]
    share = lambda rs: round(100 * sum(r["test"]["pnl"] > 0 for r in rs) / len(rs), 1) if rs else None
    med = lambda rs: round(float(np.median([r["test"]["return_pct"] for r in rs])), 2) if rs else None
    selection = {"chosen": len(chosen), "not_chosen": len(others), "chosen_test_profitable_pct": share(chosen),
                 "not_chosen_test_profitable_pct": share(others), "chosen_test_median_pct": med(chosen), "not_chosen_test_median_pct": med(others),
                 "rank_corr_choose_vs_test": round(float(pd.Series({v: r["choose"]["return_pct"] for v, r in rows.items()}).rank().corr(
                     pd.Series({v: r["test"]["return_pct"] for v, r in rows.items()}).rank())), 3)}
    json.dump({"fingerprint": fp, "windows": {"choose": [str(CHOOSE[0].date()), str(CHOOSE[1].date())], "test": [str(TEST[0].date()), str(TEST[1].date())]},
               "families": {f: {"variants": len(vs), "chosen": sum(rows[v]["chosen"] for v, _, _ in vs)} for f, vs in FAMILIES.items()},
               "finalists": [rows[v] for v in finals], "benchmarks": bench, "selection": selection, "all": list(rows.values())},
              open(os.path.join(RES, "summary.json"), "w", encoding="utf-8"), indent=1, default=str)
    print(json.dumps(selection))
    for k, v in bench.items():
        print(k, "choose", v["choose"]["return_pct"], "% | test", v["test"]["return_pct"], "%")
    return 0


if __name__ == "__main__":
    sys.exit(main())
