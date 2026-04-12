# Dependencies Prompt Pack

Review the following repo issues and propose concrete fixes.
Keep changes minimal, preserve behavior, and call out verification steps.

## Issue 1: Root bot dependencies omit anthropic

- Severity: high
- Category: dependencies
- File: `requirements.txt`
- Detail: The documented autonomous Python bot imports anthropic, but the root requirements file does not install it.
- Evidence: `autonomous_ai_bot.py imports anthropic while requirements.txt does not mention it`
- Requested action: Either add the dependency or refactor the bot to avoid mandatory runtime imports before configuration checks.
