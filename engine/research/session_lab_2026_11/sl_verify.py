"""Independent checks of the lab's finalists.

1. Fills: every finalist's orders are re-filled by a plain loop written apart
   from sl_core.fill (market / stop / limit entries, the 1 bp trade-through
   rule, gaps, stop-before-target, time exits, bad days, one position at a
   time). Trades must match exactly. Cross-sectional finalists are rebuilt from
   minute data and compared with the price-point version in sl_run.
2. No look-ahead: finalists are re-run on data cut at 2024-07-01; every trade
   that closed before the cut must be identical.

Usage: python -I sl_verify.py
"""
import json
import os
import sys

import numpy as np

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from sl_core import DATA, DAY, MAKER, SYMBOLS, TAKER, THROUGH, Market, run_orders  # noqa: E402
import sl_families as F  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
CUT = 1719792000  # 2024-07-01 00:00 UTC


def ref_fill(mk, orders):
    t, o, h, l = mk.t, mk.o, mk.h, mk.l
    out, busy = [], -1
    for od in sorted(orders, key=lambda x: x["ts"]):
        ts = int(od["ts"])
        if ts < busy:
            continue
        day = ts // DAY * DAY
        if day in mk.bad_days:
            continue
        end = day + DAY
        exit_by = min(int(od.get("exit_by") or end), end)
        last_i = min((exit_by - mk.t0) // 60, mk.n - 1)
        i = (ts - mk.t0) // 60
        if i < 0 or i >= last_i:
            continue
        side, kind = od["side"], od.get("entry", "market")
        slip = mk.slip
        if kind == "market":
            j, px, intrabar, ecost = i, o[i], False, TAKER + slip
        else:
            L = od["level"]
            until = min((int(od.get("valid_until") or exit_by) - mk.t0) // 60, last_i)
            j = None
            for m in range(i, until):
                if kind == "stop":
                    trig = h[m] >= L if side > 0 else l[m] <= L
                else:
                    trig = l[m] <= L * (1 - THROUGH) if side > 0 else h[m] >= L * (1 + THROUGH)
                if trig:
                    j = m
                    break
            if j is None:
                continue
            if kind == "stop":
                gap, ecost = (o[j] >= L if side > 0 else o[j] <= L), TAKER + slip
            else:
                gap, ecost = (o[j] <= L if side > 0 else o[j] >= L), MAKER
            px = o[j] if gap else L
            intrabar = not gap
        stop = od.get("stop")
        if stop is not None and side * (px - stop) <= 0:
            continue
        tgt = od.get("target")
        if tgt is None and od.get("target_r") and stop is not None:
            tgt = px + side * od["target_r"] * abs(px - stop)
        if tgt is not None and side * (tgt - px) <= 0:
            continue
        if j >= last_i:
            continue
        res = None
        m = j
        if intrabar:
            if stop is not None and (l[j] <= stop if side > 0 else h[j] >= stop):
                res = (t[j] + 60, stop, "SL")
            m = j + 1
        while res is None and m < last_i:
            if m != j:
                if stop is not None and side * (o[m] - stop) <= 0:
                    res = (t[m], o[m], "SL")
                    break
                if tgt is not None and side * (o[m] - tgt) >= 0:
                    res = (t[m], o[m], "TP")
                    break
            if stop is not None and (l[m] <= stop if side > 0 else h[m] >= stop):
                res = (t[m] + 60, stop, "SL")
                break
            if tgt is not None and (h[m] >= tgt * (1 + THROUGH) if side > 0 else l[m] <= tgt * (1 - THROUGH)):
                res = (t[m] + 60, tgt, "TP")
                break
            m += 1
        if res is None:
            res = (t[last_i], o[last_i], "TIME")
        out.append((int(t[j]), int(res[0]), side, float(px), float(res[1]), res[2]))
        busy = int(res[0])
    return out


class Cut(Market):
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


def main():
    s = json.load(open(os.path.join(HERE, "results", "summary.json"), encoding="utf-8"))
    fins = [f["variant"] for f in s["finalists"]]
    singles = [v for v in fins if not v.startswith("C|")]
    cross = [v for v in fins if v.startswith("C|")]
    checked, bad, trunc_n, trunc_bad = 0, [], 0, []
    for sym in SYMBOLS:
        mk, cut = Market(sym), Cut(sym, CUT)
        f, fc = F.Features(mk), F.Features(cut)
        for v in singles:
            od = F.orders(f, v)
            a = run_orders(mk, od)
            b = ref_fill(mk, od)
            checked += len(a)
            a5 = [(x[0], x[1], x[2], x[5]) for x in a]
            b5 = [(x[0], x[1], x[2], x[5]) for x in b]
            if a5 != b5:
                first = next((p for p in zip(a5, b5) if p[0] != p[1]), None)
                bad.append(f"{v} {sym}: {len(a)} vs {len(b)} trades; first difference {first}")
            ac = [x for x in run_orders(cut, F.orders(fc, v)) if x[1] < CUT - 60]
            af = [x for x in a if x[1] < CUT - 60]
            trunc_n += len(af)
            if key(ac) != key(af):
                trunc_bad.append(f"{v} {sym}: {len(af)} full-data trades vs {len(ac)} on cut data")
        del mk, cut, f, fc
        print(sym, "checked", flush=True)
    if cross:
        mkts = {s_: Market(s_) for s_ in SYMBOLS}
        feats = {s_: F.Features(m) for s_, m in mkts.items()}
        z = np.load(os.path.join(DATA, "finalist_trades.npz"))
        for v in cross:
            p = v.split("|")
            ords = F.orders_C(mkts, feats, p[1], p[2], p[3], int(p[4]))
            for s_ in SYMBOLS:
                ref = ref_fill(mkts[s_], ords[s_])
                got = z[f"{v}::{s_}".replace("|", "_")]
                checked += len(got)
                if [(r[0], r[1], r[2]) for r in ref] != [(int(g[0]), int(g[1]), int(g[2])) for g in got]:
                    bad.append(f"{v} {s_}: price-point basket differs from minute-data rebuild ({len(got)} vs {len(ref)})")
    out = {"finalists": fins, "fill_trades_checked": checked, "fill_mismatches": bad[:30],
           "truncation_cut": "2024-07-01", "truncation_trades": trunc_n, "truncation_mismatches": trunc_bad[:30]}
    json.dump(out, open(os.path.join(HERE, "results", "verify.json"), "w", encoding="utf-8"), indent=1)
    print(json.dumps(out, indent=1))
    return 0 if not bad and not trunc_bad else 1


if __name__ == "__main__":
    sys.exit(main())
