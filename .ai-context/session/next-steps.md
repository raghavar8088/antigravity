# Next Steps

## Recommended
- Stop adding AI optimization layers unless a real bottleneck appears.
- Start building or improving trading features using the new low-token context workflow.
- Keep `.ai-context/session/` updated at the end of substantial work sessions.

## Before New Feature Work
1. Read `.ai-context/README_FOR_AI.md`.
2. Read `.ai-context/session/current-work.md`.
3. Read `.ai-context/session/recent-decisions.md`.
4. Use the relevant prompt in `.ai-context/prompts/`.
5. Query Graphify for the target subsystem.
6. Open exact source files only after the graph or maps identify them.

## Maintenance
- Run `npm run ai-context:refresh` after broad structural changes.
- Run `npm run ai-context:maps` after symbol/dependency changes.
- Update `known-issues.md` when a persistent workflow issue is found.
- Update `recent-decisions.md` when an architecture or safety decision is made.
