# Repository Summary

This repository is an institutional trading platform with a Next.js TypeScript client/API layer, a Go execution engine, and supporting Python/bridge code for AI, research, and broker integration.

Use this file first for orientation, then query Graphify or the JSON maps for exact symbols.

## Primary Code Areas
- `client/`: Next.js app, API routes, dashboard components, trading hooks, paper trading helpers, broker clients, analytics, and UI state.
- `engine/`: Go trading engine, strategy registry, risk gates, OMS v3, execution, ledger, reconciliation, kill switch, persistence, observability, and benchmarks.
- `bridge/`: Broker and integration bridge code.
- `brain/`: AI/research support code.
- `infrastructure/`: database, deployment, and operational assets.
- `docs/` and root markdown files: runbooks, audit reports, architecture notes, and phase plans.

## Low-Token AI Workflow
1. Read this file or `AI_CONTEXT.md`.
2. Run `npm run graphify:query -- "question"` or a scoped query through `scripts/graphify_workflow.py`.
3. Use `.ai-context/repo-map.json`, `symbols.json`, `dependencies.json`, and `call-graph.json` for structured lookup.
4. Read raw source only after the graph or maps identify exact files and symbols.

## Generated Context Files
- `.ai-context/repomix-output.md`: compressed repository pack from Repomix.
- `.ai-context/repo-map.json`: module buckets, file counts, symbol counts, top files, and communities.
- `.ai-context/symbols.json`: symbols grouped by source file.
- `.ai-context/dependencies.json`: extracted import/reference relationships by file.
- `.ai-context/call-graph.json`: extracted call relationships by source symbol.
