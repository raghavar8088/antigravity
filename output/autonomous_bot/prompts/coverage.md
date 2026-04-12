# Coverage Prompt Pack

Review the following repo issues and propose concrete fixes.
Keep changes minimal, preserve behavior, and call out verification steps.

## Issue 1: Existing autonomous bot only scans Python files

- Severity: high
- Category: coverage
- File: `autonomous_ai_bot.py`
- Detail: The current repo contains major Go, TypeScript, and JavaScript surfaces that are invisible to the existing analyzer.
- Requested action: Use a repo-wide scanner with language-aware checks across engine, client, bridge, and infrastructure.

## Issue 2: Client surface is large enough to require dedicated checks

- Severity: medium
- Category: coverage
- Detail: A large Next.js/TypeScript surface should not be delegated to a Python-only review loop.
- Evidence: `Detected 109 TypeScript files`
- Requested action: Keep separate client verification stages such as eslint, build, and targeted UI agent tasks.
