"""Write REPORT.md and report/walk-forward.html from results/summary.json.

Usage: python -I wf_report.py
"""
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results")
MODE = {"LO": "long only", "LS": "long and short", "SO": "short only"}
DAYS = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"]


def describe(v):
    p = v.split("|")
    f = p[0]
    if f == "T1":
        return (f"Price vs {p[1]}-day average ({MODE[p[2]]})",
                f"Every day, hold each coin {'long when' if p[2] == 'LO' else ('short when' if p[2] == 'SO' else 'long when')} yesterday's close is "
                f"{'above' if p[2] != 'SO' else 'below'} its {p[1]}-day average" + (", short when below" if p[2] == "LS" else ", otherwise stay out") + ".")
    if f in ("T2", "T3"):
        a, b = p[1].split("/")
        kind = "moving-average" if f == "T2" else "EMA"
        return (f"{a}/{b}-day {kind} crossover ({MODE[p[2]]})", f"Hold each coin in the direction of the {a}-day {kind} against the {b}-day one ({MODE[p[2]]}).")
    if f == "T4":
        a, b = p[1].split("/")
        return (f"Donchian {a}/{b} breakout ({MODE[p[2]]})",
                f"Trade a close beyond the last {a} days' high or low ({MODE[p[2]]}); exit on a close beyond the last {b} days' opposite extreme.")
    if f == "T5":
        return (f"{p[1]}-day momentum, {p[2]} ({MODE[p[3]]})", f"Hold each coin in the direction of its {p[1]}-day price change ({MODE[p[3]]}), decided {p[2]}.")
    if f == "M1":
        return (f"RSI({p[1]}) dip-buying under {p[2]}, exit above {p[3]}" + (", above the 200-day average" if p[4] == "sma200" else "") + f" ({MODE[p[5]]})",
                f"Buy a coin when its daily RSI({p[1]}) closes under {p[2]} (sell over {100 - int(p[2])} if long and short); exit when RSI crosses {p[3]} or after 5 days.")
    if f == "M2":
        return (f"Bollinger({p[1]}, {p[2]}) daily mean reversion ({MODE[p[4]]})", f"Buy a close under the lower band, exit at the middle band or after {p[3]} days.")
    if f == "M3":
        return (f"{p[1]}-day low reversal, hold {p[2]} days ({MODE[p[3]]})",
                f"Buy a coin whose close is its lowest of the last {p[1]} days" + (" (sell a {}-day high)".format(p[1]) if p[3] == "LS" else "") + f"; hold {p[2]} days.")
    if f == "X1":
        return (f"Cross-coin {p[1]}-day momentum, {p[2]} each side, {p[3]}",
                f"Buy the {p[2]} coins that rose most over the last {p[1]} days and sell the {p[2]} that fell most; rebalance {p[3]}.")
    if f == "X2":
        return (f"Cross-coin {p[1]}-day reversal, {p[2]} each side, {p[3]}",
                f"{'Every Monday' if p[3] == 'weekly' else 'Every day'} buy the {p[2]} coin{'s' if p[2] != '1' else ''} that fell most over the last {p[1]} days and "
                f"sell the {p[2]} that rose most; hold until the next rebalance.")
    if f == "X3":
        return (f"Funding carry, {p[1]}-day average, {p[2]} each side, {p[3]}",
                f"{'Every Monday' if p[3] == 'weekly' else 'Every day'} sell the {p[2]} coins with the highest average funding rate over the last {p[1]} days and buy the "
                f"{p[2]} with the lowest; hold until the next rebalance. The shorts collect funding from crowded longs.")
    if f == "S1":
        return (f"One-day {'momentum' if p[2] == 'mom' else 'reversal'} after a {p[1]}-sd day",
                f"After a day that moved more than {p[1]} standard deviations, hold each coin {'with' if p[2] == 'mom' else 'against'} that move for one day.")
    if f == "B1":
        return (f"BTC {p[1]}-day trend steers all coins ({MODE[p[2]]})",
                f"When BTC closed {'below' if p[2] == 'SO' else 'above'} its {p[1]}-day average, hold all 16 coins {'short' if p[2] == 'SO' else 'long'}"
                + ("; short when below." if p[2] == "LS" else "; otherwise stay out."))
    if f == "V1":
        return (f"{p[1]}-day trend ({MODE[p[2]]}) only when volatility is {p[3]}",
                f"Price vs {p[1]}-day average, traded only while 20-day volatility is {'below' if p[3] == 'low' else 'above'} its 180-day median.")
    if f == "D1":
        return (f"{DAYS[int(p[1])]} {p[2]}", f"{'Buy' if p[2] == 'long' else 'Sell'} all 16 coins at {DAYS[int(p[1])]}'s 00:00 UTC open and close 24 hours later.")
    return v, ""


def main():
    S = json.load(open(os.path.join(RES, "summary.json"), encoding="utf-8"))
    pre = json.load(open(os.path.join(RES, "prereg.json"), encoding="utf-8"))
    fins = []
    for r in S["finalists"]:
        t, rules = describe(r["variant"])
        fins.append({"id": r["variant"], "family": r["family"][3:], "title": t, "rules": rules, "capital": r["capital"],
                     "choose": {k: r["choose"][k] for k in ("pnl", "return_pct", "months_up", "t_daily", "max_dd_pct", "entries", "months")},
                     "test": {k: r["test"][k] for k in ("pnl", "return_pct", "months_up", "max_dd_pct", "entries", "months")}, "pass": r["pass"]})
    fins.sort(key=lambda r: (not r["pass"], -r["test"]["return_pct"]))
    bench = {k: {"choose": v["choose"]["return_pct"], "test": v["test"]["return_pct"],
                 "months": {m: round(x / (160000 if "16" in k else 10000) * 100, 2) for m, x in {**v["choose"]["months"], **v["test"]["months"]}.items()}}
             for k, v in S["benchmarks"].items()}
    data = {"fingerprint": S["fingerprint"], "registered": pre["registered_utc"], "families": S["families"], "finalists": fins,
            "selection": S["selection"], "bench": bench, "variants": len(S["all"])}
    tpl = open(os.path.join(HERE, "report_template.html"), encoding="utf-8").read()
    os.makedirs(os.path.join(HERE, "report"), exist_ok=True)
    open(os.path.join(HERE, "report", "walk-forward.html"), "w", encoding="utf-8", newline="\n").write(
        tpl.replace("/*DATA*/null", json.dumps(data, separators=(",", ":")).replace("<", chr(92) + "u003c")))
    sel = S["selection"]
    L = ["# Walk-forward swing test: chosen on January-June 2026, tested on July-October 2026\n",
         f"Protocol fingerprint {S['fingerprint']} recorded {pre['registered_utc']} before the first run. 16 USDT perpetuals, daily decisions, "
         "Delta India taker fee + slippage + real funding, no leverage.\n",
         f"- {len(S['all'])} variants in {len(S['families'])} families; {sel['chosen']} chosen on January-June (profit, 4+ of 6 months up, 6+ entries, "
         "drawdown <= 25%); one finalist per family.",
         f"- **{sum(r['pass'] for r in fins)} of {len(fins)} finalists passed** July-October (profit and 3+ of 4 months up).",
         f"- Chosen variants made money in the test {sel['chosen_test_profitable_pct']}% of the time (median {sel['chosen_test_median_pct']:+.2f}%), "
         f"against {sel['not_chosen_test_profitable_pct']}% for the rest (median {sel['not_chosen_test_median_pct']:+.2f}%); rank correlation between "
         f"January-June and July-October returns {sel['rank_corr_choose_vs_test']}.",
         f"- Buy and hold all 16 coins: January-June {bench['Buy and hold all 16 coins']['choose']:+.2f}%, July-October {bench['Buy and hold all 16 coins']['test']:+.2f}%.\n",
         "| Strategy | Capital | Jan-Jun | Months up | Jul-Oct | Months up | Result |", "|---|---:|---:|---:|---:|---:|---|"]
    for r in fins:
        L.append(f"| {r['title']} (`{r['id']}`) | ${r['capital']:,.0f} | {r['choose']['return_pct']:+.2f}% | {r['choose']['months_up']}/6 | "
                 f"{r['test']['return_pct']:+.2f}% | {r['test']['months_up']}/4 | {'PASS' if r['pass'] else 'fail'} |")
    L.append("")
    for r in fins:
        if r["pass"]:
            L.append(f"**{r['title']}**: {r['rules']}\n")
            L.append("| " + " | ".join(m[5:] for m in sorted({**r['choose']['months'], **r['test']['months']})) + " |")
            L.append("|" + "---:|" * 10)
            ms = {**r["choose"]["months"], **r["test"]["months"]}
            L.append("| " + " | ".join(f"{ms[m] / r['capital'] * 100:+.2f}%" for m in sorted(ms)) + " |\n")
    open(os.path.join(HERE, "REPORT.md"), "w", encoding="utf-8", newline="\n").write("\n".join(L) + "\n")
    print("report written:", sum(r["pass"] for r in fins), "passed of", len(fins))


if __name__ == "__main__":
    sys.exit(main())
