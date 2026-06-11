# OBJECTIVE 5 — SEP DATA PIPELINE

## Flow

```
sep_evidence CLI (engine/cmd/sep_evidence)
  → sep_reports/strategy_evidence.json (filesystem)
  → readSepStrategyEvidence() (client/src/lib/sep/sepPipeline.ts)
  → /api/sep/rankings (fallback: Mongo strategy_scores)
  → StrategyIntelligenceDashboard
```

## API Routes

- `GET /api/sep/rankings?view=rankings|top|bottom|retirement`
- `GET /api/sep/top?limit=20`
- `GET /api/sep/bottom?limit=20`
- `GET /api/sep/retirement-candidates`

## Configuration

- `SEP_REPORTS_DIR` — path to SEP output (default `../sep_reports`)
- Generate reports: `cd engine && go run ./cmd/sep_evidence --out ../sep_reports`

## Caching

- Filesystem read on each request (no stale cache when reports regenerated)
- Mongo fallback cached via existing strategy_scores refresh cadence
