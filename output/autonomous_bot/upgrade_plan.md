# Autonomous Upgrade Plan

Generated: 2026-04-12T10:12:20.780384+00:00

## Priorities

1. [HIGH] Existing autonomous bot only scans Python files
   - Category: coverage
   - File: `autonomous_ai_bot.py`
   - Action: Use a repo-wide scanner with language-aware checks across engine, client, bridge, and infrastructure.
2. [HIGH] Observed bridge failure: Failed to submit prompt to ChatGPT
   - Category: browser-automation
   - File: `bridge\bridge-decisions.jsonl`
   - Action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.
3. [HIGH] Observed bridge failure: No JSON object found in ChatGPT response
   - Category: browser-automation
   - File: `bridge\bridge-decisions.jsonl`
   - Action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.
4. [HIGH] Observed bridge failure: Request failed with status code 404
   - Category: browser-automation
   - File: `bridge\bridge-decisions.jsonl`
   - Action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.
5. [HIGH] Observed bridge failure: Timed out waiting for ChatGPT to finish responding
   - Category: browser-automation
   - File: `bridge\bridge-decisions.jsonl`
   - Action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.
6. [HIGH] Observed bridge failure: Unable to recover ChatGPT page
   - Category: browser-automation
   - File: `bridge\bridge-decisions.jsonl`
   - Action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.
7. [HIGH] Root bot dependencies omit anthropic
   - Category: dependencies
   - File: `requirements.txt`
   - Action: Either add the dependency or refactor the bot to avoid mandatory runtime imports before configuration checks.
8. [HIGH] Verification command failed: python_bot_import
   - Category: verification
   - Action: Fix the failure and rerun `C:\Users\ragha\AppData\Local\Programs\Python\Python311\python.exe autonomous_ai_bot.py` from `C:\Trading apllication`.
9. [MEDIUM] Agent setup is documented but not orchestrated in code
   - Category: orchestration
   - File: `AGENT_SETUP.md`
   - Action: Generate machine-readable task packs and prompts so agents can be driven from a repeatable workflow.
10. [MEDIUM] Client surface is large enough to require dedicated checks
   - Category: coverage
   - Action: Keep separate client verification stages such as eslint, build, and targeted UI agent tasks.
11. [MEDIUM] Go engine is pinned to Go 1.20
   - Category: upgrade
   - File: `engine\go.mod`
   - Action: Review compatibility for upgrading to a newer Go release and run targeted engine tests before adopting it.
12. [LOW] Operational artifact is stored in-repo: agent-config.json
   - Category: operations
   - File: `agent-config.json`
   - Action: Move generated runtime outputs under output/ or tmp/, or ensure they are gitignored if they should not be versioned.

## Execution Tracks

### Foundation
Make the autonomous bot reliable in this environment.

- Replace Python-only scanning with repo-wide scanning.
- Make dependency checks explicit before importing provider SDKs.
- Generate stable machine-readable reports and task packs.

### Verification
Ensure fixes are accepted only after local validation.

- Run targeted Go tests.
- Run Next.js lint/build checks.
- Capture failures and feed them back into issue planning.

### Agent Handoff
Turn issues into reusable work packets for Claude, ChatGPT, and Codex agents.

- Generate prompt packs grouped by subsystem.
- Write JSON task manifests that browser or editor automation can consume.
- Track task status in a stable output directory.

### Browser Fallback
Use browser AI tools only when direct agent execution is unavailable.

- Keep prompts deterministic and self-contained.
- Queue one task at a time with expected outputs.
- Require verification after any pasted fix returns.
