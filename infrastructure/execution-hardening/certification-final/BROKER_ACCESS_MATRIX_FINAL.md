# BROKER_ACCESS_MATRIX_FINAL

| # | Path | Action | PMS | KillSwitch | RiskV2 | OMS | ETP | Verdict |
|---|------|--------|-----|------------|--------|-----|-----|---------|
| 1 | ProcessExecutionRequest → delta fillFn → SubmitOrder | Place | ✓ | ✓ | ✓ | ✓ | ✓ | **PASS** |
| 2 | WireDeltaBridge OnOpen → institutionalOpen | Place | ✓ | ✓ | ✓ | ✓ | ✓ | **PASS** |
| 3 | WireDeltaBridge OnClose → institutionalClose → SubmitReduceOnlyOrder | Close | ✓ | ✓ | ✓ | ✓ | ✓ | **PASS** |
| 4 | Bridge monitor → OnClose | Close | ✓ | ✓ | ✓ | ✓ | ✓ | **PASS** |
| 5 | ExecuteEmergencyFlatten → ExecuteSignal | Paper flatten | bypass sizing | bypass active-KS block | bypass sizing | ✓ | ✓ | **PASS*** |
| 6 | ProcessExecutionRequest venue=angelone | Place | — | — | — | — | Rejected | **N/A** (no adapter) |
| 7 | angelone/order, cancel-order, delta/mirror, spot, testnet/* POST | Any | — | — | — | — | 410 | **BLOCKED** |
| 8 | delta/testnet/cancel-order | Cancel | — | — | — | — | 410 | **BLOCKED** |
| 9 | handleAngelOneProxy allowlist (main.go:2329) | Market data only | — | — | — | — | N/A | **PASS** |
| 10 | delta-live/enable (main.go:1459) | Toggle flag only | — | — | — | — | No broker call | **PASS** |
| 11 | SOR / BinanceLive | Place | — | — | — | — | Not wired | **INACTIVE** |

\*Emergency flatten records OMS transitions and ledger events; intentionally bypasses PMS and active kill-switch pipeline block (kill switch triggers the flatten). Sizing floor skipped per design.

## Evidence — Delta close (fixed)

```253:261:engine/internal/trading/institutional_request.go
result, err := bridge.SubmitReduceOnlyOrder(c, trade.ProductID, closeSide, trade.Contracts)
// ... inside executeThroughInstitutionalPathWithFill fillFn
```

Caller: `Bridge.OnClose` → `institutionalClose` (live_bridge.go:346-358)  
Wired: `WireDeltaBridge` (main.go:902)

## Evidence — no direct OnClose broker call

Previous bypass at `live_bridge.go:421` **removed**. Grep confirms no `b.client.PlaceOrder` outside `SubmitOrder` / `SubmitReduceOnlyOrder`.
