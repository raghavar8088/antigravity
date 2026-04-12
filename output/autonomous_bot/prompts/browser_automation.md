# Browser-Automation Prompt Pack

Review the following repo issues and propose concrete fixes.
Keep changes minimal, preserve behavior, and call out verification steps.

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

## Issue 3: Observed bridge failure: No JSON object found in ChatGPT response

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Bridge is not reliably constraining or parsing model output.
- Evidence: `No JSON object found in ChatGPT response`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 4: Observed bridge failure: Failed to submit prompt to ChatGPT

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Browser UI automation is brittle against prompt-box state.
- Evidence: `Failed to submit prompt to ChatGPT`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 5: Observed bridge failure: Timed out waiting for ChatGPT to finish responding

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Response timing is unstable for unattended runs.
- Evidence: `Timed out waiting for ChatGPT to finish responding`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 6: Observed bridge failure: Unable to recover ChatGPT page

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Recovery logic is not sufficient for long-running autonomous work.
- Evidence: `Unable to recover ChatGPT page`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.
