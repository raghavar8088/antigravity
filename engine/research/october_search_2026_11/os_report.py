"""Write REPORT.md and report/october-profitable-strategies.html for the October search
(rounds 1-3 plus every automatic round in results/auto/evaluated.jsonl).

Usage: python -I os_report.py
"""
import json
import os
import sys

import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results")
FAM = {"A": "Session-open breakout", "B": "Trend-day follow", "C": "Cross-coin", "F": "Volatility breakout", "G": "Hour-of-day",
       "N1": "EMA crossover", "N2": "Supertrend", "N3": "MACD", "N4": "VWAP pullback", "N5": "VWAP fade", "N6": "Previous-day breakout",
       "N7b": "CPR breakout", "N7f": "Pivot fade", "N8": "Volume candle", "N9": "RSI fade", "N10": "US pre-market breakout",
       "N11": "Hour-of-day", "N12": "Heikin-Ashi", "RSI": "RSI fade", "BBF": "Bollinger fade", "BBB": "Bollinger breakout",
       "STO": "Stochastic extreme", "DON": "Donchian breakout", "ROC": "Sharp-move momentum/reversal", "STK": "Candle-streak fade",
       "PIV": "Pivot levels", "TOD": "Hour-of-day", "ORB": "Session-open breakout", "CDL": "Candle pattern", "EMA": "EMA pullback",
       "KEL": "Keltner channel", "WPR": "Williams %R", "CCI": "CCI extreme", "ZEM": "EMA z-score fade", "MTF": "4h Bollinger + 1h RSI",
       "TD2": "Hour-of-day", "RS2": "RSI fade", "BB2": "Bollinger fade", "DOW": "Day-of-week window", "XS": "Cross-coin intraday",
       "ATX": "Range-expansion bar", "ICH": "Ichimoku", "SAR": "Parabolic SAR", "DMI": "DMI cross", "CMB": "RSI fade by hour and VWAP",
       "RZ": "RSI fade", "BZ": "Bollinger fade", "HZ": "Hour-of-day", "KZ": "Keltner channel", "CZ": "RSI fade by hour and VWAP",
       "MT2": "4h band + fast RSI", "FSH": "Fisher transform", "CRS": "Connors RSI", "MFI": "Money Flow Index",
       "DOF": "Fade from the day's open", "PCF": "Fade from yesterday's close", "KZH": "Keltner channel",
       "BZX": "Bollinger fade", "RZX": "RSI fade", "KZX": "Keltner channel", "ZEX": "EMA z-score fade", "HZX": "Hour-of-day",
       "MXX": "4h band + fast RSI", "STX": "Stochastic extreme", "WRX": "Williams %R", "CCX": "CCI extreme", "MFX": "Money Flow Index",
       "FSX": "Fisher transform", "DOX": "Fade from the day's open", "PCX": "Fade from yesterday's close", "ORX": "Session-open breakout"}


def title_a(vid, fallback):
    p = vid.split("|")
    if p[0] == "A":
        sess = {"US": "US-open", "LDN": "London-open", "UTC": "UTC-day"}[p[1]]
        bits = [f"{sess} {p[2]}-min breakout", {"close": "market entry", "stop": "stop entry", "retest": "limit retest"}[p[3]],
                "stop at far side" if p[4] == "far" else "stop at middle", "no target" if p[5] == "none" else f"{p[5].rstrip('0').rstrip('.')}R target"]
        bits += [{"trend": "with 10-day trend", "nr4": "after a narrow-range day", "tight": "tight opening range", "none": ""}[p[6]]]
        return ", ".join(b for b in bits if b)
    return fallback


def row(x, o, n, rnd, fam):
    pnl = o["pnl"] + n["pnl"]
    cap = o["capital"] + n["capital"]
    return {"id": x["id"], "round": rnd, "family": FAM.get(fam, fam), "title": title_a(x["id"], x["title"]), "rules": x["rules"],
            "trades": o["trades"] + n["trades"], "pnl": round(pnl, 2), "capital": cap, "return_pct": round(pnl / cap * 100, 2),
            "coins_up": o["coins_positive"] + n["coins_positive"], "orig_return_pct": o["return_pct"], "new_return_pct": n["return_pct"],
            "max_dd_pct": max(o["max_dd_pct"], n.get("max_dd_pct", 0)), "win": o.get("win_rate")}


def main():
    r1 = json.load(open(os.path.join(RES, "round1.json"), encoding="utf-8"))
    r2 = json.load(open(os.path.join(RES, "round2.json"), encoding="utf-8"))
    r3p = os.path.join(RES, "round3.json")
    r3 = json.load(open(r3p, encoding="utf-8")) if os.path.exists(r3p) else {"pool": 0, "profitable": 0, "consistent": 0, "results": []}
    rows = []
    for x in r2["results"]:
        if x["confirmed"]:
            o, n = dict(x["original6"]), dict(x["new10"])
            n.setdefault("capital", 100_000.0)
            rows.append(row(x, o, n, "1-2", x["id"].split("|")[0]))
    for x in r3["results"]:
        if x["confirmed"]:
            o, n = dict(x["original6"]), dict(x["new10"])
            n.setdefault("capital", 100_000.0)
            rows.append(row(x, o, n, "3", x["id"].split("|")[0]))
    auto = {"evaluated": 0, "profitable": 0, "consistent": 0, "rounds": 0}
    ev = os.path.join(RES, "auto", "evaluated.jsonl")
    if os.path.exists(ev):
        with open(ev, encoding="utf-8") as fh:
            for line in fh:
                x = json.loads(line)
                auto["evaluated"] += 1
                o = x.get("original6")
                if not o:
                    continue
                auto["profitable"] += bool(o["profitable"])
                auto["consistent"] += bool(o["consistent"])
                auto["rounds"] = max(auto["rounds"], x.get("round", 0))
                if x.get("confirmed"):
                    rows.append(row(x, o, x["new10"], str(x["round"]), x["family"]))
    rows.sort(key=lambda r: -r["return_pct"])
    best = {}
    for r in rows:
        best.setdefault(r["family"], r)
    funnel = {"pool": r1["pool"] + r3["pool"] + auto["evaluated"], "profitable": r1["profitable"] + r3["profitable"] + auto["profitable"],
              "consistent": r1["consistent"] + r3["consistent"] + auto["consistent"], "confirmed": len(rows),
              "rounds": max(3, auto["rounds"])}
    data = {"generated": pd.Timestamp.now(tz="UTC").strftime("%Y-%m-%d %H:%M UTC"), "funnel": funnel, "rows": rows[:60],
            "family_best": sorted(best.values(), key=lambda r: -r["return_pct"]), "total_rows": len(rows)}
    json.dump({**data, "rows": rows}, open(os.path.join(RES, "final.json"), "w", encoding="utf-8"), indent=1)
    tpl = open(os.path.join(HERE, "report_template.html"), encoding="utf-8").read()
    blob = json.dumps(data, separators=(",", ":")).replace("<", chr(92) + "u003c")
    os.makedirs(os.path.join(HERE, "report"), exist_ok=True)
    open(os.path.join(HERE, "report", "october-profitable-strategies.html"), "w", encoding="utf-8", newline="\n").write(
        tpl.replace("/*DATA*/null", blob))
    L = ["# Profitable intraday crypto strategies, October 2026\n",
         f"Generated {data['generated']}. October 2026 only (no trading on the 28th). Delta India fees, slippage and funding; "
         "$10,000 per trade, no leverage, flat by 00:00 UTC.\n",
         f"- Tested {funnel['pool']:,} strategies over {funnel['rounds']} rounds on BTC ETH SOL XRP DOGE BNB: {funnel['profitable']:,} profitable, "
         f"{funnel['consistent']:,} consistent (both halves of October, 3+ of 6 coins, 10+ trades).",
         f"- **{funnel['confirmed']:,} also made money on 10 other coins** (ADA LINK AVAX LTC DOT TRX BCH NEAR SUI APT; 5+ of 10).\n",
         "## Best per family\n", "| Family | Strategy | October return, 16 coins |", "|---|---|---:|"]
    for r in data["family_best"]:
        L.append(f"| {r['family']} | {r['title']} (`{r['id']}`) | {r['return_pct']:+.2f}% |")
    L += ["\n## Top 100\n", "| # | Strategy | Trades | October, 16 coins | 6 coins | 10 new coins | Coins up | Worst DD |",
          "|---|---|---:|---:|---:|---:|---:|---:|"]
    for i, r in enumerate(rows[:100], 1):
        L.append(f"| {i} | {r['title']} (`{r['id']}`) | {r['trades']} | **{r['return_pct']:+.2f}%** (${r['pnl']:,.0f}) | "
                 f"{r['orig_return_pct']:+.2f}% | {r['new_return_pct']:+.2f}% | {r['coins_up']}/16 | {r['max_dd_pct']:.2f}% |")
    L.append("\n## Rules (top 30)\n")
    for i, r in enumerate(rows[:30], 1):
        L.append(f"{i}. **{r['title']}**: {r['rules']}")
    open(os.path.join(HERE, "REPORT.md"), "w", encoding="utf-8", newline="\n").write("\n".join(L) + "\n")
    print(f"report: {len(rows)} confirmed of {funnel['pool']} tested")
    for i, r in enumerate(rows[:10], 1):
        print(f"{i:2d} {r['id']:<44} {r['return_pct']:+.2f}% | 6: {r['orig_return_pct']:+.2f}% | 10: {r['new_return_pct']:+.2f}% | {r['coins_up']}/16 | DD {r['max_dd_pct']}%")


if __name__ == "__main__":
    sys.exit(main())
