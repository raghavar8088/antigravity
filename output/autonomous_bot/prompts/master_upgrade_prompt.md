# Master Upgrade Prompt

You are auditing and upgrading the Trading application.
Read the issue list below, then propose the best execution order and smallest safe patches.

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

## Issue 3: Browser bridge is explicitly marked prototype-only

- Severity: high
- Category: browser-automation
- File: `bridge\README.md`
- Detail: The existing browser automation path is documented as a prototype and depends on fragile ChatGPT web selectors.
- Requested action: Treat browser chat as a fallback transport and keep core analysis and planning local and deterministic.

## Issue 4: Observed bridge failure: Request failed with status code 404

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Bridge cannot consistently reach the engine endpoints it expects.
- Evidence: `Request failed with status code 404`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 5: Observed bridge failure: No JSON object found in ChatGPT response

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Bridge is not reliably constraining or parsing model output.
- Evidence: `No JSON object found in ChatGPT response`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 6: Observed bridge failure: Failed to submit prompt to ChatGPT

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Browser UI automation is brittle against prompt-box state.
- Evidence: `Failed to submit prompt to ChatGPT`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 7: Observed bridge failure: Timed out waiting for ChatGPT to finish responding

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Response timing is unstable for unattended runs.
- Evidence: `Timed out waiting for ChatGPT to finish responding`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 8: Observed bridge failure: Unable to recover ChatGPT page

- Severity: high
- Category: browser-automation
- File: `bridge\bridge-decisions.jsonl`
- Detail: Recovery logic is not sufficient for long-running autonomous work.
- Evidence: `Unable to recover ChatGPT page`
- Requested action: Add a queue-based handoff, stronger retries, and a non-browser primary execution path.

## Issue 9: Agent setup is documented but not orchestrated in code

- Severity: medium
- Category: orchestration
- File: `AGENT_SETUP.md`
- Detail: The repo contains instructions for manually creating agents, but there is no local controller that dispatches repo issues to those agents.
- Requested action: Generate machine-readable task packs and prompts so agents can be driven from a repeatable workflow.

## Issue 10: Operational artifact is stored in-repo: agent-config.json

- Severity: low
- Category: operations
- File: `agent-config.json`
- Detail: Generated or environment-specific operational files in the repo can make autonomous runs noisy and harder to review.
- Requested action: Move generated runtime outputs under output/ or tmp/, or ensure they are gitignored if they should not be versioned.

## Issue 11: Operational artifact is stored in-repo: bridge-decisions.jsonl

- Severity: low
- Category: operations
- File: `bridge\bridge-decisions.jsonl`
- Detail: Generated or environment-specific operational files in the repo can make autonomous runs noisy and harder to review.
- Requested action: Move generated runtime outputs under output/ or tmp/, or ensure they are gitignored if they should not be versioned.

## Issue 12: Go engine is pinned to Go 1.20

- Severity: medium
- Category: upgrade
- File: `engine\go.mod`
- Detail: The engine may be missing fixes and tooling improvements available in newer supported Go versions.
- Requested action: Review compatibility for upgrading to a newer Go release and run targeted engine tests before adopting it.

## Issue 13: Client surface is large enough to require dedicated checks

- Severity: medium
- Category: coverage
- Detail: A large Next.js/TypeScript surface should not be delegated to a Python-only review loop.
- Evidence: `Detected 109 TypeScript files`
- Requested action: Keep separate client verification stages such as eslint, build, and targeted UI agent tasks.

## Issue 14: Verification command failed: python_bot_import

- Severity: high
- Category: verification
- Detail: A local verification command failed, which blocks reliable autonomous upgrades.
- Evidence: `Traceback (most recent call last):
  File "C:\Trading apllication\autonomous_ai_bot.py", line 9, in <module>
    import anthropic
ModuleNotFoundError: No module named 'anthropic'`
- Requested action: Fix the failure and rerun `C:\Users\ragha\AppData\Local\Programs\Python\Python311\python.exe autonomous_ai_bot.py` from `C:\Trading apllication`.

## Issue 15: Verification command failed: client_lint

- Severity: medium
- Category: verification
- Detail: A local verification command failed, which blocks reliable autonomous upgrades.
- Evidence: `C:\Trading apllication\client\src\hooks\useCryptoEquityEngine.ts
  888:11  error  Error: This value cannot be modified
Modifying a value previously passed as an argument to a hook is not allowed. Consider moving the modification before calling the hook.
C:\Trading apllication\client\src\hooks\useCryptoEquityEngine.ts:888:11
  886 |         const latest = engine.quotes[strategy.position.asset.symbol];
  887 |         if (latest?.price > 0) {
> 888 |           strategy.position.currentPrice = latest.price;
      |           ^^^^^^^^^^^^^^^^^ `engineRef` cannot be modified
  889 |           strategy.position.unrealizedPnl = calcPnl(strategy.position.side, strategy.position.entryPrice, latest.price, strategy.position.quantity);
  890 |           strategy.position.returnPct = strategy.position.notional > 0 ? (strategy.position.unrealizedPnl / strategy.position.notional) * 100 : 0;
  891 |           strategy.position.peakReturnPct = Math.max(strategy.position.peakReturnPct, strategy.position.returnPct);  react-hooks/immutability
âœ– 2 problems (1 error, 1 warning)`
- Requested action: Fix the failure and rerun `cmd /c npm run lint` from `C:\Trading apllication\client`.
