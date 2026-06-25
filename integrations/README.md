# External Integrations & Research Workspace

Curated, production-grade open-source repositories pulled in as **shallow clones**
(`git clone --depth 1`) for evaluation, adaptation, and integration into the
Antigravity trading platform (Next.js client + Go engine).

> This folder is **git-ignored** (see root `.gitignore`). Nothing here is committed
> to the Antigravity repo. Treat each subfolder as an upstream you can read, vendor
> selectively, or run as a sidecar service. Do **not** copy code wholesale — respect
> each project's license (noted below) and our trading safety invariants
> (kill switch, risk-gate-before-execution, fee/funding/PnL math).

## Layout (organized by functionality)

| Folder | Repo | Lang | License | Role in Antigravity |
|--------|------|------|---------|---------------------|
| `01-exchange-connectivity/ccxt` | [ccxt/ccxt](https://github.com/ccxt/ccxt) | Go/Py/TS | MIT | Unified market-data + order API for 100+ venues. Has a **Go** port → drop behind `engine/internal/exchange` adapters; collapses bespoke Binance/Delta clients. |
| `02-fix-protocol/quickfix-go` | [quickfixgo/quickfix](https://github.com/quickfixgo/quickfix) | Go | Apache-1.1 | FIX 4.0–5.0SP2 engine for institutional broker/prime connectivity. Wire into `engine/internal/executiongateway` for low-latency live order routing. |
| `03-indicators-go/indicator` | [cinar/indicator](https://github.com/cinar/indicator) | Go | AGPL-3.0 | 80+ TA indicators + streaming (Go channels). Reference/validate `engine/internal/strategy` indicator math; AGPL → use as a separate process or reimplement, do not statically link into closed source. |
| `04-backtest-execution-engine/nautilus_trader` | [nautechsystems/nautilus_trader](https://github.com/nautechsystems/nautilus_trader) | Rust/Py | LGPL-3.0 | Deterministic, nanosecond, research-to-live-parity engine. Benchmark/validate `engine/internal/backtest` (v2/v3) and event-driven OMS design. |
| `05-quant-ml-research/qlib` | [microsoft/qlib](https://github.com/microsoft/qlib) | Python | MIT | End-to-end ML quant pipeline (alpha factors, model zoo, portfolio opt). Feeds `engine/internal/ml`, `alpha`, `pms` with researched signals. |
| `05-quant-ml-research/finrl-trading` | [AI4Finance-Foundation/FinRL-Trading](https://github.com/AI4Finance-Foundation/FinRL-Trading) | Python | Apache-2.0 | RL allocators + LLM sentiment under a weight-centric, deployment-consistent contract. Prototype RL sizing for `engine/internal/kelly` / `riskv3`. |
| `06-timeseries-db/questdb` | [questdb/questdb](https://github.com/questdb/questdb) | Java | Apache-2.0 | High-throughput tick/OHLC TSDB (ILP ingest, PGWire, ASOF/SAMPLE BY). **Run via Docker** as the market-data store behind `engine/internal/marketdata`; source here is for reference only. |
| `07-realtime-messaging/centrifuge` | [centrifugal/centrifuge](https://github.com/centrifugal/centrifuge) | Go | MIT | Scalable WebSocket/SSE pub-sub for dashboard fan-out. Replace per-client polling hooks in `client/src/hooks` with push from the Go engine. |

## Recommended but NOT cloned (evaluate next)
- [nats-io/nats-server](https://github.com/nats-io/nats-server) — lightweight message bus for the `eventstore` / event-driven OMS backbone.
- [ranaroussi/quantstats](https://github.com/ranaroussi/quantstats) — portfolio tearsheets/analytics for `validation` and reporting.
- [hummingbot/hummingbot](https://github.com/hummingbot/hummingbot) — battle-tested exchange connectors + market-making strategies.
- [open-telemetry/opentelemetry-go](https://github.com/open-telemetry/opentelemetry-go) — standardize `engine/internal/observability` + `tracing`.

## How to use
```bash
# Update a single upstream
cd integrations/05-quant-ml-research/qlib && git pull --depth 1

# Re-clone everything (from repo root)
pwsh integrations/sync.ps1
```
