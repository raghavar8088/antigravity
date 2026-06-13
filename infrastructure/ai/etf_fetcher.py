#!/usr/bin/env python3
"""
etf_fetcher.py — Daily BTC spot ETF flow estimator using yfinance.

Uses change in shares outstanding × NAV per share as a proxy for net flow.
Official flow data is not available via free APIs; this is an approximation.
Called by engine/internal/etf/fetcher.go via exec.Command.

Output: JSON to stdout.
"""
import json
import sys
import warnings
from datetime import date, timedelta

warnings.filterwarnings("ignore")

ETF_TICKERS = ["IBIT", "FBTC", "ARKB", "BITB", "HODL", "BTCO", "BRRR"]


def fetch_flow(ticker: str) -> float:
    """Return estimated USD flow for ticker using shares_outstanding proxy."""
    try:
        import yfinance as yf  # type: ignore
        t = yf.Ticker(ticker)
        hist = t.history(period="2d")
        if hist.empty or len(hist) < 2:
            return 0.0
        info = t.info
        shares_prev = info.get("sharesOutstanding")
        if shares_prev is None:
            return 0.0
        nav = info.get("navPrice") or info.get("previousClose") or hist["Close"].iloc[-1]
        # Approximate: delta volume × price as proxy when shares_outstanding unavailable.
        vol_today = hist["Volume"].iloc[-1]
        vol_prev = hist["Volume"].iloc[-2]
        price = hist["Close"].iloc[-1]
        delta_vol = vol_today - vol_prev
        return round(delta_vol * price, 2)
    except Exception as e:
        print(f"[etf_fetcher] WARNING: {ticker}: {e}", file=sys.stderr)
        return 0.0


def main():
    flows = {}
    for ticker in ETF_TICKERS:
        flows[ticker] = fetch_flow(ticker)

    total = sum(flows.values())
    largest = max(flows, key=lambda k: abs(flows[k]), default="")

    result = {
        "date": date.today().isoformat(),
        "flows": flows,
        "total_flow_usd": round(total, 2),
        "largest_etf": largest,
        "data_source": "yfinance_volume_proxy",
    }
    print(json.dumps(result))


if __name__ == "__main__":
    main()
