# BTC Scalper Autonomous Trading App (Live Market Data)

This project is a React-based Bitcoin scalper app built from your auto-trading logic:

- Autonomous signal scanning (EMA, RSI, MACD, VWAP, momentum, volume)
- Auto entry and risk-managed exits (SL, TP1, TP2, TP3)
- Multi-position management with side limits
- Live `BTCUSDT` ticker + `1m` candles via WebSocket
  - Binance: `@ticker` + `@kline_1m`
  - Bybit: `tickers.BTCUSDT` + `kline.1.BTCUSDT`
- Session stats and performance snapshot
- Exchange switcher with connection health/reconnect status

## Run

```bash
npm install
npm run dev
```

## Build

```bash
npm run build
```

## Notes

- Market data is live (Binance/Bybit), but order execution is still local simulation logic.
- If Bybit stream access is restricted in your region, switch the UI source to Binance.
