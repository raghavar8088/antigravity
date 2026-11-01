"""Automatic October search, rounds 4 onward.

Draws untested variants from the fixed libraries (os_library .. os_library6,
families interleaved); once those are used up it keeps drawing new random
combinations from os_library7, os_library8 and os_library9 (46 families), so it
never runs dry. Each variant is scored on
October 2026 only: PROFITABLE / CONSISTENT on the 6 main coins, then CONFIRMED
on the 10 extra coins (same rules as rounds 1-3). Every result is appended to
results/auto/evaluated.jsonl, so a restart continues where it stopped, and the
report is rebuilt after every round. Runs until results/auto/STOP exists.

Usage: python -I os_auto.py [round_size] [max_rounds_this_run]
"""
import json
import os
import random
import sys
import time

import numpy as np

HERE = os.path.dirname(os.path.abspath(__file__))
LAB = os.path.normpath(os.path.join(HERE, "..", "session_lab_2026_11"))
sys.path.insert(0, LAB)
sys.path.insert(0, HERE)
import sl_core  # noqa: E402
import sl_families as SF  # noqa: E402
import os_families2 as N2  # noqa: E402
import os_library as L  # noqa: E402
import os_library2 as L2  # noqa: E402
import os_library3 as L3  # noqa: E402
import os_library4 as L4  # noqa: E402
import os_library5 as L5  # noqa: E402
import os_library6 as L6  # noqa: E402
import os_library7 as L7  # noqa: E402
import os_library8 as L8  # noqa: E402
import os_library9 as L9  # noqa: E402
from os_search import score  # noqa: E402

MAIN = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"]
EXTRA = ["ADAUSDT", "LINKUSDT", "AVAXUSDT", "LTCUSDT", "DOTUSDT", "TRXUSDT", "BCHUSDT", "NEARUSDT", "SUIUSDT", "APTUSDT"]
AUTO = os.path.join(HERE, "results", "auto")
EVAL = os.path.join(AUTO, "evaluated.jsonl")
PROG = os.path.join(AUTO, "progress.json")
STOP = os.path.join(AUTO, "STOP")
CROSS = {"XS": L3.xs_orders, **L9.CROSS9}  # families ranked across the coin set
LIBS = [(L9.FN9, L9.describe9), (L8.FN8, L8.describe8), (L7.FN7, L7.describe7), (L6.FN6, L6.describe6), (L5.FN5, L5.describe5), (L4.FN4, L4.describe4),
        (L3.FN3, L3.describe3), (L2.FN2, L2.describe2)]


def queue():
    sp = L.space()
    for extra in (L2.space2(), L3.space3(), L4.space4(), L5.space5(), L6.space6()):
        sp.update(extra)
    rng = random.Random(2026)
    lists = {}
    for fam, combos in sp.items():
        c = list(combos)
        rng.shuffle(c)
        lists[fam] = c
    out = []
    while any(lists.values()):
        for fam in list(lists):
            if lists[fam]:
                out.append((fam, lists[fam].pop()))
    return out


def orders_for(ctx, s, fam, p):
    for fn, _ in LIBS:
        if fam in fn:
            return fn[fam](ctx[s], *p)
    return L.orders(ctx[s], fam, p)


def describe(fam, p):
    if fam == "XS":
        return L3.describe3(fam, p)
    for fn, desc in LIBS:
        if fam in fn:
            return desc(fam, p)
    return L.describe(fam, p)


def run(ctx, coins, fam, p):
    by = {}
    per = CROSS[fam]({s: ctx[s] for s in coins}, *p) if fam in CROSS else None
    for s in coins:
        ords = per[s] if per is not None else orders_for(ctx, s, fam, p)
        tr = sl_core.run_orders(ctx[s].mk, ords)
        by[s] = np.array([x[:5] for x in tr], dtype=float).reshape(-1, 5)
    return by


def process(ctx, batch, round_no, fh):
    n_p = n_c = n_k = 0
    for fam, p in batch:
        v = L.vid(fam, p)
        try:
            sc = score(run(ctx, MAIN, fam, p), sl_core.NOTIONAL * len(MAIN), True)
        except Exception as e:  # a broken variant is logged, not fatal
            fh.write(json.dumps({"id": v, "family": fam, "error": repr(e)[:200]}) + "\n")
            continue
        title, rules = describe(fam, p)
        rec = {"id": v, "family": fam, "title": title, "rules": rules, "round": round_no}
        if sc:
            rec["original6"] = {k: sc[k] for k in ("trades", "pnl", "return_pct", "win_rate", "halves", "coins_positive",
                                                   "max_dd_pct", "capital", "profitable", "consistent")}
            n_p += sc["profitable"]
            if sc["consistent"]:
                n_c += 1
                s2 = score(run(ctx, EXTRA, fam, p), sl_core.NOTIONAL * len(EXTRA), True)
                if s2:
                    rec["new10"] = {k: s2[k] for k in ("trades", "pnl", "return_pct", "win_rate", "coins_positive", "max_dd_pct", "capital")}
                    rec["confirmed"] = bool(s2["pnl"] > 0 and s2["coins_positive"] >= 5)
                    n_k += rec["confirmed"]
        fh.write(json.dumps(rec, default=str) + "\n")
    return int(n_p), n_c, n_k


def main():
    os.makedirs(AUTO, exist_ok=True)
    size = int(sys.argv[1]) if len(sys.argv) > 1 else 300
    max_rounds = int(sys.argv[2]) if len(sys.argv) > 2 else 10**9  # this run only
    sl_core.DATA = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "october_extra"))
    sl_core.SLIP.update({c: 0.0005 for c in EXTRA})
    ctx = {}
    for s in MAIN + EXTRA:
        mk = sl_core.Market(s)
        ctx[s] = N2.Ctx(mk, SF.Features(mk))
    L8.ALL.update(ctx)  # the lead-lag family reads BTC / ETH from here
    done = set()
    if os.path.exists(EVAL):
        with open(EVAL, encoding="utf-8") as fh:
            for line in fh:
                done.add(json.loads(line)["id"])
    q = [(f, p) for f, p in queue() if L.vid(f, p) not in done]
    prog = json.load(open(PROG, encoding="utf-8")) if os.path.exists(PROG) else {"rounds": 0, "evaluated": 0, "profitable": 0,
                                                                                 "consistent": 0, "confirmed": 0}
    print(f"{len(done)} done, {len(q)} left in the fixed libraries, then the endless sampler", flush=True)
    import os_report  # noqa: E402
    rng = random.Random(len(done) + 7)
    ran = 0
    while not os.path.exists(STOP) and ran < max_rounds:
        ran += 1
        t0 = time.time()
        if q:
            batch, q = q[:size], q[size:]
            mode = "fixed"
        else:
            batch, tries = [], 0
            while len(batch) < size and tries < size * 100:
                fam, p = L9.sample(rng)
                tries += 1
                v = L.vid(fam, p)
                if v not in done:
                    done.add(v)
                    batch.append((fam, p))
            mode = "endless"
            if not batch:
                break
        for fam, p in batch:
            done.add(L.vid(fam, p))
        with open(EVAL, "a", encoding="utf-8") as fh:
            n_p, n_c, n_k = process(ctx, batch, prog["rounds"] + 4, fh)
        prog["rounds"] += 1
        prog["evaluated"] += len(batch)
        prog["profitable"] += n_p
        prog["consistent"] += n_c
        prog["confirmed"] += n_k
        prog["remaining_fixed"] = len(q)
        prog["mode"] = mode
        prog["last_round_seconds"] = round(time.time() - t0, 1)
        prog["updated_utc"] = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime())
        json.dump(prog, open(PROG, "w", encoding="utf-8"), indent=1)
        try:
            os_report.main()
        except Exception as e:
            print("report rebuild failed:", repr(e)[:200], flush=True)
        print(f"round {prog['rounds'] + 3} ({mode}): {len(batch)} variants, {n_p} profitable, {n_c} consistent, {n_k} confirmed "
              f"({prog['last_round_seconds']}s); totals: {prog['evaluated']} tested, {prog['confirmed']} confirmed", flush=True)
    print("stopped:", "STOP file" if os.path.exists(STOP) else "round limit for this run", flush=True)


if __name__ == "__main__":
    sys.exit(main())
