# MARKET DATA FORENSIC REPORT

## Primary Feed

| Feed | File | Function | Port/Endpoint |
|------|------|----------|---------------|
| Coinbase WS | `main.go:416–428` | `marketdata.NewCoinbaseClient().Connect` | BTC-USD public WS |
| Warmup REST | `main.go:856` | `marketdata.FetchWarmupCandles` | Coinbase REST fallback |

## Tick Path

`Coinbase tick` → `client.GetTickChannel()` → `Orchestrator.Run` (`loop.go:997`) → `processTickPipeline` (`loop.go:1012`).

## Verdict

**NOT root cause.** Market data feeds orchestrator; strategies evaluate on ticks/candles. Outage occurred with kill switch blocking execution while ticks likely continued.

## Stale Feed Detection (Post-Fix)

`ExecutionWatchdog` alerts if no tick recorded for 3 minutes (`execution_watchdog.go:defaultStaleFeedWindow`).

## Fallback Chain (from CLAUDE.md)

Delta → Binance, NSE → AngelOne — used for other products; BTC paper desk uses Coinbase primary.
