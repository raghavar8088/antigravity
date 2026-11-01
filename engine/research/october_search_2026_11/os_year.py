"""Year test of the October search's top strategies: every month of 2026 (January - October) with the same
rules, fills, fees, funding and coins as the search ($10,000 per coin on 16 coins, no leverage, flat by
00:00 UTC). Only October was used to pick these strategies, so January - September are months they have
never seen. The strategies are the top N rows of results/final.json when this runs (ids saved in the output).
Check: each strategy's October here must equal its October result in the search.

Mode "opposite" trades every signal the other way: a buy becomes a sell and a sell a buy at the same moment,
the original stop becomes the take-profit and the original target the stop, and limit entries become stop
entries at the same level (the band fades of BLX become OCO stop orders, first touch fills). The opposite
trades pay their own fees and slippage.

Usage: python -I os_year.py [top_n] [opposite]
Writes results/year2026.json (or results/year2026_opposite.json) and puts it into the report template.
"""
import json
import os
import sys

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.normpath(os.path.join(HERE, "..", "session_lab_2026_11")))
sys.path.insert(0, HERE)
import sl_core  # noqa: E402
import sl_families as SF  # noqa: E402
import os_families2 as N2  # noqa: E402
import os_auto as A  # noqa: E402  (dispatch over every library)
import os_library8 as L8  # noqa: E402
from os_families2 import atr_bars  # noqa: E402
from os_library import window  # noqa: E402

Y0 = int(pd.Timestamp("2026-01-01", tz="UTC").timestamp())
Y1 = int(pd.Timestamp("2026-11-01", tz="UTC").timestamp())
OCT0 = int(pd.Timestamp("2026-10-01", tz="UTC").timestamp())
COINS = A.MAIN + A.EXTRA
CAP = sl_core.NOTIONAL * len(COINS)
DAY, H = sl_core.DAY, sl_core.H


def set_period(t0, t1):
    """The libraries read the trading period from module constants OCT0 / OCT1; point them all at 2026."""
    for name, mod in list(sys.modules.items()):
        if name.startswith("os_") and hasattr(mod, "OCT0"):
            mod.OCT0, mod.OCT1 = t0, t1


def parse(vid):
    fam, *raw = vid.split("|")
    out = []
    for x in raw:
        for cast in (int, float):
            try:
                out.append(cast(x))
                break
            except ValueError:
                pass
        else:
            out.append(x)
    return fam, tuple(out)


def opposite(o):
    """The same order traded the other way: side flipped, stop and target swapped, limit <-> stop entry."""
    if "target_r" in o:
        raise NotImplementedError("R-multiple targets need the fill price to mirror")
    m = dict(o)
    m["side"] = -o["side"]
    m["entry"] = {"market": "market", "limit": "stop", "stop": "limit"}[o.get("entry", "market")]
    m["stop"], m["target"] = o.get("target"), o.get("stop")
    if m["target"] is None:
        del m["target"]
    return m


def blx_opposite(cx, tf, band, n, k, vb, st, hold, sess):
    """BLX the other way: after each close, a buy stop at the upper band and a sell stop at the lower band (OCO,
    the first touched fills, as sl_core.fill triggers stops); stop at the middle, take-profit st ATR beyond the band."""
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    if band == "bb":
        mid, sd = c.rolling(n).mean(), c.rolling(n).std(ddof=0)
        lo, hi = mid - k * sd, mid + k * sd
    else:
        mid = c.ewm(span=n, adjust=False).mean()
        lo, hi = mid - k * a, mid + k * a
    mk, tc = cx.mk, b["tc"].to_numpy()
    lo, hi, mid, a = lo.to_numpy(), hi.to_numpy(), mid.to_numpy(), a.to_numpy()
    out = []
    for i in np.nonzero((tc >= Y0) & (tc < Y1) & np.isfinite(lo) & np.isfinite(a))[0]:
        t = int(tc[i])
        w = window(cx, t // DAY * DAY, sess)
        if w is None or not (w[0] <= t < w[1] - 300):
            continue
        exit_by = int(min(t + hold * H, w[1]))
        vu = min(t + vb * tf * 60, exit_by)
        i0, i1 = (int(x) for x in mk.idx([t, vu]))
        hit = L8.first_touch(mk.h[i0:i1] >= hi[i], mk.l[i0:i1] <= lo[i])
        if hit is None:
            continue
        side = hit[1]
        lvl = hi[i] if side > 0 else lo[i]
        out.append({"ts": t, "side": side, "entry": "stop", "level": float(lvl), "valid_until": int(vu), "stop": float(mid[i]),
                    "target": float(lvl + side * st * a[i]), "exit_by": exit_by})
    return out


def run_opposite(ctx, coins, fam, p):
    if fam in A.CROSS:
        per = {s: [opposite(o) for o in ords] for s, ords in A.CROSS[fam]({s: ctx[s] for s in coins}, *p).items()}
    elif fam == "BLX":
        per = {s: blx_opposite(ctx[s], *p) for s in coins}
    else:
        per = {s: [opposite(o) for o in A.orders_for(ctx, s, fam, p)] for s in coins}
    by = {}
    for s in coins:
        tr = sl_core.run_orders(ctx[s].mk, per[s])
        by[s] = np.array([x[:5] for x in tr], dtype=float).reshape(-1, 5)
    return by


def dd_pct(t):
    cum = t.sort_values("xt")["pnl"].cumsum().to_numpy()
    return float((np.maximum.accumulate(np.concatenate([[0.0], cum]))[1:] - cum).max() / CAP * 100) if len(cum) else 0.0


def month_rows(t):
    rows = []
    for m in pd.period_range("2026-01", "2026-10", freq="M"):
        tm = t[t["month"] == m]
        bc = tm.groupby("sym")["pnl"].sum()
        main, extra = tm[tm["sym"].isin(A.MAIN)]["pnl"].sum(), tm[tm["sym"].isin(A.EXTRA)]["pnl"].sum()
        rows.append({"month": str(m), "trades": int(len(tm)), "pnl": round(float(tm["pnl"].sum()), 2),
                     "return_pct": round(float(tm["pnl"].sum() / CAP * 100), 2),
                     "gross_pct": round(float(tm["gross"].sum() * sl_core.NOTIONAL / CAP * 100), 2),
                     "win_rate": round(float((tm["net"] > 0).mean() * 100), 1) if len(tm) else None,
                     "coins_up": int((bc > 0).sum()), "main6_pct": round(float(main / (sl_core.NOTIONAL * 6) * 100), 2),
                     "extra10_pct": round(float(extra / (sl_core.NOTIONAL * 10) * 100), 2), "max_dd_pct": round(dd_pct(tm), 2)})
    return rows


def main():
    top = int(sys.argv[1]) if len(sys.argv) > 1 else 5
    opp = len(sys.argv) > 2 and sys.argv[2] == "opposite"
    fin = json.load(open(os.path.join(HERE, "results", "final.json"), encoding="utf-8"))
    picks = fin["rows"][:top]
    if opp:  # the opposite of the same five as the year test, even if the live ranking has moved since
        base = json.load(open(os.path.join(HERE, "results", "year2026.json"), encoding="utf-8"))["strategies"][:top]
        picks = [{**next((r for r in fin["rows"] if r["id"] == b["id"]), b), "return_pct": b["october_here_pct"],
                  "orig": b} for b in base]
    sl_core.DATA = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "year2026"))
    sl_core.SLIP.update({c: 0.0005 for c in A.EXTRA})
    set_period(Y0, Y1)
    ctx = {}
    for s in COINS:
        mk = sl_core.Market(s)
        ctx[s] = N2.Ctx(mk, SF.Features(mk))
    L8.ALL.update(ctx)
    out = []
    for rank, r in enumerate(picks, 1):
        fam, p = parse(r["id"])
        if opp:
            by = {**run_opposite(ctx, A.MAIN, fam, p), **run_opposite(ctx, A.EXTRA, fam, p)}
        else:
            by = {**A.run(ctx, A.MAIN, fam, p), **A.run(ctx, A.EXTRA, fam, p)}  # two sets, as in the search (cross-coin ranks)
        t = pd.DataFrame([(s, x[0], x[1], x[3], x[4]) for s, a in by.items() for x in a], columns=["sym", "et", "xt", "gross", "net"])
        t = t[(t["et"] >= Y0) & (t["et"] < Y1)].copy()
        t["pnl"] = t["net"] * sl_core.NOTIONAL
        t["month"] = pd.to_datetime(t["et"], unit="s").dt.to_period("M")
        months = month_rows(t)
        pre = t[t["et"] < OCT0]
        oct_here = months[-1]["return_pct"]
        rec = {"rank": rank, "id": r["id"], "family": r["family"], "title": ("Opposite of: " if opp else "") + r["title"], "rules": r["rules"],
               "october_in_search_pct": r["return_pct"], "october_here_pct": oct_here,
               "october_matches": None if opp else abs(oct_here - r["return_pct"]) < 0.005,
               "months": months, "ytd_pnl": round(float(t["pnl"].sum()), 2), "ytd_return_pct": round(float(t["pnl"].sum() / CAP * 100), 2),
               "jan_sep_pnl": round(float(pre["pnl"].sum()), 2), "jan_sep_return_pct": round(float(pre["pnl"].sum() / CAP * 100), 2),
               "months_up": int(sum(m["pnl"] > 0 for m in months)), "jan_sep_months_up": int(sum(m["pnl"] > 0 for m in months[:-1])),
               "trades": int(len(t)), "max_dd_pct": round(dd_pct(t), 2),
               "gross_ytd_pct": round(float(t["gross"].sum() * sl_core.NOTIONAL / CAP * 100), 2)}
        if opp:
            rec["original"] = {k: r["orig"][k] for k in ("ytd_return_pct", "jan_sep_return_pct", "october_here_pct", "months_up", "trades")}
        out.append(rec)
        print(f"#{rank} {'OPPOSITE ' if opp else ''}{r['id']}: Oct {oct_here:+.2f}% (search {r['return_pct']:+.2f}%), Jan-Sep "
              f"{rec['jan_sep_return_pct']:+.2f}%, 2026 {rec['ytd_return_pct']:+.2f}% (gross {rec['gross_ytd_pct']:+.2f}%), "
              f"{rec['months_up']}/10 months up, {rec['trades']} trades, max DD {rec['max_dd_pct']}%", flush=True)
        print("   " + "  ".join(f"{m['month'][5:]}:{m['return_pct']:+.2f}%" for m in months), flush=True)
    res = {"generated": pd.Timestamp.now(tz="UTC").strftime("%Y-%m-%d %H:%M UTC"), "capital": CAP, "coins": COINS,
           "period": "2026-01-01 to 2026-10-31", "mode": "opposite" if opp else "as found", "strategies": out}
    json.dump(res, open(os.path.join(HERE, "results", "year2026_opposite.json" if opp else "year2026.json"), "w", encoding="utf-8"), indent=1)
    inject(res, ("/*ANTI*/", "/*ENDANTI*/") if opp else ("/*YEAR*/", "/*END*/"))
    return 0 if opp or all(x["october_matches"] for x in out) else 1


def inject(res, marks):
    """Put the results into the report template (between the two markers); os_report copies the template into the
    page on its next round. Written to a temporary file and swapped in, so a rebuild never reads half a template."""
    path = os.path.join(HERE, "report_template.html")
    s = open(path, encoding="utf-8").read()
    if marks[0] not in s:
        print("template has no", marks[0], "marker; page not updated")
        return
    a, b = s.index(marks[0]) + len(marks[0]), s.index(marks[1])
    blob = json.dumps(res, separators=(",", ":")).replace("<", chr(92) + "u003c")
    open(path + ".tmp", "w", encoding="utf-8", newline="\n").write(s[:a] + blob + s[b:])
    os.replace(path + ".tmp", path)


if __name__ == "__main__":
    sys.exit(main())
