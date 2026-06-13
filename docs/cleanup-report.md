# Cleanup Report

## Safety Policy Applied

- BTC Futures Trading: protected.
- Graphify code/folders/artifacts: protected.
- Uncertain files: not deleted; listed for manual review.

## High-Confidence Cleanup Already Performed

Earlier cleanup removed obsolete non-BTC market code and root-level report clutter. Active source scans now show no explicit `NIFTY`, `NSE`, `BSE`, `MCX`, or `AngelOne` references in:

- `client/src`
- `engine/internal`
- `engine/cmd`

## Files/Folders Removed In Earlier Cleanup

Categories removed:

- root audit/report `.md` files,
- old autonomous bot scripts,
- old Indian-market/MCX/AngelOne/NIFTY client routes/hooks/components,
- old NIFTY/AngelOne engine routes/handlers/helpers,
- non-BTC `brain/` and `infrastructure/` folders,
- temporary/cache/log folders outside protected Graphify paths.

## Files Added In This Pass

- `docs/architecture-overview.md`
- `docs/module-index.md`
- `docs/dependency-map.md`
- `docs/api-index.md`
- `docs/coding-standards.md`
- `docs/folder-structure.md`
- `docs/repository-summary.md`
- `docs/onboarding-guide.md`
- `docs/cleanup-report.md`
- `docs/architecture-improvements.md`
- `docs/risk-report.md`
- `docs/token-optimization-report.md`

## Protected From Cleanup

- all BTC Futures Trading source and tests,
- all engine order/risk/OMS/ledger/reconciliation/killswitch code,
- `engine/vendor/`,
- every `graphify-out/` folder,
- `.cursor/skills/trading-app-guide/*`,
- deployment files needed for client/engine operation.

## Manual Review Cleanup Candidates

Do not delete automatically:

- `client/docs/*`: several docs are BTC-related or may contain operational history.
- `engine/*.md`, `engine/phase22e_reports/*`, `engine/docs/*`: mostly generated reports, but may contain BTC strategy evidence.
- `engine/cmd/seed_db/main.go`: local seeding utility with hardcoded local DSN; quarantine/document before production use.
- `engine/cmd/backtest/main.go`: simple mock-history driver; not main runtime, but keep until replacement is confirmed.
- `engine/cmd/sep_evidence/*`: reporting/evidence tooling; not runtime, but may be useful.
- Older parallel engine layers such as `reconciliation` vs `reconciliationv2`, `riskv3` vs `risk`/`risk/v2`, and `mongopersist` vs `paperpersist`: do not delete without compile/import/persistence audit.
- `engine/internal/execution/binance_live.go`: has comments indicating not fully wired; audit before enabling or removing.
- `.github/workflows/keep-alive.yml`: likely stale if still targeting Render; verify before removal.
- `scripts/deploy.sh`, `scripts/build.bat`, `fix.bat`, `push.bat`: may reference old paths or automation; manual review required.
- `grafana/docker-compose*.yml`: verify missing `infrastructure/...` references before use.
- `client/package.json` candidates: `@supabase/*`, `exceljs`, `https-proxy-agent`, `undici`; usage must be verified with build/tests before removal.
- `.venv/`: local environment, not app source; user may delete locally if not needed.

## Estimated Reduction

Current scan excluding `.git`, `.venv`, `vendor`, `node_modules`, `.next`:

- 2,351 files
- ~38 MB
- 42 markdown files

Most token reduction now comes from using `docs/` as the AI entry point and excluding vendor/cache/local-env folders from AI context.
