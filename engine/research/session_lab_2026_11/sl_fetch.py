"""Data for the session-strategy lab: Binance USDT-M perpetual 1m candles and
funding for BTC ETH SOL XRP DOGE BNB, January 2021 to October 2026.

2021-01 .. 2025-09 comes from Binance Vision monthly archives (SHA-256 checked);
2025-10 .. 2026-10 is reused from the earlier study's verified cache
(engine/data/historical/intraday20), including its spot-rebuilt 2026-10-28.

Output (git-ignored) in engine/data/historical/session_lab/:
  <SYM>.npz   t (unix s), o h l c v (float64) on a complete UTC minute grid, and
              `bad_day` = day starts that must not be traded (rebuilt from spot,
              or more than 30 minutes missing in the exchange's own data)
  <SYM>_funding.npz, data_report.json

Usage: python -I sl_fetch.py
"""
import hashlib
import io
import json
import os
import sys
import time
import urllib.error
import urllib.request
import zipfile
from concurrent.futures import ThreadPoolExecutor

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
HIST = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical"))
OUT = os.path.join(HIST, "session_lab")
RAW = os.path.join(OUT, "raw")
PREV = os.path.join(HIST, "intraday20")
V = "https://data.binance.vision/data/futures/um/monthly"
SYMBOLS = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"]
MONTHS = [f"{y}-{m:02d}" for y in range(2021, 2026) for m in range(1, 13) if (y, m) <= (2025, 9)]


def get(url, timeout=180):
    with urllib.request.urlopen(url, timeout=timeout) as r:
        return r.read()


def fetch(url):
    path = os.path.join(RAW, url.split("/monthly/", 1)[1].replace("/", "__"))
    last = None
    for attempt in range(5):
        try:
            want = get(url + ".CHECKSUM", 60).decode().split()[0].lower()
            if os.path.exists(path):
                raw = open(path, "rb").read()
                if hashlib.sha256(raw).hexdigest() == want:
                    return raw
            raw = get(url)
            if hashlib.sha256(raw).hexdigest() != want:
                raise ValueError("checksum mismatch")
            open(path, "wb").write(raw)
            return raw
        except urllib.error.HTTPError as e:
            if e.code == 404:
                return None
            last = e
        except Exception as e:
            last = e
        time.sleep(3 * (attempt + 1))
    raise RuntimeError(f"{url}: {last}")


def read_csv(raw):
    with zipfile.ZipFile(io.BytesIO(raw)) as z:
        body = z.read(z.namelist()[0]).decode()
    first = body.split("\n", 1)[0].split(",")[0].strip()
    return pd.read_csv(io.StringIO(body), header=None if first.isdigit() else 0)


def secs(x):
    x = np.asarray(x, dtype=np.int64)
    return np.where(x > 10**14, x // 1_000_000, np.where(x > 10**11, x // 1000, x))


def one(sym):
    urls = [f"{V}/klines/{sym}/1m/{sym}-1m-{m}.zip" for m in MONTHS]
    with ThreadPoolExecutor(max_workers=6) as ex:
        raws = list(ex.map(fetch, urls))
    missing = [u.rsplit("/", 1)[1] for u, r in zip(urls, raws) if r is None]
    frames = []
    for r in raws:
        if r is None:
            continue
        df = read_csv(r)
        c = df.columns
        frames.append(pd.DataFrame({"t": secs(df[c[0]].to_numpy()) // 60 * 60, "o": df[c[1]].astype(float),
                                    "h": df[c[2]].astype(float), "l": df[c[3]].astype(float),
                                    "c": df[c[4]].astype(float), "v": df[c[5]].astype(float)}))
    old = pd.concat(frames).sort_values("t").drop_duplicates("t")
    prev = pd.read_pickle(os.path.join(PREV, f"{sym}.pkl"))
    rebuilt_days = set((prev.index[prev["rebuilt"]].to_numpy() // 86400 * 86400).tolist())
    prev = prev.reset_index()[["t", "o", "h", "l", "c", "v"]]
    k = pd.concat([old[old["t"] < prev["t"].iloc[0]], prev]).sort_values("t").drop_duplicates("t")
    grid = np.arange(int(pd.Timestamp("2021-01-01", tz="UTC").timestamp()), int(k["t"].iloc[-1]) + 60, 60, dtype=np.int64)
    df = k.set_index("t").reindex(grid)
    present = df["c"].notna().to_numpy()
    days = grid // 86400 * 86400
    miss_per_day = pd.Series(~present).groupby(days).sum()
    bad = set(miss_per_day[miss_per_day > 30].index.tolist()) | rebuilt_days
    df["c"] = df["c"].ffill().bfill()
    for col in ("o", "h", "l"):
        df[col] = df[col].fillna(df["c"])
    df["v"] = df["v"].fillna(0.0)
    np.savez(os.path.join(OUT, f"{sym}.npz"), t=grid, o=df["o"].to_numpy(), h=df["h"].to_numpy(),
             l=df["l"].to_numpy(), c=df["c"].to_numpy(), v=df["v"].to_numpy(), bad_day=np.array(sorted(bad), dtype=np.int64))
    fr = []
    for m in MONTHS:
        r = fetch(f"{V}/fundingRate/{sym}/{sym}-fundingRate-{m}.zip")
        if r is not None:
            d = read_csv(r)
            fr.append(pd.DataFrame({"t": secs(d[d.columns[0]].to_numpy()) // 60 * 60, "rate": d[d.columns[-1]].astype(float)}))
    f_old = pd.concat(fr)
    f_new = pd.read_pickle(os.path.join(PREV, f"{sym}_funding.pkl"))
    f = pd.concat([f_old[f_old["t"] < f_new["t"].iloc[0]], f_new]).sort_values("t").drop_duplicates("t")
    np.savez(os.path.join(OUT, f"{sym}_funding.npz"), t=f["t"].to_numpy(np.int64), rate=f["rate"].to_numpy(float))
    iso = lambda s: pd.Timestamp(int(s), unit="s").strftime("%Y-%m-%d")
    rep = {"first": iso(grid[0]), "last": iso(grid[-1]), "minutes": int(len(grid)),
           "missing_minutes": int((~present).sum()), "missing_archives": missing,
           "bad_days": [iso(x) for x in sorted(bad)], "funding_prints": int(len(f))}
    print(sym, {k_: (v_ if k_ != "bad_days" else len(v_)) for k_, v_ in rep.items()}, flush=True)
    return sym, rep


def main():
    os.makedirs(RAW, exist_ok=True)
    t0 = time.time()
    report = dict(one(s) for s in SYMBOLS)
    json.dump(report, open(os.path.join(OUT, "data_report.json"), "w"), indent=1)
    print(f"done in {time.time() - t0:.0f}s", flush=True)


if __name__ == "__main__":
    sys.exit(main())
