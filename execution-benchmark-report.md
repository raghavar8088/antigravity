# Execution Benchmark Report

Generated: 2026-05-31

## Benchmark Suite

Implemented in `client/src/internal/execution/execution_v2_benchmark.test.ts`.

Current focused benchmark:

- Aggregates 1000 scored signals through `SignalAggregatorV2`.
- Verifies no throttling or sleeps are used.
- Enforces a sub-50 ms aggregation envelope for the synthetic 1000-signal batch.

## Current Result

Command:

```bash
cd client
npm run test -- src/internal/oms/oms_v2.test.ts src/internal/trading/aggregator_v2.test.ts src/internal/execution/pipeline_v2.test.ts src/internal/execution/execution_v2_benchmark.test.ts
```

Result:

- Test files: 4 passed.
- Tests: 7 passed.
- V2 benchmark: passed.

## Benchmark Coverage Added

- OMS state transition correctness.
- Aggregator deduplication.
- Aggregator conflict handling.
- Deterministic paper pipeline path from signal to filled OMS order.
- 1000 scored-signal aggregation throughput.

## Remaining Benchmarks To Add During Integration

- End-to-end signal-to-order latency using real BTC futures snapshots.
- 100-strategy, 500-strategy, and full-roster strategy evaluation timing.
- Exchange adapter submit/ack timing for paper and Delta testnet.
- Event bus backpressure and async worker queue latency.
- Memory and CPU profiling for multi-symbol scaling.
