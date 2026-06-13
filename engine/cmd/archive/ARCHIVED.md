# Archived Engine Binaries

These CLI binaries are retired from active development. They were used during the institutional validation and evidence-gathering phases (Phase 22–29) of the BTC-PILOT strategy certification programme. The validation conclusions they produced are preserved in `engine/docs/` and `data/audit/`.

They are kept for historical reference only. **Do not wire them into production builds or CI.**

| Directory | Purpose | Retired |
|-----------|---------|---------|
| `phase22e/` | Phase 22E Profitability Validation — ledger-backed certification reports | 2026-06 |
| `phase22f/` | Phase 22F Institutional 1000-Trade Validation & Edge Verification | 2026-06 |
| `phase23a/` | Phase 23A Massive Historical Validation & Institutional Edge Certification | 2026-06 |
| `phase23b/` | Phase 23B+23C Institutional Validation Pipeline — real Binance data, real strategy replay | 2026-06 |
| `phase24/`  | Phase 24 Institutional Real Edge Certification — definitive multi-timeframe certification | 2026-06 |
| `phase25/`  | Phase 25 Live Edge Verification & Capital Deployment — live-forward simulation vs Phase 24 baseline | 2026-06 |
| `phase29/`  | Phase 29 Institutional Evidence Extraction & Final Deployment Decision — full Phase 24–28 synthesis | 2026-06 |

## Why archived?

The WINNERS_ONLY gate is now enforced in `engine/internal/strategy/curated_registry.go`. Strategies that passed phases 22–29 are already in the live registry; strategies that failed are excluded. Re-running these binaries would not change the registry and is not required for ongoing operations.
