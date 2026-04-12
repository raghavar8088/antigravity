# Master Upgrade Prompt Part 1

You are helping audit and upgrade a coding repository.
Reply briefly and concretely.
Use exactly these sections:
1. Findings
2. Fix plan
3. Verification
Do not include long reasoning.

## Issue 1: Root bot dependencies omit anthropic

- Severity: high
- Category: dependencies
- File: `requirements.txt`
- Detail: The documented autonomous Python bot imports anthropic, but the root requirements file does not install it.
- Evidence: `autonomous_ai_bot.py imports anthropic while requirements.txt does not mention it`
- Requested action: Either add the dependency or refactor the bot to avoid mandatory runtime imports before configuration checks.

## Issue 2: Existing autonomous bot only scans Python files

- Severity: high
- Category: coverage
- File: `autonomous_ai_bot.py`
- Detail: The current repo contains major Go, TypeScript, and JavaScript surfaces that are invisible to the existing analyzer.
- Requested action: Use a repo-wide scanner with language-aware checks across engine, client, bridge, and infrastructure.

## Issue 3: Observed bridge failure: Request failed with status code 404

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Bridge cannot consistently reach the engine endpoints it expects.
- Evidence: `Request failed with status code 404`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

Keep the answer under 250 words.
If a fix is not needed, say why in one sentence.
