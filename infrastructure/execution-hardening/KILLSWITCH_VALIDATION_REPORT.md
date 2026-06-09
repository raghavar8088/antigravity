# Kill Switch Validation Report

## Enforcement point

`engine/internal/risk/gate/pipeline.go` lines 51-54:

```go
if p.killSwitch != nil && p.killSwitch.IsActive() {
    return Decision{Status: DecisionBlocked, Reason: "kill switch active: ..."}
}
```

## Path coverage

| Path | Kill switch checked? |
|------|---------------------|
| `executeThroughInstitutionalPathWithFill` | YES (PreTradeRiskPipeline) |
| `ProcessExecutionRequest` | YES (explicit + pipeline) |
| Delta `OnOpen` via institutional handler | YES (via ETP) |
| Delta `OnClose` | YES (`SetKillCheck` before broker) |
| Retired Next.js broker routes | N/A (410) |

## Admin kill switch

- `POST /api/admin/kill` — `engine/internal/admin/killswitch.go:55`
- Requires admin secret + RBAC `system.shutdown`
