"""Write REPORT.md and report/october-profitable-strategies.html for the October search.

Usage: python -I os_report.py   (after os_search.py and os_round2.py)
"""
import json
import os
import sys

import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results")


def title(vid, fallback):
    """Name that carries the settings that tell close variants apart."""
    p = vid.split("|")
    if p[0] == "A":
        sess = {"US": "US-open", "LDN": "London-open", "UTC": "UTC-day"}[p[1]]
        bits = [f"{sess} {p[2]}-min breakout", {"close": "market entry", "stop": "stop entry", "retest": "limit retest"}[p[3]],
                "stop at far side" if p[4] == "far" else "stop at middle", "no target" if p[5] == "none" else f"{p[5].rstrip('0').rstrip('.')}R target"]
        bits += [{"trend": "with 10-day trend", "nr4": "after a narrow-range day", "tight": "tight opening range", "none": ""}[p[6]]]
        return ", ".join(b for b in bits if b)
    if p[0] == "B":
        return f"Follow a {p[2]} ATR move 1 h after the US open" + (", with 10-day trend" if p[3] == "trend" else "") +             (", stop at day open" if p[4] == "open" else ", stop halfway back")
    return fallback


def main():
    r1 = json.load(open(os.path.join(RES, "round1.json"), encoding="utf-8"))
    r2 = json.load(open(os.path.join(RES, "round2.json"), encoding="utf-8"))
    r3p = os.path.join(RES, "round3.json")
    r3 = json.load(open(r3p, encoding="utf-8")) if os.path.exists(r3p) else None
    pool = [(x, "Rounds 1-2") for x in r2["results"]] + ([(x, "Round 3") for x in r3["results"]] if r3 else [])
    rows = []
    for x, rnd in pool:
        if not x["confirmed"]:
            continue
        a, b = x["original6"], x["new10"]
        pnl = a["pnl"] + b["pnl"]
        cap = a["capital"] + b.get("capital", 0)
        trades = a["trades"] + b["trades"]
        coins_up = a["coins_positive"] + b["coins_positive"]
        rows.append({"id": x["id"], "round": rnd, "title": title(x["id"], x["title"]), "rules": x["rules"], "trades": trades, "pnl": round(pnl, 2),
                     "capital": cap, "return_pct": round(pnl / cap * 100, 2), "coins_up": coins_up,
                     "orig_return_pct": a["return_pct"], "orig_pnl": a["pnl"], "orig_win": a["win_rate"],
                     "new_return_pct": b["return_pct"], "new_pnl": b["pnl"], "new_win": b.get("win_rate"),
                     "max_dd_pct": max(a["max_dd_pct"], b.get("max_dd_pct", 0)), "halves": a["halves"]})
    rows.sort(key=lambda r: -r["return_pct"])
    funnel = {"pool": r1["pool"] + (r3["pool"] if r3 else 0), "profitable": r1["profitable"] + (r3["profitable"] if r3 else 0),
              "consistent": r1["consistent"] + (r3["consistent"] if r3 else 0), "confirmed": len(rows)}
    data = {"generated": pd.Timestamp.now(tz="UTC").strftime("%Y-%m-%d"), "funnel": funnel, "rows": rows}
    json.dump(data, open(os.path.join(RES, "final.json"), "w", encoding="utf-8"), indent=1)
    tpl = open(os.path.join(HERE, "report_template.html"), encoding="utf-8").read()
    blob = json.dumps(data, separators=(",", ":")).replace("<", chr(92) + "u003c")
    os.makedirs(os.path.join(HERE, "report"), exist_ok=True)
    open(os.path.join(HERE, "report", "october-profitable-strategies.html"), "w", encoding="utf-8", newline="\n").write(
        tpl.replace("/*DATA*/null", blob))
    L = ["# Profitable intraday crypto strategies, October 2026\n",
         f"Generated {data['generated']}. October 2026 only (1-31 Oct, no trading on the 28th). Delta India fees, slippage and funding; "
         "$10,000 per trade, no leverage, flat by 00:00 UTC.\n",
         f"- Round 1: {funnel['pool']} strategies on BTC ETH SOL XRP DOGE BNB; {funnel['profitable']} profitable, "
         f"{funnel['consistent']} consistent (both halves of October, at least 3 of 6 coins, 10+ trades).",
         f"- Round 2: the consistent ones run unchanged on 10 other coins (ADA LINK AVAX LTC DOT TRX BCH NEAR SUI APT), October only; "
         f"**{funnel['confirmed']} made money there too** (5+ of 10 coins).\n",
         "| # | Strategy | Trades | October return, 16 coins | Original 6 | New 10 | Coins up (of 16) | Worst drawdown |",
         "|---|---|---:|---:|---:|---:|---:|---:|"]
    for i, r in enumerate(rows, 1):
        L.append(f"| {i} | {r['title']} (`{r['id']}`) | {r['trades']} | **{r['return_pct']:+.2f}%** (${r['pnl']:,.0f}) | "
                 f"{r['orig_return_pct']:+.2f}% | {r['new_return_pct']:+.2f}% | {r['coins_up']} | {r['max_dd_pct']:.2f}% |")
    L.append("\n## Rules\n")
    for i, r in enumerate(rows, 1):
        L.append(f"{i}. **{r['title']}**: {r['rules']}")
    L.append("\nReproduce: `python -I os_search.py && python -I os_fetch_extra.py && python -I os_round2.py && python -I os_report.py`")
    open(os.path.join(HERE, "REPORT.md"), "w", encoding="utf-8", newline="\n").write("\n".join(L) + "\n")
    print(len(rows), "confirmed strategies written")
    for i, r in enumerate(rows[:12], 1):
        print(f"{i:2d} {r['id']:<34} n={r['trades']:4d} 16-coin {r['return_pct']:+.2f}% (${r['pnl']:,.0f}) | 6: {r['orig_return_pct']:+.2f}% "
              f"| 10: {r['new_return_pct']:+.2f}% | coins up {r['coins_up']}/16 | DD {r['max_dd_pct']}%")


if __name__ == "__main__":
    sys.exit(main())
