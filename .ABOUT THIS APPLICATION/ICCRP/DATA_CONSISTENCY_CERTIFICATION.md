# OBJECTIVE 9 — DATA CONSISTENCY CERTIFICATION

| Check | Authority | UI Consumer | Status |
|-------|-----------|-------------|--------|
| Balance | `paper_state.balance` | useEngineState, PaperDesk, Ribbon | PASS |
| Equity | `paper_state.equity` | PaperDesk, Portfolio, Ribbon | PASS |
| PnL | `portfolioAccountingService` | TerminalDashboard, Portfolio | PASS |
| Positions | `paper_positions` | snapshot → terminalStore | PASS |
| Orders | `paper_orders` | event-center | PASS |
| Strategy count | `strategy_scores` | strategy-intelligence | PASS |
| Trade count | `paper_trades` | closed stats merge | PASS |

## Mismatches Fixed

| Issue | Root Cause | Fix |
|-------|------------|-----|
| Terminal showed fake BTC price | Synthetic snapshot | REST `/api/btc/price` |
| Research tournament fake stats | Hardcoded widgets | Removed; live API |
| Overview $1M balance | useEngineState hardcode | `/api/paper-desk/state` |
| QuickTrade fake size | Hardcoded UI | Removed |

## Validation

Run with authenticated session against live MongoDB:

```bash
curl -b cookies.txt /api/paper-desk/snapshot
curl -b cookies.txt /api/paper-desk/state
# Compare balance/equity fields — must match
```
