"""Write REPORT.md and report/crypto-intraday-test.html from the run's results.

Usage: python -I ci_report.py   (after ci_run.py and ci_verify.py)
"""
import json
import math
import os
import sys

import numpy as np
import pandas as pd

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from ci_core import DATA, FEE_LEG, SLIP_LEG, SYMBOLS  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results")
OCT = int(pd.Timestamp("2026-10-01", tz="UTC").timestamp())
T_BAR = 2.81  # one-sided 5% after a Bonferroni split over 20 strategies
MON = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]


def sgn(v, d, suffix="", sign=True):
    if v is None or (isinstance(v, float) and math.isnan(v)):
        return "–"
    if abs(v) < 0.5 * 10 ** -d:
        v = 0.0
    s = f"{abs(v):,.{d}f}{suffix}"
    return ("−" + s) if v < 0 else (("+" + s) if sign and v > 0 else s)


def usd(v):
    if v is None:
        return "–"
    s = f"${abs(v):,.0f}"
    return ("−" + s) if v < -0.5 else (("+" + s) if v > 0.5 else "$0")


def gross_edge():
    t = pd.read_csv(os.path.join(DATA, "trades_all.csv"))
    out = {}
    for sid, g in t.groupby("strategy"):
        row = {}
        for key, part in (("ctx", g[g["entry_t"] < OCT]), ("oct", g[g["entry_t"] >= OCT])):
            r = part["gross"].to_numpy()
            row[f"{key}_n"] = int(len(r))
            row[f"{key}_bp"] = round(float(r.mean() * 1e4), 2) if len(r) else None
            row[f"{key}_t"] = round(float(r.mean() / (r.std(ddof=1) / math.sqrt(len(r)))), 2) if len(r) > 1 and r.std(ddof=1) > 0 else None
        out[sid] = row
    pd.DataFrame([{"id": k, **v} for k, v in out.items()]).to_csv(os.path.join(RES, "gross_edge.csv"), index=False)
    return out


def main():
    s = json.load(open(os.path.join(RES, "summary.json"), encoding="utf-8"))
    ver = json.load(open(os.path.join(RES, "verify.json"), encoding="utf-8"))
    pre = json.load(open(os.path.join(RES, "prereg.json"), encoding="utf-8"))
    rep = json.load(open(os.path.join(DATA, "data_report.json"), encoding="utf-8"))
    ge = gross_edge()
    rt_low = round(2 * (FEE_LEG + min(SLIP_LEG.values())) * 1e4, 1)
    rt_high = round(2 * (FEE_LEG + max(SLIP_LEG.values())) * 1e4, 1)

    strategies = []
    for x in s["strategies"]:
        g = ge.get(x["id"], {})
        strategies.append({
            **{k: x[k] for k in ("id", "name", "family", "source", "idea", "rules", "capital", "october", "context")},
            "by_coin": {c: {"pnl": v["pnl"], "trades": v["trades"]} for c, v in x["october_by_coin"].items()},
            "monthly": x["monthly_pnl"], "total12": round(sum(x["monthly_pnl"].values()), 2),
            "gross_ctx_bp": g.get("ctx_bp"), "gross_ctx_t": g.get("ctx_t"), "gross_oct_bp": g.get("oct_bp"),
        })
    sig = sorted([x for x in strategies if (x["gross_ctx_t"] or 0) >= T_BAR and (x["gross_ctx_bp"] or 0) > 0],
                 key=lambda x: -x["gross_ctx_bp"])
    best = sig[0] if sig else max(strategies, key=lambda x: x["gross_ctx_bp"] or -1e9)
    am = pre.get("amendments", [])
    data = {
        "generated": pd.Timestamp.now(tz="UTC").strftime("%Y-%m-%d"),
        "costs": {"rt_low": rt_low, "rt_high": rt_high}, "strategies": strategies, "months": s["months"],
        "coins": SYMBOLS, "persistence": s["persistence"],
        "totals": {"oct_trades": s["october_totals"]["trades"], "oct_pnl": s["october_totals"]["pnl"],
                   "oct_gross": s["october_totals"]["gross_pnl"], "ctx_pnl": s["context_totals"]["pnl"],
                   "ctx_positive": s["context_totals"]["strategies_positive"]},
        "best_edge": {"name": best["name"], "ctx_bp": best["gross_ctx_bp"], "ctx_t": best["gross_ctx_t"]},
        "significant": [{"name": x["name"], "bp": x["gross_ctx_bp"], "t": x["gross_ctx_t"]} for x in sig],
        "verify": ver, "data": {"minutes": rep["BTCUSDT"]["minutes"]},
        "prereg": {"first": pre["fingerprint"], "last": am[-1]["to"] if am else pre["fingerprint"],
                   "why": am[-1]["why"] if am else "none"},
    }
    tpl = open(os.path.join(HERE, "report_template.html"), encoding="utf-8").read()
    blob = json.dumps(data, separators=(",", ":"), default=str).replace("<", chr(92) + "u003c")
    os.makedirs(os.path.join(HERE, "report"), exist_ok=True)
    with open(os.path.join(HERE, "report", "crypto-intraday-test.html"), "w", encoding="utf-8", newline="\n") as f:
        f.write(tpl.replace("/*DATA*/null", blob))

    # ── REPORT.md ────────────────────────────────────────────────────────────
    P, T = s["persistence"], data["totals"]
    oct_sorted = sorted(strategies, key=lambda x: -x["october"]["pnl"])
    pos = [x for x in strategies if x["october"]["pnl"] > 0]
    L = []
    w = L.append
    w("# Twenty intraday crypto strategies, October 2026\n")
    w(f"Generated {data['generated']} by `ci_report.py`. Rules fingerprint {data['prereg']['first']} registered before the "
      f"first backtest; one amendment ({data['prereg']['last']}), explained under Checks.\n")
    w("## Verdict\n")
    w(f"- {len(pos)} of 20 strategies made money in October 2026"
      + (f", and that one ({pos[0]['name']}) on {pos[0]['october']['trades']} trade" if len(pos) == 1 else "")
      + f". All 20 together: {T['oct_trades']:,} trades, {usd(T['oct_pnl'])} net, {usd(T['oct_gross'])} before costs.")
    w(f"- Over the 11 months before (Nov 2025 to Sep 2026), {T['ctx_positive']} of 20 made money.")
    w(f"- Only {len(sig)} show a before-cost edge that clears the multiple-testing bar (t {T_BAR}): "
      + "; ".join(f"{x['name']} {sgn(x['bp'], 1, ' bp')} a trade (t {x['t']:.2f})" for x in data["significant"])
      + f". A Delta taker round trip costs {rt_low} to {rt_high} bp, so neither covers it.")
    n = len(P["last_month_top5"])
    w(f"- Trading each month's top 5 the next month lost money in {n - P['top5_positive']} of {n} months. "
      f"The five best over Nov–Sep ({', '.join(P['context_top5'])}) made {usd(P['context_top5_october_pnl'])} in October.\n")
    w("## October 2026, all 20 strategies\n")
    w("$10,000 on each of the six coins per strategy ($50,000 for S18, which skips BTC); every trade $10,000 notional, 1x. "
      "28 October excluded (Binance never published that day's perpetual data).\n")
    w("| Strategy | Trades | Win rate | Before costs / trade | Net / trade | Net | Return | Max DD | t |")
    w("|---|---:|---:|---:|---:|---:|---:|---:|---:|")
    for x in oct_sorted:
        o = x["october"]
        w(f"| {x['id']} {x['name']} | {o['trades']} | {sgn(o.get('win_rate'), 1, '%', False)} | {sgn(o.get('avg_gross_bp'), 1, ' bp')} | "
          f"{sgn(o.get('avg_net_bp'), 1, ' bp')} | {usd(o['pnl'])} | {sgn(o['return_pct'], 2, '%')} | {usd(o.get('max_dd')).replace('+', '')} | "
          f"{sgn(o.get('t_stat'), 2, '', False)} |")
    w(f"| **All 20** | {T['oct_trades']:,} | | | | **{usd(T['oct_pnl'])}** | | | |\n")
    w("### October by coin (net $ on that coin's $10,000)\n")
    w("| Strategy | " + " | ".join(c.replace("USDT", "") for c in SYMBOLS) + " |")
    w("|---|" + "---:|" * len(SYMBOLS))
    for x in oct_sorted:
        w(f"| {x['id']} {x['name']} | " + " | ".join(usd(x["by_coin"][c]["pnl"]) if c in x["by_coin"] else "–" for c in SYMBOLS) + " |")
    w("")
    w("## Edge before costs\n")
    w(f"Average move captured per trade before any cost. A Delta taker round trip costs {rt_low} bp (BTC, ETH) to {rt_high} bp (alts).\n")
    w("| Strategy | Nov–Sep trades | Nov–Sep before costs | t | October before costs | Break-even cost a side |")
    w("|---|---:|---:|---:|---:|---:|")
    for x in sorted(strategies, key=lambda x: -(x["gross_ctx_bp"] or -1e9)):
        g = ge.get(x["id"], {})
        w(f"| {x['id']} {x['name']} | {g.get('ctx_n', 0)} | {sgn(x['gross_ctx_bp'], 2, ' bp')} | {sgn(x['gross_ctx_t'], 2, '', False)} | "
          f"{sgn(x['gross_oct_bp'], 2, ' bp')} | {sgn((x['gross_ctx_bp'] or 0) / 2, 2, ' bp')} |")
    w("")
    w("## Twelve months, same rules (net $)\n")
    w("| Strategy | " + " | ".join(f"{MON[int(m[5:]) - 1]} {m[2:4]}" for m in s["months"]) + " | Total |")
    w("|---|" + "---:|" * (len(s["months"]) + 1))
    for x in sorted(strategies, key=lambda x: -x["total12"]):
        w(f"| {x['id']} {x['name']} | " + " | ".join(usd(x["monthly"][m]) for m in s["months"]) + f" | {usd(x['total12'])} |")
    w("")
    w("## Does last month's top 5 win next month?\n")
    w("| Picked in | Top 5 | Their next month | Any 5, next month, average |")
    w("|---|---|---:|---:|")
    for p in P["last_month_top5"]:
        w(f"| {p['picked_in']} | {' '.join(p['top5'])} | {usd(p['top5_next_pnl'])} | {usd(p['all20_next_pnl_avg_per5'])} |")
    w(f"\nMonth-to-month rank correlation averages {P['mean_spearman']}: the rules that trade least lose least every month, "
      "so the ranking is stable without being useful.\n")
    w("## The 20 strategies\n")
    for x in strategies:
        w(f"### {x['id']} {x['name']} ({x['family']})\n")
        w(f"- **Idea:** {x['idea']}")
        w(f"- **Rules:** {x['rules']}")
        w(f"- **Source:** {x['source']}")
        w(f"- **Result:** October {usd(x['october']['pnl'])} on {x['october']['trades']} trades; Nov–Sep {usd(x['context']['pnl'])} "
          f"on {x['context']['trades']} trades; before costs {sgn(x['gross_ctx_bp'], 2, ' bp')} a trade (t {sgn(x['gross_ctx_t'], 2, '', False)}).\n")
    w("## Data, method and checks\n")
    w("- Binance Vision USDT-M perpetuals (SHA-256 checked): 1m candles with taker-buy volume, 1m premium index, 5m open interest "
      "and long/short ratios, funding prints; BTC ETH SOL XRP DOGE BNB, Oct 2025 (warm-up) to Oct 2026.")
    w("- 28 Oct 2026 perpetual candles were never published: rebuilt from Binance spot (volume scaled to perpetual size) for the "
      "indicators only; no trades that day. The premium index is missing on 28-29 Oct; those minutes stay empty.")
    w("- Signals read closed bars only; entries fill at the next 1m open; positions are walked minute by minute (gap fills at "
      "the open, stop first when a minute reaches both levels); flat by 00:00 UTC; one position per strategy per coin.")
    w(f"- Costs a side: Delta India taker 0.05% + 18% GST = {FEE_LEG * 1e4:.1f} bp, plus 2 bp (BTC, ETH) or 4 bp (alts) "
      "spread and slippage; Binance funding prints, signed.")
    w(f"- Fills re-checked by a separately written simulator: {ver['fill_trades_checked']:,} trades, "
      f"{len(ver['fill_mismatches'])} mismatches. No look-ahead: a re-run on data cut at {ver['truncation_cut']} reproduces "
      f"all {ver['truncation_trades']:,} earlier trades, {len(ver['truncation_mismatches'])} mismatches.")
    w(f"- Amendment after the first run ({data['prereg']['first']} -> {data['prereg']['last']}): {data['prereg']['why']}\n")
    w("## Limits\n")
    w("- Binance prices and order flow stand in for Delta, where the trades would go; Delta's alt spreads are usually wider than 4 bp.")
    w("- One month is a small sample; the 11 earlier months show which results repeat.")
    w("- Taker fills on every side; resting limit orders would pay Delta's 0.02% maker fee, but breakouts and stops rarely fill that way.\n")
    w("## Reproduce\n")
    w("```")
    w("cd engine/research/crypto_intraday20_2026_11")
    w("python -I ci_fetch.py && python -I ci_run.py && python -I ci_verify.py && python -I ci_report.py")
    w("```")
    with open(os.path.join(HERE, "REPORT.md"), "w", encoding="utf-8", newline="\n") as f:
        f.write("\n".join(L) + "\n")
    print("wrote REPORT.md and report/crypto-intraday-test.html")


if __name__ == "__main__":
    sys.exit(main())
