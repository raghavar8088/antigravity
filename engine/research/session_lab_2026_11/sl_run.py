"""Session-strategy lab: discovery, selection, hold-out.

Protocol (fixed before the first run, fingerprinted in results/prereg.json):
  discovery  2021-01-01 .. 2023-12-31  all 880 variants
  eligible   >= 150 trades, net profit in EACH of 2021, 2022 and 2023, and a
             daily-P&L t-statistic >= 2.0
  finalists  per family, the 3 eligible variants with the highest discovery t,
             skipping one whose daily P&L correlates > 0.7 with a finalist
             already chosen
  hold-out   2024-01-01 .. 2025-09-30, never looked at before this run; only
             finalists are scored. PASS = net profit, daily t >= the one-sided
             5% Bonferroni bar for the number of finalists, profit in both
             halves (Jan-Oct 2024, Nov 2024-Sep 2025), and (single-coin
             families) profit on at least 4 of the 6 coins
  recent     2025-10-01 .. 2026-10-31 reported for every finalist (this window
             was used by the previous study, so it is not part of the pass rule)
Daily P&L = sum over coins of the day's trades (by entry day), $10,000 per trade.

Usage: python -I sl_run.py
"""
import hashlib
import json
import math
import os
import sys
import time
from concurrent.futures import ProcessPoolExecutor

import numpy as np
import pandas as pd

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from sl_core import DATA, DAY, H, MAKER, NOTIONAL, SLIP, SYMBOLS, TAKER, Market, run_orders  # noqa: E402
import sl_families as F  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results")
ts_ = lambda s: int(pd.Timestamp(s, tz="UTC").timestamp())
DISC = (ts_("2021-01-01"), ts_("2024-01-01"))
HOLD = (ts_("2024-01-01"), ts_("2025-10-01"))
HOLD_HALVES = [(ts_("2024-01-01"), ts_("2024-11-01")), (ts_("2024-11-01"), ts_("2025-10-01"))]
RECENT = (ts_("2025-10-01"), ts_("2026-11-01"))
YEARS = [(y, ts_(f"{y}-01-01"), ts_(f"{y + 1}-01-01")) for y in (2021, 2022, 2023)]


def fingerprint():
    h = hashlib.sha256()
    for f in ("sl_core.py", "sl_families.py", "sl_run.py"):
        h.update(open(os.path.join(HERE, f), "rb").read().replace(b"\r\n", b"\n"))
    return h.hexdigest()[:16]


def norm_ppf(p):
    lo, hi = -10.0, 10.0
    for _ in range(100):
        mid = (lo + hi) / 2
        if 0.5 * math.erfc(-mid / math.sqrt(2)) < p:
            lo = mid
        else:
            hi = mid
    return (lo + hi) / 2


def coin_worker(sym):
    """All non-cross-sectional variants on one coin, plus the price points family C needs."""
    mk = Market(sym)
    f = F.Features(mk)
    out = {}
    for vid in F.variant_ids():
        if vid.startswith("C|"):
            continue
        tr = run_orders(mk, F.orders(f, vid))
        out[vid] = np.array([(a, b, c, d, e) for a, b, c, d, e, _ in tr], dtype=float).reshape(-1, 5)
    # inputs for the cross-sectional family: prices at 04/08/12 UTC and at day boundaries
    days = f.days
    pts = {"days": days, "bad": np.array([int(d) in mk.bad_days for d in days])}
    for x in (4, 8, 12):
        i = np.minimum(mk.idx(days + x * H), mk.n - 1)
        pts[f"o{x}"] = mk.o[i]
        pts[f"c{x}"] = mk.c[i - 1]
        pts[f"c{x}_24"] = mk.c[np.maximum(i - 1 - 1440, 0)]
    pts["o0"] = mk.o[mk.idx(days)]
    pts["onext"] = mk.o[np.minimum(mk.idx(days + DAY), mk.n - 1)]
    pts["fund_t"], pts["fund_r"], pts["slip"] = mk.fund_t, mk.fund_r, mk.slip
    return sym, out, pts


def family_c(pts):
    """Cross-sectional baskets from the price points: {vid: {sym: trades}}."""
    syms = list(pts)
    days = pts[syms[0]]["days"]
    res = {}
    for x, basis, mode, legs in F.GRID_C:
        vid = "C|%02d|%s|%s|%d" % (x if isinstance(x, int) else int(x), basis, mode, legs)
        xi = int(x)
        per = {s: [] for s in syms}
        for k in range(15, len(days)):
            if any(pts[s]["bad"][k] for s in syms):
                continue
            ret = {s: pts[s][f"c{xi}"][k] / (pts[s]["o0"][k] if basis == "day" else pts[s][f"c{xi}_24"][k]) - 1 for s in syms}
            order = sorted(syms, key=lambda s: ret[s], reverse=True)
            top, bottom = order[:legs], order[-legs:]
            longs, shorts = (top, bottom) if mode == "mom" else (bottom, top)
            et, xt = int(days[k]) + xi * H, int(days[k]) + DAY
            for s, side in [(s, 1) for s in longs] + [(s, -1) for s in shorts]:
                p = pts[s]
                px, xp = p[f"o{xi}"][k], p["onext"][k]
                gross = side * (xp - px) / px
                fm = (p["fund_t"] > et) & (p["fund_t"] < xt)
                net = gross - 2 * (TAKER + p["slip"]) - side * float(p["fund_r"][fm].sum())
                per[s].append((et, xt, side, gross, net))
        res[vid] = {s: np.array(v, dtype=float).reshape(-1, 5) for s, v in per.items()}
    return res


def window(a, w):
    return a[(a[:, 0] >= w[0]) & (a[:, 0] < w[1])]


def daily(trades_by_coin, w):
    days = np.arange(w[0], w[1], DAY)
    pnl = np.zeros(len(days))
    for a in trades_by_coin.values():
        a = window(a, w)
        if len(a):
            np.add.at(pnl, ((a[:, 0] - w[0]) // DAY).astype(int), a[:, 4] * NOTIONAL)
    return pnl


def stats(trades_by_coin, w):
    allt = np.concatenate([window(a, w) for a in trades_by_coin.values()]) if trades_by_coin else np.zeros((0, 5))
    d = daily(trades_by_coin, w)
    n = len(allt)
    sd = d.std(ddof=1)
    pnl = float(d.sum())
    cum = np.cumsum(d)
    out = {"trades": int(n), "pnl": round(pnl, 2),
           "gross_pnl": round(float(allt[:, 3].sum() * NOTIONAL), 2) if n else 0.0,
           "t_daily": round(float(d.mean() / sd * math.sqrt(len(d))), 2) if sd > 0 else None,
           "win_rate": round(float((allt[:, 4] > 0).mean() * 100), 1) if n else None,
           "avg_net_bp": round(float(allt[:, 4].mean() * 1e4), 2) if n else None,
           "avg_gross_bp": round(float(allt[:, 3].mean() * 1e4), 2) if n else None,
           "max_dd": round(float((np.maximum.accumulate(np.concatenate([[0], cum]))[1:] - cum).max()), 2),
           "by_coin": {s: round(float(window(a, w)[:, 4].sum() * NOTIONAL), 2) for s, a in trades_by_coin.items()}}
    return out, d


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
                   "protocol": __doc__.split("Usage:")[0].strip(), "families": F.__doc__.strip(),
                   "variants": len(F.variant_ids()), "maker": MAKER, "taker": TAKER, "slip": SLIP},
                  open(pre, "w", encoding="utf-8"), indent=1)
    t0 = time.time()
    trades, pts = {}, {}
    with ProcessPoolExecutor(max_workers=3) as ex:  # ~250 MB a worker
        for sym, out, p in ex.map(coin_worker, SYMBOLS):
            for vid, a in out.items():
                trades.setdefault(vid, {})[sym] = a
            pts[sym] = p
            print(f"{sym} done ({time.time() - t0:.0f}s)", flush=True)
    trades.update(family_c(pts))
    print(f"{len(trades)} variants filled ({time.time() - t0:.0f}s)", flush=True)

    # discovery: every variant
    disc = {}
    for vid, by in trades.items():
        st, d = stats(by, DISC)
        st["years"] = {y: round(float(sum(window(a, (a0, a1))[:, 4].sum() for a in by.values()) * NOTIONAL), 2)
                       for y, a0, a1 in YEARS}
        st["eligible"] = bool(st["trades"] >= 150 and all(v > 0 for v in st["years"].values()) and (st["t_daily"] or 0) >= 2.0)
        disc[vid] = (st, d)
    finalists = []
    for fam in "ABCFG":
        cands = sorted([v for v in disc if v[0] == fam and disc[v][0]["eligible"]], key=lambda v: -disc[v][0]["t_daily"])
        chosen = []
        for v in cands:
            if len(chosen) == 3:
                break
            if all(np.corrcoef(disc[v][1], disc[c][1])[0, 1] <= 0.7 for c in chosen):
                chosen.append(v)
        finalists += chosen
    bar = norm_ppf(1 - 0.05 / max(len(finalists), 1))
    print(f"discovery: {sum(disc[v][0]['eligible'] for v in disc)} eligible of {len(disc)}; {len(finalists)} finalists; "
          f"hold-out bar t >= {bar:.2f}", flush=True)

    # hold-out and recent: finalists only
    final = []
    for v in finalists:
        by = trades[v]
        h, _ = stats(by, HOLD)
        halves = [stats(by, w)[0]["pnl"] for w in HOLD_HALVES]
        coins_pos = sum(x > 0 for x in h["by_coin"].values())
        need_coins = not v.startswith("C|")
        passed = bool(h["pnl"] > 0 and (h["t_daily"] or 0) >= bar and all(x > 0 for x in halves)
                      and (coins_pos >= 4 or not need_coins))
        r, _ = stats(by, RECENT)
        final.append({"variant": v, "discovery": disc[v][0], "holdout": h, "holdout_halves": halves,
                      "holdout_coins_positive": coins_pos, "pass": passed, "recent": r})
        print(f"{v:<40} disc t {disc[v][0]['t_daily']:5.2f} ${disc[v][0]['pnl']:>9,.0f} | hold t {h['t_daily']} "
              f"${h['pnl']:>9,.0f} halves {halves} coins+ {coins_pos} {'PASS' if passed else 'fail'} | "
              f"recent ${r['pnl']:>9,.0f} t {r['t_daily']}", flush=True)

    fam_summary = {}
    for fam in "ABCFG":
        vs = [v for v in disc if v[0] == fam]
        fam_summary[fam] = {"variants": len(vs), "disc_positive": sum(disc[v][0]["pnl"] > 0 for v in vs),
                            "disc_gross_positive": sum(disc[v][0]["gross_pnl"] > 0 for v in vs),
                            "eligible": sum(disc[v][0]["eligible"] for v in vs)}
    leader = sorted(disc, key=lambda v: -(disc[v][0]["t_daily"] or -99))[:40]
    json.dump({"fingerprint": fp, "variants": len(disc), "holdout_bar": round(bar, 2), "families": fam_summary,
               "finalists": final, "discovery_top40": [{"variant": v, **{k: disc[v][0][k] for k in
                                                       ("trades", "pnl", "gross_pnl", "t_daily", "avg_net_bp", "years", "eligible")}}
                                                      for v in leader]},
              open(os.path.join(RES, "summary.json"), "w", encoding="utf-8"), indent=1, default=str)
    np.savez_compressed(os.path.join(DATA, "finalist_trades.npz"),
                        **{f"{v}::{s}".replace("|", "_"): a for v in finalists for s, a in trades[v].items()})
    print(json.dumps(fam_summary), f"({time.time() - t0:.0f}s)")


if __name__ == "__main__":
    sys.exit(main())
