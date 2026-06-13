# Token Optimization Report

## Baseline Metrics

Scan excluding `.git`, `.venv`, `vendor`, `node_modules`, and `.next`:

- Files: 2,351
- TypeScript/TSX: 662
- Go: 747
- Markdown: 42
- Bytes: ~38 MB

## AI Context Strategy

Future AI sessions should load:

1. `docs/repository-summary.md`
2. `docs/architecture-overview.md`
3. `docs/module-index.md`
4. `docs/dependency-map.md`
5. `.cursor/skills/trading-app-guide/SKILL.md`

Only then follow specific imports for the task.

## Expected Token Reduction

Estimated context reduction for common tasks:

- BTC futures bug: 70-85% fewer files scanned if starting from docs + protected file map.
- Engine issue: 60-75% fewer files scanned if starting from engine module index.
- API route issue: 50-70% fewer files scanned via API prefix index.
- Onboarding/new agent: 80%+ reduction versus scanning root reports/vendor/cache.

## Files To Exclude From Routine AI Context

- `engine/vendor/`
- `.venv/`
- `node_modules/`
- `.next/`
- build/log/cache outputs
- large report folders unless the task is specifically about historical audits
- Graphify cache internals unless graph debugging is requested

## Files To Prefer

- `docs/*.md`
- `.cursor/skills/trading-app-guide/SKILL.md`
- `client/package.json`
- `engine/go.mod`
- focused BTC files listed in `docs/module-index.md`

## Step-by-Step Implementation Summary

1. Protected BTC Futures Trading and Graphify.
2. Built repo/module/dependency/API documentation.
3. Added manual-review cleanup reports instead of deleting uncertain files.
4. Added validation, rollback, and risk guidance.
5. Confirmed active source scans no longer show old non-BTC market references in protected code paths.

## Before vs After Structure

Before this documentation pass, AI assistants had to infer architecture from many scattered files.

After:

```text
docs/
├── api-index.md
├── architecture-improvements.md
├── architecture-overview.md
├── cleanup-report.md
├── coding-standards.md
├── dependency-map.md
├── folder-structure.md
├── module-index.md
├── onboarding-guide.md
├── repository-summary.md
├── risk-report.md
└── token-optimization-report.md
```

## Testing Checklist

- Run `cd client && npm run test`.
- Run `cd client && npm run build`.
- Run `cd engine && go test ./...`.
- Run `cd engine && go build ./...`.
- Manually smoke test BTC paper desk, engine health, kill switch, and Delta BTC probes.
