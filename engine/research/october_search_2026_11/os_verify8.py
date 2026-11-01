"""Checks for library 8 before it joins the endless search.

1. Every family runs and describes itself on random parameters (no exceptions).
2. No look-ahead: each variant is rebuilt on data cut at 2026-10-20 00:00 UTC; every trade that closed before the
   cut must be identical (entry, exit, side, gross, net) to the full-data run.
Writes results/verify8.json.

Usage: python -I os_verify8.py [variants_per_family]
"""
import json
import os
import random
import sys
import time

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
LAB = os.path.normpath(os.path.join(HERE, "..", "session_lab_2026_11"))
sys.path.insert(0, LAB)
sys.path.insert(0, HERE)
import sl_core  # noqa: E402
import sl_families as SF  # noqa: E402
import os_families2 as N2  # noqa: E402
import os_library8 as L8  # noqa: E402

COINS = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "ADAUSDT"]
CUT = int(pd.Timestamp("2026-10-20", tz="UTC").timestamp())


class Cut(sl_core.Market):
    def __init__(self, sym, cut):
        super().__init__(sym)
        k = (cut - self.t0) // 60
        for a in ("t", "o", "h", "l", "c", "v"):
            setattr(self, a, getattr(self, a)[:k])
        self.n = k
        keep = self.fund_t < cut
        self.fund_t, self.fund_r = self.fund_t[keep], self.fund_r[keep]
        self.daily = self.daily[self.daily["t"] < cut].reset_index(drop=True)


def key(tr):
    return [(a, b, c, round(d, 12), round(e, 12)) for a, b, c, d, e, *_ in tr]


def draw(rng, fam):
    while True:
        f, p = L8.sample(rng)
        if f == fam:
            return p


def main():
    per = int(sys.argv[1]) if len(sys.argv) > 1 else 6
    sl_core.DATA = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "october_extra"))
    sl_core.SLIP.update({c: 0.0005 for c in ["ADAUSDT"]})
    full, cut = {}, {}
    for s in COINS:
        mk, mc = sl_core.Market(s), Cut(s, CUT)
        full[s], cut[s] = N2.Ctx(mk, SF.Features(mk)), N2.Ctx(mc, SF.Features(mc))
        # a bar closing exactly at the cut is booked to the cut day, which the cut data lacks; its order can never fill
        # there (no minutes left), so any index will do for the lookup
        cut[s].kday.setdefault(CUT, len(cut[s].f.days) - 1)
    rng = random.Random(8)
    report, problems = {}, []
    for fam in L8.FAMS8:
        n_tr = n_chk = 0
        t0 = time.time()
        for _ in range(per):
            p = draw(rng, fam)
            v = fam + "|" + "|".join(str(x) for x in p)
            try:
                L8.describe8(fam, p)
                for s in COINS:
                    L8.ALL.clear()
                    L8.ALL.update(full)
                    a = sl_core.run_orders(full[s].mk, L8.FN8[fam](full[s], *p))
                    L8.ALL.clear()
                    L8.ALL.update(cut)
                    b = sl_core.run_orders(cut[s].mk, L8.FN8[fam](cut[s], *p))
                    af = [x for x in a if x[1] < CUT - 60]
                    bf = [x for x in b if x[1] < CUT - 60]
                    n_tr += len(a)
                    n_chk += len(af)
                    if key(af) != key(bf):
                        first = next((q for q in zip(key(af), key(bf)) if q[0] != q[1]), None)
                        problems.append(f"{v} {s}: {len(af)} full-data trades vs {len(bf)} on cut data; first difference {first}")
            except Exception as e:
                problems.append(f"{v}: {e!r}"[:300])
        report[fam] = {"variants": per, "trades": n_tr, "checked_before_cut": n_chk,
                       "seconds_per_variant_4_coins": round((time.time() - t0) / per / 2, 3)}
        print(fam, report[fam], flush=True)
    out = {"cut": "2026-10-20", "coins": COINS, "families": report, "problems": problems[:50], "n_problems": len(problems)}
    json.dump(out, open(os.path.join(HERE, "results", "verify8.json"), "w", encoding="utf-8"), indent=1)
    print(json.dumps({"n_problems": len(problems), "problems": problems[:20]}, indent=1))
    return 0 if not problems else 1


if __name__ == "__main__":
    sys.exit(main())
