"""Daily data for the swing-strategy lab.

Binance Vision spot 1d candles (the longest consistent price history) for 12
coins from their listing (BTC and ETH: Aug 2017) to 31 Oct 2026, and USDT-M
perpetual funding prints from each perpetual's listing (BTC: Sep 2019).
Every archive is SHA-256 checked. Output (git-ignored):
engine/data/historical/swing_lab/daily.pkl and funding.pkl.

Usage: python -I sw_fetch.py
"""
import hashlib
import io
import os
import sys
import time
import urllib.error
import urllib.request
import zipfile
from concurrent.futures import ThreadPoolExecutor
from datetime import date, timedelta

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "swing_lab"))
RAW = os.path.join(OUT, "raw")
V = "https://data.binance.vision/data"
COINS = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT",
         "ADAUSDT", "LTCUSDT", "LINKUSDT", "TRXUSDT", "XLMUSDT", "EOSUSDT"]
MONTHS = [f"{y}-{m:02d}" for y in range(2017, 2027) for m in range(1, 13) if (2017, 8) <= (y, m) <= (2026, 10)]


def get(url, timeout=90):
    with urllib.request.urlopen(url, timeout=timeout) as r:
        return r.read()


def fetch(url):
    path = os.path.join(RAW, url.split("/data/", 1)[1].replace("/", "__"))
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
        time.sleep(2 * (attempt + 1))
    raise RuntimeError(f"{url}: {last}")


def read_csv(raw):
    with zipfile.ZipFile(io.BytesIO(raw)) as z:
        body = z.read(z.namelist()[0]).decode()
    first = body.split("\n", 1)[0].split(",")[0].strip()
    return pd.read_csv(io.StringIO(body), header=None if first.isdigit() else 0)


def secs(x):
    x = np.asarray(x, dtype=np.int64)
    return np.where(x > 10**14, x // 1_000_000, np.where(x > 10**11, x // 1000, x))


def klines(sym):
    urls = [f"{V}/spot/monthly/klines/{sym}/1d/{sym}-1d-{m}.zip" for m in MONTHS]
    with ThreadPoolExecutor(max_workers=8) as ex:
        raws = list(ex.map(fetch, urls))
    if raws[-1] is None:  # October 2026 not yet packed into a monthly file: use the daily files
        days = [date(2026, 10, 1) + timedelta(days=k) for k in range(31)]
        with ThreadPoolExecutor(max_workers=8) as ex:
            raws += list(ex.map(fetch, [f"{V}/spot/daily/klines/{sym}/1d/{sym}-1d-{d.isoformat()}.zip" for d in days]))
    rows = []
    for r in raws:
        if r is None:
            continue
        df = read_csv(r)
        c = df.columns
        rows.append(pd.DataFrame({"day": secs(df[c[0]].to_numpy()) // 86400 * 86400, "o": df[c[1]].astype(float),
                                  "h": df[c[2]].astype(float), "l": df[c[3]].astype(float), "c": df[c[4]].astype(float),
                                  "v": df[c[7]].astype(float)}))
    k = pd.concat(rows).drop_duplicates("day").sort_values("day")
    k["sym"] = sym
    return k


def funding(sym):
    urls = [f"{V}/futures/um/monthly/fundingRate/{sym}/{sym}-fundingRate-{m}.zip" for m in MONTHS if m >= "2019-09"]
    with ThreadPoolExecutor(max_workers=8) as ex:
        raws = list(ex.map(fetch, urls))
    rows = []
    for r in raws:
        if r is None:
            continue
        d = read_csv(r)
        rows.append(pd.DataFrame({"t": secs(d[d.columns[0]].to_numpy()) // 60 * 60, "rate": d[d.columns[-1]].astype(float)}))
    if not rows:
        return pd.DataFrame(columns=["t", "rate", "sym"])
    f = pd.concat(rows).drop_duplicates("t").sort_values("t")
    f["sym"] = sym
    return f


def main():
    os.makedirs(RAW, exist_ok=True)
    t0 = time.time()
    daily, fund = [], []
    for s in COINS:
        k, f = klines(s), funding(s)
        daily.append(k)
        fund.append(f)
        print(s, "daily", len(k), pd.Timestamp(int(k["day"].iloc[0]), unit="s").date(), "->",
              pd.Timestamp(int(k["day"].iloc[-1]), unit="s").date(), "| funding", len(f),
              (pd.Timestamp(int(f["t"].iloc[0]), unit="s").date() if len(f) else "-"), flush=True)
    pd.concat(daily).to_pickle(os.path.join(OUT, "daily.pkl"))
    pd.concat(fund).to_pickle(os.path.join(OUT, "funding.pkl"))
    print(f"done in {time.time() - t0:.0f}s")


if __name__ == "__main__":
    sys.exit(main())
