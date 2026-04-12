# Upgrade Prompt Pack

Review the following repo issues and propose concrete fixes.
Keep changes minimal, preserve behavior, and call out verification steps.

## Issue 1: Go engine is pinned to Go 1.20

- Severity: medium
- Category: upgrade
- File: `engine\go.mod`
- Detail: The engine may be missing fixes and tooling improvements available in newer supported Go versions.
- Requested action: Review compatibility for upgrading to a newer Go release and run targeted engine tests before adopting it.
