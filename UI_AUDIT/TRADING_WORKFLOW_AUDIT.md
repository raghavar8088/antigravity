# PHASE 10 — TRADING WORKFLOW AUDIT
## Forensic Audit | Trading Platform | 2026-06-11

---

## WORKFLOW TRACE: Trader → Dashboard → Signal → Risk → OMS → Execution → Portfolio → PnL

### Step 1: Trader Opens Dashboard
**Route**: `/` or `/terminal/execution`
**What happens**: TerminalDashboard renders with mock data. Trader sees BTC price, positions, alerts that are all hardcoded.
**Workflow gap**: Trader immediately has incorrect operational context.

### Step 2: Trader Looks for Signal Activity
**Available**: 
- BTC Futures page (`/btc-future-trading`): Signal trace panel with funnel metrics
- Paper Desk: recent trades list (real)
**Missing**: 
- No live signal ticker showing signals as they generate
- No "signals in last 5 minutes" counter
- No notification when a new signal passes all gates
**Gap severity**: HIGH — trader is passive, not informed

### Step 3: Trader Checks Risk Before Trading
**Available**:
- Terminal Risk Module: VaR, heat, drawdown (MOCK DATA)
- Paper Desk summary: drawdown (live)
**Missing**:
- Live VaR
- Live heat level vs threshold
- Current regime vs strategy eligibility matrix
**Gap severity**: CRITICAL — trader cannot make risk-informed decisions from the primary risk view

### Step 4: Trader Reviews OMS State
**Available**:
- Paper Desk OMS tab: paper orders (view-only, lazy-loaded)
**Missing**:
- Live broker OMS state
- Order modification controls
- Order prioritization
**Gap severity**: HIGH

### Step 5: Execution Occurs (Autonomous)
**What happens**: Go engine submits order to broker
**Trader visibility**: NONE while order is in-flight
**Trader learns about fill**: Only when next paper desk poll (up to 5s later) shows new trade in recent trades list
**Gap severity**: HIGH — no real-time fill notification

### Step 6: Trader Checks Portfolio Impact
**Available**: Paper Desk shows updated balance, equity, PnL after fill
**Missing**: 
- Immediate fill notification
- Position sizing impact display
- Portfolio heat recalculation display
**Gap severity**: MEDIUM

### Step 7: Trader Monitors PnL
**Available**: Paper Desk summary KPIs, DailyPnlLedger component
**Missing**: 
- Live PnL ticker per position
- Strategy-level PnL breakdown in primary view
**Gap severity**: MEDIUM

---

## MISSING VISIBILITY FINDINGS

| Workflow Step | Missing Capability |
|--------------|-------------------|
| Signal monitoring | Live signal feed; no alert on signal |
| Risk check | Live VaR/heat (only mock available) |
| OMS review | Order modification; live broker orders |
| Execution monitoring | Real-time fill notification |
| Portfolio impact | Immediate position update after fill |
| Alert triage | No alert system for autonomous events |

---

## EXCESS COMPLEXITY FINDINGS

1. **Two parallel signal engines**: In-browser engine (hooks) and Go engine (backend) exist simultaneously. A trader cannot easily distinguish which signals came from which engine.

2. **Two OMS layers**: Paper OMS (v3 in Go engine) and in-browser OMS v2 (`lib/internal/oms/oms_v2.ts`). No UI clarifies which layer a particular order went through.

3. **Scattered state**: Position data lives in MongoDB (paper desk), in-browser state (scalper engines), and Go engine SQLite. No unified view.

4. **Navigation fragmentation**: Core trading activities span at least 4 routes (`/`, `/terminal/execution`, `/paper-desk`, `/btc-future-trading`). A trader supervising live operations must visit multiple pages.

---

## CONFUSING WORKFLOW FINDINGS

1. **The "Kill" button in DashboardHeader** calls `resolveEngineApiUrl()` directly (not via `/api/engine/*` proxy). This means it may hit `http://localhost:8080` in development and the Lightsail IP in production — but there is no indicator of which engine it's targeting.

2. **`actionsEnabled` prop in DashboardHeader**: The kill button and reset button are only active if `actionsEnabled` is passed as `true`. The component renders disabled buttons otherwise. A trader seeing a disabled Kill button has no explanation why it doesn't work.

3. **Terminal shows "connected: true" on initial mock load** (from `terminalSnapshot.ts` line 18: `connected: true`). This is a lie — the WebSocket is not connected.

4. **Paper Desk lazy-loading**: The OMS orders tab is not loaded until clicked. A trader who never clicks the OMS tab has never seen the OMS state. There is no automatic alert if OMS orders pile up in a pending state.

---

## WORKFLOW AUDIT SCORECARD

| Workflow Aspect | Score |
|----------------|-------|
| Signal visibility | 3/10 |
| Risk checkpoint | 1/10 |
| OMS workflow | 2/10 |
| Execution feedback | 2/10 |
| Portfolio tracking | 5/10 |
| PnL monitoring | 5/10 |
| Alert integration | 1/10 |
| Navigation efficiency | 3/10 |

**Overall Workflow Score: 2.75/10**

A trader using this system for autonomous supervision must navigate 4+ pages, accept mock data as real in the primary terminal, and has no real-time notification system. The workflow has critical gaps at the signal, risk, and execution phases.
