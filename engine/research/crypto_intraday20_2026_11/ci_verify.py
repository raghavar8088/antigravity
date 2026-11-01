"""Independent checks of the 20-strategy run.

1. Fills: every entry the strategies produced is re-filled by a second, plain
   minute-by-minute simulator written separately from ci_core.simulate; every
   trade from 2025-11 on must match (times, prices, exit reason).
2. No look-ahead: all 20 strategies are re-run on data cut at 2026-10-16 00:00.
   Every trade that closed before the cut must be identical to the full run;
   a rule that peeked at any later bar would change.

Usage: python -I ci_verify.py
"""
import json
import os
import sys

import numpy as np
import pandas as pd

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import ci_core  # noqa: E402
from ci_core import DAY, FEE_LEG, SLIP_LEG, SYMBOLS, Market, simulate  # noqa: E402
from ci_strategies import STRATEGIES  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
START = int(pd.Timestamp("2025-11-01", tz="UTC").timestamp())
CUT = int(pd.Timestamp("2026-10-16", tz="UTC").timestamp())


def ref_fill(mk, entries):
    """Plain re-implementation of the fill rules, one minute at a time."""
    out, busy = [], -1
    for e in sorted(entries, key=lambda x: x["ts"]):
        ts = int(e["ts"])
        if ts < busy:
            continue
        day_end = (ts // DAY + 1) * DAY
        if ts >= day_end - 60 or (day_end - DAY) in mk.rebuilt_days:
            continue
        i = (ts - mk.t0) // 60
        if i < 0 or i >= mk.n - 1:
            continue
        side, entry = e["side"], mk.o[i]
        stop = e.get("stop", None)
        if stop is None and e.get("stop_dist") is not None:
            stop = entry - side * e["stop_dist"]
        tgt = e.get("target", None)
        if tgt is None and e.get("target_dist") is not None:
            tgt = entry + side * e["target_dist"]
        if stop is not None and (side == 1 and stop >= entry or side == -1 and stop <= entry):
            continue
        if tgt is not None and (side == 1 and tgt <= entry or side == -1 and tgt >= entry):
            continue
        last = day_end
        if e.get("exit_by"):
            last = min(last, int(e["exit_by"]))
        ex = e.get("exits")
        if ex is not None:
            later = [x for x in ex if x > ts]
            if later:
                last = min(last, int(later[0]))
        j = i
        res = None
        stop_at = min(last, int(mk.t[mk.n - 1]))  # the data ends at 23:59 on its last day
        while j < mk.n:
            tj = mk.t[j]
            if tj >= stop_at:
                res = (tj, mk.o[j], "EOD" if last == day_end else "TIME")
                break
            o, h, l = mk.o[j], mk.h[j], mk.l[j]
            if j > i and stop is not None and ((side == 1 and o <= stop) or (side == -1 and o >= stop)):
                res = (tj, o, "SL")
                break
            if j > i and tgt is not None and ((side == 1 and o >= tgt) or (side == -1 and o <= tgt)):
                res = (tj, o, "TP")
                break
            if stop is not None and ((side == 1 and l <= stop) or (side == -1 and h >= stop)):
                res = (tj + 60, stop, "SL")
                break
            if tgt is not None and ((side == 1 and h >= tgt) or (side == -1 and l <= tgt)):
                res = (tj + 60, tgt, "TP")
                break
            j += 1
        if res is None:
            continue
        out.append((ts, int(res[0]), float(entry), float(res[1]), res[2]))
        busy = int(res[0])
    return out


class Cut(Market):
    """A market whose data ends at CUT, as if the run happened that day."""

    def __init__(self, sym, cut):
        super().__init__(sym)
        k = int((cut - self.t0) // 60)
        for a in ("t", "o", "h", "l", "c", "v", "tbv", "prem"):
            setattr(self, a, getattr(self, a)[:k])
        self.n = k
        self.cpv, self.cv, self.ctb = self.cpv[:k + 1], self.cv[:k + 1], self.ctb[:k + 1]
        self.metrics = {m: v[:k] for m, v in self.metrics.items()}
        keep = self.fund_t < cut
        self.fund_t, self.fund_r = self.fund_t[keep], self.fund_r[keep]
        self._bars = {}


def main():
    full = {s: Market(s) for s in SYMBOLS}
    cut = {s: Cut(s, CUT) for s in SYMBOLS}
    fill_checked, fill_bad, trunc_n, trunc_bad = 0, [], 0, []
    for st in STRATEGIES:
        for s in SYMBOLS:
            ent = st["fn"](full[s], full)
            a = [x for x in simulate(full[s], ent) if x["entry_t"] >= START]
            b = [x for x in ref_fill(full[s], ent) if x[0] >= START]
            fill_checked += len(a)
            if len(a) != len(b):
                fill_bad.append(f"{st['id']} {s}: {len(a)} trades vs {len(b)} in the reference")
            else:
                for x, y in zip(a, b):
                    if (x["entry_t"], x["exit_t"], x["reason"]) != (y[0], y[1], y[4]) or abs(x["exit"] - y[3]) > 1e-9 * y[3]:
                        fill_bad.append(f"{st['id']} {s} {x['entry_t']}: {x['exit_t']} {x['reason']} vs {y[1]} {y[4]}")
                        break
            c = [x for x in simulate(cut[s], st["fn"](cut[s], cut)) if START <= x["entry_t"] and x["exit_t"] < CUT - 60]
            f = [x for x in a if x["exit_t"] < CUT - 60]
            trunc_n += len(f)
            key = lambda x: (x["entry_t"], x["exit_t"], x["side"], round(x["entry"], 10), round(x["exit"], 10), x["reason"])
            if [key(x) for x in c] != [key(x) for x in f]:
                trunc_bad.append(f"{st['id']} {s}: {len(f)} full-data trades vs {len(c)} on cut data")
        print(st["id"], "checked", flush=True)
    out = {"fill_trades_checked": fill_checked, "fill_mismatches": fill_bad[:30], "truncation_cut": "2026-10-16",
           "truncation_trades": trunc_n, "truncation_mismatches": trunc_bad[:30]}
    json.dump(out, open(os.path.join(HERE, "results", "verify.json"), "w", encoding="utf-8"), indent=1)
    print(json.dumps(out, indent=1))
    return 0 if not fill_bad and not trunc_bad else 1


if __name__ == "__main__":
    sys.exit(main())
