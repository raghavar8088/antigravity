# Master Upgrade Prompt Part 2

You are helping audit and upgrade a coding repository.
Reply briefly and concretely.
Use exactly these sections:
1. Findings
2. Fix plan
3. Verification
Do not include long reasoning.

## Issue 1: Observed bridge failure: No JSON object found in ChatGPT response

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Bridge is not reliably constraining or parsing model output.
- Evidence: `No JSON object found in ChatGPT response`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 2: Observed bridge failure: Failed to submit prompt to ChatGPT

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Browser UI automation is brittle against prompt-box state.
- Evidence: `Failed to submit prompt to ChatGPT`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 3: Observed bridge failure: Timed out waiting for ChatGPT to finish responding

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Response timing is unstable for unattended runs.
- Evidence: `Timed out waiting for ChatGPT to finish responding`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

Keep the answer under 250 words.
If a fix is not needed, say why in one sentence.
