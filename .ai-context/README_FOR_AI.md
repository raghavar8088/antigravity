# README For AI

This is the entry point for low-token AI work in this repository.

## Read Order
Read files in this order:

1. `.ai-context/session/current-work.md`
2. `.ai-context/session/active-features.md`
3. `.ai-context/session/recent-decisions.md`
4. `.ai-context/session/known-issues.md`
5. `.ai-context/session/next-steps.md`
6. `.ai-context/repository-summary.md`
7. `.ai-context/architecture.md`
8. `.ai-context/module-index.md`
9. `.ai-context/dependency-map.md`
10. `.ai-context/execution-flow.md`
11. `.ai-context/domain-model.md`
12. `.ai-context/business-rules.md`
13. `.ai-context/glossary.md`
14. `.ai-context/protected-modules.md`
15. `.ai-context/versions/current.md`
16. `.ai-context/strategy-index.md`
17. `.ai-context/trading-engine.md`
18. `.ai-context/api-index.md`
19. `.ai-context/owners.json`
20. `.ai-context/system-manifest.json`
21. `.ai-context/repomix-output.md`
22. `.ai-context/repo-map.json`
23. `.ai-context/call-graph.json`

Do not scan the entire repository unless implementation details are required.

## Open Source Files Only When
- Modifying code.
- Debugging a concrete failure.
- Adding a feature.
- Tracing runtime behavior after Graphify or the maps identify likely files.
- Verifying exact line-level behavior.

## Prefer These First
- `AI_CONTEXT.md`
- `.ai-context/session/*.md`
- `.ai-context/*.md`
- `.ai-context/owners.json`
- `.ai-context/system-manifest.json`
- `npm run graphify:query -- "question"`
- `python scripts/graphify_workflow.py query --scope client "question"`
- `python scripts/graphify_workflow.py query --scope engine-internal "question"`
- `.ai-context/repo-map.json`
- `.ai-context/dependencies.json`

## Avoid
- Reading `graphify-out/graph.json` directly.
- Reading `.ai-context/symbols.json` or `.ai-context/call-graph.json` in full.
- Broad recursive file reads.
- Opening large market data, logs, generated reports, screenshots, or binary artifacts.
- Reading or indexing `runtime/`; it is intentionally outside source context.
- Editing protected trading modules without reading `.ai-context/protected-modules.md`.

## If You Need Exact Symbols
Use targeted search or small scripts against:

- `.ai-context/symbols.json`
- `.ai-context/dependencies.json`
- `.ai-context/call-graph.json`

Then read only the specific source files and line ranges needed.
