"""Round 2 of the October search: the strategies that were consistent on the
original 6 coins in October 2026, run unchanged on 10 coins they were not
picked on (ADA LINK AVAX LTC DOT TRX BCH NEAR SUI APT), October 2026 only.

CONFIRMED = net profit on the 10 new coins in October and profit on at least
5 of them. Costs as before; slippage 5 bp a side on these smaller coins.

Usage: python -I os_round2.py   (after os_search.py and os_fetch_extra.py)
"""
import json
import os
import sys

import numpy as np

HERE = os.path.dirname(os.path.abspath(__file__))
LAB = os.path.normpath(os.path.join(HERE, "..", "session_lab_2026_11"))
sys.path.insert(0, LAB)
sys.path.insert(0, HERE)  # python -I leaves the script folder off the path
import sl_core  # noqa: E402
import sl_families as F  # noqa: E402
from os_search import OCT, score  # noqa: E402

EXTRA = ["ADAUSDT", "LINKUSDT", "AVAXUSDT", "LTCUSDT", "DOTUSDT", "TRXUSDT", "BCHUSDT", "NEARUSDT", "SUIUSDT", "APTUSDT"]
sl_core.DATA = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "october_extra"))
sl_core.SLIP.update({c: 0.0005 for c in EXTRA})
RES = os.path.join(HERE, "results")


def main():
    r1 = json.load(open(os.path.join(RES, "round1.json"), encoding="utf-8"))
    cons = [r for r in r1["results"] if r["consistent"]]
    mkts = {s: sl_core.Market(s) for s in EXTRA}
    feats = {s: F.Features(m) for s, m in mkts.items()}
    out = []
    for r in cons:
        vid = r["id"]
        if r["source"] != "session lab":
            continue
        by = {}
        if vid.startswith("C|"):
            p = vid.split("|")
            ords = F.orders_C(mkts, feats, p[1], p[2], p[3], int(p[4]))
            for s in EXTRA:
                tr = sl_core.run_orders(mkts[s], [o for o in ords[s] if OCT[0] <= o["ts"] < OCT[1]])
                by[s] = np.array([x[:5] for x in tr], dtype=float).reshape(-1, 5)
            cap = sl_core.NOTIONAL * 2 * int(p[4])
            single = False
        else:
            for s in EXTRA:
                ords = [o for o in F.orders(feats[s], vid) if OCT[0] <= o["ts"] < OCT[1]]
                tr = sl_core.run_orders(mkts[s], ords)
                by[s] = np.array([x[:5] for x in tr], dtype=float).reshape(-1, 5)
            cap = sl_core.NOTIONAL * len(EXTRA)
            single = True
        sc = score(by, cap, single) or {"trades": 0, "pnl": 0.0, "return_pct": 0.0, "coins_positive": 0, "by_coin": {}}
        confirmed = bool(sc["pnl"] > 0 and (sc["coins_positive"] >= 5 or not single))
        out.append({"id": vid, "title": r["title"], "rules": r["rules"], "original6": {k: r[k] for k in
                    ("trades", "pnl", "return_pct", "win_rate", "halves", "coins_positive", "max_dd_pct", "by_coin", "capital")},
                    "new10": sc, "confirmed": confirmed})
        print(f"{vid:<36} 6 coins ${r['pnl']:>7,.0f} ({r['return_pct']:5.2f}%) | new 10 coins n={sc['trades']:3d} "
              f"${sc['pnl']:>7,.0f} ({sc['return_pct']:5.2f}%) coins+ {sc['coins_positive']} {'CONFIRMED' if confirmed else ''}", flush=True)
    conf = [o for o in out if o["confirmed"]]
    json.dump({"tested": len(out), "confirmed": len(conf), "results": out}, open(os.path.join(RES, "round2.json"), "w", encoding="utf-8"),
              indent=1, default=str)
    print(f"\nROUND 2: {len(conf)} of {len(out)} consistent October strategies also made money on the 10 new coins in October")


if __name__ == "__main__":
    sys.exit(main())
