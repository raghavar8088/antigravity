"""Write REPORT.md and report/session-strategy-lab.html from the lab's results.

Usage: python -I sl_report.py   (after sl_run.py and sl_verify.py)
"""
import json
import math
import os
import sys

import numpy as np
import pandas as pd

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from sl_core import DATA, NOTIONAL, SYMBOLS  # noqa: E402
from sl_describe import describe  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results")
FAMILY = {"A": "Session opening-range breakout", "B": "Trend-day continuation", "C": "Cross-coin intraday",
          "F": "Volatility breakout", "G": "Time-of-day drift"}


def usd(v):
    if v is None:
        return "–"
    s = f"${abs(v):,.0f}"
    return ("−" + s) if v < -0.5 else (("+" + s) if v > 0.5 else "$0")


def num(v, d=2):
    if v is None or (isinstance(v, float) and math.isnan(v)):
        return "–"
    return ("−" if v < 0 else "") + f"{abs(v):,.{d}f}"


def main():
    s = json.load(open(os.path.join(RES, "summary.json"), encoding="utf-8"))
    ver = json.load(open(os.path.join(RES, "verify.json"), encoding="utf-8"))
    pre = json.load(open(os.path.join(RES, "prereg.json"), encoding="utf-8"))
    rep = json.load(open(os.path.join(DATA, "data_report.json"), encoding="utf-8"))
    z = np.load(os.path.join(DATA, "finalist_trades.npz"))
    months = pd.period_range("2021-01", "2026-10", freq="M")
    fins = []
    for f in s["finalists"]:
        v = f["variant"]
        title, rules = describe(v)
        arr = np.concatenate([z[f"{v}::{sym}".replace("|", "_")] for sym in SYMBOLS])
        if len(arr):
            et = pd.to_datetime(arr[:, 0], unit="s").to_period("M")
            m = pd.Series(arr[:, 4] * NOTIONAL).groupby(et).sum().reindex(months, fill_value=0.0)
            dd = pd.Series(arr[:, 4] * NOTIONAL).groupby((arr[:, 0] // 86400).astype(int)).sum()
            daily_cum = dd.sort_index().cumsum()
            cum = {"d": [int(x) for x in daily_cum.index], "v": [round(float(x), 2) for x in daily_cum.to_numpy()]}
        else:
            m = pd.Series(0.0, index=months)
            cum = {"d": [], "v": []}
        yearly = {str(y): round(float(m[[p.year == y for p in m.index]].sum()), 2) for y in range(2021, 2027)}
        fins.append({**f, "title": title, "rules": rules, "family": FAMILY[v[0]],
                     "monthly": {str(p): round(float(x), 2) for p, x in m.items()}, "yearly": yearly, "cum": cum})
    passed = [f for f in fins if f["pass"]]
    data = {
        "generated": pd.Timestamp.now(tz="UTC").strftime("%Y-%m-%d"),
        "variants": s["variants"], "bar": s["holdout_bar"], "families": s["families"], "family_names": FAMILY,
        "eligible": sum(x["eligible"] for x in s["families"].values()), "finalists": fins, "passed": len(passed),
        "top40": [{**x, "title": describe(x["variant"])[0]} for x in s["discovery_top40"]],
        "verify": ver, "fingerprint": pre["fingerprint"],
        "verify_sample": (json.load(open(os.path.join(RES, "verify_sample.json"), encoding="utf-8"))
                          if os.path.exists(os.path.join(RES, "verify_sample.json")) else None), "amendments": pre.get("amendments", []),
        "data": {"minutes": rep["BTCUSDT"]["minutes"], "bad_days": sorted({d for r in rep.values() for d in r["bad_days"]})},
    }
    tpl = open(os.path.join(HERE, "report_template.html"), encoding="utf-8").read()
    blob = json.dumps(data, separators=(",", ":"), default=str).replace("<", chr(92) + "u003c")
    os.makedirs(os.path.join(HERE, "report"), exist_ok=True)
    with open(os.path.join(HERE, "report", "session-strategy-lab.html"), "w", encoding="utf-8", newline="\n") as f:
        f.write(tpl.replace("/*DATA*/null", blob))

    L = []
    w = L.append
    w("# Session strategy lab: searching for intraday crypto strategies that survive costs\n")
    w(f"Generated {data['generated']}. Protocol and grid fingerprinted before the first run ({pre['fingerprint']}).\n")
    w("## Result\n")
    w(f"- {s['variants']} variants in 5 families were run on BTC, ETH, SOL, XRP, DOGE and BNB perpetuals with Delta India costs.")
    w(f"- Discovery (2021-2023): {data['eligible']} variants made money in each of the three years with daily t >= 2; "
      f"{len(fins)} finalists were chosen by the pre-set rule.")
    w(f"- Hold-out (Jan 2024 - Sep 2025, unseen until then): **{len(passed)} of {len(fins)} passed** "
      f"(profit, daily t >= {s['holdout_bar']}, profit in both halves, and on 4 of 6 coins).\n")
    w("## Finalists\n")
    w("| Strategy | Discovery 2021-23 | t | Hold-out 2024-Sep 25 | t | Halves | Coins + | Pass | Recent Oct 25-Oct 26 |")
    w("|---|---:|---:|---:|---:|---|---:|---|---:|")
    for f in fins:
        w(f"| {f['title']} (`{f['variant']}`) | {usd(f['discovery']['pnl'])} | {num(f['discovery']['t_daily'])} | "
          f"{usd(f['holdout']['pnl'])} | {num(f['holdout']['t_daily'])} | {' / '.join(usd(x) for x in f['holdout_halves'])} | "
          f"{f['holdout_coins_positive']} | {'PASS' if f['pass'] else 'no'} | {usd(f['recent']['pnl'])} |")
    w("")
    for f in fins:
        w(f"### {f['title']} {'(PASSED)' if f['pass'] else ''}\n")
        w(f"{f['rules']}\n")
        w("| " + " | ".join(f["yearly"]) + " |")
        w("|" + "---:|" * len(f["yearly"]))
        w("| " + " | ".join(usd(x) for x in f["yearly"].values()) + " |\n")
        w(f"Hold-out by coin: " + ", ".join(f"{k.replace('USDT', '')} {usd(v)}" for k, v in f["holdout"]["by_coin"].items()) + "\n")
    w("## Families\n")
    w("| Family | Variants | Profitable in discovery | Profitable before costs | Eligible |")
    w("|---|---:|---:|---:|---:|")
    for k, x in s["families"].items():
        w(f"| {FAMILY[k]} | {x['variants']} | {x['disc_positive']} | {x['disc_gross_positive']} | {x['eligible']} |")
    w("\n## Checks\n")
    w(f"- Independent re-fill of all finalist trades: {ver['fill_trades_checked']:,} trades, {len(ver['fill_mismatches'])} mismatches.")
    if data["verify_sample"]:
        vs = data["verify_sample"]
        w(f"- Engine check on {len(vs['sample_variants'])} variants covering every entry type: {vs['trades_checked']:,} trades, "
          f"{len(vs['mismatches'])} mismatches.")
    w(f"- No look-ahead: re-run on data cut at {ver['truncation_cut']}: {ver['truncation_trades']:,} earlier trades, "
      f"{len(ver['truncation_mismatches'])} mismatches.")
    for a in pre.get("amendments", []):
        w(f"- Amendment {a['from']} -> {a['to']}: {a['why']}")
    w("\n## Reproduce\n")
    w("```")
    w("cd engine/research/session_lab_2026_11")
    w("python -I sl_fetch.py && python -I sl_run.py && python -I sl_verify.py && python -I sl_report.py")
    w("```")
    with open(os.path.join(HERE, "REPORT.md"), "w", encoding="utf-8", newline="\n") as f:
        f.write("\n".join(L) + "\n")
    print("wrote REPORT.md and report/session-strategy-lab.html;", len(passed), "passed of", len(fins))


if __name__ == "__main__":
    sys.exit(main())
