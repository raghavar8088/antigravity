# Browser-Automation Prompt Pack Part 3

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

Keep the answer under 250 words.
If a fix is not needed, say why in one sentence.
