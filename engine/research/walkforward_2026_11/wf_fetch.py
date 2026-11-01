"""Daily candles and funding for the walk-forward lab: the 16 coins of the October search.

July 2024 - November 2025 (indicator warm-up only) from Binance Vision USDT-M monthly 1d klines (SHA-256 checked).
December 2025 - October 2026 from the 1m files of the year test (engine/data/historical/year2026), so the months
that are traded use the same prices as every earlier test. December 2025 is fetched both ways as a cross-check.
Funding: the year-test prints (December 2025 - October 2026). Output (git-ignored): engine/data/historical/walkforward/.

Usage: python -I wf_fetch.py
"""
import os
import sys
from concurrent.futures import ThreadPoolExecutor

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.normpath(os.path.join(HERE, "..", "october_search_2026_11")))
import os_fetch_extra as FX  # noqa: E402

HIST = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical"))
OUT = os.path.join(HIST, "walkforward")
FX.RAW = os.path.join(OUT, "raw")
COINS = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"] + FX.EXTRA
WARM = [f"{y}-{m:02d}" for y in (2024, 2025) for m in range(1, 13) if "2024-07" <= f"{y}-{m:02d}" <= "2025-12"]
T_DEC = int(pd.Timestamp("2025-12-01", tz="UTC").timestamp())


def from_minutes(sym):
    z = np.load(os.path.join(HIST, "year2026", f"{sym}.npz"))
    k = 1440
    n = len(z["t"]) // k * k
    return pd.DataFrame({"day": z["t"][:n:k], "o": z["o"][:n:k], "h": z["h"][:n].reshape(-1, k).max(1), "l": z["l"][:n].reshape(-1, k).min(1),
                         "c": z["c"][k - 1:n:k], "v": z["v"][:n].reshape(-1, k).sum(1)})


def one(sym):
    with ThreadPoolExecutor(max_workers=6) as ex:
        raws = list(ex.map(FX.fetch, [f"{FX.V}/futures/um/monthly/klines/{sym}/1d/{sym}-1d-{m}.zip" for m in WARM]))
    missing = [m for m, r in zip(WARM, raws) if r is None]
    k = pd.concat([FX.bars(r) for r in raws if r is not None]).rename(columns={"t": "day"})
    k["day"] = k["day"] // 86400 * 86400
    k = k.drop_duplicates("day").sort_values("day")
    mins = from_minutes(sym)
    dec_k, dec_m = k[k["day"] >= T_DEC].set_index("day"), mins[mins["day"] < T_DEC + 31 * 86400].set_index("day")
    common = dec_k.index.intersection(dec_m.index)
    worst = float((dec_k.loc[common, ["o", "h", "l", "c"]] / dec_m.loc[common, ["o", "h", "l", "c"]] - 1).abs().max().max()) if len(common) else None
    d = pd.concat([k[k["day"] < T_DEC], mins]).sort_values("day")
    d["sym"] = sym
    gaps = int((np.diff(d["day"].to_numpy()) != 86400).sum())
    print(f"{sym}: {len(d)} days {pd.Timestamp(int(d['day'].iat[0]), unit='s').date()} -> {pd.Timestamp(int(d['day'].iat[-1]), unit='s').date()}, "
          f"gaps {gaps}, months not published {missing}, Dec 2025 1d vs 1m worst difference {worst}", flush=True)
    f = np.load(os.path.join(HIST, "year2026", f"{sym}_funding.npz"))
    return d, pd.DataFrame({"t": f["t"], "rate": f["rate"], "sym": sym}), gaps == 0 and worst is not None and worst < 1e-9


def main():
    os.makedirs(FX.RAW, exist_ok=True)
    days, fund, ok = [], [], []
    for s in COINS:
        d, f, good = one(s)
        days.append(d)
        fund.append(f)
        ok.append(good)
    pd.concat(days, ignore_index=True).to_pickle(os.path.join(OUT, "daily.pkl"))
    pd.concat(fund, ignore_index=True).to_pickle(os.path.join(OUT, "funding.pkl"))
    print("all consistent:", all(ok))
    return 0 if all(ok) else 1


if __name__ == "__main__":
    sys.exit(main())
