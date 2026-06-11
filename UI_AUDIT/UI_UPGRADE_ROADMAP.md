# PHASE 19 — UI UPGRADE ROADMAP
## Forensic Audit | Trading Platform | 2026-06-11

---

## PRIORITIZATION CRITERIA
1. Safety impact (prevents operational mistakes / false confidence)
2. Risk reduction (closes gaps that could cause financial harm)
3. Operational value (enables tasks currently impossible)

---

## 30-DAY ROADMAP — SAFETY & WIRING

**Goal**: Eliminate false confidence and broken admin operations. Make current dashboards trustworthy.

### Week 1: Emergency Safety Fixes

| Task | File(s) | Outcome |
|------|---------|---------|
| Replace mock initial terminal snapshot with empty/disconnected state | `terminalSnapshot.ts` | Terminal no longer shows false data |
| Add disconnected banner to AppShell | `AppShell.tsx` | Trader immediately knows terminal is not live |
| Set ENGINE_ADMIN_SECRET in .env.local and Vercel env | `.env.local` | Admin operations unblocked |
| Add kill switch status polling (10s) | New `useKillSwitchStatus.ts` | KS state visible on dashboard |
| Wire KS status chip to DashboardHeader + TopBar | `DashboardHeader.tsx`, `AppShell.tsx` TopBar | Kill switch state always visible |

### Week 2: Data Integrity

| Task | File(s) | Outcome |
|------|---------|---------|
| Add last-updated timestamp to Paper Desk | `PaperDeskDashboard.tsx` | Staleness immediately visible |
| Wire strategy health names (not just counts) | health tab component | "2 critical" → specific strategy names |
| Add "connection state" badge to all primary pages | Layout components | Every page shows its data source status |
| Auto-open OMS tab if pending order count > 0 | `PaperDeskDashboard.tsx` | Pending orders not silently ignored |

### Week 3: Broker Status Basics

| Task | File(s) | Outcome |
|------|---------|---------|
| Build basic broker health hook (4 brokers) | New `useBrokerHealth.ts` | Feed status visible |
| Add broker status chips to status bar | New status bar component | Feed outages immediately visible |
| Add last-trade timestamp with "silence alert" | `PaperDeskDashboard.tsx` | No-trade condition surfaces |
| Add watchdog last-ping timestamp | Engine health endpoint | Engine crash detectable |

### Week 4: Alert Infrastructure v1

| Task | File(s) | Outcome |
|------|---------|---------|
| Build `AlertFeed` component from real sources | New component | Real alerts replace mock tape |
| Route: KS activation, strategy critical, connection loss → alerts | Alert aggregator | Critical events surface without navigation |
| Add browser Notification API for critical alerts | Alert component | Away-from-screen awareness |
| Remove dead code: `InstitutionalRiskCenter.tsx` | Delete or connect | No more dead components |

---

## 90-DAY ROADMAP — OPERATIONAL COMPLETENESS

**Goal**: Every operational gap from Phase 3 audit has at least a partial solution.

### Month 2: Reconciliation + OMS

| Task | Priority |
|------|---------|
| New Go engine endpoint: `GET /api/reconciliation/status` | HIGH |
| New Next.js page: `/reconciliation` with status, drift, event log | HIGH |
| OMS cancel button in PaperOmsPanel | HIGH |
| Order lifecycle timeline in OMS panel | MEDIUM |
| Slippage column in trades table | MEDIUM |

### Month 3: Terminal Live Wiring

| Task | Priority |
|------|---------|
| Composite live terminal data hook (replaces mock snapshot with real polling) | CRITICAL |
| Wire `/terminal/execution` to real positions (paper desk data) | CRITICAL |
| Wire `/terminal/risk` to real risk data from engine | CRITICAL |
| Wire `/terminal/analytics` to real equity / trade history | HIGH |
| Persistent system status bar on all pages | HIGH |
| Strategy supervision page v1 (view all 600+, health per strategy) | HIGH |

---

## 180-DAY ROADMAP — INSTITUTIONAL GRADE

**Goal**: Platform achieves VERDICT 1 or VERDICT 2 from Phase 20 criteria.

### Month 4–5: Advanced Capabilities

| Task | Priority |
|------|---------|
| System Command Center (new home route) | CRITICAL |
| Strategy Administration Panel (enable/disable per strategy) | HIGH |
| Multi-instrument portfolio aggregation | HIGH |
| Risk center wired to Go engine risk gate | CRITICAL |
| Go engine WebSocket stream (TerminalSnapshot deltas) | HIGH |

### Month 6: Hardening & Polish

| Task | Priority |
|------|---------|
| Mobile monitoring view (tablet-optimized) | MEDIUM |
| Alert history with acknowledge/dismiss | HIGH |
| Execution quality metrics dashboard | MEDIUM |
| Design system unification (single light/dark system) | MEDIUM |
| 72-hour autonomous operation stress test | CRITICAL |
| Accessibility audit and remediation | MEDIUM |

---

## NORTH STAR METRICS (measure progress)

| Metric | Current | 30d Target | 90d Target | 180d Target |
|--------|---------|-----------|-----------|------------|
| Pages showing live data | 2/11 | 4/11 | 8/11 | 11/11 |
| Critical op gaps closed | 0/5 | 3/5 | 5/5 | 5/5 |
| Alert types wired | 0/10 | 3/10 | 7/10 | 10/10 |
| Kill switch visible | NO | YES | YES | YES |
| Reconciliation visible | NO | NO | YES | YES |
| False data on home page | YES | NO | NO | NO |
| Strategy names in health | NO | YES | YES | YES |
| 72h autonomous safe | NO | PARTIAL | PARTIAL | YES |
