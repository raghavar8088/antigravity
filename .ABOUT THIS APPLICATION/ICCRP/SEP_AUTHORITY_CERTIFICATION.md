# SEP AUTHORITY CERTIFICATION — ICCF-LDAP Phase 6

---

## Pipeline Components

| Component | File | Lines |
|-----------|------|-------|
| `SEP_REPORTS_DIR` | `sepPipeline.ts` | L20 |
| Filesystem read | `readSepStrategyEvidence()` | L22-45 |
| Availability check | `sepReportsAvailable()` | L48-50 |
| API ingestion | `sep/rankings/route.ts` | L43-53 (filesystem first) |

---

## Authority Flow

```
SEP_REPORTS_DIR/strategy_evidence.json
  → readSepStrategyEvidence()
  → /api/sep/rankings (source: "sep_filesystem")
  → [NOT WIRED TO TERMINAL UI DIRECTLY]

Fallback (no file):
  Mongo strategy_scores + strategy_health
  → source: "mongo_strategy_intelligence"
  → /api/strategy-intelligence (terminal Research + Strategies pages)
```

---

## Filesystem Verification

```bash
Glob: **/sep_reports/** → 0 files in repository
Default path: path.join(process.cwd(), "..", "sep_reports")  # sepPipeline.ts:20
```

**Runtime proof required in prod:** Check `SEP_REPORTS_DIR` env on Vercel/Lightsail.

---

## Metric Authenticity

| Metric | SEP File Field | Mongo Fallback | Terminal Display |
|--------|---------------|----------------|------------------|
| PF | `ProfitFactor` / `profit_factor` L36 | `s.profit_factor` from scores | Strategies page — PASS |
| Expectancy | `Expectancy` L37 | `s.expectancy` | PASS |
| Sharpe | `SharpeRatio` L38 | **null** in rankings fallback L80 | Research shows **derived** pseudo-Sharpe L108 mapSnapshot |
| Evidence | `EvidenceScore` L41 | Computed in strategy-intelligence L67-71 | PASS |

---

## Synthetic SEP Values

No hardcoded SEP rows found in `sepPipeline.ts` — reads JSON only.

**Risk:** Stale SEP file takes precedence over live Mongo with no TTL (`rankings/route.ts:44-52`).

---

## Terminal UI Connection

**FAIL — indirect only:**
- `/terminal/research` uses `/api/strategy-intelligence`, **not** `/api/sep/*`
- `/terminal/strategies` uses `/api/strategy-intelligence`
- SEP APIs exist but **no terminal page calls them**

---

## Phase 6 Verdict

**FAIL for end-to-end SEP certification.** Pipeline code is sound; filesystem absent in repo; terminal UI does not consume SEP routes; Research page displays non-Sharpe as Sharpe.

---

## Remediation

1. **P0** — Wire Research/Strategies to `/api/sep/rankings` when `sep_available`, else Mongo.
2. **P0** — Add SEP file `computed_at` + reject if older than 24h.
3. **P1** — Display `source` badge in UI (`sep_filesystem` vs `mongo`).
4. **P1** — Use real `sharpe_ratio` from SEP/Mongo; remove `evidence_score/50`.
