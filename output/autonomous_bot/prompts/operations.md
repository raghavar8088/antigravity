# Operations Prompt Pack

Review the following repo issues and propose concrete fixes.
Keep changes minimal, preserve behavior, and call out verification steps.

## Issue 1: Operational artifact is stored in-repo: agent-config.json

- Severity: low
- Category: operations
- File: `agent-config.json`
- Detail: Generated or environment-specific operational files in the repo can make autonomous runs noisy and harder to review.
- Requested action: Move generated runtime outputs under output/ or tmp/, or ensure they are gitignored if they should not be versioned.

## Issue 2: Operational artifact is stored in-repo: bridge-decisions.jsonl

- Severity: low
- Category: operations
- File: `bridge\bridge-decisions.jsonl`
- Detail: Generated or environment-specific operational files in the repo can make autonomous runs noisy and harder to review.
- Requested action: Move generated runtime outputs under output/ or tmp/, or ensure they are gitignored if they should not be versioned.
