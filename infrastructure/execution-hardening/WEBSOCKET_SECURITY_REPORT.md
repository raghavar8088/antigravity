# WebSocket Security Report

## Audited handlers

| Location | Direction | Execution commands? |
|----------|-----------|---------------------|
| `client/src/hooks/useLiveBTCMarket.ts` | inbound market data | NO |
| `client/src/lib/terminal/terminalStore.tsx` | inbound snapshots | NO |
| `engine/internal/marketdata/coinbase.go` | inbound ticks | NO |

## Verdict

No websocket message type in this repository triggers BUY/SELL/EXECUTE/CONFIRM/OVERRIDE without backend validation.

External `NEXT_PUBLIC_TERMINAL_WS_URL` server behavior: NOT VERIFIED.
