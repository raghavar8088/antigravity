# Orchestration Prompt Pack Part 1

You are helping audit and upgrade a coding repository.
Reply briefly and concretely.
Use exactly these sections:
1. Findings
2. Fix plan
3. Verification
Do not include long reasoning.

## Issue 1: Agent setup is documented but not orchestrated in code

- Severity: medium
- Category: orchestration
- File: `AGENT_SETUP.md`
- Detail: The repo contains instructions for manually creating agents, but there is no local controller that dispatches repo issues to those agents.
- Requested action: Generate machine-readable task packs and prompts so agents can be driven from a repeatable workflow.

Keep the answer under 250 words.
If a fix is not needed, say why in one sentence.
