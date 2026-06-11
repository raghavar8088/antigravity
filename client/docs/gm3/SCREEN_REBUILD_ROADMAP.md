# SCREEN REBUILD ROADMAP — GM3-ICCTP Phase 18

| Screen | Current | Target | Effort | ROI | Priority |
|--------|---------|--------|--------|-----|----------|
| Command Center | 62 | 92 | M | High | **P0 — Done partial** |
| Execution | 78 | 90 | S | High | P1 |
| Analytics | 72 | 88 | S | Medium | P1 |
| Research | 70 | 88 | S | Medium | P1 |
| Risk | 68 | 90 | S | High | P1 |
| Health | 65 | 85 | S | Medium | P2 |
| Diagnostics | 60 | 82 | S | Low | P3 |
| Settings | 40 | 80 | M | Medium | P2 |
| Strategies | 35 | 88 | L | High | **P0** |
| Portfolio | 35 | 88 | L | High | **P0** |
| Events | 38 | 88 | L | High | **P0** |
| Mock Trading | 55 | 90 | XL | Critical | P1 |
| Login/Sign-in | 50 | 85 | M | Medium | P2 |

**Effort:** S = 1–2 days, M = 3–5 days, L = 1–2 weeks, XL = 2–3 weeks

---

## Priority Queue

### P0 — Highest ROI (Legacy slate dashboards)
1. `/terminal/events` — EventCenter inline styles
2. `/terminal/portfolio` — PortfolioAnalyticsDashboard
3. `/terminal/strategies` — consolidate with Research

### P1 — Modern pages polish
4. `/terminal/execution` — responsive chart height
5. `/terminal/risk` — fix drawdown tone
6. Mock Trading — migrate to M3 shell

### P2 — Completeness
7. Settings expansion
8. Health skeleton loaders
9. Login/sign-in Google auth card

---

## Score Methodology

- **90+** Google Cloud Console quality: unified tokens, skeleton states, a11y, calm hierarchy
- **70–89** Modern but minor gaps (hardcoded colors, missing empty states)
- **50–69** Functional but inconsistent (M3 shell + legacy content)
- **<50** Legacy inline dashboard — requires full rebuild
