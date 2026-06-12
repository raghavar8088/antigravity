# PAGE BLUEPRINTS — ICC-NRP

## Shared Layout

All pages use `m3-page-stack`, `m3-kpi-strip`, `TerminalCard`, `Metric` — M3 token system via `m3-tokens.css`.

---

## Grade Pages (`/terminal/grade-1` … `/terminal/grade-5`)

**Component:** `GradeStageCenter.tsx`

### Pipeline Visualization
- `PromotionTower` with MongoDB tower counts (`GradeStageCenter.tsx:155–167`)
- Highlights current stage via `selectedStatus={status}`

### KPI Strip (7 metrics)
| KPI | Source |
|-----|--------|
| Total Strategies | `summary.totalStrategies` |
| Promotion Candidates | `summary.promotionCandidates` |
| Demotion Candidates | `summary.demotionCandidates` |
| Average PF | `summary.avgProfitFactor` |
| Average Sharpe | `summary.avgSharpe` |
| Average Expectancy | `summary.avgExpectancy` |
| Average Drawdown | `summary.avgDrawdown` |

API: `GET /api/strategy-authority/stage?status=GRADE_N` → `getStrategiesByStatus()`

Unavailable: displays `—` (`GradeStageCenter.tsx:168`, `214`)

### Strategy Table Columns
| Column | Field |
|--------|-------|
| Strategy ID | `strategy_id` |
| Strategy Name | `strategy_name` |
| Family | `family` |
| Grade | `current_status` → G5…G1 |
| Win Rate | `metrics.winRate` |
| Profit Factor | `metrics.profitFactor` |
| Expectancy | `metrics.expectancy` |
| Sharpe | `metrics.sharpeRatio` |
| Drawdown | `metrics.maxDrawdown` |
| Promotion Progress | `PromotionProgressCell` — bars for Trades, WR, PF, Sharpe, DD |
| Current Status | ↑ PROMOTE / ↓ DEMOTE / RETIRE / ACTIVE |

### Promotion Progress Visualization
`PromotionProgressCell` (`GradeStageCenter.tsx:44–99`):
- Overall progress bar (0–100%)
- Per-gate mini progress bars (not plain numbers only)

---

## Mock Trading Engine Page (`/terminal/mock-engine`)

**Component:** `MockEngineCenter.tsx`

### Sections
| Section | Component | Data API |
|---------|-----------|----------|
| Institutional Pipeline | `PromotionTower` | `/api/strategy-authority/counts` |
| Engine Population KPI | inline | `/api/strategy-authority/stage?status=MAIN_ENGINE` |
| Family Distribution | `FamilyLeaderboard` | `/api/strategy-authority/families` |
| Regime Distribution | `RegimeIntelligence` | regime API |
| Portfolio Allocation | `AllocationView` | `/api/strategy-authority/allocation` |
| Correlation Heatmap | `CorrelationMatrix` | `/api/strategy-authority/correlation` |
| Main Engine Survivors | `MainEngineSurvivors` | `/api/strategy-authority/main-engine` |
| Engine Roster — Ranked | inline table | stage + allocation |

### Ranking Sort Keys
Authority Score (default), PF, Sharpe, Drawdown, Allocation Weight — `MockEngineCenter.tsx:163–178`

---

## Data Flow (all pages)

```
MongoDB (strategy_authority_profiles + mock_trades)
  → strategyAuthorityMongo.ts
  → /api/strategy-authority/*
  → React fetch in page components
  → UI (no terminal store for ISPAP — direct API authority)
```
