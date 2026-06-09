# 19 — Residual Risk Register

**Post Phase 3–5 Target State** — risks remaining after full deployment and validation.

---

## Risk Matrix

| ID | Risk | Probability | Impact | Residual Score | Mitigation | Owner |
|----|------|-------------|--------|----------------|------------|-------|
| R-01 | Leader election race during deploy window | Low | Critical | Medium | Wire HA before multi-task; deploy one task first | Eng |
| R-02 | OMS ack-before-fill gap on crash | Medium | High | Medium | Boot replay + exchange poll on recovery | Eng |
| R-03 | Angel One on Vercel (execution coupling) | Medium | High | Medium | Migrate to engine Phase 3 | Eng |
| R-04 | Secret rotation Lambda placeholder | Low | High | Low | Implement per-key rotation logic | SRE |
| R-05 | Cross-region DR not operational | Low | Critical | High | Phase 5 DR stack + quarterly drill | SRE |
| R-06 | Reconciliation false positives | Low | Medium | Low | Tune tolerances; alert routing | Eng |
| R-07 | MongoDB read model drift from ledger | Medium | Medium | Medium | Periodic reconciliation job | Eng |
| R-08 | Vercel outage (dashboard only) | Medium | Low | Low | Engine autonomous on ECS | Ops |
| R-09 | Exchange API rate limits | Medium | Medium | Medium | Backoff + circuit breaker | Eng |
| R-10 | Strategy overfitting (XP_* strategies) | High | High | High | WINNERS_ONLY gate; retire losers | Quant |
| R-11 | Regulatory audit trail gaps | Low | High | Low | S3 audit logs + ledger immutability | Compliance |
| R-12 | Insider threat (admin secret abuse) | Low | Critical | Medium | RBAC + audit log + dual control | Security |
| R-13 | Aurora Serverless cold start | Low | Medium | Low | Min ACU 0.5; pre-warm before market | SRE |
| R-14 | WAF false positives on trading payloads | Medium | Low | Low | Count mode on SizeRestrictions_BODY | SRE |
| R-15 | Git history credential exposure | High | Critical | High | filter-repo + rotate ALL keys | Security |

---

## Accepted Risks (With Waiver)

| ID | Risk | Acceptance Rationale | Review Date |
|----|------|---------------------|-------------|
| — | None currently | All CRITICAL risks must be closed pre-go-live | — |

---

## Risk Trend

| Phase | Open CRITICAL | Open HIGH | Production Score |
|-------|---------------|-----------|------------------|
| Pre Phase 1 | 8 | 12 | 46 |
| Post Phase 1+2 | 4 | 6 | 59 |
| Post Phase 3 (target) | 0 | 2 | 85 |
| Post Phase 4+5 (target) | 0 | 0 | 93 |

---

## Monitoring & Review

- **Weekly:** SRE reviews open HIGH risks
- **Monthly:** Risk register update in production-readiness folder
- **Quarterly:** Full risk reassessment post DR/chaos drill

---

## Escalation

| Residual Score | Action |
|----------------|--------|
| Critical | Block go-live until mitigated |
| High | Mitigation plan required within 7 days |
| Medium | Track in backlog, 30-day SLA |
| Low | Accept with annual review |

**Residual Risk Register Readiness:** 90/100
