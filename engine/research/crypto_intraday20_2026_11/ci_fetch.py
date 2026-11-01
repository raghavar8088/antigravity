"""Download the data for the 20-strategy intraday crypto study.

Binance Vision, USDT-M perpetual futures, SHA-256 checked against the published
CHECKSUM files, for BTC ETH SOL XRP DOGE BNB:
  klines 1m               open/high/low/close, volume, taker-buy volume
  premiumIndexKlines 1m   perp premium over the index (what funding is built from)
  metrics 5m              open interest, top-trader and all-account long/short ratios
  fundingRate             the 8h (or 4h) funding prints
Months 2025-10 .. 2026-10 (2025-10 is warm-up only).

Binance has not published the perpetual 1m file for 2026-10-28 (any coin). That
day is rebuilt from Binance SPOT 1m bars so indicators that look back across it
stay real: prices as traded on spot, volumes scaled to perpetual size by the
perpetual/spot volume ratio of the 7 days before, taker imbalance as on spot.
Rebuilt days are marked (column `rebuilt`) and no strategy trades on them.
Premium-index days that are missing stay empty rather than carried forward.

Output (git-ignored): engine/data/historical/intraday20/<SYM>.pkl, one DataFrame
per symbol on a complete UTC minute grid, plus <SYM>_metrics.pkl,
<SYM>_funding.pkl and data_report.json.

Usage: python -I ci_fetch.py
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
from datetime import date, timedelta

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "intraday20"))
RAW = os.path.join(OUT, "raw")
V = "https://data.binance.vision/data/futures/um"
SYMBOLS = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "BNBUSDT"]
MONTHS = [f"{y}-{m:02d}" for y, m in [(2025, 10), (2025, 11), (2025, 12)] + [(2026, k) for k in range(1, 10)]]
OCT = [date(2026, 10, 1) + timedelta(days=k) for k in range(31)]
METRIC_DAYS = [date(2025, 10, 1) + timedelta(days=k) for k in range((date(2026, 10, 31) - date(2025, 10, 1)).days + 1)]


def get(url, timeout=120):
    with urllib.request.urlopen(url, timeout=timeout) as r:
        return r.read()


def fetch(url):
    """Verified bytes of one archive (a verified local copy is reused); None if not published."""
    rel = url.split("/data/futures/um/", 1)[1] if "/futures/um/" in url else "spot/" + url.split("/data/spot/", 1)[1]
    path = os.path.join(RAW, rel.replace("/", "__"))
    last = None
    for attempt in range(4):
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


def klines(sym, kind):
    urls = [f"{V}/monthly/{kind}/{sym}/1m/{sym}-1m-{m}.zip" for m in MONTHS]
    urls += [f"{V}/daily/{kind}/{sym}/1m/{sym}-1m-{d.isoformat()}.zip" for d in OCT]
    with ThreadPoolExecutor(max_workers=8) as ex:
        raws = list(ex.map(fetch, urls))
    missing = [u.rsplit("/", 1)[1] for u, r in zip(urls, raws) if r is None]
    frames = []
    for r in raws:
        if r is None:
            continue
        df = read_csv(r)
        c = df.columns
        frames.append(pd.DataFrame({
            "t": secs(df[c[0]].to_numpy()), "o": df[c[1]].astype(float), "h": df[c[2]].astype(float),
            "l": df[c[3]].astype(float), "c": df[c[4]].astype(float), "v": df[c[5]].astype(float),
            "tbv": df[c[9]].astype(float)}))
    k = pd.concat(frames).sort_values("t").drop_duplicates("t").reset_index(drop=True)
    k["t"] = k["t"] // 60 * 60
    return k.drop_duplicates("t"), missing


def spot_day(sym, d):
    raw = fetch(f"https://data.binance.vision/data/spot/daily/klines/{sym}/1m/{sym}-1m-{d.isoformat()}.zip")
    if raw is None:
        return None
    df = read_csv(raw)
    c = df.columns
    return pd.DataFrame({"t": secs(df[c[0]].to_numpy()) // 60 * 60, "o": df[c[1]].astype(float),
                         "h": df[c[2]].astype(float), "l": df[c[3]].astype(float), "c": df[c[4]].astype(float),
                         "v": df[c[5]].astype(float), "tbv": df[c[9]].astype(float)})


def rebuild(sym, k, missing_days):
    """Spot bars for the missing perpetual days, volume scaled to perpetual size."""
    parts = []
    for d in missing_days:
        prior = [d - timedelta(days=j) for j in range(1, 8)]
        ratios = []
        for pd_ in prior:
            sp = spot_day(sym, pd_)
            lo, hi = pd.Timestamp(pd_).value // 10**9, pd.Timestamp(pd_).value // 10**9 + 86400
            fv = k[(k["t"] >= lo) & (k["t"] < hi)]["v"].sum()
            if sp is not None and sp["v"].sum() > 0 and fv > 0:
                ratios.append(fv / sp["v"].sum())
        sp = spot_day(sym, d)
        if sp is None or not ratios:
            continue
        r = float(np.median(ratios))
        sp["v"] *= r
        sp["tbv"] *= r
        parts.append(sp)
    return parts


def metrics(sym):
    urls = [f"{V}/daily/metrics/{sym}/{sym}-metrics-{d.isoformat()}.zip" for d in METRIC_DAYS]
    with ThreadPoolExecutor(max_workers=8) as ex:
        raws = list(ex.map(fetch, urls))
    frames = [read_csv(r) for r in raws if r is not None]
    m = pd.concat(frames)
    m["t"] = (pd.to_datetime(m["create_time"]) - pd.Timestamp("1970-01-01")) // pd.Timedelta(seconds=1)
    keep = ["t", "sum_open_interest", "sum_open_interest_value", "count_toptrader_long_short_ratio",
            "sum_toptrader_long_short_ratio", "count_long_short_ratio", "sum_taker_long_short_vol_ratio"]
    m = m[keep].sort_values("t").drop_duplicates("t").reset_index(drop=True)
    return m, sum(r is None for r in raws)


def funding(sym):
    urls = [f"{V}/monthly/fundingRate/{sym}/{sym}-fundingRate-{m}.zip" for m in MONTHS + ["2026-10"]]
    frames = []
    for r in map(fetch, urls):
        if r is None:
            continue
        df = read_csv(r)
        c = df.columns
        frames.append(pd.DataFrame({"t": secs(df[c[0]].to_numpy()) // 60 * 60, "rate": df[c[-1]].astype(float)}))
    return pd.concat(frames).sort_values("t").drop_duplicates("t").reset_index(drop=True)


def one(sym):
    k, miss_k = klines(sym, "klines")
    p, miss_p = klines(sym, "premiumIndexKlines")
    missing_days = [date.fromisoformat(m.split("-1m-")[1][:10]) for m in miss_k if len(m.split("-1m-")[1]) == 14]
    spot = rebuild(sym, k, missing_days)
    rebuilt = set()
    if spot:
        sp = pd.concat(spot)
        rebuilt = set(sp["t"].to_numpy() // 86400 * 86400)
        k = pd.concat([k, sp]).sort_values("t").drop_duplicates("t").reset_index(drop=True)
    grid = np.arange(k["t"].iloc[0], k["t"].iloc[-1] + 60, 60, dtype=np.int64)
    df = k.set_index("t").reindex(grid)
    filled = int(df["c"].isna().sum())
    df["c"] = df["c"].ffill()
    for col in ("o", "h", "l"):
        df[col] = df[col].fillna(df["c"])
    df[["v", "tbv"]] = df[["v", "tbv"]].fillna(0.0)
    df["prem"] = p.set_index("t")["c"].reindex(grid).ffill(limit=5)
    df["rebuilt"] = np.isin(grid // 86400 * 86400, list(rebuilt))
    df.index.name = "t"
    df.to_pickle(os.path.join(OUT, f"{sym}.pkl"))
    m, miss_m = metrics(sym)
    m.to_pickle(os.path.join(OUT, f"{sym}_metrics.pkl"))
    f = funding(sym)
    f.to_pickle(os.path.join(OUT, f"{sym}_funding.pkl"))
    iso = lambda s: pd.Timestamp(int(s), unit="s").strftime("%Y-%m-%d %H:%M")
    rep = {"first": iso(grid[0]), "last": iso(grid[-1]), "minutes": int(len(grid)), "filled_minutes": filled,
           "rebuilt_from_spot": sorted(pd.Timestamp(int(x), unit="s").strftime("%Y-%m-%d") for x in rebuilt),
           "missing_kline_archives": miss_k, "missing_premium_archives": miss_p, "missing_metric_days": miss_m,
           "metrics_rows": int(len(m)), "funding_prints": int(len(f)), "premium_nan": int(df["prem"].isna().sum())}
    print(sym, rep, flush=True)
    return sym, rep


def main():
    os.makedirs(RAW, exist_ok=True)
    t0 = time.time()
    report = dict(one(s) for s in SYMBOLS)
    json.dump(report, open(os.path.join(OUT, "data_report.json"), "w"), indent=1)
    print(f"done in {time.time() - t0:.0f}s", flush=True)


if __name__ == "__main__":
    sys.exit(main())
