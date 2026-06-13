# AI Context

Use this file as the first low-token orientation pass, then query Graphify for exact relationships.

For richer but still compact context, use `.ai-context/`:
- `.ai-context/README_FOR_AI.md`
- `.ai-context/repository-summary.md`
- `.ai-context/architecture.md`
- `.ai-context/module-index.md`
- `.ai-context/domain-model.md`
- `.ai-context/business-rules.md`
- `.ai-context/glossary.md`
- `.ai-context/protected-modules.md`
- `.ai-context/versions/current.md`
- `.ai-context/owners.json`
- `.ai-context/system-manifest.json`
- `.ai-context/execution-flow.md`
- `.ai-context/repomix-output.md`
- `.ai-context/repo-map.json`
- `.ai-context/symbols.json`
- `.ai-context/dependencies.json`
- `.ai-context/call-graph.json`

## System Shape
This is a multi-market algorithmic trading platform. The main UI and API layer live in `client/` as a Next.js TypeScript app. The execution engine lives in `engine/` as Go. Supporting AI/research and broker bridge code live in `brain/` and `bridge/`.

Primary deployment:
- Frontend and API routes: Vercel from `client/`.
- Go engine: AWS Lightsail.
- Databases: MongoDB Atlas, PostgreSQL Neon, Redis, plus local SQLite fallback.

## Trading Flow
Market data enters from Coinbase, Binance, Delta Exchange, AngelOne, Yahoo Finance, and NSE fallbacks. Signals flow through strategy selection, policy gates, risk gates, OMS v3, execution, ledger, reconciliation, kill switch checks, persistence, and finally dashboard/API presentation.

Keep these invariants intact:
- Funding, liquidation, fee, and PnL math must not drift.
- NSE/BSE flows must respect market sessions; crypto is 24/7.
- The kill switch must remain wired in production paths.
- WINNERS_ONLY strategy filtering is active; do not re-add known losing strategies.

## Key Areas
- `client/src/app/api/`: Next.js API routes for paper desk, options, NIFTY, Delta live, AngelOne, cron/admin, auth, and engine proxying.
- `client/src/components/`: Trading dashboard UI panels.
- `client/src/hooks/`: Client data hooks and polling/state adapters.
- `client/src/lib/`: Paper trading, broker persistence, analytics, strategy authority, shared types, and domain helpers.
- `engine/cmd/antigravity/`: Main Go engine entry point.
- `engine/internal/strategy/`: Strategy registry and strategy families.
- `engine/internal/risk/` and `engine/internal/risk/gate/`: Risk decisions and gates.
- `engine/internal/omsv3/`: Event-driven order management.
- `engine/internal/killswitch/`: Production safety controls.
- `engine/internal/ledger/`: Fills, balances, and financial event tracking.
- `engine/internal/reconciliation/`: State and broker reconciliation.
- `engine/internal/marketdata/`: Exchange and market data adapters.

## Low-Token Workflow
Use Graphify before broad source exploration:

```bash
npm run graphify:query -- "how does X connect to Y?"
python scripts/graphify_workflow.py query --scope client "question"
python scripts/graphify_workflow.py query --scope engine-internal "question"
npm run graphify:stats
```

Use raw file reads only after Graphify narrows the file or symbol. Prefer line-range reads over full-file reads for large modules.

After small code edits:

```bash
npm run graphify:update
```

After broad structural changes or when scoped graphs are missing:

```bash
npm run graphify:rebuild
```

## Debugging Pattern
For desk bugs, trace:

signal -> gate/policy -> open position -> mark/exit -> paper math -> persistence -> API route -> UI display

For engine bugs, trace:

market data -> strategy -> risk gate -> OMS v3 -> execution -> ledger -> reconciliation -> kill switch -> persistence
