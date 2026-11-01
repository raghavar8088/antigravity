"""1m perpetual data for 10 more coins, August-October 2026, for round 2 of the
October search (August and September are only indicator warm-up).

Binance Vision USDT-M klines (monthly Aug/Sep, daily October) and funding,
SHA-256 checked. 2026-10-28 was never published for perpetuals: rebuilt from
spot 1m with volume scaled to perpetual size, and blocked from trading, as in
the earlier studies. Output (git-ignored): engine/data/historical/october_extra/.

Usage: python -I os_fetch_extra.py
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
OUT = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "october_extra"))
RAW = os.path.join(OUT, "raw")
V = "https://data.binance.vision/data"
EXTRA = ["ADAUSDT", "LINKUSDT", "AVAXUSDT", "LTCUSDT", "DOTUSDT", "TRXUSDT", "BCHUSDT", "NEARUSDT", "SUIUSDT", "APTUSDT"]
OCT = [date(2026, 10, 1) + timedelta(days=k) for k in range(31)]


def get(url, timeout=120):
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


def bars(raw):
    with zipfile.ZipFile(io.BytesIO(raw)) as z:
        body = z.read(z.namelist()[0]).decode()
    first = body.split("\n", 1)[0].split(",")[0].strip()
    df = pd.read_csv(io.StringIO(body), header=None if first.isdigit() else 0)
    c = df.columns
    t = np.asarray(df[c[0]], dtype=np.int64)
    t = np.where(t > 10**14, t // 1_000_000, np.where(t > 10**11, t // 1000, t)) // 60 * 60
    return pd.DataFrame({"t": t, "o": df[c[1]].astype(float), "h": df[c[2]].astype(float), "l": df[c[3]].astype(float),
                         "c": df[c[4]].astype(float), "v": df[c[5]].astype(float)})


def one(sym):
    urls = [f"{V}/futures/um/monthly/klines/{sym}/1m/{sym}-1m-{m}.zip" for m in ("2026-08", "2026-09")]
    urls += [f"{V}/futures/um/daily/klines/{sym}/1m/{sym}-1m-{d.isoformat()}.zip" for d in OCT]
    with ThreadPoolExecutor(max_workers=8) as ex:
        raws = list(ex.map(fetch, urls))
    parts = [bars(r) for r in raws if r is not None]
    missing = [u.rsplit("-1m-", 1)[1][:10] for u, r in zip(urls, raws) if r is None]
    k = pd.concat(parts).drop_duplicates("t").sort_values("t")
    bad = set()
    for d in missing:
        if len(d) != 10:
            continue
        sp_raw = fetch(f"{V}/spot/daily/klines/{sym}/1m/{sym}-1m-{d}.zip")
        if sp_raw is None:
            bad.add(int(pd.Timestamp(d, tz="UTC").timestamp()))
            continue
        sp = bars(sp_raw)
        prior = k[(k["t"] >= int(pd.Timestamp(d, tz="UTC").timestamp()) - 7 * 86400) & (k["t"] < int(pd.Timestamp(d, tz="UTC").timestamp()))]
        sp["v"] *= (prior["v"].sum() / 7) / max(sp["v"].sum(), 1e-9) if len(prior) else 1.0
        k = pd.concat([k, sp]).drop_duplicates("t").sort_values("t")
        bad.add(int(pd.Timestamp(d, tz="UTC").timestamp()))
    grid = np.arange(int(pd.Timestamp("2026-08-01", tz="UTC").timestamp()), int(pd.Timestamp("2026-11-01", tz="UTC").timestamp()), 60, dtype=np.int64)
    df = k.set_index("t").reindex(grid)
    present = df["c"].notna().to_numpy()
    miss_day = pd.Series(~present).groupby(grid // 86400 * 86400).sum()
    bad |= set(int(x) for x in miss_day[miss_day > 30].index)
    df["c"] = df["c"].ffill().bfill()
    for col in ("o", "h", "l"):
        df[col] = df[col].fillna(df["c"])
    df["v"] = df["v"].fillna(0.0)
    np.savez(os.path.join(OUT, f"{sym}.npz"), t=grid, o=df["o"].to_numpy(), h=df["h"].to_numpy(), l=df["l"].to_numpy(),
             c=df["c"].to_numpy(), v=df["v"].to_numpy(), bad_day=np.array(sorted(bad), dtype=np.int64))
    fr = []
    for m in ("2026-08", "2026-09", "2026-10"):
        r = fetch(f"{V}/futures/um/monthly/fundingRate/{sym}/{sym}-fundingRate-{m}.zip")
        if r is not None:
            with zipfile.ZipFile(io.BytesIO(r)) as z:
                d = pd.read_csv(io.StringIO(z.read(z.namelist()[0]).decode()))
            tt = np.asarray(d[d.columns[0]], dtype=np.int64)
            fr.append(pd.DataFrame({"t": np.where(tt > 10**11, tt // 1000, tt) // 60 * 60, "rate": d[d.columns[-1]].astype(float)}))
    f = pd.concat(fr).drop_duplicates("t").sort_values("t") if fr else pd.DataFrame({"t": [], "rate": []})
    np.savez(os.path.join(OUT, f"{sym}_funding.npz"), t=f["t"].to_numpy(np.int64), rate=f["rate"].to_numpy(float))
    print(sym, "minutes", len(grid), "missing", int((~present).sum()), "bad days",
          [pd.Timestamp(x, unit="s").date().isoformat() for x in sorted(bad)], "funding", len(f), flush=True)


def main():
    os.makedirs(RAW, exist_ok=True)
    for s in EXTRA:
        one(s)


if __name__ == "__main__":
    sys.exit(main())
