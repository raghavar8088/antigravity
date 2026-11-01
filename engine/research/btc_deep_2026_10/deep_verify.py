"""Independent checks of the fixed-clock simulator's trades.

1. Mechanics, trade by trade (every trade of every top-10 strategy, re-walked
   on the raw 1m bars in numpy, not in Go):
     - the entry fills at the open of the 1m bar starting at the decision time,
       and the decision time is a 15m close;
     - the exit is the FIRST minute after entry whose range reaches the stop or
       target (stop first when one minute reaches both), at that level, or at
       the open when the minute opens beyond it; or the 7-day time stop;
     - one position at a time, at most 5 entries per UTC day.
2. No look-ahead, by truncation: cmd/deep_backtest is re-run on a copy of the
   data that ENDS on 2022-06-30. Every trade that closed before the cut must be
   identical in both runs: a decision that used any later bar would change.

Usage: python -I deep_verify.py <path to deep_backtest binary>
"""
import json
import os
import shutil
import subprocess
import sys

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
ENGINE = os.path.normpath(os.path.join(HERE, "..", ".."))
DEEP = os.path.join(ENGINE, "data", "historical", "deep")
CUT = pd.Timestamp("2022-07-01", tz="UTC")


def load_1m():
    df = pd.read_csv(os.path.join(DEEP, "BTCUSDT_1m.csv"), usecols=[0, 1, 2, 3, 4])
    return df["time"].to_numpy(np.int64), df[["open", "high", "low", "close"]].to_numpy()


def check_mechanics(trades, t, ohlc):
    problems = []
    for strat, g in trades.groupby("strategy"):
        g = g.sort_values("entry")
        ent, ext = g["entry"].to_numpy(np.int64), g["exit"].to_numpy(np.int64)
        if (ent[1:] < ext[:-1]).any():
            problems.append(f"{strat}: overlapping positions")
        days = pd.Series(ent // 86400).value_counts()
        if (days > 5).any():
            problems.append(f"{strat}: more than 5 entries in a UTC day")
        for r, e, x in zip(g.itertuples(), ent, ext):
            if e % 900:
                problems.append(f"{strat} {e}: entry not on a 15m boundary")
                continue
            i = np.searchsorted(t, e)
            if i >= len(t) or t[i] != e or abs(ohlc[i, 0] - r.entry_px) > 1e-6 * r.entry_px:
                problems.append(f"{strat} {e}: entry price is not the open of the bar at T")
                continue
            long = r.side == "LONG"
            sl = r.entry_px * (1 - r.sl_pct / 100) if long else r.entry_px * (1 + r.sl_pct / 100)
            tp = r.entry_px * (1 + r.tp_pct / 100) if long else r.entry_px * (1 - r.tp_pct / 100)
            j = i
            want = None
            while j < len(t):
                bt = t[j]
                o, h, lo, _ = ohlc[j]
                if bt >= e + 7 * 86400:
                    want = (bt, o, "TIME")
                    break
                if bt != e:
                    if (long and o <= sl) or (not long and o >= sl):
                        want = (bt, o, "SL")
                        break
                    if (long and o >= tp) or (not long and o <= tp):
                        want = (bt, o, "TP")
                        break
                if (long and lo <= sl) or (not long and h >= sl):
                    want = (bt + 60, sl, "SL")
                    break
                if (long and h >= tp) or (not long and lo <= tp):
                    want = (bt + 60, tp, "TP")
                    break
                j += 1
            if want is None:
                want = (t[-1] + 60, ohlc[-1, 3], "END")
            if want[0] != x or want[2] != r.reason or abs(want[1] - r.exit_px) > 1e-6 * r.entry_px:
                problems.append(f"{strat} {e}: exit {x} {r.reason} {r.exit_px} but re-walk gives {want}")
    return problems


def run(binary, data_dir, out_dir, names, to):
    subprocess.run([binary, "-mode", "fixed", "-data", data_dir, "-out", out_dir, "-from", "2018-01-01",
                    "-to", to, "-only", ",".join(names), "-workers", "8"], check=True, cwd=ENGINE,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    return pd.read_csv(os.path.join(out_dir, "trades_fixed.csv"))


def main():
    binary = sys.argv[1]
    summary = json.load(open(os.path.join(HERE, "results", "summary.json"), encoding="utf-8"))
    names = sorted(set(summary["lists"]["full"]["chosen"]) | set(summary["lists"]["train"]["chosen"]))
    trades = pd.read_csv(os.path.join(HERE, "results", "top10_trades.csv"))
    for c in ("entry", "exit"):
        trades[c] = (pd.to_datetime(trades[c], utc=True) - pd.Timestamp(0, tz="UTC")) // pd.Timedelta(seconds=1)
    t, ohlc = load_1m()
    problems = check_mechanics(trades, t, ohlc)
    print(f"mechanics: {len(trades)} trades of {trades.strategy.nunique()} strategies re-walked; "
          f"{len(problems)} problems")
    for p in problems[:20]:
        print("  ", p)

    # truncation: same strategies on data that ends at the cut
    scratch = os.path.join(DEEP, "verify_cut")
    os.makedirs(scratch, exist_ok=True)
    cut_s = int(CUT.timestamp())
    with open(os.path.join(DEEP, "BTCUSDT_1m.csv")) as src, open(os.path.join(scratch, "BTCUSDT_1m.csv"), "w") as dst:
        dst.write(src.readline())
        for line in src:
            if int(line.split(",", 1)[0]) >= cut_s:
                break
            dst.write(line)
    shutil.copy(os.path.join(DEEP, "BTCUSDT_funding.csv"), os.path.join(scratch, "BTCUSDT_funding.csv"))
    full = run(binary, DEEP, os.path.join(scratch, "run_full"), names, "2026-10-31")
    short = run(binary, scratch, os.path.join(scratch, "run_cut"), names, "2022-07-01")
    key = ["strategy", "side", "entry", "exit", "entry_px", "exit_px", "reason", "sl_pct", "tp_pct"]
    a = full[full["exit"] < cut_s - 60][key].reset_index(drop=True)
    b = short[short["exit"] < cut_s - 60][key].reset_index(drop=True)
    same = len(a) == len(b) and a.sort_values(key[:3]).reset_index(drop=True).equals(
        b.sort_values(key[:3]).reset_index(drop=True))
    print(f"truncation: {len(a)} trades closed before {CUT.date()} with all data, {len(b)} with data cut there; "
          f"identical: {same}")
    shutil.rmtree(scratch)
    out = {"mechanics_trades": int(len(trades)), "mechanics_problems": problems[:50],
           "truncation_cut": str(CUT.date()), "truncation_trades": int(len(a)), "truncation_identical": bool(same)}
    with open(os.path.join(HERE, "results", "verify.json"), "w", encoding="utf-8") as f:
        json.dump(out, f, indent=1)
    return 0 if (not problems and same) else 1


if __name__ == "__main__":
    sys.exit(main())
