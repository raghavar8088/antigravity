# Autonomous Repo Bot

`autonomous_repo_bot.py` is the repo-wide autonomous audit bot for this application.

It is designed for the real shape of this codebase:
- Go engine
- Next.js client
- Node browser bridge
- Python AI service
- repo config and documentation files

## What It Does

1. Scans the repository across multiple languages and config types.
2. Detects known issues in the current automation setup.
3. Optionally runs local verification commands.
4. Generates:
   - `output/autonomous_bot/report.md`
   - `output/autonomous_bot/upgrade_plan.md`
   - `output/autonomous_bot/issues.json`
   - `output/autonomous_bot/browser_handoff.json`
   - prompt packs for Claude and ChatGPT
   - agent task manifests for coding agents

## Quick Start

Static analysis only:

```powershell
python autonomous_repo_bot.py
```

Static analysis plus local verification commands:

```powershell
python autonomous_repo_bot.py --run-commands
```

## Generated Output

### Report

`output/autonomous_bot/report.md`

Human-readable summary of:
- issues
- verification command status
- recommended upgrades

### Upgrade Plan

`output/autonomous_bot/upgrade_plan.md`

Priority-ordered execution plan with upgrade tracks.

### Browser Handoff

`output/autonomous_bot/browser_handoff.json`

Use this when you want browser automation to:
1. open a prompt pack
2. paste it into Claude or ChatGPT
3. capture the answer
4. route it into your coding agent workflow

### Prompt Packs

`output/autonomous_bot/prompts/*.md`

These are ready to paste into Claude or ChatGPT.

### Agent Tasks

`output/autonomous_bot/agent_tasks/*.json`

These are machine-readable issue clusters for coding agents such as Codex or Claude-based code agents.

### Execution Tasks

After browser AI replies are saved, convert them into local execution tasks:

```powershell
python browser_response_processor.py
```

This writes:
- `output/autonomous_bot/execution_tasks/*.json`
- `output/autonomous_bot/execution_queue.json`
- `output/autonomous_bot/execution_summary.md`

These files bridge the gap between browser AI advice and local code-change work.

## Recommended Workflow

1. Run the bot.
2. Read `report.md` and `upgrade_plan.md`.
3. Start with `prompts/master_upgrade_prompt.md`.
4. Paste the prompt into Claude or ChatGPT if you want an external second opinion.
5. Paste the resulting fixes or plan into your coding agent.
6. Re-run `python autonomous_repo_bot.py --run-commands` after fixes.
7. Run `python browser_response_processor.py` to turn saved browser answers into local execution tasks.

## Important Notes

- This bot does not depend on Anthropic or OpenAI SDKs.
- It works in a local-only mode and still produces actionable upgrade outputs.
- Browser AI tools are treated as optional helpers, not the primary control path.
- The older `autonomous_ai_bot.py` is still present, but it is Python-only and not sufficient for the whole repo.
