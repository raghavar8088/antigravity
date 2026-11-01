"""Round 3 of the October search: 332 new intraday variants (os_families2.py),
October 2026 only, same two steps as before:
  1. on BTC ETH SOL XRP DOGE BNB: PROFITABLE / CONSISTENT (both halves of
     October, 3+ of 6 coins, 10+ trades)
  2. consistent ones unchanged on ADA LINK AVAX LTC DOT TRX BCH NEAR SUI APT:
     CONFIRMED = profit there and on 5+ of the 10.

Usage: python -I os_round3.py
"""
import json
import os
import sys
import time

import numpy as np

HERE = os.path.dirname(os.path.abspath(__file__))
LAB = os.path.normpath(os.path.join(HERE, "..", "session_lab_2026_11"))
sys.path.insert(0, LAB)
sys.path.insert(0, HERE)
import sl_core  # noqa: E402
import sl_families as SF  # noqa: E402
import os_families2 as N  # noqa: E402
from os_search import score  # noqa: E402

MAIN = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"]
EXTRA = ["ADAUSDT", "LINKUSDT", "AVAXUSDT", "LTCUSDT", "DOTUSDT", "TRXUSDT", "BCHUSDT", "NEARUSDT", "SUIUSDT", "APTUSDT"]
LABDATA = sl_core.DATA
OUT = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "october_extra"))
RES = os.path.join(HERE, "results")
START = 1785542400  # 2026-08-01 00:00 UTC


def slice_main():
    """Aug-Oct 2026 copies of the 6 main coins next to the 10 extra ones."""
    for s in MAIN:
        dst = os.path.join(OUT, f"{s}.npz")
        if os.path.exists(dst):
            continue
        z = np.load(os.path.join(LABDATA, f"{s}.npz"))
        k = z["t"] >= START
        np.savez(dst, t=z["t"][k], o=z["o"][k], h=z["h"][k], l=z["l"][k], c=z["c"][k], v=z["v"][k],
                 bad_day=z["bad_day"][z["bad_day"] >= START])
        f = np.load(os.path.join(LABDATA, f"{s}_funding.npz"))
        kf = f["t"] >= START
        np.savez(os.path.join(OUT, f"{s}_funding.npz"), t=f["t"][kf], rate=f["rate"][kf])


def run(coins, grid):
    ctx = {}
    for s in coins:
        mk = sl_core.Market(s)
        ctx[s] = N.Ctx(mk, SF.Features(mk))
    res = {}
    for g in grid:
        by = {}
        for s in coins:
            tr = sl_core.run_orders(ctx[s].mk, N.orders(ctx[s], g))
            by[s] = np.array([x[:5] for x in tr], dtype=float).reshape(-1, 5)
        res[N.vid(g)] = by
    return res


def main():
    t0 = time.time()
    slice_main()
    sl_core.DATA = OUT
    sl_core.SLIP.update({c: 0.0005 for c in EXTRA})
    r_main = run(MAIN, N.GRID)
    print(f"{len(N.GRID)} variants on 6 coins ({time.time() - t0:.0f}s)", flush=True)
    scored = []
    for g in N.GRID:
        v = N.vid(g)
        sc = score(r_main[v], sl_core.NOTIONAL * len(MAIN), True)
        if sc:
            t, r = N.describe(g)
            scored.append({"id": v, "grid": list(g), "title": t, "rules": r, **sc})
    cons = [x for x in scored if x["consistent"]]
    prof = [x for x in scored if x["profitable"]]
    print(f"profitable {len(prof)}, consistent {len(cons)}", flush=True)
    grid_c = [tuple(x["grid"]) for x in cons]
    r_new = run(EXTRA, grid_c) if grid_c else {}
    out = []
    for x in cons:
        sc = score(r_new[x["id"]], sl_core.NOTIONAL * len(EXTRA), True) or {"trades": 0, "pnl": 0.0, "return_pct": 0.0, "coins_positive": 0}
        confirmed = bool(sc["pnl"] > 0 and sc["coins_positive"] >= 5)
        out.append({"id": x["id"], "title": x["title"], "rules": x["rules"],
                    "original6": {k: x[k] for k in ("trades", "pnl", "return_pct", "win_rate", "halves", "coins_positive", "max_dd_pct", "by_coin", "capital")},
                    "new10": sc, "confirmed": confirmed})
        print(f"{x['id']:<36} 6 coins ${x['pnl']:>7,.0f} ({x['return_pct']:5.2f}%) | new 10 n={sc['trades']:3d} ${sc['pnl']:>7,.0f} "
              f"({sc['return_pct']:5.2f}%) coins+ {sc['coins_positive']} {'CONFIRMED' if confirmed else ''}", flush=True)
    json.dump({"round": 3, "pool": len(scored), "profitable": len(prof), "consistent": len(cons),
               "confirmed": sum(o["confirmed"] for o in out), "results": out, "all": scored},
              open(os.path.join(RES, "round3.json"), "w", encoding="utf-8"), indent=1, default=str)
    print(f"\nROUND 3: {len(scored)} scored, {len(prof)} profitable, {len(cons)} consistent, "
          f"{sum(o['confirmed'] for o in out)} confirmed ({time.time() - t0:.0f}s)")


if __name__ == "__main__":
    sys.exit(main())
