# Orchestration Prompt Pack

Review the following repo issues and propose concrete fixes.
Keep changes minimal, preserve behavior, and call out verification steps.

## Issue 1: Agent setup is documented but not orchestrated in code

- Severity: medium
- Category: orchestration
- File: `AGENT_SETUP.md`
- Detail: The repo contains instructions for manually creating agents, but there is no local controller that dispatches repo issues to those agents.
- Requested action: Generate machine-readable task packs and prompts so agents can be driven from a repeatable workflow.
