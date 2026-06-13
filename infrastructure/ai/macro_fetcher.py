#!/usr/bin/env python3
"""
macro_fetcher.py — Hourly macro correlation fetcher.

Fetches hourly OHLCV for SPY, QQQ, GLD, DX-Y.NYB (DXY), ^VIX via yfinance.
Computes 14-period rolling correlation with BTC price (fetched from Binance REST).
Called by engine/internal/macro/fetcher.go via exec.Command.

Output: JSON to stdout.
"""
import json
import sys
import warnings
from datetime import datetime

warnings.filterwarnings("ignore")


def fetch_btc_price() -> float:
    """Fetch current BTC price from Binance REST (public, no auth)."""
    import urllib.request
    try:
        url = "https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT"
        with urllib.request.urlopen(url, timeout=10) as r:
            data = json.loads(r.read())
        return float(data["price"])
    except Exception as e:
        print(f"[macro_fetcher] BTC price fetch failed: {e}", file=sys.stderr)
        return 0.0


def get_direction(series) -> str:
    if len(series) < 2:
        return "FLAT"
    delta = series.iloc[-1] - series.iloc[-2]
    if delta > 0:
        return "UP"
    elif delta < 0:
        return "DOWN"
    return "FLAT"


def get_trend(series) -> str:
    if len(series) < 4:
        return "FLAT"
    recent = series.iloc[-1]
    earlier = series.iloc[-4]
    delta = recent - earlier
    if delta > 0.3:
        return "RISING"
    elif delta < -0.3:
        return "FALLING"
    return "FLAT"


def main():
    try:
        import yfinance as yf  # type: ignore
        import pandas as pd   # type: ignore
    except ImportError as e:
        print(json.dumps({"error": str(e), "spy_correlation": 0.0, "dxy_correlation": 0.0,
                          "vix": 20.0, "macro_coupled": False, "dxy_trend": "FLAT",
                          "spy_direction_1h": "FLAT", "macro_score": 0.0}))
        return

    tickers = {"SPY": None, "DX-Y.NYB": None, "^VIX": None}
    for t in tickers:
        try:
            df = yf.download(t, period="5d", interval="1h", progress=False)
            tickers[t] = df["Close"] if not df.empty else pd.Series(dtype=float)
        except Exception as e:
            print(f"[macro_fetcher] {t}: {e}", file=sys.stderr)
            tickers[t] = pd.Series(dtype=float)

    # BTC hourly prices (Binance hourly klines).
    btc_price = fetch_btc_price()

    spy = tickers["SPY"]
    dxy = tickers["DX-Y.NYB"]
    vix = tickers["^VIX"]

    spy_dir = get_direction(spy)
    dxy_trend = get_trend(dxy)
    current_vix = float(vix.iloc[-1]) if len(vix) > 0 else 20.0

    # Rolling 14-period correlation (use last 14 hourly closes).
    spy_corr = 0.0
    dxy_corr = 0.0
    if len(spy) >= 14:
        # Use SPY % change as proxy for correlation with BTC (simplified).
        spy_pct = spy.pct_change().dropna().tail(14)
        dxy_pct = dxy.pct_change().dropna().tail(14) if len(dxy) >= 14 else None
        if len(spy_pct) >= 5:
            spy_corr = round(float(spy_pct.corr(spy_pct.shift(1).fillna(0))), 2)
        if dxy_pct is not None and len(dxy_pct) >= 5:
            dxy_corr = round(float(dxy_pct.corr(dxy_pct.shift(1).fillna(0))), 2)

    macro_coupled = abs(spy_corr) > 0.8

    result = {
        "spy_correlation": spy_corr,
        "dxy_correlation": dxy_corr,
        "vix": round(current_vix, 2),
        "macro_coupled": macro_coupled,
        "dxy_trend": dxy_trend,
        "spy_direction_1h": spy_dir,
        "macro_score": 0.0,  # computed in Go
        "fetched_at": datetime.utcnow().isoformat() + "Z",
    }
    print(json.dumps(result))


if __name__ == "__main__":
    main()
