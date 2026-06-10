# Forensic Audit Index — Mock Trading Outage 2026-06-10

All reports for Phase 1–17. Primary findings in **ROOT_CAUSE_ANALYSIS.md** and **FINAL_MOCK_TRADING_CERTIFICATION.md**.

| Phase | Report | Key Finding |
|-------|--------|-------------|
| 1 | [MOCK_EXECUTION_CALL_GRAPH.md](./MOCK_EXECUTION_CALL_GRAPH.md) | Full chain; kill switch blocks at pipeline.go:51 |
| 2 | [SIGNAL_GENERATION_AUDIT.md](./SIGNAL_GENERATION_AUDIT.md) | Strategies run; outage is post-signal |
| 3 | [MARKET_DATA_FORENSIC_REPORT.md](./MARKET_DATA_FORENSIC_REPORT.md) | Coinbase WS primary feed — not root cause |
| 4 | [OMS_FORENSIC_REPORT.md](./OMS_FORENSIC_REPORT.md) | OMS creates orders until kill switch |
| 5 | [EXECUTION_ENGINE_REPORT.md](./EXECUTION_ENGINE_REPORT.md) | Orchestrator alive; orders blocked at risk gate |
| 6 | [MOCK_BROKER_REPORT.md](./MOCK_BROKER_REPORT.md) | PaperClient functional when reached |
| 7 | [DATABASE_FORENSIC_REPORT.md](./DATABASE_FORENSIC_REPORT.md) | No corruption; kill events in Postgres ledger |
| 8 | [RISK_BLOCKING_REPORT.md](./RISK_BLOCKING_REPORT.md) | Kill switch = primary blocker |
| 9 | [KILLSWITCH_FORENSIC_REPORT.md](./KILLSWITCH_FORENSIC_REPORT.md) | OMS_DESYNC from recon false positive |
| 10 | [RECONCILIATION_FORENSIC_REPORT.md](./RECONCILIATION_FORENSIC_REPORT.md) | Equity + side mismatch bugs |
| 11 | [CONFIGURATION_AUDIT.md](./CONFIGURATION_AUDIT.md) | ENGINE_EXECUTION_AUTHORITY=1 expected |
| 12 | [EVENT_LOOP_FORENSIC_REPORT.md](./EVENT_LOOP_FORENSIC_REPORT.md) | Loops alive; not dead worker issue |
| 13 | [OUTAGE_TIMELINE_REPORT.md](./OUTAGE_TIMELINE_REPORT.md) | First failure ≤10s post-boot balance cycle |
| 14 | [ROOT_CAUSE_ANALYSIS.md](./ROOT_CAUSE_ANALYSIS.md) | Recon v2 false CRITICAL → kill switch |
| 15 | [FIX_IMPLEMENTATION_REPORT.md](./FIX_IMPLEMENTATION_REPORT.md) | 10 files patched |
| 16 | [END_TO_END_VALIDATION_REPORT.md](./END_TO_END_VALIDATION_REPORT.md) | Unit tests PASS |
| 17 | [SELF_HEALING_UPGRADE_REPORT.md](./SELF_HEALING_UPGRADE_REPORT.md) | Watchdog + auto-release |
| — | [FINAL_MOCK_TRADING_CERTIFICATION.md](./FINAL_MOCK_TRADING_CERTIFICATION.md) | Certification Q1–Q15 |
