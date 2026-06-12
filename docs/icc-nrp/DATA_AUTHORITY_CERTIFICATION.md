# DATA AUTHORITY CERTIFICATION — ICC-NRP

## Authority Chain

```
MongoDB Atlas
  ├── strategy_authority_profiles  (ISPAP_PROFILES_COLLECTION)
  └── mock_trades                  (MOCK_TRADES_COLLECTION)
        ↓
strategyAuthorityMongo.ts
        ↓
/api/strategy-authority/*
        ↓
UI Components (GradeStageCenter, MockEngineCenter, M3AppShell)
```

## MongoDB Collections

| Collection | Constant | File:Line |
|------------|----------|-----------|
| `strategy_authority_profiles` | `ISPAP_PROFILES_COLLECTION` | `strategyAuthorityMongo.ts:26` |
| `mock_trades` | `MOCK_TRADES_COLLECTION` | `strategyAuthorityMongo.ts:29` |

## Count Authority (Sidebar)

| Step | Evidence |
|------|----------|
| Query | `getPipelineCounts()` aggregates `current_status` from profiles (`strategyAuthorityMongo.ts:706–714`) |
| API | `counts/route.ts:12` — returns `{ counts: { byStatus } }` |
| Guard | Returns 503 `MONGO_NOT_CONFIGURED` if no URI (`counts/route.ts:8–9`) |
| UI | `usePipelineCounts` — no synthetic fallback (`usePipelineCounts.ts:34–38`) |
| Display | `—` when `hasAuthority === false` (`formatNavCount:48`) |

**Certification:** Sidebar counts are MongoDB-backed. **PASS**

## Stage Strategy Authority (Grade Pages)

| Step | Evidence |
|------|----------|
| Query | `getStrategiesByStatus(status)` — full profile list + `computeMetricsForStrategy` from `mock_trades` (`strategyAuthorityMongo.ts:548–580`) |
| API | `stage/route.ts:27` |
| KPIs | Computed from real metrics arrays, not hardcoded (`strategyAuthorityMongo.ts:571–579`) |
| Empty/unavailable | UI shows `—` (`GradeStageCenter.tsx:214`) |

**Certification:** Grade page metrics are MongoDB-backed. **PASS**

## Mock Engine Page Authority

| Widget | API | Mongo Function |
|--------|-----|----------------|
| Population | `/api/strategy-authority/stage?status=MAIN_ENGINE` | `getStrategiesByStatus` |
| Family Distribution | `/api/strategy-authority/families` | `getFamilyIntelligence` |
| Allocation | `/api/strategy-authority/allocation` | `allocationEngine` |
| Correlation | `/api/strategy-authority/correlation` | `correlationEngine` |
| Survivors | `/api/strategy-authority/main-engine` | `getMainEngineStrategies` |

All existing portfolio intelligence APIs read from MongoDB via `portfolioIntelligenceMongo.ts` / `strategyAuthorityMongo.ts`.

**Certification:** Engine page metrics are MongoDB-backed. **PASS**

## Prohibited Patterns — Not Found

| Pattern | Status |
|---------|--------|
| Hardcoded sidebar counts | **Absent** — only `formatNavCount` from API |
| Placeholder metrics on grade pages | **Absent** — `—` on failure |
| Mock data arrays in new components | **Absent** |
| Terminal store synthetic fallback for ISPAP | **N/A** — ISPAP uses direct API fetch |

## Minor Gap

`PromotionTower` default `totalStrategies = 305` prop (`PromotionTower.tsx:200`) is a display denominator fallback when `totalStrategies` prop not passed. Grade/engine pages do not pass explicit total — bar percentages use catalog constant. **Does not affect count badges or KPI values.**
