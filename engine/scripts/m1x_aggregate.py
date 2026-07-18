"""Aggregate M1X qualification results across symbols into one honest matrix.

Reads engine/data/m1x_<SYMBOL>_<mode>.json (plus the original BTC files
m1x_scalp_<mode>.json) and prints:
  - every strict passer (strategy x symbol x mode) under the M1X bar
  - near-misses (train PF >= 1.0) so genuine almost-edges are visible
  - a per-symbol summary: candidates, train-promising, passers, best PF
"""
import glob
import json
import os
import sys

DATA = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "data"))


def load_all():
    docs = []
    for path in sorted(glob.glob(os.path.join(DATA, "m1x_*.json"))):
        base = os.path.basename(path)
        if base in ("m1x_smoke.json",):
            continue
        try:
            d = json.load(open(path))
        except Exception as e:
            print(f"skip {base}: {e}")
            continue
        if "rows" not in d:
            continue
        if base.startswith("m1x_scalp_"):
            sym = "BTCUSDT"
        else:
            sym = base.replace("m1x_", "").rsplit("_", 1)[0]
        docs.append((sym, d.get("mode", "?"), d))
    return docs


def main():
    docs = load_all()
    if not docs:
        print("no result files found")
        return

    passers, nearmiss = [], []
    print(f"{'symbol':10s} {'mode':6s} {'cand':>4s} {'promo':>5s} {'pass':>4s} {'best train PF (N>=100)':>24s}")
    for sym, mode, d in docs:
        rows = d["rows"]
        promo = [r for r in rows if r.get("train_promising")]
        strict = [r for r in rows if r.get("strict_oos_pass")]
        real = [r for r in rows if r["train"]["trades"] >= 100]
        best = max(real, key=lambda r: r["train"]["profit_factor"], default=None)
        btxt = f"{best['strategy']} {best['train']['profit_factor']:.2f}" if best else "-"
        print(f"{sym:10s} {mode:6s} {len(rows):4d} {len(promo):5d} {len(strict):4d} {btxt:>34s}")
        for r in strict:
            passers.append((sym, mode, r))
        for r in rows:
            t = r["train"]
            if t["trades"] >= 100 and t["profit_factor"] >= 1.0 and not r.get("strict_oos_pass"):
                nearmiss.append((sym, mode, r))

    print("\n=== STRICT PASSERS (M1X bar: OOS N>=200, PF>=1.2, Sh>=0.5, DD<=25%, both halves PF>=1.0) ===")
    if not passers:
        print("NONE")
    for sym, mode, r in passers:
        v = r["validate"]
        print(f"PASS {sym} {mode} {r['strategy']} ({r.get('exit_profile')}): "
              f"N={v['trades']} WR={v['win_rate_pct']}% PF={v['profit_factor']} Sh={v['sharpe']} "
              f"ret={v['return_pct']}% DD={v['max_dd_pct']}% H1={v['h1_pf']} H2={v['h2_pf']} "
              f"missed={v.get('missed_fills', 0)} deskbar={'PASS' if r.get('desk_bar_pass') else 'no'}")

    print("\n=== NEAR-MISSES (train PF >= 1.0 at N>=100, did not pass) ===")
    if not nearmiss:
        print("NONE")
    for sym, mode, r in sorted(nearmiss, key=lambda x: -x[2]["train"]["profit_factor"]):
        t, v = r["train"], r["validate"]
        print(f"     {sym} {mode} {r['strategy']} ({r.get('exit_profile')}): "
              f"trainN={t['trades']} trainPF={t['profit_factor']} trainWR={t['win_rate_pct']}% | "
              f"oosN={v['trades']} oosPF={v['profit_factor']} oosSh={v['sharpe']} H1={v['h1_pf']} H2={v['h2_pf']}")


if __name__ == "__main__":
    main()
