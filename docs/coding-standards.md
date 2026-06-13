# Coding Standards

## Protected Invariants

Never change without explicit request and focused tests:

- BTC futures funding, liquidation, fee, margin, and PnL math.
- Paper trade booking and displayed trade values.
- Risk gates, kill switch wiring, OMS authority, reconciliation checks.
- Strategy gate order and signal trace semantics.
- Graphify code/folders and generated `graphify-out/` artifacts.

## TypeScript / Next.js

- Keep pure trading logic in `client/src/lib/trading/`.
- Keep server-only database/broker code out of client components.
- Use existing zod schemas for route validation.
- Prefer small, typed helpers over repeated route-specific parsing.
- Add tests near changed trading logic.
- Use `NEXT_PUBLIC_*` only for browser-safe values.

## Go Engine

- Keep order flow through execution gateway, risk, OMS, ledger, and kill switch.
- Do not bypass reconciliation or persistence hooks in production paths.
- Prefer package-local tests for risk/accounting changes.
- Keep `cmd/antigravity/main.go` as wiring only where possible; domain logic belongs in `internal/*`.
- Do not hand-edit `engine/vendor/`.

## Repository Hygiene

- Do not add root-level audit reports or long-form generated docs.
- Put durable docs in `docs/`.
- Put operational scripts in `scripts/` or `client/scripts/`.
- Mark uncertain cleanup candidates in `docs/risk-report.md` rather than deleting them.

## AI Context Hygiene

- Start with `docs/repository-summary.md`, then `docs/module-index.md`.
- For BTC desk bugs, follow:
  `signals -> policy -> open position -> mark/exit -> paper math -> UI`.
- For engine bugs, follow:
  `marketdata -> strategy -> risk -> OMS/execution -> ledger -> reconciliation`.
- Do not scan `engine/vendor/`, `.venv/`, `node_modules/`, `.next/`, or Graphify cache unless the task specifically requires it.
