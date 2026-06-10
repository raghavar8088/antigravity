# CONFIGURATION AUDIT

## Flags That Affect Mock Trading

| Variable | Default | File | Effect on mock trading |
|----------|---------|------|------------------------|
| `ENGINE_EXECUTION_AUTHORITY` | `1` (on) | `client/src/lib/engineAuthority.ts:6–9` | Go engine sole executor; TS worker disabled |
| `NEXT_PUBLIC_ENGINE_EXECUTION_AUTHORITY` | `1` | `useBTCFuturesScalperEngine.ts:2676` | Browser skips local execution |
| `OWNER_ACCOUNT_KEY` | `mock_trading_default` | `paperpersist/accountkey.go` | Mongo account alignment |
| `DATABASE_URL` | required prod | `main.go:707` | Durable ledger + kill switch persistence |
| `MONGODB_URI` | required | `main.go:593` | Trade persistence to UI |
| `DELTA_API_KEY/SECRET` | optional | `wiring.go:73` | Enables Delta recon authority |
| `PAPER_OMS_ADMIN_OVERRIDE` | unset | `paper_oms_handler.go:56–62` | Blocks `/paper/` mutations |

## NOT FOUND (no env toggles)

- `MOCK_TRADING_ENABLED` — zero matches in repo
- `PAPER_TRADING_ENABLED` — zero matches
- `EXECUTION_ENABLED` — zero matches
- `TRADING_ENABLED` — zero matches

## Hardcoded Paper Config

| Setting | Value | File:Line |
|---------|-------|-----------|
| Initial balance | $1,000,000 | `main.go:75` `initialPaperBalanceUSD` |
| Account ID | `btc-paper-1` | `loop.go:40` |

## Outage Configuration Factor

**No config flag disabled trading.** Outage was **code bug in recon v2** deployed via commit `33c614a8`, not env misconfiguration.
