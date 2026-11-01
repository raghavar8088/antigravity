"""Write REPORT.md and report/btc-deep-backtest.html from the analysis results.

Usage: python -I deep_report.py
"""
import json
import os
import re
import sys

import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
ENGINE = os.path.normpath(os.path.join(HERE, "..", ".."))
RES = os.path.join(HERE, "results")
DEEP = os.path.join(ENGINE, "data", "historical", "deep")
YEARS = list(range(2018, 2027))
MON = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]


def pct(x, d=1, sign=True):
    if x is None:
        return "–"
    if abs(x) < 0.5 * 10 ** -d:
        x = 0.0
    s = f"{abs(x):,.{d}f}%"
    if x < 0:
        return "−" + s
    return ("+" + s) if sign and x > 0 else s


def num(x, d=2):
    if x is None:
        return "–"
    s = f"{abs(x):,.{d}f}"
    return ("−" + s) if x < 0 else s


def btc_context():
    df = pd.read_csv(os.path.join(DEEP, "BTCUSDT_1m.csv"), usecols=["time", "open", "close"])
    df["ts"] = pd.to_datetime(df["time"], unit="s", utc=True)
    df = df[df["ts"] >= pd.Timestamp("2018-01-01", tz="UTC")]
    m = df.groupby(df["ts"].dt.tz_localize(None).dt.to_period("M"))["close"].last()
    yearly = {}
    for y in YEARS:
        d = df[df["ts"].dt.year == y]
        yearly[y] = round(float((d["close"].iloc[-1] / d["open"].iloc[0] - 1) * 100), 1)
    first, last = float(df["open"].iloc[0]), float(df["close"].iloc[-1])
    return {"months": [str(p) for p in m.index], "close": [round(float(v), 0) for v in m.to_numpy()],
            "yearly": yearly, "first": round(first), "last": round(last)}


def probe_numbers():
    txt = open(os.path.join(RES, "probe.txt"), encoding="utf-8").read()
    eng, fix = txt.split("FIXED clock")
    def grab(block):
        ev = int(re.search(r"evaluations (\d+)", block).group(1))
        age = re.search(r"entry tick ([0-9hms]+)", block).group(1)
        stale = float(re.search(r"stale >15m: ([0-9.]+)%", block).group(1))
        la = re.search(r"1m ([0-9.]+)%\s+1h ([0-9.]+)%\s+4h ([0-9.]+)%", block)
        return {"evaluations": ev, "age": age, "stale_pct": stale,
                "lookahead_1m": float(la.group(1)), "lookahead_1h": float(la.group(2)), "lookahead_4h": float(la.group(3))}
    return {"engine": grab(eng), "fixed": grab(fix)}


def slim(p):
    keep = ("trades", "net_pct", "per_year_pct", "win_rate_pct", "profit_factor", "avg_trade_bp", "t_stat",
            "max_dd_pct", "monthly_sharpe", "positive_months_pct", "trades_per_year", "avg_hold_h",
            "long_share_pct", "tp_exit_pct")
    return {k: p.get(k) for k in keep}


def main():
    s = json.load(open(os.path.join(RES, "summary.json"), encoding="utf-8"))
    ver = json.load(open(os.path.join(RES, "verify.json"), encoding="utf-8"))
    tab = pd.read_csv(os.path.join(RES, "strategies.csv"))
    btc = btc_context()
    probe = probe_numbers()
    u, lists, wl = s["universe"], s["lists"], s["whitelist"]

    def strat(r):
        return {"rank": r["rank"], "name": r["strategy"], "desc": r["description"], "tf": r["timeframes"],
                "full": slim(r["full"]), "train": slim(r["train"]), "test": slim(r["test"]),
                "gross": r["full_gross"]["net_pct"], "binance": r["full_binance"]["net_pct"],
                "engine": r["engine_full"], "yearly": r["yearly"], "monthly": r["monthly"]}

    scatter = [{"n": row.strategy, "fx": row.binance_full_net_pct, "en": row.engine_full_net_pct,
                "t": int(row.delta_full_trades)} for row in tab.itertuples()
               if pd.notna(row.engine_full_net_pct) and row.delta_full_trades > 0]
    demo = next((d for d in s["demo_only"] if d["strategy"].startswith("Big_Up")), None)
    data = {
        "generated": pd.Timestamp.now(tz="UTC").strftime("%Y-%m-%d"),
        "costs": s["costs_per_leg_bp"], "data": {k: s["data"][k] for k in (
            "first_bar", "last_bar", "bars", "missing_minutes", "holes", "largest_holes", "zero_volume_bars",
            "off_grid_times_realigned", "funding_first", "funding_last", "funding_prints", "cross_check_old_cache")},
        "universe": {k: v for k, v in u.items() if k != "engine"}, "engine": u.get("engine"),
        "top": [strat(r) for r in lists["full"]["top10"]], "book": lists["full"]["book"],
        "trainTop": [strat(r) for r in lists["train"]["top10"]], "trainBook": lists["train"]["book"],
        "skippedFull": lists["full"]["skipped_near_duplicates"], "skippedTrain": lists["train"]["skipped_near_duplicates"],
        "eligibleFull": lists["full"]["eligible"], "eligibleTrain": lists["train"]["eligible"],
        "btc": btc, "probe": probe, "verify": ver, "scatter": scatter,
        "whitelist": {k: v for k, v in wl.items() if k != "rows"},
        "whitelistRows": sorted(wl["rows"], key=lambda r: -r["full_net_pct"]),
        "demo": {"name": demo["strategy"], "desc": demo["description"], "full": slim(demo["full"]),
                 "train": slim(demo["train"]), "test": slim(demo["test"])} if demo else None,
    }

    tpl = open(os.path.join(HERE, "report_template.html"), encoding="utf-8").read()
    os.makedirs(os.path.join(HERE, "report"), exist_ok=True)
    blob = json.dumps(data, separators=(",", ":"), default=str).replace("<", chr(92) + "u003c")
    html = tpl.replace("/*DATA*/null", blob)
    with open(os.path.join(HERE, "report", "btc-deep-backtest.html"), "w", encoding="utf-8", newline="\n") as f:
        f.write(html)

    # ── REPORT.md ────────────────────────────────────────────────────────────
    top, book, tbook = data["top"], data["book"], data["trainBook"]
    L = []
    w = L.append
    w("# BTC deep backtest: 334 registry strategies, January 2018 to October 2026\n")
    w(f"Generated {data['generated']} by `engine/research/btc_deep_2026_10/deep_report.py`. Every number below is "
      "reproducible from the scripts in this folder and `engine/cmd/deep_backtest`.\n")
    w("## Verdict\n")
    w(f"- Every OHLCV-compatible strategy in the scalpers registry ({u['strategies']}) was replayed on "
      f"{data['data']['bars']:,} Binance BTCUSDT one-minute bars, {data['data']['first_bar'][:10]} to "
      f"{data['data']['last_bar'][:10]}, trading from 2018-01-01: {u['trades_fixed']:,} trades.")
    w(f"- Before costs {u['gross_positive_full']} strategies make money. After Delta India's taker fee (0.05% + 18% GST) "
      f"and 2 bp of spread and slippage a leg, {u['net_positive_full']['delta']} do. The average trade goes from "
      f"{num(u['mean_gross_trade_bp'])} bp gross to {num(u['mean_net_trade_bp'])} bp net.")
    w(f"- No strategy clears the multiple-testing bar. The best full-period t-statistic is {num(u['full_best_t'])}; "
      f"the largest t that luck alone produces among {u['strategies']} tries is about {num(u['expected_max_t_under_null'])}, "
      f"and the Bonferroni 5% bar is {num(u['bonferroni_t_one_sided_5pct'])}.")
    w(f"- The ten best over the whole period, as an equal-weight book ($10,000 each, 1x notional), made "
      f"{pct(book['full_pct'])} in 8.8 years, max drawdown {pct(book['max_dd_pct'], sign=False)}. That list is chosen with hindsight.")
    w(f"- Chosen the honest way, on 2018-2022 only, the ten best made {pct(tbook['train_pct'])} there and "
      f"{pct(tbook['test_pct'])} over 2023-01 to 2026-10. Ranking a backtest like this does not pick strategies that keep working.")
    pe = probe["engine"]
    w(f"- The engine's existing backtest harness (the one that qualified the live 49-strategy whitelist) evaluates every "
      f"strategy on data {pe['age'].replace('0s', '')} old ({pe['stale_pct']:.0f}% of {pe['evaluations']:,} evaluations), and on "
      f"{pe['lookahead_4h']}% of them the newest 4h candle closes after the entry. Under it "
      f"{data['engine']['net_positive_full_engine_harness']} strategies look profitable; on the corrected clock at the same "
      f"cost, {data['engine']['net_positive_full_fixed_binance_cost']}.\n")

    w("## The top 10, ranked on the whole period (hindsight)\n")
    w("Rule: t-statistic of net return per trade over 2018-01-01 to 2026-10-30, at least 80 trades, positive net; "
      "a strategy whose monthly returns correlate above 0.7 with one already chosen is skipped "
      f"({', '.join(x['strategy'] for x in data['skippedFull'])} were near-copies of a higher one). "
      "Costs: Delta India taker both legs plus 2 bp, signed Binance funding. $10,000 per strategy, every trade $10,000 notional.\n")
    w("| # | Strategy | Trades | Per year | Win rate | Profit factor | Net 2018-26 | Max DD | t | 2018-22 | 2023-26 |")
    w("|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
    for r in top:
        f, a, b = r["full"], r["train"], r["test"]
        w(f"| {r['rank']} | `{r['name']}` | {f['trades']} | {f['trades_per_year']} | {f['win_rate_pct']}% | "
          f"{num(f['profit_factor'])} | {pct(f['net_pct'])} | {pct(f['max_dd_pct'], sign=False)} | {num(f['t_stat'])} | "
          f"{pct(a['net_pct'])} | {pct(b['net_pct'])} |")
    w(f"| | **Book of ten** | {book['trades']} | | | | **{pct(book['full_pct'])}** | {pct(book['max_dd_pct'], sign=False)} | | "
      f"{pct(book['train_pct'])} | {pct(book['test_pct'])} |\n")
    w("What each one does (the registry's own description):\n")
    for r in top:
        w(f"{r['rank']}. `{r['name']}`: {r['desc']} ({'/'.join(r['tf'] or [])}; long share {r['full']['long_share_pct']}%, "
          f"average hold {r['full']['avg_hold_h']} h, target hit on {r['full']['tp_exit_pct']}% of trades).")
    w("")
    w("### Yearly net return, % of the strategy's $10,000\n")
    w("| Strategy | " + " | ".join(str(y) if y < 2026 else "2026 to 30 Oct" for y in YEARS) + " |")
    w("|---|" + "---:|" * len(YEARS))
    for r in top:
        w(f"| `{r['name']}` | " + " | ".join(pct(r["yearly"][str(y)]) for y in YEARS) + " |")
    w("| **Book of ten** | " + " | ".join(f"**{pct(book['yearly'][str(y)])}**" for y in YEARS) + " |")
    w("| BTC buy and hold | " + " | ".join(pct(btc["yearly"][y], 0) for y in YEARS) + " |\n")
    w("### Monthly net return, % of the strategy's $10,000\n")
    w("Months are booked on the exit date. A blank month had no closed trade.\n")
    for r in top + [{"rank": "", "name": "Book of ten (equal weight)", "monthly": book["monthly"]}]:
        w(f"#### {r['rank']}{'. ' if r['rank'] else ''}{r['name']}\n")
        w("| Year | " + " | ".join(MON) + " | Year |")
        w("|---|" + "---:|" * 13)
        for y in YEARS:
            vals = [r["monthly"].get(f"{y}-{m:02d}") for m in range(1, 13)]
            cells = ["" if (v is None or v == 0) else pct(v) for v in vals]
            tot = sum(v for v in vals if v is not None)
            w(f"| {y} | " + " | ".join(cells) + f" | {pct(tot)} |")
        w("")

    w("## Does the ranking predict anything?\n")
    w(f"The same rule run on 2018-2022 only ({data['eligibleTrain']} eligible strategies), then frozen:\n")
    w("| # | Strategy | 2018-22 trades | 2018-22 net | t | 2023-26 trades | 2023-26 net | t |")
    w("|---|---|---:|---:|---:|---:|---:|---:|")
    for r in data["trainTop"]:
        a, b = r["train"], r["test"]
        w(f"| {r['rank']} | `{r['name']}` | {a['trades']} | {pct(a['net_pct'])} | {num(a['t_stat'])} | {b['trades']} | "
          f"{pct(b['net_pct'])} | {num(b['t_stat'])} |")
    w(f"| | **Book of ten** | | **{pct(tbook['train_pct'])}** | | | **{pct(tbook['test_pct'])}** | |\n")
    pos = sum(1 for r in data["trainTop"] if r["test"]["net_pct"] > 0)
    w(f"{pos} of the 10 stayed positive out of sample; the book's monthly Sharpe went from {num(tbook['monthly_sharpe'])} "
      f"over the whole run to {num(tbook['test_monthly_sharpe'])} in the hold-out. Across the universe, "
      f"{u['net_positive_train']} strategies were net-positive in 2018-2022, {u['net_positive_test']} in 2023-2026, and "
      f"{u['net_positive_train_and_test']} in both.\n")

    w("## The existing harness trades on 3.5-hour-old data\n")
    w("`cmd/deep_backtest -mode probe` runs a probe strategy through `backtest.ScalerV3Adapter` + the v3 engine and records what "
      "it can see when it is evaluated (the entry fills at that tick):\n")
    w(f"- The newest 15m bar in the context is {pe['age'].replace('0s', '')} old at the entry tick, on {pe['stale_pct']:.0f}% of "
      f"{pe['evaluations']:,} evaluations. The v3 engine skips strategies until its 60-price warm-up is full (15 candles), "
      "but the adapter numbers candles by counting the calls it receives, so it builds the context 14 candles behind the tick.")
    w(f"- On {pe['lookahead_4h']}% of evaluations the newest 4h candle closes after the entry tick: `ContextBuilder.BuildAt` "
      "includes the 1h and 4h candles still forming at its cut-off, with their final high, low and close.")
    w(f"- `ctx.Now` is the open of the context bar, 15 minutes before the data's cut-off; `ctx.Price` is that bar's close.")
    w(f"- Effect over 2018-2026 at the engine's own Binance cost: {data['engine']['net_positive_full_engine_harness']} of "
      f"{u['strategies']} strategies net-positive under the harness against "
      f"{data['engine']['net_positive_full_fixed_binance_cost']} on the corrected clock; rank correlation "
      f"{num(data['engine']['rank_correlation_engine_vs_fixed'])}. The six best results under the harness:\n")
    w("| Strategy | Harness trades | Harness net | Corrected net |")
    w("|---|---:|---:|---:|")
    for r in data["engine"]["best_engine"][:6]:
        w(f"| `{r['strategy']}` | {int(r['engine_full_trades'])} | {pct(r['engine_full_net_pct'])} | {pct(r['binance_full_net_pct'])} |")
    top_gap = data["engine"]["best_engine"][0]
    w(f"\n`{top_gap['strategy']}` buys when the latest 4h candle closes in the top 20% of its range. Under the harness that "
      "candle can still be forming yet already carry its final close, so the rule knows the bar will finish strong before it "
      f"buys: {pct(top_gap['engine_full_net_pct'])} there, {pct(top_gap['binance_full_net_pct'])} on the corrected clock. "
      "`Big_Bear_Candle_Short` is the same mechanism on the short side.\n")

    w("## The live 49-strategy whitelist on the corrected clock\n")
    w(f"`data/btc_prelive_whitelist.json` was qualified by `btc_qualify_25` on the harness above (Binance 2021-07 to 2026-07, "
      f"validate {wl['validate_window'][0]} to {wl['validate_window'][1]}). On the corrected clock with Delta costs, "
      f"{wl['net_positive_validate_window']} of {wl['found']} are net-positive in that validate window and "
      f"{wl['net_positive_full']} of {wl['found']} over 2018-2026. In the top 10 above: "
      f"{', '.join('`' + n + '`' for n in wl['in_top10_full']) or 'none'}.\n")
    w("| Strategy | Trades 2018-26 | Net 2018-26 | Validate-window net | Harness net 2018-26 |")
    w("|---|---:|---:|---:|---:|")
    for r in data["whitelistRows"]:
        w(f"| `{r['strategy']}` | {r['full_trades']} | {pct(r['full_net_pct'])} | {pct(r['validate_net_pct'])} | "
          f"{pct(r['engine_full_net_pct'])} |")
    w("")
    if data["demo"]:
        d = data["demo"]
        w("## Not ranked: the registry's demo strategies\n")
        w(f"Three strategies are labelled DEMO ONLY in the registry and were left out of both rankings. "
          f"`{d['name']}` ({d['desc']}) would have placed in the full-period list: {d['full']['trades']} trades, "
          f"{pct(d['full']['net_pct'])}, t {num(d['full']['t_stat'])}, max drawdown {pct(d['full']['max_dd_pct'], sign=False)}, "
          f"{pct(d['train']['net_pct'])} in 2018-22 and {pct(d['test']['net_pct'])} in 2023-26. It is a long-only 4h "
          "Donchian(20) breakout, so most of that is BTC's own rise from about "
          f"${btc['first']:,} to ${btc['last']:,}.\n")

    dd = data["data"]
    w("## Data\n")
    w(f"- Binance Vision spot BTCUSDT 1m klines, every archive SHA-256 checked: {dd['bars']:,} bars, {dd['first_bar']} to "
      f"{dd['last_bar']} UTC. 2017-08 to 2017-12 is indicator warm-up; decisions run 2018-01-01 to 2026-10-30.")
    w(f"- {dd['holes']} holes, {dd['missing_minutes']:,} missing minutes (0.18%), all Binance outages; the largest is "
      f"{dd['largest_holes'][0]['missing_minutes']:,} minutes from {dd['largest_holes'][0]['after']}. A position open across a hole "
      "exits at the first bar after it if that bar opens beyond a level.")
    w(f"- {dd['off_grid_times_realigned']:,} bars in Dec 2017 and Feb 2018 were stamped 14-20 s past the minute and were put back "
      f"on the grid; {dd['zero_volume_bars']:,} minutes have zero volume, almost all in 2017.")
    cc = dd["cross_check_old_cache"]
    w(f"- Cross-check against the engine's existing cache: {cc['identical_ohlc']:,} of {cc['full_hours_compared']:,} complete hours "
      "have identical OHLC.")
    w(f"- Funding: {dd['funding_prints']:,} Binance USDT-M 8h prints, {dd['funding_first'][:10]} to {dd['funding_last'][:10]}; "
      "the engine's synthetic 0.01% per 8h before 2020 and for October 2026.\n")
    w("## Method\n")
    w("- Decision at every 15m close, the live desk's cycle. The context holds only bars closed by then on every timeframe "
      "(1m 100, 5m 100, 15m 60, 1h 72, 4h 100, the backtest caps); `Price` is the last 1m close and `Now` the decision time. "
      "CVD is cumulative Binance taker-buy minus taker-sell volume, compared cycle to cycle as the live desk does.")
    w("- Entry at the open of the next 1m bar. Exits walk the 1m bars: stop first when one minute reaches both levels, the "
      "bar's open when it opens beyond a level, a 7-day time stop, as in DefaultRunConfig.")
    w("- Kept from the engine: one position per strategy, at most 5 entries per UTC day, the strategy's first declared regime "
      "pinned, SL/TP distances as absolute values, 0.35%/0.70% when a signal carries none.")
    w(f"- Costs per leg: Delta {num(s['costs_per_leg_bp']['delta'], 1)} bp (primary), Binance {num(s['costs_per_leg_bp']['binance'], 1)} bp. "
      "Funding signed: a long pays positive funding.")
    w(f"- Checks: every one of the {ver['mechanics_trades']:,} trades of the 17 listed strategies re-walked on the raw bars "
      f"in numpy ({len(ver['mechanics_problems'])} mismatches); a re-run on data cut at {ver['truncation_cut']} reproduces all "
      f"{ver['truncation_trades']:,} trades that closed before the cut ({'identical' if ver['truncation_identical'] else 'NOT identical'}).\n")
    w("## Caveats\n")
    w("- Spot prices stand in for the perpetual; the basis and Delta's own funding are not modelled.")
    w("- Context the history cannot supply (order book, open interest, liquidations, DVOL, ETH) is empty, as in the engine. "
      "Strategies that need it only fire on their OHLCV branch.")
    w("- The regime is pinned; the live desk passes its classifier's label instead.")
    w("- 1x notional, no compounding. Leverage multiplies returns and drawdowns alike.")
    w("- The full-period list is chosen with hindsight from 331 candidates; read its numbers as the best case.\n")
    w("## Reproduce\n")
    w("```")
    w("# from the repository root")
    w("python -I engine/research/btc_deep_2026_10/deep_fetch.py              # 9 years of verified 1m data")
    w("cd engine && go build -o deep_backtest ./cmd/deep_backtest")
    w("./deep_backtest -mode fixed && ./deep_backtest -mode engine           # trades -> data/historical/deep/runs")
    w("./deep_backtest -mode probe > research/btc_deep_2026_10/results/probe.txt")
    w("cd research/btc_deep_2026_10 && python -I deep_analyze.py && python -I deep_verify.py ../../deep_backtest && python -I deep_report.py")
    w("```")
    with open(os.path.join(HERE, "REPORT.md"), "w", encoding="utf-8", newline="\n") as f:
        f.write("\n".join(L) + "\n")
    print("wrote REPORT.md and report/btc-deep-backtest.html", len(html) // 1024, "KB")


if __name__ == "__main__":
    sys.exit(main())
