# Observability Report

## Existing instrumentation

- `engine/internal/observability/metrics.go` — pipeline stage histograms
- `engine/internal/trading/loop.go` — `persistOMSTransition` MongoDB OMS audit
- `engine/internal/security/middleware.go` — audit log on every gated request
- `executiongateway/handler.go` — structured log on ACCEPT/REJECT

## Recommended additions (not yet implemented)

- Prometheus counter: `execution_requests_total{venue,status}`
- Trace span: `execution.request` → `risk.check` → `oms.submit` → `broker.place`

## Audit trail today

Ledger events: `EventOrderCreated`, `EventRiskBlocked`, `EventOrderFilled` in institutional path.
