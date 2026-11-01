"""1m perpetual data for all 16 coins, December 2025 - October 2026, for the year test of the
October search's top strategies (December 2025 is only indicator warm-up).

The 6 main coins are sliced from the session-lab files (Binance Vision, 2021-2026). The 10 extra coins
get December 2025 - July 2026 from Binance Vision monthly USDT-M klines and funding (SHA-256 checked,
same parser and gap rules as os_fetch_extra.py) joined to the August-October files the October search
used. Output (git-ignored): engine/data/historical/year2026/.

Usage: python -I os_year_fetch.py
"""
import io
import os
import sys
import zipfile
from concurrent.futures import ThreadPoolExecutor

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import os_fetch_extra as FX  # noqa: E402

HIST = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical"))
OUT = os.path.join(HIST, "year2026")
FX.RAW = os.path.join(OUT, "raw")
MAIN = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"]
MONTHS = ["2025-12"] + [f"2026-{m:02d}" for m in range(1, 8)]
T0 = int(pd.Timestamp("2025-12-01", tz="UTC").timestamp())
T_AUG = int(pd.Timestamp("2026-08-01", tz="UTC").timestamp())
T1 = int(pd.Timestamp("2026-11-01", tz="UTC").timestamp())
COLS = ("o", "h", "l", "c", "v")


def save(sym, t, arr, bad, ft, fr):
    np.savez(os.path.join(OUT, f"{sym}.npz"), t=t, **arr, bad_day=np.array(sorted(bad), dtype=np.int64))
    np.savez(os.path.join(OUT, f"{sym}_funding.npz"), t=ft, rate=fr)


def main_coin(sym):
    z, f = np.load(os.path.join(HIST, "session_lab", f"{sym}.npz")), np.load(os.path.join(HIST, "session_lab", f"{sym}_funding.npz"))
    k = (z["t"] >= T0) & (z["t"] < T1)
    t = z["t"][k]
    arr = {c: z[c][k] for c in COLS}
    # the October search read these coins from october_extra; check they are the same minutes and prices
    o = np.load(os.path.join(HIST, "october_extra", f"{sym}.npz"))
    a = t >= T_AUG
    same = np.array_equal(t[a], o["t"]) and all(np.allclose(arr[c][a], o[c], rtol=0, atol=0) for c in COLS)
    bad = {int(x) for x in z["bad_day"] if T0 <= x < T1}
    fk = (f["t"] >= T0) & (f["t"] < T1)
    save(sym, t, arr, bad, f["t"][fk], f["rate"][fk])
    print(sym, "sliced", len(t), "minutes; identical to october_extra Aug-Oct:", same, "bad days",
          [pd.Timestamp(x, unit="s").date().isoformat() for x in sorted(bad)], flush=True)
    return same


def extra_coin(sym):
    urls = [f"{FX.V}/futures/um/monthly/klines/{sym}/1m/{sym}-1m-{m}.zip" for m in MONTHS]
    with ThreadPoolExecutor(max_workers=4) as ex:
        raws = list(ex.map(FX.fetch, urls))
    missing = [m for m, r in zip(MONTHS, raws) if r is None]
    k = pd.concat([FX.bars(r) for r in raws if r is not None]).drop_duplicates("t").sort_values("t")
    grid = np.arange(T0, T_AUG, 60, dtype=np.int64)
    df = k.set_index("t").reindex(grid)
    present = df["c"].notna().to_numpy()
    miss_day = pd.Series(~present).groupby(grid // 86400 * 86400).sum()
    bad = {int(x) for x in miss_day[miss_day > 30].index}
    df["c"] = df["c"].ffill().bfill()
    for col in ("o", "h", "l"):
        df[col] = df[col].fillna(df["c"])
    df["v"] = df["v"].fillna(0.0)
    o = np.load(os.path.join(HIST, "october_extra", f"{sym}.npz"))
    assert int(o["t"][0]) == T_AUG and int(o["t"][-1]) == T1 - 60
    t = np.concatenate([grid, o["t"]])
    arr = {c: np.concatenate([df[c].to_numpy(float), o[c]]) for c in COLS}
    bad |= {int(x) for x in o["bad_day"]}
    fr = []
    for m in MONTHS:
        r = FX.fetch(f"{FX.V}/futures/um/monthly/fundingRate/{sym}/{sym}-fundingRate-{m}.zip")
        if r is not None:
            with zipfile.ZipFile(io.BytesIO(r)) as zz:
                d = pd.read_csv(io.StringIO(zz.read(zz.namelist()[0]).decode()))
            tt = np.asarray(d[d.columns[0]], dtype=np.int64)
            fr.append(pd.DataFrame({"t": np.where(tt > 10**11, tt // 1000, tt) // 60 * 60, "rate": d[d.columns[-1]].astype(float)}))
    fo = np.load(os.path.join(HIST, "october_extra", f"{sym}_funding.npz"))
    fr.append(pd.DataFrame({"t": fo["t"], "rate": fo["rate"]}))
    f = pd.concat(fr).drop_duplicates("t").sort_values("t")
    f = f[(f["t"] >= T0) & (f["t"] < T1)]
    save(sym, t, arr, bad, f["t"].to_numpy(np.int64), f["rate"].to_numpy(float))
    print(sym, "minutes", len(t), "missing (Dec-Jul)", int((~present).sum()), "months not published", missing, "bad days",
          [pd.Timestamp(x, unit="s").date().isoformat() for x in sorted(bad)], "funding prints", len(f), flush=True)
    return not missing


def main():
    os.makedirs(FX.RAW, exist_ok=True)
    ok = [main_coin(s) for s in MAIN] + [extra_coin(s) for s in FX.EXTRA]
    print("all complete:", all(ok))
    return 0 if all(ok) else 1


if __name__ == "__main__":
    sys.exit(main())
