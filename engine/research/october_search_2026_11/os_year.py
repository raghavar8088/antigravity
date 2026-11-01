"""Year test of the October search's top strategies: every month of 2026 (January - October) with the same
rules, fills, fees, funding and coins as the search ($10,000 per coin on 16 coins, no leverage, flat by
00:00 UTC). Only October was used to pick these strategies, so January - September are months they have
never seen. The strategies are the top N rows of results/final.json when this runs (ids saved in the output).
Check: each strategy's October here must equal its October result in the search.

Usage: python -I os_year.py [top_n]
Writes results/year2026.json.
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

Y0 = int(pd.Timestamp("2026-01-01", tz="UTC").timestamp())
Y1 = int(pd.Timestamp("2026-11-01", tz="UTC").timestamp())
OCT0 = int(pd.Timestamp("2026-10-01", tz="UTC").timestamp())
COINS = A.MAIN + A.EXTRA
CAP = sl_core.NOTIONAL * len(COINS)


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
                     "win_rate": round(float((tm["net"] > 0).mean() * 100), 1) if len(tm) else None,
                     "coins_up": int((bc > 0).sum()), "main6_pct": round(float(main / (sl_core.NOTIONAL * 6) * 100), 2),
                     "extra10_pct": round(float(extra / (sl_core.NOTIONAL * 10) * 100), 2), "max_dd_pct": round(dd_pct(tm), 2)})
    return rows


def main():
    top = int(sys.argv[1]) if len(sys.argv) > 1 else 5
    fin = json.load(open(os.path.join(HERE, "results", "final.json"), encoding="utf-8"))
    picks = fin["rows"][:top]
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
        by = {**A.run(ctx, A.MAIN, fam, p), **A.run(ctx, A.EXTRA, fam, p)}  # two sets, as in the search (cross-coin ranks)
        t = pd.DataFrame([(s, x[0], x[1], x[4]) for s, a in by.items() for x in a], columns=["sym", "et", "xt", "net"])
        t = t[(t["et"] >= Y0) & (t["et"] < Y1)].copy()
        t["pnl"] = t["net"] * sl_core.NOTIONAL
        t["month"] = pd.to_datetime(t["et"], unit="s").dt.to_period("M")
        months = month_rows(t)
        pre = t[t["et"] < OCT0]
        oct_here = months[-1]["return_pct"]
        rec = {"rank": rank, "id": r["id"], "family": r["family"], "title": r["title"], "rules": r["rules"],
               "october_in_search_pct": r["return_pct"], "october_here_pct": oct_here, "october_matches": abs(oct_here - r["return_pct"]) < 0.005,
               "months": months, "ytd_pnl": round(float(t["pnl"].sum()), 2), "ytd_return_pct": round(float(t["pnl"].sum() / CAP * 100), 2),
               "jan_sep_pnl": round(float(pre["pnl"].sum()), 2), "jan_sep_return_pct": round(float(pre["pnl"].sum() / CAP * 100), 2),
               "months_up": int(sum(m["pnl"] > 0 for m in months)), "jan_sep_months_up": int(sum(m["pnl"] > 0 for m in months[:-1])),
               "trades": int(len(t)), "max_dd_pct": round(dd_pct(t), 2)}
        out.append(rec)
        print(f"#{rank} {r['id']}: Oct {oct_here:+.2f}% (search {r['return_pct']:+.2f}%), Jan-Sep {rec['jan_sep_return_pct']:+.2f}%, "
              f"2026 {rec['ytd_return_pct']:+.2f}%, {rec['months_up']}/10 months up, max DD {rec['max_dd_pct']}%", flush=True)
        print("   " + "  ".join(f"{m['month'][5:]}:{m['return_pct']:+.2f}%" for m in months), flush=True)
    res = {"generated": pd.Timestamp.now(tz="UTC").strftime("%Y-%m-%d %H:%M UTC"), "capital": CAP, "coins": COINS,
           "period": "2026-01-01 to 2026-10-31", "strategies": out}
    json.dump(res, open(os.path.join(HERE, "results", "year2026.json"), "w", encoding="utf-8"), indent=1)
    inject(res)
    return 0 if all(x["october_matches"] for x in out) else 1


def inject(res):
    """Put the results into the report template (between /*YEAR*/ and /*END*/); os_report copies the template into the
    page on its next round. Written to a temporary file and swapped in, so a rebuild never reads half a template."""
    path = os.path.join(HERE, "report_template.html")
    s = open(path, encoding="utf-8").read()
    a, b = s.index("/*YEAR*/") + len("/*YEAR*/"), s.index("/*END*/")
    blob = json.dumps(res, separators=(",", ":")).replace("<", chr(92) + "u003c")
    open(path + ".tmp", "w", encoding="utf-8", newline="\n").write(s[:a] + blob + s[b:])
    os.replace(path + ".tmp", path)


if __name__ == "__main__":
    sys.exit(main())
