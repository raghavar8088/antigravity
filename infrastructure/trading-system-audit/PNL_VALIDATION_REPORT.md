# PnL Validation Report

**Audit date:** 2026-06-09

---

## PnL Systems

| System | Location | Scope |
|--------|----------|-------|
| Go engine close PnL | `loop.go:1705–1710`, `execution/fees.go` | Paper BTC scalper |
| Go position gross | `positions/manager.go:261–266` | Unrealized at exit |
| Client paper math | `futuresPaperMath.ts` | Browser + worker |
| Delta bridge PnL | `live_bridge.go:168–172` | Options live trades |
| Portfolio ledger | `paperpersist/portfolio_ledger.go` | Realized aggregate |

---

## 1. Go Engine PnL

### Gross PnL (Position Close)

**File:** `positions/manager.go:261–266`

```go
// Long:  (exitPrice - entryPrice) * size
// Short: (entryPrice - exitPrice) * size
```

### Fees

**File:** `execution/fees.go:14–27`

```go
entryNotional = entryPrice * quantity
exitNotional  = exitPrice * quantity
entryFee = entryNotional * BinanceFuturesTakerFeePct  // 0.05%
exitFee  = exitNotional  * BinanceFuturesTakerFeePct
```

### Net PnL

**File:** `execution/fees.go:29–32`

```go
CanonicalNetPnL(grossPnL, fees) = grossPnL - fees.EntryFee - fees.ExitFee
```

**Close pipeline:** `processCloseEvents` (`loop.go:1695–1781`):
1. `calculatePnL` (gross)
2. `CanonicalTradeFees`
3. `CanonicalNetPnL`
4. `journal.RecordTrade`
5. `portfolioLedger.RecordClose`
6. `risk.RecordPnL`

| Component | Verdict | Evidence |
|-----------|---------|----------|
| Realized PnL | **PASS** | Formula + close pipeline |
| Unrealized PnL | **PARTIAL** | Computed at exit only, not ledger-backed between ticks |
| Fees | **PASS** | Round-trip taker model |
| Funding | **FAIL** | Not in Go BTC scalper close path |
| Commissions | **PASS** | Same as fees (taker %) |
| Slippage | **FAIL** | Not applied in Go paper execution |

---

## 2. Client Paper Desk PnL

### Gross PnL

**File:** `futuresPaperMath.ts:107–119`

```typescript
// LONG:  (markOrExit - entry) / entry * notional
// SHORT: (entry - markOrExit) / entry * notional
```

### Fees

**File:** `futuresPaperMath.ts` — `paperRoundTripTakerFees(notional, takerFeePct)`

### Funding

**File:** `futuresPaperMath.ts` — `applyFundingAccrual`

- Scales by `elapsedMs / fundingIntervalMs`
- Directional: longs pay positive funding, shorts receive

**Test:** `runPaperDeskPollTick.omsIntegration.test.ts:183` — "credits positive funding to shorts"

### Net PnL on Close

**File:** `futuresPaperMath.ts:483–491`

```typescript
netPnl = grossPnl - fees - fundingCosts
// minAbsNetWinUsd bump for small positive wins
```

### Slippage

**File:** `futuresPaperMath.ts:28–37` — `paperApplyEntrySlippage`, `paperApplyExitSlippage`

Vol-scaled via `paperDynamicSlippageBps`.

| Component | Verdict | Evidence |
|-----------|---------|----------|
| Realized PnL | **PASS** | `paperNetPnlOnClose` + tests |
| Unrealized PnL | **PASS** | `paperLinearGrossPnl` on mark |
| Fees | **PASS** | Round-trip taker |
| Funding | **PASS** | Calendar-accrual with interval scaling |
| Commissions | **PASS** | Included in fees |
| Slippage | **PASS** | Entry/exit bps adjustment |

### Worker vs Hook Consistency

**Worker** (`runPaperDeskPollTick.ts:581–583`): inline `gross - fees - fundingCosts`  
**Hook** (`useBTCFuturesScalperEngine.ts:2577`): `paperNetPnlOnClose` with `minAbsNetWinUsd`

**Verdict:** **PARTIAL** — formulas equivalent but hook has min-net-win bump worker lacks.

---

## 3. Delta Bridge PnL

**File:** `live_bridge.go:168–172`

```go
// Selling mode: RealizedPnl = (FillPrice - CloseFillPrice) * Contracts
// Buying mode:  RealizedPnl = (CloseFillPrice - FillPrice) * Contracts
```

| Component | Verdict | Evidence |
|-----------|---------|----------|
| Realized PnL | **PARTIAL** | Formula correct for options premium |
| Fees | **FAIL** | Not deducted in bridge PnL |
| Funding | **FAIL** | Not applicable to options but not modeled |
| Reconciliation to OMS | **FAIL** | Bridge `LiveTrade` separate from ledger |

---

## 4. Test Coverage

| Test File | Coverage |
|-----------|----------|
| `futuresPaperMath.test.ts` | Gross, fees, net, funding, liquidation, slippage |
| `futuresDeskPolicy.test.ts` | Gate math |
| `paperDeskWorker.test.ts` | Worker tick PnL |
| `engine/internal/risk/v2/engine_test.go` | Kelly sizing |

**Verdict:** **PASS** for client paper math test coverage.

---

## 5. Mark-to-Market

| Path | Method | Verdict |
|------|--------|---------|
| Go paper | `PaperClient.GetEquityUSD` uses `lastKnownPrice` | **PARTIAL** |
| Go positions | Tick price vs SL/TP each tick | **PASS** |
| Client | `markPrice` from Delta klines | **PASS** |
| Ledger-backed MTM | Not found | **FAIL** |

---

## Phase 9 Conclusion

| PnL Component | Go Paper | Client Paper | Delta Live |
|---------------|----------|--------------|------------|
| Realized | **PASS** | **PASS** | **PARTIAL** |
| Unrealized | **PARTIAL** | **PASS** | **FAIL** |
| Fees | **PASS** | **PASS** | **FAIL** |
| Funding | **FAIL** | **PASS** | **FAIL** |
| Slippage | **FAIL** | **PASS** | **FAIL** |

**Overall Phase 9:** **PASS** for client paper desk PnL (tested); **PARTIAL** for Go engine; **FAIL** for Delta live PnL completeness.
