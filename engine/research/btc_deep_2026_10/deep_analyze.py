"""Rank the deep-backtest strategies honestly and build their yearly/monthly tables.

Reads the per-trade output of cmd/deep_backtest (fixed clock, and the engine
harness for comparison) and writes results/summary.json, results/strategies.csv
and results/top10_trades.csv.

Accounting: every strategy has its own $10,000; every trade is $10,000 notional
(1x, no compounding), so a month's return is the sum of that month's net trade
returns. Leverage scales returns and drawdowns linearly.

Costs (per leg, on notional), applied to the simulator's GROSS returns:
  delta    Delta Exchange India taker 0.05% + 18% GST = 5.9 bp, plus 2.0 bp
           half-spread and slippage -> 7.9 bp a leg, 15.8 bp a round trip (PRIMARY:
           the venue the antigravity BTC desks trade)
  binance  the engine's own model: Binance taker 4 bp + 1.5 bp half-spread +
           1 bp slippage -> 6.5 bp a leg
Funding: Binance USDT-M 8h prints, signed (longs pay positive funding), the
engine's synthetic 0.01% where no print exists (before 2020, Oct 2026).

Two top-10 lists, one rule: rank by the t-statistic of net return per trade,
greedily skipping a candidate whose monthly returns correlate > 0.7 with one
already chosen (near-identical variants of one rule would otherwise fill the
list). Strategies the registry itself labels "DEMO ONLY" are not ranked.
  full   ranked on all of 2018-01-01 .. 2026-10-30, >= 80 trades: the list
         asked for, chosen with hindsight.
  train  ranked on 2018-01-01 .. 2022-12-31 only, >= 50 trades and a positive
         net, then held untouched through 2023-01-01 .. 2026-10-30: the test of
         whether ranking a backtest like this picks anything that keeps working.

Usage: python -I deep_analyze.py
"""
import json
import math
import os
import sys

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
ENGINE = os.path.normpath(os.path.join(HERE, "..", ".."))
RUNS = os.path.join(ENGINE, "data", "historical", "deep", "runs")
RESULTS = os.path.join(HERE, "results")
WHITELIST = os.path.join(ENGINE, "data", "btc_prelive_whitelist.json")
DATA_REPORT = os.path.join(ENGINE, "data", "historical", "deep", "data_report.json")

COSTS = {"delta": 0.00079, "binance": 0.00065}
PRIMARY = "delta"
CAPITAL = 10_000.0
TRAIN = (pd.Timestamp("2018-01-01", tz="UTC"), pd.Timestamp("2023-01-01", tz="UTC"))
TEST = (pd.Timestamp("2023-01-01", tz="UTC"), pd.Timestamp("2026-10-31", tz="UTC"))
FULL = (TRAIN[0], TEST[1])
MONTHS = pd.period_range("2018-01", "2026-10", freq="M")
YEARS = list(range(2018, 2027))
MIN_TRAIN_TRADES = 50
MIN_FULL_TRADES = 80
MAX_CORR = 0.7
TOP = 10


def norm_ppf(p):
    """Inverse standard normal CDF by bisection (scipy is not installed here)."""
    lo, hi = -10.0, 10.0
    for _ in range(100):
        mid = (lo + hi) / 2
        if 0.5 * math.erfc(-mid / math.sqrt(2)) < p:
            lo = mid
        else:
            hi = mid
    return (lo + hi) / 2


def load(mode):
    path = os.path.join(RUNS, f"trades_{mode}.csv")
    if not os.path.exists(path):
        return None
    t = pd.read_csv(path)
    for c in ("decision", "entry", "exit"):
        t[c] = pd.to_datetime(t[c], unit="s", utc=True)
    t["long"] = t["side"] == "LONG"
    if mode == "engine":  # the engine already charged its own costs; NetPnL is per trade
        notional = t["entry_px"] * t["size"]
        t["net_engine"] = np.where(notional > 0, t["net_usd"] / notional, 0.0)
    for name, leg in COSTS.items():
        t[f"net_{name}"] = t["gross"] - 2 * leg - t["funding"]
    t["exit_month"] = t["exit"].dt.tz_localize(None).dt.to_period("M")
    return t


def window(t, w):
    return t[(t["entry"] >= w[0]) & (t["entry"] < w[1])]


def monthly(t, col):
    """Monthly net, % of the strategy's capital, every month of the window filled."""
    s = (t.groupby("exit_month")[col].sum() * 100).reindex(MONTHS, fill_value=0.0)
    return s


def stats(t, col, w):
    x = window(t, w)
    r = x[col].to_numpy()
    n = len(r)
    months = monthly(x, col)
    last = (w[1] - pd.Timedelta(days=1)).tz_localize(None).to_period("M")
    months = months[(months.index >= w[0].tz_localize(None).to_period("M")) & (months.index <= last)]
    out = {"trades": n}
    if n == 0:
        return out | {"net_pct": 0.0}
    cum = np.cumsum(r[np.argsort(x["exit"].to_numpy(), kind="stable")]) * 100
    peak = np.maximum.accumulate(np.concatenate([[0.0], cum]))[1:]
    wins, losses = r[r > 0].sum(), -r[r < 0].sum()
    sd = r.std(ddof=1) if n > 1 else 0.0
    years = (w[1] - w[0]).days / 365.25
    out |= {
        "net_pct": round(float(r.sum() * 100), 2),
        "per_year_pct": round(float(r.sum() * 100 / years), 2),
        "win_rate_pct": round(float((r > 0).mean() * 100), 1),
        "profit_factor": round(float(wins / losses), 3) if losses > 0 else None,
        "avg_trade_bp": round(float(r.mean() * 1e4), 2),
        "t_stat": round(float(r.mean() / (sd / math.sqrt(n))), 2) if sd > 0 else None,
        "max_dd_pct": round(float((peak - cum).max()), 2),
        "monthly_sharpe": round(float(months.mean() / months.std(ddof=1) * math.sqrt(12)), 2)
        if months.std(ddof=1) > 0 else None,
        "positive_months_pct": round(float((months > 0).mean() * 100), 1),
        "trades_per_year": round(n / years, 1),
        "avg_hold_h": round(float((x["exit"] - x["entry"]).dt.total_seconds().mean() / 3600), 1),
        "long_share_pct": round(float(x["long"].mean() * 100), 1),
        "tp_exit_pct": round(float((x["reason"] == "TP").mean() * 100), 1),
    }
    return out


def yearly(t, col):
    y = t.groupby(t["exit"].dt.year)[col].sum() * 100
    return {int(k): round(float(v), 2) for k, v in y.reindex(YEARS, fill_value=0.0).items()}


def grid(t, col):
    m = monthly(t[(t["entry"] >= FULL[0]) & (t["entry"] < FULL[1])], col)
    return {str(p): round(float(v), 2) for p, v in m.items()}


def select(fixed, names, w, min_trades):
    col = f"net_{PRIMARY}"
    last = (w[1] - pd.Timedelta(days=1)).tz_localize(None).to_period("M")
    first = w[0].tz_localize(None).to_period("M")
    rows = []
    for n in names:
        st = stats(fixed[n], col, w)
        if st["trades"] >= min_trades and st["net_pct"] > 0 and st.get("t_stat"):
            rows.append((st["t_stat"], n))
    rows.sort(reverse=True)
    chosen, skipped = [], []
    mon = {n: monthly(window(fixed[n], w), col)[lambda x: (x.index >= first) & (x.index <= last)] for _, n in rows}
    for ts, n in rows:
        if len(chosen) == TOP:
            break
        clash = [(c, float(np.corrcoef(mon[n], mon[c])[0, 1])) for c in chosen]
        clash = [(c, r) for c, r in clash if r > MAX_CORR]
        if clash:
            skipped.append({"strategy": n, "t": ts, "too_close_to": clash[0][0], "corr": round(clash[0][1], 2)})
            continue
        chosen.append(n)
    return chosen, skipped, [{"strategy": n, "t": ts} for ts, n in rows]


def profile(rank, n, fixed, eng, per):
    col = f"net_{PRIMARY}"
    t = fixed[n]
    return {
        "rank": rank, "strategy": n, "description": per[n]["description"],
        "timeframes": per[n]["timeframes"], "pinned_regime": per[n]["pinned_regime"],
        "train": stats(t, col, TRAIN), "test": stats(t, col, TEST), "full": stats(t, col, FULL),
        "full_binance": stats(t, "net_binance", FULL), "test_binance": stats(t, "net_binance", TEST),
        "full_gross": stats(t, "gross", FULL),
        "yearly": yearly(window(t, FULL), col), "monthly": grid(t, col),
        "engine_full": ({"trades": len(window(eng[n], FULL)),
                         "net_pct": round(float(window(eng[n], FULL)["net_engine"].sum() * 100), 2)}
                        if n in eng else None),
        "run": {k: per[n][k] for k in ("evaluations", "signals", "entries", "entries_with_default_sl_tp",
                                       "entries_with_wrong_side_level", "signals_over_daily_cap")},
    }


def book_of(chosen, fixed):
    """Equal-weight book: $100,000 split $10,000 per strategy."""
    col = f"net_{PRIMARY}"
    pm = pd.concat([monthly(window(fixed[n], FULL), col) for n in chosen], axis=1).mean(axis=1)
    cum = pm.cumsum()
    test = pm[pm.index >= pd.Period("2023-01", "M")]
    return {
        "monthly": {str(p): round(float(v), 2) for p, v in pm.items()},
        "yearly": {y: round(float(pm[[p.year == y for p in pm.index]].sum()), 2) for y in YEARS},
        "train_pct": round(float(pm[pm.index < pd.Period("2023-01", "M")].sum()), 2),
        "test_pct": round(float(test.sum()), 2),
        "full_pct": round(float(pm.sum()), 2),
        "positive_months_pct": round(float((pm > 0).mean() * 100), 1),
        "monthly_sharpe": round(float(pm.mean() / pm.std(ddof=1) * math.sqrt(12)), 2),
        "test_monthly_sharpe": round(float(test.mean() / test.std(ddof=1) * math.sqrt(12)), 2),
        "max_dd_pct": round(float((np.maximum.accumulate(np.concatenate([[0], cum]))[1:] - cum).max()), 2),
        "trades": int(sum(len(window(fixed[n], FULL)) for n in chosen)),
    }


def main():
    os.makedirs(RESULTS, exist_ok=True)
    fx = load("fixed")
    en = load("engine")
    meta = json.load(open(os.path.join(RUNS, "run_fixed.json"), encoding="utf-8"))
    per = {s["strategy"]: s for s in meta["per_strategy"]}
    names = sorted(per)
    fixed = {n: g for n, g in fx.groupby("strategy")}
    empty = fx.iloc[0:0]
    for n in names:
        fixed.setdefault(n, empty)
    eng = {n: g for n, g in en.groupby("strategy")} if en is not None else {}

    # ── every strategy, every window, both cost models ──────────────────────
    table = []
    for n in names:
        row = {"strategy": n, "description": per[n]["description"], "regime": per[n]["pinned_regime"],
               "timeframes": "/".join(per[n]["timeframes"] or [])}
        for cname in COSTS:
            for wname, w in (("train", TRAIN), ("test", TEST), ("full", FULL)):
                s = stats(fixed[n], f"net_{cname}", w)
                for k in ("trades", "net_pct", "win_rate_pct", "profit_factor", "t_stat", "max_dd_pct"):
                    if cname == PRIMARY or k == "net_pct":
                        row[f"{cname}_{wname}_{k}"] = s.get(k)
        s = stats(fixed[n], "gross", FULL)
        row["gross_full_net_pct"] = s.get("net_pct")
        row["gross_full_avg_trade_bp"] = s.get("avg_trade_bp")
        if n in eng:
            row["engine_full_trades"] = len(window(eng[n], FULL))
            row["engine_full_net_pct"] = round(float(window(eng[n], FULL)["net_engine"].sum() * 100), 2)
        table.append(row)
    tab = pd.DataFrame(table)
    tab.to_csv(os.path.join(RESULTS, "strategies.csv"), index=False, encoding="utf-8")

    demo = [n for n in names if "DEMO ONLY" in (per[n]["description"] or "")]
    rankable = [n for n in names if n not in demo]
    lists = {}
    for key, w, mt in (("full", FULL, MIN_FULL_TRADES), ("train", TRAIN, MIN_TRAIN_TRADES)):
        chosen, skipped, ranked = select(fixed, rankable, w, mt)
        lists[key] = {"chosen": chosen, "skipped_near_duplicates": skipped, "ranking_head": ranked[:40],
                      "eligible": len(ranked),
                      "top10": [profile(i, n, fixed, eng, per) for i, n in enumerate(chosen, 1)],
                      "book": book_of(chosen, fixed)}
    demo_rows = [profile(0, n, fixed, eng, per) for n in demo]
    col = f"net_{PRIMARY}"

    # ── the universe: how much of it clears costs at all ──────────────────────
    def count(c, w, cond):
        return int(sum(1 for n in names if cond(stats(fixed[n], c, w))))
    universe = {
        "strategies": len(names), "trades_fixed": int(len(fx)),
        "traded_at_all": int(sum(1 for n in names if len(fixed[n]) > 0)),
        "gross_positive_full": count("gross", FULL, lambda s: s["net_pct"] > 0),
        "net_positive_full": {c: count(f"net_{c}", FULL, lambda s: s["net_pct"] > 0) for c in COSTS},
        "net_positive_train": count(col, TRAIN, lambda s: s["net_pct"] > 0),
        "net_positive_test": count(col, TEST, lambda s: s["net_pct"] > 0),
        "net_positive_train_and_test": int(sum(1 for n in names if stats(fixed[n], col, TRAIN)["net_pct"] > 0
                                               and stats(fixed[n], col, TEST)["net_pct"] > 0)),
        "train_t_over_2": count(col, TRAIN, lambda s: (s.get("t_stat") or 0) > 2 and s["trades"] >= MIN_TRAIN_TRADES),
        "full_t_over_2": count(col, FULL, lambda s: (s.get("t_stat") or 0) > 2 and s["trades"] >= MIN_FULL_TRADES),
        "full_best_t": max((stats(fixed[n], col, FULL).get("t_stat") or 0) for n in rankable
                           if len(window(fixed[n], FULL)) >= MIN_FULL_TRADES),
        "bonferroni_t_one_sided_5pct": round(norm_ppf(1 - 0.05 / len(names)), 2),
        "expected_max_t_under_null": round(math.sqrt(2 * math.log(len(names))), 2),
        "mean_gross_trade_bp": round(float(fx["gross"].mean() * 1e4), 2),
        "mean_net_trade_bp": round(float(fx[col].mean() * 1e4), 2),
        "full_t_over_bonferroni": [n for n in names if len(window(fixed[n], FULL)) >= MIN_FULL_TRADES
                                   and (stats(fixed[n], col, FULL).get("t_stat") or 0) > norm_ppf(1 - 0.05 / len(names))],
        "demo_only_not_ranked": demo,
    }
    if en is not None:
        both = tab.dropna(subset=["engine_full_net_pct"])
        universe["engine"] = {
            "trades": int(len(en)),
            "net_positive_full_engine_harness": int((both["engine_full_net_pct"] > 0).sum()),
            "net_positive_full_fixed_binance_cost": int((both["binance_full_net_pct"] > 0).sum()),
            "rank_correlation_engine_vs_fixed": round(float(both["engine_full_net_pct"].rank().corr(
                both["binance_full_net_pct"].rank())), 2),
            "best_engine": both.nlargest(10, "engine_full_net_pct")[
                ["strategy", "engine_full_trades", "engine_full_net_pct", "binance_full_net_pct"]].to_dict("records"),
        }

    # ── the live whitelist under the corrected clock ─────────────────────────
    wl = json.load(open(WHITELIST, encoding="utf-8"))
    wnames = [n for n in wl["whitelist"] if n in fixed]
    vw = (pd.Timestamp(wl["validate_window"][0], tz="UTC"), pd.Timestamp(wl["validate_window"][1], tz="UTC"))
    wrows = []
    for n in wnames:
        s_full, s_val = stats(fixed[n], col, FULL), stats(fixed[n], col, vw)
        wrows.append({"strategy": n, "full_trades": s_full["trades"], "full_net_pct": s_full["net_pct"],
                      "full_pf": s_full.get("profit_factor"), "validate_trades": s_val["trades"],
                      "validate_net_pct": s_val["net_pct"], "validate_win_rate": s_val.get("win_rate_pct"),
                      "yearly": yearly(window(fixed[n], FULL), col),
                      "engine_full_net_pct": (round(float(window(eng[n], FULL)["net_engine"].sum() * 100), 2)
                                              if n in eng else None)})
    whitelist = {
        "names": len(wl["whitelist"]), "found": len(wnames), "validate_window": wl["validate_window"],
        "net_positive_full": sum(1 for r in wrows if r["full_net_pct"] > 0),
        "net_positive_validate_window": sum(1 for r in wrows if r["validate_net_pct"] > 0),
        "in_top10_full": [n for n in wnames if n in lists["full"]["chosen"]],
        "in_top10_train": [n for n in wnames if n in lists["train"]["chosen"]], "rows": wrows,
    }

    keep = sorted(set(lists["full"]["chosen"]) | set(lists["train"]["chosen"]))
    pd.concat([window(fixed[n], FULL) for n in keep]).drop(columns=["exit_month"]).to_csv(
        os.path.join(RESULTS, "top10_trades.csv"), index=False, encoding="utf-8")

    summary = {
        "method": __doc__.split("Usage:")[0].strip(),
        "windows": {"train": [str(TRAIN[0].date()), "2022-12-31"], "test": [str(TEST[0].date()), "2026-10-30"]},
        "costs_per_leg_bp": {k: round(v * 1e4, 2) for k, v in COSTS.items()}, "primary_cost": PRIMARY,
        "data": json.load(open(DATA_REPORT, encoding="utf-8")), "universe": universe, "lists": lists,
        "demo_only": demo_rows, "whitelist": whitelist,
    }
    with open(os.path.join(RESULTS, "summary.json"), "w", encoding="utf-8") as f:
        json.dump(summary, f, indent=1, default=str)

    print(json.dumps(universe, indent=1, default=str))
    for key in ("full", "train"):
        print(f"\nTOP 10 ranked on {key}:")
        for r in lists[key]["top10"]:
            a, b, c = r["train"], r["test"], r["full"]
            print(f"{r['rank']:2d} {r['strategy']:<36} train n={a['trades']:4d} {a['net_pct']:7.1f}% t={a.get('t_stat')} | "
                  f"test n={b['trades']:4d} {b['net_pct']:7.1f}% t={b.get('t_stat')} | full {c['net_pct']:7.1f}% "
                  f"t={c.get('t_stat')} PF {c.get('profit_factor')} DD {c.get('max_dd_pct')} | engine {r['engine_full']}")
        print("book:", {k: v for k, v in lists[key]["book"].items() if k != "monthly"})
        print("skipped:", [(x["strategy"], x["too_close_to"]) for x in lists[key]["skipped_near_duplicates"]])
    print("whitelist:", {k: v for k, v in whitelist.items() if k != "rows"})


if __name__ == "__main__":
    sys.exit(main())
