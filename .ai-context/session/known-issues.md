# Known Issues

## Graphify Full-Engine Extraction
Full `engine/` extraction includes markdown docs and requires an LLM API key for semantic extraction. Current workflow intentionally extracts code-only scopes:
- `client/src`
- `engine/internal`
- `engine/cmd`

## Graphify Merge Command
The installed Graphify version hit a NetworkX compatibility issue with `merge-graphs`. The repository uses `merge_graphs.py` as a deterministic local merge workaround.

## Repomix Full Pack Size
Full compressed Repomix output is very large for this repository. The default `ai-context:repomix` command uses metadata/directory mode. Use `ai-context:repomix:full` only when explicitly needed.

## Missing Optional Directories
Some docs mention `brain/` and `bridge/`, but those directories may be absent in this checkout. Scripts should not assume they exist.

## npm Warning
`npm` prints `Unknown env config "devdir"` in this environment. It has not blocked the AI context commands.
