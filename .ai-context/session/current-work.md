# Current Work

## Active Objective
Reduce AI token consumption and improve agent accuracy by giving Cursor and other AI agents compact, structured context before source exploration.

## Current State
The optimization stack is in place:
- Graphify knowledge graph and scoped graph workflow.
- Repomix metadata summary.
- `.cursorignore` and ignored `runtime/` data separation.
- `.ai-context/` documentation, maps, prompts, owners, glossary, business rules, ADRs, protected modules, and architecture versions.
- Local pre-commit hook installer and CI freshness workflow.

## Current Session Focus
Add session memory files so agents can recover work-in-progress context without rereading commits, chat history, or broad source files.

## Working Rule
Before source exploration, read `.ai-context/README_FOR_AI.md`, this session folder, and use Graphify or maps for precise relationships.
