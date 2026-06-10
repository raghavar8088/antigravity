# PHASE 11 — OVERFITTING DETECTION

**Date:** 2026-06-10

---

## Overfitting Sources in This Platform

### Source 1: Parameter-Grid Expansion Pack (CRITICAL)

**Evidence:** `engine/internal/strategy/curated_expansion_pack.go`

The expansion pack generates 301 strategies by iterating over explicit parameter arrays:
```go
for _, fast := range []int{2, 3, 4, 5, 6} {
    for idx, slow := range []int{8, 10, 12, 15, 18, 21, 26, 34} {
```

This is the textbook definition of **in-sample parameter optimization**. The parameter values (fast: 2-6, slow: 8-34) were chosen by a human who already observed price data. There is no OOS validation step. Every strategy in this pack is an artifact of fitting to historical BTC behavior.

**Overfit Score: 10/10 — Maximum overfit.** Complete removal is the only correct action.

---

### Source 2: Synthetic Certification (CRITICAL)

**Evidence:** `engine/phase22e_reports/LIVE_VS_BACKTEST_CERTIFICATION.md`

The Phase 22E certification generated 1,250 synthetic trades using `syntheticTrades()` — a deterministic function, not market data — and certified 7 strategies as "approved" based on the synthetic metrics.

When these 7 strategies were compared to live performance:
- Live PF for all 7 was 0.0
- Degradation: -100% (stated in the certification document as "PASS")

**A system that declares -100% live degradation a PASS is not a validation system. It is a certification theater.** The Phase 22E reports are actively misleading.

**Overfit Score: Not applicable (there is no fit at all — the synthetic data was pre-set to produce profitable results).**

---

### Source 3: Single-Period Backtests

The client replay ran on **one 500-bar fixture** from November 2023. This represents approximately 8.3 hours of BTC trading. The sample:
- May have been selected because it was a good trading period
- Is too small for statistical validity (113 trades)
- Cannot be extrapolated to annual performance
- Shows anomalously high profit factor (likely ~15:1) suggesting favorable regime sample

**Overfit Score: 4/10** — Not intentional overfit but single-sample bias is equivalent in effect.

---

### Source 4: Elite V2/V3 Parameter Selection

Elite V2 and V3 were designed with specific parameter values (EMA pairs, RSI periods, etc.). Without OOS validation proof, all parameter choices represent potential in-sample optimization.

The individual strategy parameters (e.g., EMA_5_13_Cross, RSI_Oversold30) were likely chosen based on perceived general wisdom or historical observation — which is a form of soft overfitting.

**Overfit Score: 5/10** — Soft overfit; may have some out-of-sample robustness if parameters are standard enough to be regime-independent.

---

### Source 5: Strategy Registry Curation Bias

The 11 strategies removed from the live registry were removed after proving losers in production. This is correct. However, the 606 remaining strategies have NOT been validated — they were simply never removed. The registry has **survivorship bias** in the wrong direction: strategies survive by default unless proven losers, rather than being required to prove winners.

**Overfit Score: 3/10** — Selection bias but not parametric overfit.

---

## Overfit Score Summary

| Strategy Group | Count | Overfit Score | Action |
|:--------------|------:|:-------------:|:-------|
| Expansion pack (XP_*) | 301 | 10/10 | Remove immediately |
| Phase 22E certified synthetic | 7 | 10/10 | Revoke certification |
| Elite V2/V3 without OOS data | 200 | 5/10 | Validate or retire |
| Intraday without OOS data | 65 | 5/10 | Validate or retire |
| Base scalpers (top 10 live PnL) | 10 | 2/10 | Validate properly |
| Alpha engines (theoretical) | 17 | 3/10 | Fix + validate |
| Client replay | 48 | 4/10 | Extend to 90+ days |

---

## Overfitting Tests (Proposed)

For any strategy that passes initial live PnL screening, apply:

### Test 1: Parameter Sensitivity
Shift each parameter ±10%. If PF drops >30%, the strategy is parameter-sensitive (overfit indicator).

### Test 2: In-Sample vs Out-of-Sample Degradation
Split historical data 70/30. If OOS PF < 0.7 × IS PF, reject.
- Acceptable degradation: ≤30% (PF IS=1.5 → OOS ≥ 1.05)
- Phase 22E found -100% degradation, which is catastrophic

### Test 3: Random Entry Baseline
Compare the strategy's win rate to a random entry at the same SL/TP geometry. If the strategy win rate is within 3% of random, there is no detectable edge.

Random entry break-even WR at 0.30% SL / 0.75% TP: ~33%
Strategy minimum required WR to beat random: ≥36% (at least 3 pp above random)

### Test 4: Monte Carlo Stability
Run 1,000 random permutations of trade sequence. If median outcome is negative but arithmetic mean is positive, the strategy has sequence-dependent positive bias (not robust).

---

## Structural Anti-Overfitting Requirements

Going forward, no strategy should be added to the production registry without:

1. **Minimum 100 trades OOS** (backtest on data not used for development)
2. **OOS Profit Factor ≥ 1.25** on at least 3 of 5 walk-forward windows
3. **Parameter stability test:** ±10% parameter shift degrades PF by ≤20%
4. **Live paper trading minimum:** 30 days, ≥30 trades before any capital allocation

---

## Phase 11 Verdict

**FAIL — severe overfitting present.**

The platform's primary overfitting mechanism is the expansion pack (301 strategies, 42% of registry). Removing it is the single most impactful anti-overfitting action available. Secondary is revoking the Phase 22E synthetic certification and replacing it with real OOS validation.

**After removing expansion pack and implementing OOS validation requirements, the remaining ~55 Go strategies are candidates for validation, not guaranteed to pass.**
