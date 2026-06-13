# Active Features

## AI Token Optimization
Status: active and mostly complete.

Implemented:
- `graphifyy` installation and Graphify project integration.
- Root Graphify scripts: `graphify:query`, `graphify:update`, `graphify:rebuild`, `graphify:stats`.
- Scoped graph workflow for `client/src`, `engine/internal`, and `engine/cmd`.
- Repomix summary workflow under `.ai-context/repomix-output.md`.
- AST-derived maps: `repo-map.json`, `symbols.json`, `dependencies.json`, and `call-graph.json`.
- `.cursorignore` for heavy data, logs, generated artifacts, and media.
- `.ai-context/README_FOR_AI.md` as the single AI entry file.
- Domain model, business rules, glossary, owners, module summaries, prompts, ADRs, protected modules, and architecture versions.
- `runtime/` directory separation for candles, ticks, backtests, logs, reports, exports, and snapshots.
- Local pre-commit hook installer and CI freshness check.

## Trading Platform Feature Work
No new trading feature is currently active in this session. Start new feature work from `.ai-context/prompts/feature.prompt`.
