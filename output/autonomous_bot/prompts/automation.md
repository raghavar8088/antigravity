# Automation Prompt Pack

Review the following repo issues and propose concrete fixes.
Keep changes minimal, preserve behavior, and call out verification steps.

## Issue 1: Verification command skipped: python_bot_import

- Severity: low
- Category: automation
- Detail: command execution disabled (use --run-commands)
- Evidence: `C:\Users\ragha\AppData\Local\Programs\Python\Python311\python.exe autonomous_ai_bot.py`
- Requested action: Run the bot with --run-commands in a prepared environment.

## Issue 2: Verification command skipped: go_targeted_tests

- Severity: low
- Category: automation
- Detail: command execution disabled (use --run-commands)
- Evidence: `go test ./internal/options ./internal/trading`
- Requested action: Run the bot with --run-commands in a prepared environment.

## Issue 3: Verification command skipped: client_lint

- Severity: low
- Category: automation
- Detail: command execution disabled (use --run-commands)
- Evidence: `cmd /c npm run lint`
- Requested action: Run the bot with --run-commands in a prepared environment.
