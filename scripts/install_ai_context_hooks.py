"""Install local git hooks that keep AI context fresh before commits."""

from __future__ import annotations

import os
import stat
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
HOOKS_DIR = ROOT / ".git" / "hooks"
PRE_COMMIT = HOOKS_DIR / "pre-commit"


HOOK = """#!/bin/sh
set -e

echo "[ai-context] refreshing generated AI context before commit..."
npm run ai-context:refresh

git add \\
  .ai-context \\
  AI_CONTEXT.md \\
  .cursor/rules/core.mdc \\
  .cursor/rules/graphify.mdc \\
  .cursorignore \\
  package.json \\
  package-lock.json \\
  scripts/ai_context_maps.py \\
  scripts/graphify_workflow.py \\
  scripts/install_ai_context_hooks.py \\
  merge_graphs.py

echo "[ai-context] refreshed and staged."
"""


def main() -> None:
    if not HOOKS_DIR.exists():
        raise SystemExit(".git/hooks not found. Run this from a git checkout.")

    PRE_COMMIT.write_text(HOOK, encoding="utf-8", newline="\n")

    mode = PRE_COMMIT.stat().st_mode
    PRE_COMMIT.chmod(mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

    print(f"installed {PRE_COMMIT.relative_to(ROOT)}")
    print("pre-commit will run `npm run ai-context:refresh` and stage generated context.")


if __name__ == "__main__":
    main()
