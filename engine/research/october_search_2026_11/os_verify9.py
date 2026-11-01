"""Checks for library 9 before it joins the endless search (same tests as os_verify8).

1. Every family runs and describes itself on random parameters (no exceptions).
2. No look-ahead: each variant is rebuilt on data cut at 2026-10-20 00:00 UTC; every trade that closed before the
   cut must be identical to the full-data run. XSX (cross-coin) is ranked within the four test coins.
Writes results/verify9.json.

Usage: python -I os_verify9.py [variants_per_family]
"""
import json
import os
import random
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.normpath(os.path.join(HERE, "..", "session_lab_2026_11")))
sys.path.insert(0, HERE)
import sl_core  # noqa: E402
import sl_families as SF  # noqa: E402
import os_families2 as N2  # noqa: E402
import os_library8 as L8  # noqa: E402
import os_library9 as L9  # noqa: E402
from os_verify8 import COINS, CUT, Cut, key  # noqa: E402


def draw(rng, fam):
    while True:
        f, p = L9.sample(rng)
        if f == fam:
            return p if fam != "XSX" else p[:3] + (1,) + p[4:]  # 4 test coins: one each side


def orders(ctxs, fam, p):
    L8.ALL.clear()
    L8.ALL.update(ctxs)
    if fam in L9.CROSS9:
        return L9.CROSS9[fam](ctxs, *p)
    return {s: L9.FN9[fam](ctxs[s], *p) for s in ctxs}


def main():
    per = int(sys.argv[1]) if len(sys.argv) > 1 else 8
    sl_core.DATA = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "october_extra"))
    sl_core.SLIP.update({"ADAUSDT": 0.0005})
    full, cut = {}, {}
    for s in COINS:
        mk, mc = sl_core.Market(s), Cut(s, CUT)
        full[s], cut[s] = N2.Ctx(mk, SF.Features(mk)), N2.Ctx(mc, SF.Features(mc))
        cut[s].kday.setdefault(CUT, len(cut[s].f.days) - 1)  # see os_verify8
    rng = random.Random(9)
    report, problems = {}, []
    for fam in L9.FAMS9:
        n_tr = n_chk = 0
        t0 = time.time()
        for _ in range(per):
            p = draw(rng, fam)
            v = fam + "|" + "|".join(str(x) for x in p)
            try:
                L9.describe9(fam, p)
                of, oc = orders(full, fam, p), orders(cut, fam, p)
                for s in COINS:
                    a = sl_core.run_orders(full[s].mk, of[s])
                    b = sl_core.run_orders(cut[s].mk, oc[s])
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
    json.dump(out, open(os.path.join(HERE, "results", "verify9.json"), "w", encoding="utf-8"), indent=1)
    print(json.dumps({"n_problems": len(problems), "problems": problems[:20]}, indent=1))
    return 0 if not problems else 1


if __name__ == "__main__":
    sys.exit(main())
