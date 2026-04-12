# Master Upgrade Prompt Part 4

You are helping audit and upgrade a coding repository.
Reply briefly and concretely.
Use exactly these sections:
1. Findings
2. Fix plan
3. Verification
Do not include long reasoning.

## Issue 1: Operational artifact is stored in-repo: bridge-decisions.jsonl

- Severity: low
- Category: operations
- File: `bridge\bridge-decisions.jsonl`
- Detail: Generated or environment-specific operational files in the repo can make autonomous runs noisy and harder to review.
- Requested action: Move generated runtime outputs under output/ or tmp/, or ensure they are gitignored if they should not be versioned.

## Issue 2: Go engine is pinned to Go 1.20

- Severity: medium
- Category: upgrade
- File: `engine\go.mod`
- Detail: The engine may be missing fixes and tooling improvements available in newer supported Go versions.
- Requested action: Review compatibility for upgrading to a newer Go release and run targeted engine tests before adopting it.

## Issue 3: Client surface is large enough to require dedicated checks

- Severity: medium
- Category: coverage
- Detail: A large Next.js/TypeScript surface should not be delegated to a Python-only review loop.
- Evidence: `Detected 109 TypeScript files`
- Requested action: Keep separate client verification stages such as eslint, build, and targeted UI agent tasks.

Keep the answer under 250 words.
If a fix is not needed, say why in one sentence.
