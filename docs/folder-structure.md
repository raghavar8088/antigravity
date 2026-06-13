# Folder Structure

Current high-signal structure:

```text
.
├── client/          # Next.js BTC trading console and API routes
├── data/            # Runtime state volume
├── docs/            # AI-optimized repository docs and reports
├── engine/          # Go trading engine
├── grafana/         # Observability dashboards/support
├── nginx/           # Reverse proxy/deployment support
├── scripts/         # Operational scripts
├── .cursor/         # Cursor rules/skills; includes protected project guidance
├── .github/         # CI/deployment automation
└── .vscode/         # Editor workspace configuration
```

Protected/support folders:

- `.cursor/skills/trading-app-guide/`: BTC Futures Trading guide; keep.
- `engine/**/graphify-out/`: Graphify knowledge graph output; keep.
- `engine/vendor/`: vendored Go dependencies; do not hand-edit.
- `.venv/`: local Python environment; not application source.

## Recommended Logical Boundaries

Do not move files without a dedicated migration PR. Preferred future layout:

```text
client/src/
├── app/                  # Next pages and route handlers
├── components/           # UI components
├── hooks/                # React hooks
├── lib/
│   ├── trading/          # Protected BTC trading domain
│   ├── analytics/        # Replay/reporting
│   ├── portfolio/        # Trade history/accounting
│   ├── broker/           # Server-side broker/session helpers
│   ├── strategyAuthority/# Strategy lifecycle
│   └── utils/            # Small shared helpers
├── server/               # Server-only integrations
└── types/                # Shared TypeScript types
```

```text
engine/
├── cmd/                  # Binaries
├── internal/
│   ├── trading/          # Orchestration
│   ├── execution*/       # Gateways/adapters
│   ├── risk*/            # Risk systems
│   ├── omsv3/            # OMS authority
│   ├── ledger/           # Event ledger
│   ├── reconciliation*/  # Drift checks
│   ├── persistence/      # Storage
│   ├── marketdata/       # BTC feeds
│   ├── options*/         # BTC options engines
│   └── strategy/         # Strategy registry
└── vendor/               # Vendored dependencies
```
