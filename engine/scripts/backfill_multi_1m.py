"""Backfill 1m klines for major crypto symbols into engine/data/historical.

Sources: Binance Vision monthly spot-kline zips (bulk) + REST /api/v3/klines
for the current partial month. Output format is the compact cache schema the
Go loaders accept: [{"time": unix_seconds, "open": .., "high": .., "low": ..,
"close": .., "volume": ..}, ...]. 5m/15m/1h files are resampled locally from
the 1m series so every timeframe is guaranteed consistent.

Usage: python backfill_multi_1m.py [SYMBOL ...]   (default: the 8 majors)
"""
import io
import json
import os
import sys
import time
import urllib.request
import zipfile
from concurrent.futures import ThreadPoolExecutor

CACHE = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "data", "historical"))
DEFAULT_SYMBOLS = ["ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "LINKUSDT", "AVAXUSDT"]
# 26 months ending 2026-06 gives a ~2.15y series: 2y qualification window + lead-in.
MONTHS = []
y, m = 2024, 5
while (y, m) <= (2026, 6):
    MONTHS.append(f"{y}-{m:02d}")
    m += 1
    if m > 12:
        y, m = y + 1, 1


def norm_ts(t):
    """Vision files switched open_time to microseconds in 2025 — normalize to seconds."""
    t = int(t)
    if t > 1e14:
        return t // 1_000_000
    if t > 1e12:
        return t // 1000
    return t


def fetch_month(sym, ym):
    url = f"https://data.binance.vision/data/spot/monthly/klines/{sym}/1m/{sym}-1m-{ym}.zip"
    for attempt in range(3):
        try:
            with urllib.request.urlopen(url, timeout=120) as r:
                raw = r.read()
            bars = []
            with zipfile.ZipFile(io.BytesIO(raw)) as z:
                with z.open(z.namelist()[0]) as f:
                    for line in io.TextIOWrapper(f, "utf-8"):
                        p = line.split(",")
                        if not p or not p[0].strip().isdigit():
                            continue  # header line in newer files
                        bars.append({
                            "time": norm_ts(p[0]),
                            "open": float(p[1]), "high": float(p[2]),
                            "low": float(p[3]), "close": float(p[4]),
                            "volume": float(p[5]),
                        })
            print(f"  {sym} {ym}: {len(bars)} bars", flush=True)
            return bars
        except urllib.error.HTTPError as e:
            if e.code == 404:
                print(f"  {sym} {ym}: 404 (not listed yet)", flush=True)
                return []
            time.sleep(2 * (attempt + 1))
        except Exception as e:
            print(f"  {sym} {ym}: retry after {e}", flush=True)
            time.sleep(2 * (attempt + 1))
    print(f"  {sym} {ym}: FAILED", flush=True)
    return []


def fetch_rest_tail(sym, start_s):
    """REST klines from start_s (exclusive) to now."""
    out = []
    start_ms = (start_s + 60) * 1000
    while True:
        url = (f"https://api.binance.com/api/v3/klines?symbol={sym}&interval=1m"
               f"&startTime={start_ms}&limit=1000")
        try:
            with urllib.request.urlopen(url, timeout=60) as r:
                rows = json.loads(r.read())
        except Exception as e:
            print(f"  {sym} tail: retry after {e}", flush=True)
            time.sleep(3)
            continue
        if not rows:
            break
        for p in rows:
            out.append({
                "time": norm_ts(p[0]),
                "open": float(p[1]), "high": float(p[2]),
                "low": float(p[3]), "close": float(p[4]),
                "volume": float(p[5]),
            })
        start_ms = rows[-1][0] + 60_000
        if len(rows) < 1000:
            break
        time.sleep(0.15)
    print(f"  {sym} tail: {len(out)} bars", flush=True)
    return out


def resample(bars, minutes):
    out = []
    cur_bucket, o, h, l, c, v = None, 0, 0, 0, 0, 0.0
    step = minutes * 60
    for b in bars:
        bucket = b["time"] // step * step
        if bucket != cur_bucket:
            if cur_bucket is not None:
                out.append({"time": cur_bucket, "open": o, "high": h, "low": l, "close": c, "volume": v})
            cur_bucket, o, h, l, c, v = bucket, b["open"], b["high"], b["low"], b["close"], b["volume"]
        else:
            h = max(h, b["high"])
            l = min(l, b["low"])
            c = b["close"]
            v += b["volume"]
    if cur_bucket is not None:
        out.append({"time": cur_bucket, "open": o, "high": h, "low": l, "close": c, "volume": v})
    return out


def load_existing_1m(path):
    with open(path) as f:
        data = json.load(f)
    bars = []
    for b in data:
        if "time" in b:
            bars.append({"time": norm_ts(b["time"]), "open": float(b["open"]), "high": float(b["high"]),
                         "low": float(b["low"]), "close": float(b["close"]), "volume": float(b["volume"])})
        else:  # HistoricalCandle format with RFC3339 OpenTime
            import datetime
            t = datetime.datetime.fromisoformat(b["OpenTime"].replace("Z", "+00:00"))
            bars.append({"time": int(t.timestamp()), "open": float(b["Open"]), "high": float(b["High"]),
                         "low": float(b["Low"]), "close": float(b["Close"]), "volume": float(b["Volume"])})
    bars.sort(key=lambda x: x["time"])
    return bars


def write_json(path, bars):
    with open(path, "w") as f:
        f.write('[')
        f.write(','.join(json.dumps(b, separators=(",", ":")) for b in bars))
        f.write(']')
    print(f"  wrote {os.path.basename(path)}: {len(bars)} bars", flush=True)


def do_symbol(sym):
    print(f"=== {sym} ===", flush=True)
    p1m = os.path.join(CACHE, f"{sym}_1m.json")
    if os.path.exists(p1m) and os.path.getsize(p1m) > 50_000_000:
        print(f"  {sym}_1m.json exists — loading for resample/tail top-up", flush=True)
        bars = load_existing_1m(p1m)
    else:
        with ThreadPoolExecutor(max_workers=6) as ex:
            chunks = list(ex.map(lambda ym: fetch_month(sym, ym), MONTHS))
        bars = [b for ch in chunks for b in ch]
        bars.sort(key=lambda x: x["time"])
    if not bars:
        print(f"  {sym}: NO DATA — skipped", flush=True)
        return
    # top up to now via REST, dedupe on time
    tail = fetch_rest_tail(sym, bars[-1]["time"])
    seen = bars[-1]["time"]
    bars.extend(b for b in tail if b["time"] > seen)
    write_json(p1m, bars)
    for minutes, tag in [(5, "5m"), (15, "15m"), (60, "1h")]:
        write_json(os.path.join(CACHE, f"{sym}_{tag}.json"), resample(bars, minutes))
    first = time.strftime("%Y-%m-%d", time.gmtime(bars[0]["time"]))
    last = time.strftime("%Y-%m-%d", time.gmtime(bars[-1]["time"]))
    print(f"  {sym} done: {len(bars)} 1m bars, {first} -> {last}", flush=True)


def main():
    syms = sys.argv[1:] or DEFAULT_SYMBOLS
    print(f"cache dir: {CACHE}", flush=True)
    for sym in syms:
        do_symbol(sym)


if __name__ == "__main__":
    main()
