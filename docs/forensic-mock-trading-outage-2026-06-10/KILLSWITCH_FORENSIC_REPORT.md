# KILLSWITCH FORENSIC REPORT

## Is kill switch active?

**At audit time (local):** Unknown without live engine — check `GET /api/admin/ks/status` (`main.go:1641–1647`).

**Mechanism:** `killswitch.Service.IsActive()` (`service.go:62–66`) reads in-memory `active` flag.

## Has it triggered recently?

**Likely YES** post-`33c614a8` deploy via reconciliation:

- Trigger: `TriggerOMSDesync` (`killswitch_hook.go:25`)
- Reason pattern: `reconciliation critical drift (balance): ... equity_drift`

## Was it left active?

**YES** — kill switch does not self-release (`service.go:115–116` pre-fix).

Persists to Postgres via `EventKillSwitchTriggered` (`service.go:154–170`).

## Startup replay

**Pre-fix:** No restore — restart cleared in-memory flag but recon re-triggered within 10s.

**Post-fix:** `RestoreFromLedger()` (`service.go:65–110`) replays ledger; auto-releases stale recon false positives.

## Can kill switch permanently disable execution?

**YES** while `active=true`:

- `PreTradeRiskPipeline` blocks (`pipeline.go:51–54`)
- `ProcessExecutionRequest` blocks (`institutional_request.go:16–20`)
- Engine process continues; only new orders blocked (`loop.go:262–265`)

## Release paths

| Path | File | Lines |
|------|------|-------|
| Admin API | `main.go` | `POST /api/admin/ks/release` 1623–1639 |
| Auto-healer | `killswitch/service.go` | `RestoreFromLedger` + `shouldAutoReleaseReconFalsePositive` |
| Nuclear | `admin/killswitch.go` | `cancelFunc()` halts master loop |
