# Repository Summary

This repository is a BTC futures and crypto trading platform.

## What To Read First

1. `docs/architecture-overview.md`
2. `docs/module-index.md`
3. `.cursor/skills/trading-app-guide/SKILL.md`
4. `client/package.json`
5. `engine/cmd/antigravity/main.go`

## Application Core

- `client/`: operator UI, API routes, BTC paper/mock trading workflows.
- `engine/`: Go trading engine, BTC feeds, risk, execution, options, persistence, observability.

## Protected Business Domain

BTC Futures Trading is business-critical. Preserve:

- strategy definitions and signal logic,
- entry/exit gates,
- fee/funding/liquidation math,
- paper PnL/accounting,
- replay and diagnostics,
- worker/cron parity,
- engine risk/OMS/ledger/kill switch paths.

Graphify is also protected. Keep all `graphify-out/` folders and related rule files.

## Current Scale

Approximate read-only metrics excluding `.git`, `.venv`, `vendor`, `node_modules`, and `.next`:

- Files: 2,351
- TypeScript/TSX: 662
- Go: 747
- Markdown: 42
- Bytes: ~38 MB

## Validation Commands

```bash
cd client
npm run test
npm run build
```

```bash
cd engine
go test ./...
go build ./...
```

Note: local validation depends on installed Node modules and Go availability.
