# Risk Report

## Requires Manual Verification

- Dependency removals in `client/package.json`.
- Any deletion under `client/docs/` or `engine/*reports*`.
- Any movement of BTC files.
- Any change to engine risk, OMS, ledger, reconciliation, persistence, or kill switch packages.
- Any cleanup of Graphify folders or generated graph files.
- `client/ecosystem.worker.config.cjs`: references a worker script that was not found in the current checkout; treat as protected worker/cron surface until replacement is traced.
- `.github/workflows/keep-alive.yml`: appears stale if it still targets Render; verify before deleting because workflows are deployment-adjacent.
- `.github/workflows/deploy-ecs.yml` and `.github/workflows/deploy-k8s.yml`: transition/future AWS paths; verify infra status before editing further.

## Potentially Used Files

Treat as used until proven otherwise:

- `client/scripts/*`
- `client/docs/*`
- `engine/docs/*`
- `engine/phase22e_reports/*`
- `engine/*.md`
- `scripts/*`
- `.github/*`
- `grafana/*`
- `nginx/*`
- `data/*`
- `.claude/settings*.json`
- `agent-config.json`
- `grafana/*`
- `nginx/*`

## Security Hygiene Items

- `LightsailDefaultKey-ap-south-1.pem` was reported by the ops mapper as present/tracked. Treat as a potential secret hygiene issue; rotate/remove only with explicit approval and deployment verification.
- Root log/build artifacts may be tracked historical files. Remove only if `git status` confirms they are not needed and the user approves.

## Potentially Used Dependencies

Review with clean build/test before removal:

- `@supabase/ssr`
- `@supabase/supabase-js`
- `exceljs`
- `https-proxy-agent`
- `undici`

## Potential Breaking Changes

- Removing docs that encode strategy evidence or operational runbooks.
- Removing scripts referenced by deployment or local workflows.
- Removing database columns from persistence code without migrations.
- Moving files imported by Next.js app routes.
- Reorganizing engine packages without updating Go imports.
- Removing compatibility redirect routes `/btc-future-trading`, `/paper-desk`, or `/paperdesk`.
- Removing local engine tools such as `engine/cmd/seed_db`, `engine/cmd/backtest`, or `engine/cmd/sep_evidence` without documenting replacements.

## Migration Risks

- Next.js app router paths are filesystem-driven.
- Engine wiring is centralized in `cmd/antigravity/main.go`; small changes can affect boot.
- Paper PnL depends on exact helper semantics.
- Worker/browser parity can break if one path is refactored alone.
- Graphify cache may be large but is intentionally protected.

## Rollback Plan

1. Use `git status --short` to inspect changed/deleted files.
2. Restore only the affected files from Git history.
3. Re-run targeted tests for changed modules.
4. For BTC behavior issues, prioritize rollback of:
   - `client/src/lib/trading/*`
   - `client/src/lib/analytics/futures*`
   - `engine/internal/trading`
   - `engine/internal/execution*`
   - `engine/internal/risk*`
   - `engine/internal/omsv3`
   - `engine/internal/ledger`
   - `engine/internal/reconciliation*`
5. Do not use destructive Git reset unless explicitly approved.

## Validation Checklist

- `client` TypeScript/build passes.
- BTC paper desk opens/closes positions correctly.
- Funding, fee, liquidation, and PnL tests pass.
- Engine builds and boots.
- `/health`, `/ready`, `/metrics` respond.
- Kill switch status/trigger/resume still works.
- Delta BTC paths still work when configured.
