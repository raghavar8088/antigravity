"""October 2026 strategy search: every intraday strategy built so far, scored on
1-31 October 2026 only (no trading on the 28th: Binance never published that
day's perpetual data).

Round 1 pool: the 880 session-lab variants (engine/research/session_lab_2026_11)
and the 20 strategies of the October study (engine/research/crypto_intraday20_2026_11).
Same fills and costs as those studies: Delta India taker 0.05% + 18% GST plus
2 bp (BTC, ETH) or 4 bp (alts) slippage, maker 0.02% + GST for resting limits,
real funding; $10,000 notional per trade, flat by 00:00 UTC.

A strategy is PROFITABLE if its October net profit is positive, and CONSISTENT
if in addition it made at least 10 trades, made money in both halves of the
month (1-15 and 16-31 October) and, when it trades single coins, made money on
at least 3 of the 6.

Usage: python -I os_search.py
"""
import json
import os
import sys
import time
from concurrent.futures import ProcessPoolExecutor

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
LAB = os.path.normpath(os.path.join(HERE, "..", "session_lab_2026_11"))
CI = os.path.normpath(os.path.join(HERE, "..", "crypto_intraday20_2026_11"))
sys.path.insert(0, LAB)
import sl_run  # noqa: E402
from sl_core import NOTIONAL, SYMBOLS  # noqa: E402
from sl_describe import describe  # noqa: E402

RES = os.path.join(HERE, "results")
OCT = (int(pd.Timestamp("2026-10-01", tz="UTC").timestamp()), int(pd.Timestamp("2026-11-01", tz="UTC").timestamp()))
MID = int(pd.Timestamp("2026-10-16", tz="UTC").timestamp())


def score(trades_by_coin, capital, single):
    rows = []
    for sym, a in trades_by_coin.items():
        a = a[(a[:, 0] >= OCT[0]) & (a[:, 0] < OCT[1])]
        for r in a:
            rows.append((sym, r[0], r[1], r[2], r[3], r[4]))
    if not rows:
        return None
    t = pd.DataFrame(rows, columns=["sym", "et", "xt", "side", "gross", "net"])
    t["pnl"] = t["net"] * NOTIONAL
    t = t.sort_values("xt")
    cum = t["pnl"].cumsum().to_numpy()
    dd = float((np.maximum.accumulate(np.concatenate([[0.0], cum]))[1:] - cum).max())
    by_coin = t.groupby("sym")["pnl"].sum()
    halves = [float(t[t["et"] < MID]["pnl"].sum()), float(t[t["et"] >= MID]["pnl"].sum())]
    out = {"trades": int(len(t)), "pnl": round(float(t["pnl"].sum()), 2),
           "return_pct": round(float(t["pnl"].sum() / capital * 100), 2),
           "gross_pnl": round(float(t["gross"].sum() * NOTIONAL), 2),
           "win_rate": round(float((t["net"] > 0).mean() * 100), 1),
           "avg_net_bp": round(float(t["net"].mean() * 1e4), 2), "max_dd": round(dd, 2),
           "max_dd_pct": round(dd / capital * 100, 2), "halves": [round(x, 2) for x in halves],
           "by_coin": {k: round(float(v), 2) for k, v in by_coin.items()}, "capital": capital,
           "coins_positive": int((by_coin > 0).sum())}
    out["profitable"] = out["pnl"] > 0
    out["consistent"] = bool(out["profitable"] and out["trades"] >= 10 and all(h > 0 for h in halves)
                             and (out["coins_positive"] >= 3 or not single))
    return out


def main():
    os.makedirs(RES, exist_ok=True)
    t0 = time.time()
    trades, pts = {}, {}
    with ProcessPoolExecutor(max_workers=3) as ex:
        for sym, out, p in ex.map(sl_run.coin_worker, SYMBOLS):
            for vid, a in out.items():
                trades.setdefault(vid, {})[sym] = a
            pts[sym] = p
            print(f"{sym} done ({time.time() - t0:.0f}s)", flush=True)
    trades.update(sl_run.family_c(pts))
    results = []
    for vid, by in trades.items():
        single = not vid.startswith("C|")
        cap = NOTIONAL * (len(SYMBOLS) if single else 2 * int(vid.split("|")[4]))
        sc = score(by, cap, single)
        if sc:
            title, rules = describe(vid)
            results.append({"id": vid, "source": "session lab", "title": title, "rules": rules, **sc})
    # the 20 strategies of the October study, from their recorded October trades
    ot = pd.read_csv(os.path.join(CI, "results", "october_trades.csv"))
    meta = {s["id"]: s for s in json.load(open(os.path.join(CI, "results", "summary.json"), encoding="utf-8"))["strategies"]}
    for sid, g in ot.groupby("strategy"):
        by = {sym: gg[["entry_t", "exit_t", "side", "gross", "net"]].to_numpy(float) for sym, gg in g.groupby("sym")}
        cap = meta[sid]["capital"]
        sc = score(by, cap, True)
        if sc:
            results.append({"id": sid, "source": "October study", "title": meta[sid]["name"], "rules": meta[sid]["rules"], **sc})
    results.sort(key=lambda r: -r["pnl"])
    prof = [r for r in results if r["profitable"]]
    cons = [r for r in results if r["consistent"]]
    json.dump({"round": 1, "pool": len(results), "profitable": len(prof), "consistent": len(cons), "results": results},
              open(os.path.join(RES, "round1.json"), "w", encoding="utf-8"), indent=1, default=str)
    print(f"\nROUND 1: {len(results)} strategies scored on October 2026; {len(prof)} profitable, {len(cons)} consistent "
          f"({time.time() - t0:.0f}s)")
    for r in cons[:25]:
        print(f"{r['id']:<36} n={r['trades']:4d} net ${r['pnl']:>8,.0f} ({r['return_pct']:6.2f}%) win {r['win_rate']}% "
              f"halves {r['halves']} coins+ {r['coins_positive']} DD {r['max_dd_pct']}%")


if __name__ == "__main__":
    sys.exit(main())
