# Architecture Improvements

## Previous Architecture

- Mixed BTC trading code with legacy non-BTC market code and reports.
- Important BTC paths existed, but AI assistants had to scan many files to discover the flow.
- Documentation was scattered across root/client/engine report files.
- Graphify artifacts existed but the CLI was unavailable in the shell.

## New Architecture Direction

- BTC Futures Trading is explicitly documented as the protected domain.
- Root `docs/` now provides a short AI-first entry point.
- Client and engine module boundaries are indexed.
- API route groups and service interactions are summarized.
- Manual-review cleanup candidates are separated from safe cleanup.
- Graphify artifacts are explicitly protected.

## Benefits

- Lower AI context load: assistants can start from `docs/repository-summary.md`.
- Safer refactors: protected BTC files and invariants are listed.
- Faster onboarding: entry points, commands, APIs, and data stores are in one place.
- Better risk control: uncertain files are not deleted blindly.

## Recommended Future Refactors

These require focused PRs and tests:

1. Split `engine/cmd/antigravity/main.go` wiring into smaller boot modules.
2. Consolidate repeated Next.js route response/error helpers.
3. Group terminal UI pages by feature ownership.
4. Add machine-readable indexes generated from imports once Graphify CLI is available.
5. Audit package dependencies with a clean install/build before removing any packages.

## Protected BTC Architecture

```text
client BTC UI/hooks
  -> trading policy/signals/math
  -> paper state/persistence
  -> Next API routes
  -> engine execution/risk/OMS/ledger
  -> reconciliation/kill switch
```

No future architecture cleanup should bypass this flow.
