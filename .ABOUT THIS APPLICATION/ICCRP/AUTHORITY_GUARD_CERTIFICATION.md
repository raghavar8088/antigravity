# AUTHORITY GUARD CERTIFICATION

**Status:** PASS  
**Audited:** 2026-06-11

## Coverage Matrix

| Route | Panel | Guard | Authority Source |
|-------|-------|-------|------------------|
| `/terminal/execution` | ExecutionCenter | Layout guard | terminalStore |
| `/terminal/risk` | RiskModule | Layout guard | terminalStore |
| `/terminal/research` | ResearchCenter | Layout guard | terminalStore |
| `/terminal/strategies` | StrategyIntelligenceDashboard | Layout guard | terminalStore |
| `/terminal/analytics` | AnalyticsCenter | Layout guard | terminalStore |
| `/terminal/portfolio` | PortfolioAnalyticsDashboard | Layout guard | terminalStore |
| `/terminal/events` | EventCenter | Layout guard | terminalStore |
| `/terminal/journal` | TradeJournalPro | Layout guard | terminalStore |

## Unified Layer

```
TerminalLayoutClient.tsx
  → TerminalSnapshotProvider [terminalStore.tsx:227]
  → InstitutionalTerminalShell [TerminalShell.tsx]
  → TerminalAuthorityGuard [TerminalAuthorityGuard.tsx]
  → {page content}
```

## Degraded State

When `!terminalHasAuthority(snapshot)`:
- Full-page guard message: `BACKEND AUTHORITY UNAVAILABLE`
- Shell header shows rose authority badge
- Price/metrics show `—` not zeros

## Files Changed

- `client/src/app/terminal/layout.tsx`
- `client/src/components/terminal/institutional/TerminalLayoutClient.tsx` (new)
- `client/src/lib/terminal/terminalStore.tsx` (provider)
- Individual pages: duplicate guards removed
