"""Build the deep BTCUSDT history for the multi-year strategy backtest.

Sources (Binance Vision, the only Binance host reachable from here):
  spot 1m klines   monthly zips 2017-08 .. 2026-09, daily zips 2026-10-01 .. 2026-10-30
  USDT-M funding   monthly zips 2020-01 .. 2026-09 (Vision has nothing earlier)

Every archive is checked against the SHA-256 that Vision publishes beside it,
so a truncated or corrupted download fails loudly instead of becoming a gap.

Output, in engine/data/historical/deep/ (git-ignored; Binance data is not ours
to redistribute, so it is never committed):
  BTCUSDT_1m.csv       time,open,high,low,close,volume,taker_buy   (unix s, UTC)
  BTCUSDT_funding.csv  time,rate                                   (unix s, per 8h)
  data_report.json     coverage, every hole in the 1m series, sanity counts,
                       and a cross-check against the engine's existing cache

Usage: python -I deep_fetch.py
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
from datetime import date, datetime, timedelta, timezone

import numpy as np
import pandas as pd

HERE = os.path.dirname(os.path.abspath(__file__))
ENGINE = os.path.normpath(os.path.join(HERE, "..", ".."))
OUT = os.path.join(ENGINE, "data", "historical", "deep")
RAW = os.path.join(OUT, "raw")
OLD_CACHE = os.path.join(ENGINE, "data", "historical", "BTCUSDT_1h.json")
VISION = "https://data.binance.vision/data"
SYMBOL = "BTCUSDT"

FIRST_MONTH = (2017, 8)
LAST_MONTH = (2026, 9)
DAILY_FROM = date(2026, 10, 1)
DAILY_TO = date(2026, 10, 30)
FUNDING_FIRST = (2020, 1)


def months(first, last):
    y, m = first
    while (y, m) <= last:
        yield f"{y}-{m:02d}"
        m += 1
        if m > 12:
            y, m = y + 1, 1


def kline_urls():
    for ym in months(FIRST_MONTH, LAST_MONTH):
        yield f"{VISION}/spot/monthly/klines/{SYMBOL}/1m/{SYMBOL}-1m-{ym}.zip"
    d = DAILY_FROM
    while d <= DAILY_TO:
        yield f"{VISION}/spot/daily/klines/{SYMBOL}/1m/{SYMBOL}-1m-{d.isoformat()}.zip"
        d += timedelta(days=1)


def funding_urls():
    for ym in months(FUNDING_FIRST, LAST_MONTH):
        yield f"{VISION}/futures/um/monthly/fundingRate/{SYMBOL}/{SYMBOL}-fundingRate-{ym}.zip"


def get(url, timeout=120):
    with urllib.request.urlopen(url, timeout=timeout) as r:
        return r.read()


def fetch_verified(url):
    """Download url into RAW (reusing a verified copy) and return its bytes."""
    path = os.path.join(RAW, url.rsplit("/", 1)[1])
    last = None
    for attempt in range(4):
        try:
            want = get(url + ".CHECKSUM", 60).decode().split()[0].lower()
            if os.path.exists(path):
                with open(path, "rb") as f:
                    raw = f.read()
                if hashlib.sha256(raw).hexdigest() == want:
                    return raw
            raw = get(url)
            got = hashlib.sha256(raw).hexdigest()
            if got != want:
                raise ValueError(f"checksum mismatch {got[:12]} != {want[:12]}")
            with open(path, "wb") as f:
                f.write(raw)
            return raw
        except urllib.error.HTTPError as e:
            if e.code == 404:
                raise FileNotFoundError(url) from e
            last = e
        except Exception as e:  # network hiccup or bad archive: retry
            last = e
        time.sleep(2 * (attempt + 1))
    raise RuntimeError(f"{url}: {last}")


def read_zip_csv(raw):
    with zipfile.ZipFile(io.BytesIO(raw)) as z:
        body = z.read(z.namelist()[0]).decode()
    first = body.split("\n", 1)[0].split(",")[0].strip()
    header = None if first.isdigit() else 0
    return pd.read_csv(io.StringIO(body), header=header)


def to_seconds(t):
    """Vision switched open_time to microseconds in 2025 — normalise to seconds."""
    t = np.asarray(t, dtype=np.int64)
    return np.where(t > 10**14, t // 1_000_000, np.where(t > 10**11, t // 1000, t))


def parse_klines(raw):
    df = read_zip_csv(raw)
    cols = df.columns
    return np.column_stack([
        to_seconds(df[cols[0]].to_numpy()).astype(np.float64),
        df[cols[1]].to_numpy(np.float64), df[cols[2]].to_numpy(np.float64),
        df[cols[3]].to_numpy(np.float64), df[cols[4]].to_numpy(np.float64),
        df[cols[5]].to_numpy(np.float64), df[cols[9]].to_numpy(np.float64),
    ])


def parse_funding(raw):
    df = read_zip_csv(raw)
    cols = df.columns
    # calc_time, funding_interval_hours, last_funding_rate
    return np.column_stack([
        to_seconds(df[cols[0]].to_numpy()).astype(np.float64),
        df[cols[-1]].to_numpy(np.float64),
    ])


def iso(t):
    return datetime.fromtimestamp(int(t), timezone.utc).strftime("%Y-%m-%d %H:%M")


def load_old_cache():
    """The engine's existing 1h cache (2021-01 .. 2026-07), for the cross-check.
    The 1h file is used rather than the 453 MB 1m file so the check fits in memory."""
    if not os.path.exists(OLD_CACHE):
        return None
    with open(OLD_CACHE) as f:
        rows = json.load(f)
    t = np.array([datetime.fromisoformat(r["OpenTime"].replace("Z", "+00:00")).timestamp() for r in rows])
    ohlc = np.array([[r["Open"], r["High"], r["Low"], r["Close"]] for r in rows])
    return t.astype(np.int64), ohlc


def hourly(t, bars):
    """UTC-aligned 1h OHLC from the 1m bars (only hours with all 60 minutes)."""
    hour = t // 3600 * 3600
    starts = np.concatenate([[0], np.nonzero(np.diff(hour))[0] + 1])
    ends = np.concatenate([starts[1:], [len(t)]])
    full = (ends - starts) == 60
    s, e = starts[full], ends[full]
    o = bars[s, 1]
    h = np.maximum.reduceat(bars[:, 2], starts)[full]
    lo = np.minimum.reduceat(bars[:, 3], starts)[full]
    c = bars[e - 1, 4]
    return hour[s], np.column_stack([o, h, lo, c])


def main():
    os.makedirs(RAW, exist_ok=True)
    t0 = time.time()
    urls = list(kline_urls())
    print(f"{len(urls)} kline archives -> {OUT}", flush=True)

    def one(url):
        return url, parse_klines(fetch_verified(url))

    with ThreadPoolExecutor(max_workers=6) as ex:
        parts = []
        for k, (url, arr) in enumerate(ex.map(one, urls), 1):
            parts.append(arr)
            if k % 12 == 0 or k == len(urls):
                print(f"  {k}/{len(urls)} {url.rsplit('/', 1)[1]} ({time.time() - t0:.0f}s)", flush=True)
    bars = np.concatenate(parts)
    bars = bars[np.argsort(bars[:, 0], kind="stable")]
    t = bars[:, 0].astype(np.int64)
    # Dec 2017 and Feb 2018 archives stamp bars 14-20 s past the minute after an
    # outage; put them back on the minute grid (the one bar that then collides
    # with its neighbour is dropped as a duplicate below).
    off_grid = int((t % 60 != 0).sum())
    t = t // 60 * 60
    bars[:, 0] = t
    keep = np.concatenate([[True], np.diff(t) > 0])
    dupes = int((~keep).sum())
    bars, t = bars[keep], t[keep]

    o, h, lo, c, v = bars[:, 1], bars[:, 2], bars[:, 3], bars[:, 4], bars[:, 5]
    bad_ohlc = int(((lo > np.minimum(o, c)) | (h < np.maximum(o, c)) | (lo <= 0)).sum())
    zero_vol = int((v <= 0).sum())

    step = np.diff(t)
    holes = np.nonzero(step > 60)[0]
    gaps = [{"after": iso(t[i]), "resumes": iso(t[i + 1]), "missing_minutes": int(step[i] // 60 - 1)}
            for i in holes]
    gaps.sort(key=lambda g: -g["missing_minutes"])
    expected = (t[-1] - t[0]) // 60 + 1

    with open(os.path.join(OUT, f"{SYMBOL}_1m.csv"), "w", newline="") as f:
        f.write("time,open,high,low,close,volume,taker_buy\n")
        np.savetxt(f, bars, fmt=["%d", "%.8g", "%.8g", "%.8g", "%.8g", "%.8g", "%.8g"], delimiter=",")
    print(f"1m: {len(t):,} bars {iso(t[0])} -> {iso(t[-1])}, {len(gaps)} holes", flush=True)

    with ThreadPoolExecutor(max_workers=6) as ex:
        fparts = list(ex.map(lambda u: parse_funding(fetch_verified(u)), list(funding_urls())))
    fund = np.concatenate(fparts)
    fund = fund[np.argsort(fund[:, 0], kind="stable")]
    fkeep = np.concatenate([[True], np.diff(fund[:, 0]) > 0])
    fund = fund[fkeep]
    with open(os.path.join(OUT, f"{SYMBOL}_funding.csv"), "w", newline="") as f:
        f.write("time,rate\n")
        np.savetxt(f, fund, fmt=["%d", "%.8g"], delimiter=",")
    print(f"funding: {len(fund):,} prints {iso(fund[0, 0])} -> {iso(fund[-1, 0])}", flush=True)

    cross = None
    old = load_old_cache()
    if old is not None:
        ot, oohlc = old
        ht, hohlc = hourly(t, bars)
        idx = np.clip(np.searchsorted(ht, ot), 0, len(ht) - 1)
        match = ht[idx] == ot
        diff = np.abs(hohlc[idx[match]] - oohlc[match])
        cross = {"old_cache_hours": int(len(ot)), "full_hours_compared": int(match.sum()),
                 "identical_ohlc": int((diff.max(axis=1) < 1e-9).sum()),
                 "max_abs_diff": float(diff.max()) if match.any() else None}
        print(f"cross-check vs old 1h cache: {cross}", flush=True)

    report = {
        "symbol": SYMBOL, "source": "Binance Vision spot 1m klines + USDT-M fundingRate archives, SHA-256 verified",
        "first_bar": iso(t[0]), "last_bar": iso(t[-1]), "bars": int(len(t)),
        "expected_minutes": int(expected), "missing_minutes": int(expected - len(t)),
        "holes": len(gaps), "holes_over_60min": sum(1 for g in gaps if g["missing_minutes"] > 60),
        "largest_holes": gaps[:25], "duplicate_rows_dropped": dupes,
        "bad_ohlc_rows": bad_ohlc, "zero_volume_bars": zero_vol, "off_grid_times_realigned": off_grid,
        "funding_first": iso(fund[0, 0]), "funding_last": iso(fund[-1, 0]), "funding_prints": int(len(fund)),
        "cross_check_old_cache": cross,
    }
    with open(os.path.join(OUT, "data_report.json"), "w") as f:
        json.dump(report, f, indent=1)
    print(json.dumps({k: v for k, v in report.items() if k != "largest_holes"}, indent=1), flush=True)
    print(f"done in {time.time() - t0:.0f}s", flush=True)


if __name__ == "__main__":
    sys.exit(main())
