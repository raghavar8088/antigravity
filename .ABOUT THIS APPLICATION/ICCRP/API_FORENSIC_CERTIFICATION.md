# API FORENSIC CERTIFICATION — ICCF-LDAP Phase 5

---

## Summary Table

| API | Auth | Error Handling | Authority Source | Freshness | Verdict |
|-----|------|----------------|------------------|-----------|---------|
| `/api/sep/rankings` | JWT session | mongoUnconfigured/Unavailable | SEP filesystem → Mongo fallback | File mtime / Mongo computed_at | PASS |
| `/api/sep/top` | Delegates to rankings | Same | Same | Same | PASS |
| `/api/sep/bottom` | Delegates to rankings | Same | Same | Same | PASS |
| `/api/sep/retirement-candidates` | Delegates to rankings | Same | Same | Same | PASS |
| `/api/paper-desk/risk-metrics` | JWT | mongo errors | Mongo `paper_trades` (5000 limit) | Query-time | PASS |
| `/api/paper-trades/correlation-matrix` | JWT | mongo errors | Mongo trades 30d window | Query-time | PASS |
| `/api/paper-trades/regime-analysis` | JWT | mongo errors | Mongo trades + regime_at_entry | Query-time | PASS |
| `/api/engine/reconciliation` | JWT | Partial — engine optional | Mongo state freshness + Go `/api/reconciliation/status` | State snapped_at + engine events | PASS with gaps |
| `/api/engine/events` | JWT | SSE error frames | `buildPlatformEvents` | 3s SSE loop | PASS — **not wired to terminal UI** |

---

## Detailed Forensics

### `/api/sep/rankings` — `sep/rankings/route.ts`

- **Auth:** `getAuthenticatedApiSession()` L36-37
- **Primary authority:** `readSepStrategyEvidence()` → `SEP_REPORTS_DIR/strategy_evidence.json` (`sepPipeline.ts:22-45`)
- **Fallback:** Mongo `listStrategyScores` + health L58-63
- **Failure:** `mongoUnconfigured()` / `mongoUnavailable()` L55,107
- **Filesystem status:** No `sep_reports/` in repo — production likely on Mongo fallback
- **Gap:** SEP filesystem takes precedence without freshness/staleness check

### `/api/sep/top|bottom|retirement-candidates`

Thin wrappers setting `view` param (`top/route.ts:6-9`). Same authority chain.

### `/api/paper-desk/risk-metrics` — `risk-metrics/route.ts`

- **Auth:** L11-12
- **Source:** Mongo trades projection L17-22
- **Compute:** `computePortfolioRiskMetrics(trades, PAPER_DESK_STARTING_BALANCE)` L30
- **Failure:** try/catch → `mongoUnavailable` L32-33

### `/api/paper-trades/correlation-matrix`

- 30-day cutoff L14
- Pearson matrix from closed trade daily PnL L29
- Empty trades → empty matrix (UI shows NO DATA)

### `/api/paper-trades/regime-analysis`

- Regime from `regime_at_entry` or payload L28-30
- `aggregateByRegime(trades)` L34

### `/api/engine/reconciliation`

- Mongo state staleness >120s → AMBER L29-31
- Engine proxy optional L53-74 (failure swallowed)
- **Gap:** No auth on engine fetch (server-side only — OK)

### `/api/engine/events`

- SSE stream L16-33
- Polls `buildPlatformEvents` every 3s
- **UI consumption:** **NONE** — grep shows zero client imports
- Terminal uses `/api/event-center` (JSON poll) instead

### Related (terminal-critical, not in original list)

| API | Used By | Verdict |
|-----|---------|---------|
| `/api/paper-desk/snapshot` | `terminalStore` | PASS — aggregated Mongo authority |
| `/api/strategy-intelligence` | Research, Strategies | PASS — `profit_factor: null` in portfolio_stats L123 |
| `/api/event-center` | EventCenter | PASS |
| `/api/risk-ribbon` | Global layout | PASS — mislabeled TODAY PnL L103 |
| `/api/btc/price` | Snapshot mapper | PASS |

---

## Authentication Proof

All audited routes call `getAuthenticatedApiSession()` first. Unauthenticated → redirect/401 via auth helper.

`/terminal/*` pages protected by middleware JWT (`middleware.ts:101-104, 167-172`).

---

## Phase 5 Verdict

**PASS with gaps.** APIs are authenticated and Mongo/engine-backed. Gaps: SEP staleness, SSE unused, strategy-intelligence null PF, risk-ribbon mislabeled PnL.
