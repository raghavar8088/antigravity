# Master Upgrade Prompt Part 3

You are helping audit and upgrade a coding repository.
Reply briefly and concretely.
Use exactly these sections:
1. Findings
2. Fix plan
3. Verification
Do not include long reasoning.

## Issue 1: Observed bridge failure: Unable to recover ChatGPT page

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Recovery logic is not sufficient for long-running autonomous work.
- Evidence: `Unable to recover ChatGPT page`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 2: Agent setup is documented but not orchestrated in code

- Severity: medium
- Category: orchestration
- File: `AGENT_SETUP.md`
- Detail: The repo contains instructions for manually creating agents, but there is no local controller that dispatches repo issues to those agents.
- Requested action: Generate machine-readable task packs and prompts so agents can be driven from a repeatable workflow.

## Issue 3: Operational artifact is stored in-repo: agent-config.json

- Severity: low
- Category: operations
- File: `agent-config.json`
- Detail: Generated or environment-specific operational files in the repo can make autonomous runs noisy and harder to review.
- Requested action: Move generated runtime outputs under output/ or tmp/, or ensure they are gitignored if they should not be versioned.

Keep the answer under 250 words.
If a fix is not needed, say why in one sentence.
