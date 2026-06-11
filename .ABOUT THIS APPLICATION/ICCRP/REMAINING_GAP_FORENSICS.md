# REMAINING GAP FORENSICS

**Audit date:** 2026-06-11  
**Scope:** Verdict-1 readiness for Institutional Command Center (`/terminal/*`)

---

## Gap 1 — Per-Strategy Sharpe

### Source Evidence

| File | Lines | Finding |
|------|-------|---------|
| `client/src/lib/paperDeskClient.ts` | 151–163 | `StrategyScoreDoc` has no `sharpe` field |
| `client/src/lib/terminal/mapSnapshotToTerminalDelta.ts` | 114 | `sharpe: null` — no synthetic mapping |
| `client/src/components/terminal/institutional/ResearchCenter.tsx` | 46, 60–62 | Column header `Sharpe (N/A)`; cell renders `—` |

### Classification

| Dimension | Verdict |
|-----------|---------|
| Cosmetic? | **Yes** — column exists but honestly empty |
| Operational? | **No** — operator cannot act on fake Sharpe |
| Data integrity? | **No** — null is not coerced to 0 |
| Certification blocker? | **No** |

### Reachability

```
Mongo strategy_scores (no sharpe field)
  → listStrategyScores() [paperDeskClient.ts:485]
  → mapStrategies() sets sharpe:null [mapSnapshotToTerminalDelta.ts:114]
  → ResearchCenter renders "—" [ResearchCenter.tsx:60]
```

**Conclusion:** Non-material. Operator uses expectancy, PF, evidence_score for rankings — all Mongo-backed.

---

## Gap 2 — Legacy TerminalDashboard

### Source Evidence

| File | Lines | Finding |
|------|-------|---------|
| `client/src/components/TerminalDashboard.tsx` | 254–378 | Separate authority via `usePaperDesk`, not `terminalStore` |
| `client/src/components/TerminalDashboard.tsx` | 290, 284 | `winRate ?? 0`, `unrealizedPnl ?? 0` — zero fallbacks before auth |
| `client/src/components/TerminalDashboard.tsx` | 257 | `useMockCandleBuilder` — mock regime input |
| `client/src/app/page.tsx` | 1–5 | **Remediation:** redirect to `/terminal/execution` |

### Classification (Before Fix)

| Dimension | Verdict |
|-----------|---------|
| Cosmetic? | **No** |
| Operational? | **Yes** — home page showed metrics without ICC authority |
| Data integrity? | **Partial** — Mongo when connected, zeros when not |
| ICC blocker? | **Yes** — default landing bypassed institutional guard |

### Classification (After Fix)

Home route redirects to ICC. Legacy component remains in repo but **not reachable** as default operator path.

**Conclusion:** Blocker **closed** by redirect. Residual code is dead-path for certification scope.

---

## Gap 3 — Portfolio Dual-Fetch

### Source Evidence

| File | Lines | Finding |
|------|-------|---------|
| `client/src/components/PortfolioAnalyticsDashboard.tsx` | 96–106 | Fetches `/api/paper-desk/portfolio` |
| `client/src/app/api/paper-desk/portfolio/route.ts` | 17 | `getPortfolioAccountingSnapshot(accountKey)` |
| `client/src/app/api/paper-desk/snapshot/route.ts` | 43–51 | Same service + `getPortfolioExtendedMetrics()` |
| `client/src/components/terminal/institutional/TerminalLayoutClient.tsx` | 12 | Page only renders after `TerminalAuthorityGuard` |

### Classification

| Dimension | Verdict |
|-----------|---------|
| Cosmetic? | **Yes** — architectural duplication only |
| Operational? | **No** — same Mongo formulas |
| Data integrity? | **No** — not dual-authority, dual-transport |
| Stale state? | **Low** — polls every 10s vs store 3–5s |
| Certification blocker? | **No** |

### Authority Proof

Both paths call `getPortfolioAccountingSnapshot()` → `computeExtendedMetricsFromRecords()` for PF/Sharpe.

**Conclusion:** Non-material temporal skew only.

---

## Summary Table

| Gap | Material? | ICC Blocker? | Status |
|-----|-----------|--------------|--------|
| Per-strategy Sharpe | No | No | Accepted as N/A |
| Legacy dashboard | Was Yes | Was Yes | **Closed** (redirect) |
| Portfolio dual-fetch | No | No | Accepted |

---

## ROI to Fully Eliminate (Optional)

| Item | Effort | ROI |
|------|--------|-----|
| Add sharpe to Go strategy_scores | 2–3 days | Low — expectancy/PF sufficient |
| Delete TerminalDashboard.tsx | 1 hour | Medium — code hygiene |
| Wire portfolio page to terminalStore | 2 hours | Low — consistency only |

None required for Verdict 1.
