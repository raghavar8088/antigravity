"""Write REPORT.md and report/swing-strategy-lab.html from the swing lab results.

Usage: python -I sw_report.py   (after sw_lab.py and sw_verify.py)
"""
import json
import os
import sys

import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results")


def describe(v):
    p = v.split("|")
    side = {"LO": "long only", "LS": "long and short"}
    if p[0] == "T1":
        return (f"{p[1]}-day moving-average trend ({side[p[2]]})",
                f"Every day, hold each coin long when yesterday's close is above its {p[1]}-day average"
                + (", otherwise short." if p[2] == "LS" else ", otherwise stay out."))
    if p[0] == "T2":
        f, s = p[1].split("/")
        return (f"{f}/{s}-day moving-average crossover ({side[p[2]]})",
                f"Long while the {f}-day average is above the {s}-day average"
                + (", short while below." if p[2] == "LS" else ", out otherwise."))
    if p[0] == "T3":
        a, b = p[1].split("/")
        return (f"Donchian {a}/{b} breakout ({side[p[2]]})",
                f"Buy a close above the highest high of the last {a} days and sell when it closes below the lowest low of the last {b}"
                + (" (and the mirror for shorts)." if p[2] == "LS" else "."))
    if p[0] == "T4":
        return (f"{p[1]}-day time-series momentum, {p[2]} ({side[p[3]]})",
                f"Hold each coin in the direction of its last {p[1]} days' return, rebalanced {p[2]}"
                + ("." if p[3] == "LS" else "; out when that return is negative."))
    if p[0] == "X1":
        return (f"Cross-coin {p[1]}-day momentum, top/bottom {p[2]}",
                f"Each Monday rank 12 coins by their last {p[1]} days' return; long the top {p[2]}, short the bottom {p[2]}.")
    if p[0] == "X2":
        reb = "daily" if int(p[1]) < 7 else "weekly"
        return (f"Cross-coin {p[1]}-day reversal, {p[2]} each side",
                f"Rebalanced {reb}: rank 12 coins by their last {p[1]} days' return; long the {p[2]} weakest, short the {p[2]} strongest.")
    if p[0] == "X3":
        return (f"Funding carry, {p[1]}-day average, {p[2]} each side, {p[3]}",
                f"Rebalanced {p[3]}: rank coins by their average perpetual funding over {p[1]} day(s); "
                f"short the {p[2]} highest (collect funding), long the {p[2]} lowest.")
    if p[0] == "M1":
        return (f"Daily RSI(2) dip-buying, under {p[1]}" + (", above the 200-day average" if p[2] == "sma200" else "")
                + f" ({side[p[3]]})",
                f"Buy when the 2-day RSI closes under {p[1]}" + (" and price is above its 200-day average" if p[2] == "sma200" else "")
                + "; exit above 70 or after 5 days" + (" (mirror for shorts)." if p[3] == "LS" else "."))
    if p[0] == "S1":
        return (f"One-day {'momentum' if p[2] == 'mom' else 'reversal'}, {p[1]} sd",
                f"Hold each coin for the next UTC day {'with' if p[2] == 'mom' else 'against'} yesterday's move when it exceeded "
                f"{p[1]} standard deviations of the last 20 days.")
    return v, ""


def main():
    s = json.load(open(os.path.join(RES, "summary.json"), encoding="utf-8"))
    ver = json.load(open(os.path.join(RES, "verify.json"), encoding="utf-8"))
    pre = json.load(open(os.path.join(RES, "prereg.json"), encoding="utf-8"))
    fins = []
    for f in s["finalists"]:
        t, r = describe(f["variant"])
        fins.append({**f, "title": t, "rules": r})
    allv = []
    for x in s["discovery_all"]:
        allv.append({**x, "title": describe(x["variant"])[0]})
    allv.sort(key=lambda x: -(x["t_daily"] or -99))
    data = {"generated": pd.Timestamp.now(tz="UTC").strftime("%Y-%m-%d"), "families": s["families"], "finalists": fins,
            "benchmarks": s["benchmarks"], "top": allv[:30], "variants": len(allv), "verify": ver,
            "fingerprint": pre["fingerprint"], "amendments": pre.get("amendments", []),
            "synthetic_share": s["synthetic_funding_share_disc"]}
    tpl = open(os.path.join(HERE, "report_template.html"), encoding="utf-8").read()
    blob = json.dumps(data, separators=(",", ":"), default=str).replace("<", chr(92) + "u003c")
    os.makedirs(os.path.join(HERE, "report"), exist_ok=True)
    with open(os.path.join(HERE, "report", "swing-strategy-lab.html"), "w", encoding="utf-8", newline="\n") as fh:
        fh.write(tpl.replace("/*DATA*/null", blob))

    passed = [f for f in fins if f["pass"]]
    L = []
    w = L.append
    w("# Swing strategy lab: multi-day crypto strategies under a locked protocol\n")
    w(f"Generated {data['generated']}. Protocol fingerprint {pre['fingerprint']} recorded before the first run.\n")
    w("## Result\n")
    w(f"- {len(allv)} variants in {len(s['families'])} families; discovery 2018-2022, hold-out 2023 to Oct 2026 (finalists only).")
    w(f"- **{len(passed)} of {len(fins)} finalists passed** (profit, daily t >= 2, both hold-out halves positive, 4 of 6 coins for single-coin families).\n")
    w("| Strategy | Discovery %/yr | t | Hold-out %/yr | t | Halves ($) | Max DD | Pass |")
    w("|---|---:|---:|---:|---:|---|---:|---|")
    for f in fins:
        d, h = f["discovery"], f["holdout"]
        w(f"| {f['title']} (`{f['variant']}`) | {d['annual_pct']}% | {d['t_daily']} | {h['annual_pct']}% | {h['t_daily']} | "
          f"{' / '.join(f'{x:,.0f}' for x in f['holdout_halves'])} | {h['max_dd_pct']}% | {'PASS' if f['pass'] else 'no'} |")
    w("")
    for f in fins:
        w(f"### {f['title']}{' (PASSED)' if f['pass'] else ''}\n")
        w(f"{f['rules']} Capital ${f['capital']:,.0f}.\n")
        yrs = {**f["discovery"]["by_year_pct"], **f["holdout"]["by_year_pct"]}
        w("| " + " | ".join(yrs) + " |")
        w("|" + "---:|" * len(yrs))
        w("| " + " | ".join(f"{v}%" for v in yrs.values()) + " |\n")
    w("## Benchmarks (always long, same costs and funding)\n")
    for k, v in s["benchmarks"].items():
        w(f"- {k}: discovery {v['discovery']['annual_pct']}%/yr (max DD {v['discovery']['max_dd_pct']}%), "
          f"hold-out {v['holdout']['annual_pct']}%/yr (max DD {v['holdout']['max_dd_pct']}%).")
    w("\n## Families\n")
    w("| Family | Variants | Profitable in discovery | Eligible |")
    w("|---|---:|---:|---:|")
    for k, x in s["families"].items():
        w(f"| {k} | {x['variants']} | {x['disc_positive']} | {x['eligible']} |")
    w("\n## Checks\n")
    w(f"- Positions and P&L of every finalist recomputed by independent loops: {len(ver['position_mismatches'])} position and "
      f"{len(ver['pnl_mismatches'])} P&L mismatches.")
    w(f"- No look-ahead: re-run on data cut at {ver['truncation_cut']}: {len(ver['truncation_mismatches'])} mismatches.")
    w(f"- Funding before each perpetual existed (2018-2019 mostly) is a synthetic 0.01% per 8 h: "
      f"{data['synthetic_share'] * 100:.0f}% of coin-days in discovery.\n")
    w("## Reproduce\n```\ncd engine/research/swing_lab_2026_11\npython -I sw_fetch.py && python -I sw_lab.py && python -I sw_verify.py && python -I sw_report.py\n```")
    open(os.path.join(HERE, "REPORT.md"), "w", encoding="utf-8", newline="\n").write("\n".join(L) + "\n")
    print("wrote REPORT.md and report/swing-strategy-lab.html;", len(passed), "passed of", len(fins))


if __name__ == "__main__":
    sys.exit(main())
