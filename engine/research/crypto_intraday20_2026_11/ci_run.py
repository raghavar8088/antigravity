"""Run the 20 intraday strategies on 6 coins and score them.

Test month (the one asked for): October 2026, 1-31 Oct, no trading on the 28th
(rebuilt from spot because Binance never published that day's perpetual file).
Context: November 2025 to September 2026 with the same rules, untouched, to show
whether one month's ranking says anything about the next.

The SHA-256 of ci_core.py + ci_strategies.py is written to results/prereg.json
before the first full run and checked on every later one.

Usage: python -I ci_run.py [--smoke]   (--smoke: November 2025 trade counts only)
"""
import hashlib
import json
import math
import os
import sys
import time

import numpy as np
import pandas as pd

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))  # python -I leaves the script folder off the path
from ci_core import DATA, FEE_LEG, NOTIONAL, SLIP_LEG, SYMBOLS, Market, simulate
from ci_strategies import STRATEGIES

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results")
START = int(pd.Timestamp("2025-11-01", tz="UTC").timestamp())
OCT = int(pd.Timestamp("2026-10-01", tz="UTC").timestamp())
END = int(pd.Timestamp("2026-11-01", tz="UTC").timestamp())


def fingerprint():
    h = hashlib.sha256()
    for f in ("ci_core.py", "ci_strategies.py"):
        h.update(open(os.path.join(HERE, f), "rb").read().replace(b"\r\n", b"\n"))
    return h.hexdigest()[:16]


def stats(g, capital):
    n = len(g)
    if n == 0:
        return {"trades": 0, "pnl": 0.0, "return_pct": 0.0}
    net, pnl = g["net"].to_numpy(), g["pnl"].to_numpy()
    cum = np.cumsum(pnl[np.argsort(g["exit_t"].to_numpy(), kind="stable")])
    dd = float((np.maximum.accumulate(np.concatenate([[0.0], cum]))[1:] - cum).max())
    loss = -pnl[pnl < 0].sum()
    sd = net.std(ddof=1) if n > 1 else 0.0
    return {
        "trades": int(n), "pnl": round(float(pnl.sum()), 2), "return_pct": round(float(pnl.sum() / capital * 100), 2),
        "gross_pnl": round(float(g["gross"].sum() * NOTIONAL), 2),
        "win_rate": round(float((net > 0).mean() * 100), 1),
        "avg_gross_bp": round(float(g["gross"].mean() * 1e4), 2), "avg_net_bp": round(float(net.mean() * 1e4), 2),
        "profit_factor": round(float(pnl[pnl > 0].sum() / loss), 2) if loss > 0 else None,
        "t_stat": round(float(net.mean() / (sd / math.sqrt(n))), 2) if sd > 0 else None,
        "max_dd": round(dd, 2), "long_share": round(float((g["side"] > 0).mean() * 100), 1),
        "avg_hold_min": round(float(((g["exit_t"] - g["entry_t"]) / 60).mean()), 1),
    }


def main():
    smoke = "--smoke" in sys.argv
    fp = fingerprint()
    os.makedirs(RES, exist_ok=True)
    pre = os.path.join(RES, "prereg.json")
    if not smoke:
        if os.path.exists(pre):
            old = json.load(open(pre, encoding="utf-8"))
            allowed = ([a["to"] for a in old.get("amendments", [])] or [old["fingerprint"]])[-1]
            if allowed != fp:
                sys.exit(f"strategy code changed since pre-registration ({allowed} -> {fp}); log an amendment first")
        else:
            json.dump({"fingerprint": fp, "registered_utc": pd.Timestamp.now(tz="UTC").isoformat(timespec="seconds"),
                       "test": "2026-10-01..2026-10-31 (no trades on the rebuilt 2026-10-28)",
                       "context": "2025-11-01..2026-09-30", "symbols": SYMBOLS,
                       "fee_leg": FEE_LEG, "slip_leg": SLIP_LEG, "notional": NOTIONAL,
                       "strategies": [{k: s[k] for k in ("id", "name", "family", "source", "idea", "rules")} for s in STRATEGIES]},
                      open(pre, "w", encoding="utf-8"), indent=1)
    t0 = time.time()
    mk = {s: Market(s) for s in SYMBOLS}
    rows = []
    for st in STRATEGIES:
        n0 = len(rows)
        for s in SYMBOLS:
            for x in simulate(mk[s], st["fn"](mk[s], mk)):
                x["strategy"] = st["id"]
                rows.append(x)
        print(f"{st['id']} {st['name']:<36} {len(rows) - n0:6d} trades ({time.time() - t0:.0f}s)", flush=True)
    t = pd.DataFrame(rows)
    t = t[(t["entry_t"] >= START) & (t["entry_t"] < END)].reset_index(drop=True)
    t["month"] = pd.to_datetime(t["entry_t"], unit="s").dt.strftime("%Y-%m")
    if smoke:
        print(t[t["month"] == "2025-11"].groupby("strategy").size().to_string())
        return
    t.to_csv(os.path.join(DATA, "trades_all.csv"), index=False)
    oct_t = t[t["entry_t"] >= OCT]
    oct_t.to_csv(os.path.join(RES, "october_trades.csv"), index=False, encoding="utf-8")

    months = sorted(t["month"].unique())
    summary = {"fingerprint": fp, "months": months, "strategies": []}
    monthly = pd.DataFrame(0.0, index=[s["id"] for s in STRATEGIES], columns=months)
    for st in STRATEGIES:
        g = t[t["strategy"] == st["id"]]
        coins = [s for s in SYMBOLS if not (st["id"] == "S18" and s == "BTCUSDT")]
        cap = NOTIONAL * len(coins)
        go = g[g["entry_t"] >= OCT]
        gc = g[g["entry_t"] < OCT]
        for m, v in g.groupby("month")["pnl"].sum().items():
            monthly.loc[st["id"], m] = v
        summary["strategies"].append({
            **{k: st[k] for k in ("id", "name", "family", "source", "idea", "rules")},
            "capital": cap, "october": stats(go, cap), "context": stats(gc, cap),
            "october_by_coin": {s: stats(go[go["sym"] == s], NOTIONAL) for s in coins},
            "monthly_pnl": {m: round(float(monthly.loc[st["id"], m]), 2) for m in months},
        })

    # Does a month's ranking say anything about the next month?
    pairs, picks = [], []
    for a, b in zip(months[:-1], months[1:]):
        x, y = monthly[a], monthly[b]
        pairs.append({"from": a, "to": b, "spearman": round(float(x.rank().corr(y.rank())), 2)})
        top = x.sort_values(ascending=False).index[:5]
        picks.append({"picked_in": a, "traded_in": b, "top5": list(top),
                      "top5_next_pnl": round(float(y[top].sum()), 2), "all20_next_pnl_avg_per5": round(float(y.mean() * 5), 2)})
    ctx_total = monthly[[m for m in months if m < "2026-10"]].sum(axis=1)
    top_ctx = list(ctx_total.sort_values(ascending=False).index[:5])
    summary["persistence"] = {
        "month_to_month": pairs,
        "mean_spearman": round(float(np.mean([p["spearman"] for p in pairs])), 2),
        "last_month_top5": picks,
        "top5_beat_average": sum(p["top5_next_pnl"] > p["all20_next_pnl_avg_per5"] for p in picks),
        "top5_positive": sum(p["top5_next_pnl"] > 0 for p in picks),
        "context_top5": top_ctx,
        "context_top5_october_pnl": round(float(monthly.loc[top_ctx, "2026-10"].sum()), 2),
    }
    summary["october_totals"] = {
        "trades": int(len(oct_t)), "pnl": round(float(oct_t["pnl"].sum()), 2),
        "gross_pnl": round(float(oct_t["gross"].sum() * NOTIONAL), 2),
        "strategies_positive": int(sum(s["october"]["pnl"] > 0 for s in summary["strategies"])),
        "strategies_gross_positive": int(sum(s["october"].get("gross_pnl", 0) > 0 for s in summary["strategies"])),
    }
    summary["context_totals"] = {
        "strategies_positive": int(sum(s["context"]["pnl"] > 0 for s in summary["strategies"])),
        "pnl": round(float(t[t["entry_t"] < OCT]["pnl"].sum()), 2),
    }
    json.dump(summary, open(os.path.join(RES, "summary.json"), "w", encoding="utf-8"), indent=1)

    print(f"\nOCTOBER 2026 (fingerprint {fp})")
    for s in sorted(summary["strategies"], key=lambda s: -s["october"]["pnl"]):
        o, c = s["october"], s["context"]
        print(f"{s['id']} {s['name']:<36} n={o['trades']:4d} net ${o['pnl']:>9,.0f} ({o['return_pct']:6.2f}%) "
              f"win {o.get('win_rate')}% bp {o.get('avg_net_bp')} t {o.get('t_stat')} | prior 11m ${c['pnl']:>10,.0f} n={c['trades']}")
    print(json.dumps({k: v for k, v in summary["persistence"].items() if k not in ("month_to_month", "last_month_top5")}))
    print(summary["october_totals"], summary["context_totals"])


if __name__ == "__main__":
    sys.exit(main())
