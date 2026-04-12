# Browser-Automation Prompt Pack Part 1

You are helping audit and upgrade a coding repository.
Reply briefly and concretely.
Use exactly these sections:
1. Findings
2. Fix plan
3. Verification
Do not include long reasoning.

## Issue 1: Browser bridge is explicitly marked prototype-only

- Severity: high
- Category: browser-automation
- File: `bridge\README.md`
- Detail: The existing browser automation path is documented as a prototype and depends on fragile ChatGPT web selectors.
- Requested action: Treat browser chat as a fallback transport and keep core analysis and planning local and deterministic.

## Issue 2: Observed bridge failure: Request failed with status code 404

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Bridge cannot consistently reach the engine endpoints it expects.
- Evidence: `Request failed with status code 404`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

Keep the answer under 250 words.
If a fix is not needed, say why in one sentence.
