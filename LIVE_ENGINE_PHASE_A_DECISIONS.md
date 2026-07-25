# Live Engine — Phase A written decisions (required before implementation)

Branch: `feat/live-engine-delta-options`. No orders can be placed until Phase A is complete.

---

## A.1 — Which Delta adapter the mandated path terminates in

### Decision: **(ii) — delete `engine/internal/execution/delta_adapter.go`.** Do not rewrite it.

### Why (grounded in a full trace of the live path, not the spec's assumption)

The spec assumed the mandated order path terminates in `execution.DeltaAdapter`. It does not. Trace:

```
POST /api/execution/request
  → executiongateway.Handler.ServeHTTP            (handler.go)
  → orchestrator.ProcessExecutionRequest          (the gateway; risk gate + OMS live here)
  → delta bridge institutional open/close handler (trading/institutional_request.go:150,214)
  → o.executeThroughInstitutionalPathWithFill(...) (risk gate + OMS v3 run here)
  → fillFn: bridge.Client().FindOptionProduct(...) → bridge.SubmitOrder(...)
  → delta.Client.PlaceOrder(...)                   (REAL hmac.New(sha256.New,...) signing)
```

So the **working option path already terminates in `delta.Client`** — real HMAC signing, real option product resolution via `FindOptionProduct` against `/v2/products?contract_types=put_options,call_options`, real fills. It reaches Delta through `executeThroughInstitutionalPathWithFill`, i.e. **after** the institutional risk gate + OMS, which is exactly the mandated architecture.

Meanwhile `execution.DeltaAdapter`:
- has a **fabricated signature** — `deltaSign` discards the secret (`_ = secret`) and returns `"hmac_<method>_<ts>"`;
- is **perp-only and cannot address an option** — `deltaProductID` is a static symbol→int map (BTCUSD→139) and returns 0 for everything else; option products are per-strike/per-expiry IDs a static map cannot express;
- has **hollow getters** — `GetPosition`/`GetBalance` discard the body; `GetOrder` returns `"UNKNOWN"`;
- has **zero call sites** — verified: `grep -rn "NewDeltaAdapter\|DeltaAdapter\b\|deltaSign\|deltaProductID\|deltaOrderType" engine/ --include=*.go` matches only the file itself. `executiongateway` never references it.

### Why not (i) rewrite it
Rewriting the stub to real HMAC + option resolution would **duplicate** what `delta.Client` already does correctly, creating two independent Delta signing/parsing code paths that can drift — precisely the "two adapters where one is a trap" the spec forbids. There is no caller to justify a second implementation: the OMS-terminating path is already `delta.Client`. Rewriting it would add a real-money code path with no consumer, which is worse than deleting it.

### Consequence / what I will do
- Delete `engine/internal/execution/delta_adapter.go` in full (removes the fabricated `deltaSign` from the tree, per the explicit instruction "do not leave `deltaSign` returning a fabricated signature").
- No production wiring changes needed — nothing imports it.
- `delta.Client` remains the single place that signs and talks to Delta for order placement. `reconciliationv2.DeltaReconciliationAdapter` remains the (separate, real, HMAC-signed) read path for reconciliation — see A.2; it is intentionally not merged here because its job (account-wide position/fill/funding sweep for drift detection) differs from order placement, and it is already real and tested.

---

## A.2 — What reconciliation actually reads (the dangerous silent-false-green)

### Finding: reconciliation does **not** touch the execution stub. It uses `reconciliationv2.DeltaReconciliationAdapter`, which is real.

- Restart recovery calls `reconciliationv2.ReconcileOnRestart(...)` (main.go:911, :1007) and `reconciliationv2.WireProduction(...)` (main.go:1452).
- That adapter signs with **real** `hmac.New(sha256.New, ...)` (delta_reconciliation.go:5-6,424-426) and its `GetPositions` (:169) hits `/v2/positions/margined` and parses `product_symbol/side/size/entry_price/mark_price/margin` via a `numString` type that tolerates Delta's string-encoded numbers. Covered by `TestGetPositions_UsesMarginedEndpointAndParsesStringFields`.
- `execution.DeltaAdapter.GetPosition` (the hollow one) is on **no** reconciliation path.

### The gap the spec is right about: "a control that cannot fail is not a control."
Today reconciliation reads real data, but there is **no test that fails if a future change makes reconciliation report success against an empty or unparsed broker response**. `GetPositions` silently returns an empty slice on a body it cannot parse into the expected shape (any parse mismatch → zero `result` entries → empty positions, no error). I will add a guard test that:
1. feeds the adapter a non-empty margined-endpoint body and asserts positions are actually parsed (fails if it silently returns empty), and
2. asserts an unparseable/garbage body is treated as an error, not as "zero positions = reconciled".

This makes the false-green path testable and locks it.

---

## A.3 — Direct-order enforcement

### Findings (current tree already partially hardened; spec line numbers are stale)
- `PlaceManualOrder` is **already disabled** — live_bridge.go:575 returns `"direct PlaceManualOrder disabled — use POST /api/execution/request with venue=delta"`. Good; keep it.
- The kill switch is wired into the bridge (`SetKillCheck`, institutional_request.go:144) and checked on the institutional path.
- **Still a gap:** `SubmitOrder`/`SubmitReduceOnlyOrder` are exported and documented "callable only from institutional fill callbacks" — a comment, not enforcement. Nothing structurally prevents a future caller in-package (or a new call site) from invoking them outside the risk-gate/kill-switch path.

### What I will do
Make it structural rather than documentary: gate `SubmitOrder`/`SubmitReduceOnlyOrder` behind a private effector guarded by a per-call token that only the institutional fill path holds (so an order cannot reach Delta without having passed through `executeThroughInstitutionalPathWithFill`, which runs the risk gate and kill-switch check), and add a test proving a direct call without that provenance is rejected before any HTTP call. Confirm the retired Next.js routes still return `blockedDirectExecutionRoute()`.

---

**Status:** A.1 decided and implementing now (delete stub). A.2/A.3 implementations follow with the tests named above. No control weakened; nothing armed; branch only.
