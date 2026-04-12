#!/usr/bin/env python3
"""
Autonomous repo-wide audit and upgrade planner for the Trading application.

This bot is designed to work even without external AI providers:
- scans the repository across Go, Python, TypeScript/JavaScript, and config files
- runs available local verification commands
- produces a structured issue inventory and upgrade plan
- writes prompt packs that can be pasted into Claude, ChatGPT, or coding agents

External browser or agent automation can consume the generated files from:
  output/autonomous_bot/
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from collections import Counter, defaultdict
from dataclasses import dataclass, asdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable


ROOT = Path(__file__).resolve().parent
OUTPUT_DIR = ROOT / "output" / "autonomous_bot"
PROMPTS_DIR = OUTPUT_DIR / "prompts"
AGENT_TASKS_DIR = OUTPUT_DIR / "agent_tasks"
PROMPT_CHUNK_SIZE = 2

EXCLUDED_DIRS = {
    ".git",
    ".next",
    ".venv",
    "venv",
    "__pycache__",
    "node_modules",
    ".gocache-test",
    ".gotmp-test",
    "tmp",
    "output",
}

SCAN_EXTENSIONS = {
    ".go",
    ".py",
    ".ts",
    ".tsx",
    ".js",
    ".jsx",
    ".json",
    ".yaml",
    ".yml",
    ".md",
}


@dataclass
class Issue:
    severity: str
    category: str
    title: str
    detail: str
    file: str | None = None
    line: int | None = None
    evidence: str | None = None
    recommendation: str | None = None
    source: str = "static"


@dataclass
class CommandResult:
    name: str
    command: str
    cwd: str
    success: bool
    exit_code: int
    duration_seconds: float
    stdout_tail: list[str]
    stderr_tail: list[str]
    skipped: bool = False
    skip_reason: str | None = None


class AutonomousRepoBot:
    def __init__(self, root: Path, run_commands: bool = False) -> None:
        self.root = root
        self.run_commands = run_commands
        self.files_by_ext: dict[str, list[Path]] = defaultdict(list)
        self.issues: list[Issue] = []
        self.command_results: list[CommandResult] = []
        self.summary: dict[str, object] = {}

    def run(self) -> int:
        self._ensure_output_dirs()
        self.scan_repository()
        self.detect_static_issues()
        self.run_verification_commands()
        self.summarize()
        self.write_outputs()
        self.print_summary()
        return 0

    def _ensure_output_dirs(self) -> None:
        PROMPTS_DIR.mkdir(parents=True, exist_ok=True)
        AGENT_TASKS_DIR.mkdir(parents=True, exist_ok=True)

    def scan_repository(self) -> None:
        for path in self.root.rglob("*"):
            if not path.is_file():
                continue
            if any(part in EXCLUDED_DIRS for part in path.parts):
                continue
            if path.suffix.lower() in SCAN_EXTENSIONS:
                self.files_by_ext[path.suffix.lower()].append(path)

    def detect_static_issues(self) -> None:
        self._check_root_requirements()
        self._check_python_only_bot_gap()
        self._check_browser_bridge_risks()
        self._check_agent_handoff_gap()
        self._check_sensitive_files()
        self._check_go_versions()
        self._check_large_client_surface()

    def run_verification_commands(self) -> None:
        checks = [
            ("python_bot_import", [sys.executable, "autonomous_ai_bot.py"], self.root),
            ("go_targeted_tests", ["go", "test", "./internal/options", "./internal/trading"], self.root / "engine"),
            ("client_lint", ["cmd", "/c", "npm", "run", "lint"], self.root / "client"),
        ]

        for name, command, cwd in checks:
            if not cwd.exists():
                self.command_results.append(
                    CommandResult(
                        name=name,
                        command=" ".join(command),
                        cwd=str(cwd),
                        success=False,
                        exit_code=-1,
                        duration_seconds=0.0,
                        stdout_tail=[],
                        stderr_tail=[],
                        skipped=True,
                        skip_reason="working directory missing",
                    )
                )
                continue

            if not self.run_commands:
                self.command_results.append(
                    CommandResult(
                        name=name,
                        command=" ".join(command),
                        cwd=str(cwd),
                        success=False,
                        exit_code=-1,
                        duration_seconds=0.0,
                        stdout_tail=[],
                        stderr_tail=[],
                        skipped=True,
                        skip_reason="command execution disabled (use --run-commands)",
                    )
                )
                continue

            self.command_results.append(self._run_command(name, command, cwd))

        self._issues_from_command_results()

    def summarize(self) -> None:
        severities = Counter(issue.severity for issue in self.issues)
        categories = Counter(issue.category for issue in self.issues)
        file_counts = {ext: len(paths) for ext, paths in sorted(self.files_by_ext.items())}
        command_status = {
            result.name: {
                "success": result.success,
                "skipped": result.skipped,
                "exit_code": result.exit_code,
            }
            for result in self.command_results
        }

        self.summary = {
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "root": str(self.root),
            "total_scanned_files": sum(file_counts.values()),
            "file_counts_by_extension": file_counts,
            "issues_total": len(self.issues),
            "issues_by_severity": dict(severities),
            "issues_by_category": dict(categories),
            "command_status": command_status,
            "upgrade_tracks": self._build_upgrade_tracks(),
        }

    def write_outputs(self) -> None:
        issue_inventory = [asdict(issue) for issue in self.issues]
        command_results = [asdict(result) for result in self.command_results]

        (OUTPUT_DIR / "issues.json").write_text(
            json.dumps(issue_inventory, indent=2),
            encoding="utf-8",
        )
        (OUTPUT_DIR / "commands.json").write_text(
            json.dumps(command_results, indent=2),
            encoding="utf-8",
        )
        (OUTPUT_DIR / "summary.json").write_text(
            json.dumps(self.summary, indent=2),
            encoding="utf-8",
        )
        self._write_prompt_packs()
        self._write_agent_tasks()
        (OUTPUT_DIR / "upgrade_plan.md").write_text(
            self._render_upgrade_plan_markdown(),
            encoding="utf-8",
        )
        (OUTPUT_DIR / "report.md").write_text(
            self._render_report_markdown(),
            encoding="utf-8",
        )
        (OUTPUT_DIR / "browser_handoff.json").write_text(
            json.dumps(self._build_browser_handoff_manifest(), indent=2),
            encoding="utf-8",
        )

    def print_summary(self) -> None:
        severities = self.summary.get("issues_by_severity", {})
        print("AUTONOMOUS REPO BOT")
        print(f"Root: {self.root}")
        print(f"Scanned files: {self.summary.get('total_scanned_files', 0)}")
        print(f"Issues found: {self.summary.get('issues_total', 0)}")
        print(
            "Severity breakdown: "
            f"high={severities.get('high', 0)} "
            f"medium={severities.get('medium', 0)} "
            f"low={severities.get('low', 0)}"
        )
        print(f"Report: {OUTPUT_DIR / 'report.md'}")
        print(f"Upgrade plan: {OUTPUT_DIR / 'upgrade_plan.md'}")

    def _run_command(self, name: str, command: list[str], cwd: Path) -> CommandResult:
        started = datetime.now()
        try:
            proc = subprocess.run(
                command,
                cwd=cwd,
                capture_output=True,
                text=True,
                timeout=180,
                check=False,
            )
            duration = (datetime.now() - started).total_seconds()
            return CommandResult(
                name=name,
                command=" ".join(command),
                cwd=str(cwd),
                success=proc.returncode == 0,
                exit_code=proc.returncode,
                duration_seconds=duration,
                stdout_tail=self._tail_lines(proc.stdout),
                stderr_tail=self._tail_lines(proc.stderr),
            )
        except Exception as exc:
            duration = (datetime.now() - started).total_seconds()
            return CommandResult(
                name=name,
                command=" ".join(command),
                cwd=str(cwd),
                success=False,
                exit_code=-1,
                duration_seconds=duration,
                stdout_tail=[],
                stderr_tail=[str(exc)],
            )

    def _issues_from_command_results(self) -> None:
        for result in self.command_results:
            if result.skipped:
                self.issues.append(
                    Issue(
                        severity="low",
                        category="automation",
                        title=f"Verification command skipped: {result.name}",
                        detail=result.skip_reason or "Verification was skipped.",
                        evidence=result.command,
                        recommendation="Run the bot with --run-commands in a prepared environment.",
                        source="verification",
                    )
                )
                continue

            if result.success:
                continue

            severity = "medium"
            if result.name == "python_bot_import":
                severity = "high"
            self.issues.append(
                Issue(
                    severity=severity,
                    category="verification",
                    title=f"Verification command failed: {result.name}",
                    detail="A local verification command failed, which blocks reliable autonomous upgrades.",
                    evidence="\n".join(result.stderr_tail or result.stdout_tail),
                    recommendation=f"Fix the failure and rerun `{result.command}` from `{result.cwd}`.",
                    source="verification",
                )
            )

    def _check_root_requirements(self) -> None:
        req = self.root / "requirements.txt"
        if not req.exists():
            return
        contents = req.read_text(encoding="utf-8", errors="ignore").lower()
        if "anthropic" not in contents:
            self.issues.append(
                Issue(
                    severity="high",
                    category="dependencies",
                    title="Root bot dependencies omit anthropic",
                    detail="The documented autonomous Python bot imports anthropic, but the root requirements file does not install it.",
                    file=str(req.relative_to(self.root)),
                    evidence="autonomous_ai_bot.py imports anthropic while requirements.txt does not mention it",
                    recommendation="Either add the dependency or refactor the bot to avoid mandatory runtime imports before configuration checks.",
                )
            )

    def _check_python_only_bot_gap(self) -> None:
        bot = self.root / "autonomous_ai_bot.py"
        if not bot.exists():
            return
        contents = bot.read_text(encoding="utf-8", errors="ignore")
        if "rglob('*.py')" in contents or 'rglob("*.py")' in contents:
            self.issues.append(
                Issue(
                    severity="high",
                    category="coverage",
                    title="Existing autonomous bot only scans Python files",
                    detail="The current repo contains major Go, TypeScript, and JavaScript surfaces that are invisible to the existing analyzer.",
                    file=str(bot.relative_to(self.root)),
                    recommendation="Use a repo-wide scanner with language-aware checks across engine, client, bridge, and infrastructure.",
                )
            )

    def _check_browser_bridge_risks(self) -> None:
        log_path = self.root / "bridge" / "bridge-decisions.jsonl"
        readme = self.root / "bridge" / "README.md"
        if readme.exists():
            contents = readme.read_text(encoding="utf-8", errors="ignore").lower()
            if "prototype-only" in contents:
                self.issues.append(
                    Issue(
                        severity="high",
                        category="browser-automation",
                        title="Browser bridge is explicitly marked prototype-only",
                        detail="The existing browser automation path is documented as a prototype and depends on fragile ChatGPT web selectors.",
                        file=str(readme.relative_to(self.root)),
                        recommendation="Treat browser chat as a fallback transport and keep core analysis and planning local and deterministic.",
                    )
                )
        if log_path.exists():
            contents = log_path.read_text(encoding="utf-8", errors="ignore")
            patterns = {
                "Request failed with status code 404": "Bridge cannot consistently reach the engine endpoints it expects.",
                "No JSON object found in ChatGPT response": "Bridge is not reliably constraining or parsing model output.",
                "Failed to submit prompt to ChatGPT": "Browser UI automation is brittle against prompt-box state.",
                "Timed out waiting for ChatGPT to finish responding": "Response timing is unstable for unattended runs.",
                "Unable to recover ChatGPT page": "Recovery logic is not sufficient for long-running autonomous work.",
            }
            for pattern, detail in patterns.items():
                if pattern in contents:
                    self.issues.append(
                        Issue(
                            severity="high",
                            category="browser-automation",
                            title=f"Observed bridge failure: {pattern}",
                            detail=detail,
                            file=str(log_path.relative_to(self.root)),
                            evidence=pattern,
                            recommendation="Add a queue-based handoff, stronger retries, and a non-browser primary execution path.",
                        )
                    )

    def _check_agent_handoff_gap(self) -> None:
        setup = self.root / "AGENT_SETUP.md"
        if not setup.exists():
            return
        contents = setup.read_text(encoding="utf-8", errors="ignore").lower()
        if "create agent" in contents and "execute necessary api calls or commands" in contents:
            self.issues.append(
                Issue(
                    severity="medium",
                    category="orchestration",
                    title="Agent setup is documented but not orchestrated in code",
                    detail="The repo contains instructions for manually creating agents, but there is no local controller that dispatches repo issues to those agents.",
                    file=str(setup.relative_to(self.root)),
                    recommendation="Generate machine-readable task packs and prompts so agents can be driven from a repeatable workflow.",
                )
            )

    def _check_sensitive_files(self) -> None:
        tracked = [
            self.root / "agent-config.json",
            self.root / "bridge" / "bridge-decisions.jsonl",
        ]
        for path in tracked:
            if not path.exists():
                continue
            self.issues.append(
                Issue(
                    severity="low",
                    category="operations",
                    title=f"Operational artifact is stored in-repo: {path.name}",
                    detail="Generated or environment-specific operational files in the repo can make autonomous runs noisy and harder to review.",
                    file=str(path.relative_to(self.root)),
                    recommendation="Move generated runtime outputs under output/ or tmp/, or ensure they are gitignored if they should not be versioned.",
                )
            )

    def _check_go_versions(self) -> None:
        go_mod = self.root / "engine" / "go.mod"
        if not go_mod.exists():
            return
        contents = go_mod.read_text(encoding="utf-8", errors="ignore")
        match = re.search(r"^go\s+(\d+\.\d+)", contents, flags=re.MULTILINE)
        if match and match.group(1) == "1.20":
            self.issues.append(
                Issue(
                    severity="medium",
                    category="upgrade",
                    title="Go engine is pinned to Go 1.20",
                    detail="The engine may be missing fixes and tooling improvements available in newer supported Go versions.",
                    file=str(go_mod.relative_to(self.root)),
                    recommendation="Review compatibility for upgrading to a newer Go release and run targeted engine tests before adopting it.",
                )
            )

    def _check_large_client_surface(self) -> None:
        ts_files = len(self.files_by_ext.get(".ts", [])) + len(self.files_by_ext.get(".tsx", []))
        if ts_files >= 40:
            self.issues.append(
                Issue(
                    severity="medium",
                    category="coverage",
                    title="Client surface is large enough to require dedicated checks",
                    detail="A large Next.js/TypeScript surface should not be delegated to a Python-only review loop.",
                    evidence=f"Detected {ts_files} TypeScript files",
                    recommendation="Keep separate client verification stages such as eslint, build, and targeted UI agent tasks.",
                )
            )

    def _build_upgrade_tracks(self) -> list[dict[str, object]]:
        tracks = [
            {
                "name": "Foundation",
                "goal": "Make the autonomous bot reliable in this environment.",
                "steps": [
                    "Replace Python-only scanning with repo-wide scanning.",
                    "Make dependency checks explicit before importing provider SDKs.",
                    "Generate stable machine-readable reports and task packs.",
                ],
            },
            {
                "name": "Verification",
                "goal": "Ensure fixes are accepted only after local validation.",
                "steps": [
                    "Run targeted Go tests.",
                    "Run Next.js lint/build checks.",
                    "Capture failures and feed them back into issue planning.",
                ],
            },
            {
                "name": "Agent Handoff",
                "goal": "Turn issues into reusable work packets for Claude, ChatGPT, and Codex agents.",
                "steps": [
                    "Generate prompt packs grouped by subsystem.",
                    "Write JSON task manifests that browser or editor automation can consume.",
                    "Track task status in a stable output directory.",
                ],
            },
            {
                "name": "Browser Fallback",
                "goal": "Use browser AI tools only when direct agent execution is unavailable.",
                "steps": [
                    "Keep prompts deterministic and self-contained.",
                    "Queue one task at a time with expected outputs.",
                    "Require verification after any pasted fix returns.",
                ],
            },
        ]
        return tracks

    def _build_browser_handoff_manifest(self) -> dict[str, object]:
        task_files = sorted(str(path.relative_to(self.root)) for path in AGENT_TASKS_DIR.glob("*.json"))
        prompt_files = sorted(
            str(path.relative_to(self.root))
            for path in PROMPTS_DIR.glob("*part_*.md")
        )
        return {
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "root": str(self.root),
            "status": "ready",
            "instructions": [
                "Open one prompt file at a time in Claude or ChatGPT.",
                "Collect the answer or proposed patch.",
                "Paste or route the result into your coding agent.",
                "Run verification before marking the task complete.",
            ],
            "prompt_files": prompt_files,
            "task_files": task_files,
            "primary_report": str((OUTPUT_DIR / "report.md").relative_to(self.root)),
            "upgrade_plan": str((OUTPUT_DIR / "upgrade_plan.md").relative_to(self.root)),
        }

    def _write_prompt_packs(self) -> None:
        grouped = defaultdict(list)
        for issue in self.issues:
            grouped[issue.category].append(issue)

        for category, issues in grouped.items():
            for chunk_index, chunk in enumerate(self._chunk_issues(issues[:10], PROMPT_CHUNK_SIZE), start=1):
                path = PROMPTS_DIR / f"{self._slug(category)}_part_{chunk_index}.md"
                path.write_text(self._render_prompt_pack(f"{category.title()} Prompt Pack Part {chunk_index}", chunk), encoding="utf-8")

        for chunk_index, chunk in enumerate(self._chunk_issues(self.issues[:12], 3), start=1):
            (PROMPTS_DIR / f"master_upgrade_prompt_part_{chunk_index}.md").write_text(
                self._render_prompt_pack(f"Master Upgrade Prompt Part {chunk_index}", chunk),
                encoding="utf-8",
            )

    def _write_agent_tasks(self) -> None:
        groups = {
            "engine_backend": self._filter_issues(lambda issue: issue.file and issue.file.startswith("engine/")),
            "client_frontend": self._filter_issues(lambda issue: issue.file and issue.file.startswith("client/")),
            "automation_orchestration": self._filter_issues(
                lambda issue: issue.category in {"automation", "browser-automation", "orchestration", "verification"}
            ),
        }

        for name, issues in groups.items():
            payload = {
                "task_name": name,
                "generated_at": datetime.now(timezone.utc).isoformat(),
                "goal": self._task_goal(name),
                "issues": [asdict(issue) for issue in issues[:12]],
                "expected_outputs": [
                    "Code changes or a precise no-change explanation",
                    "Verification notes",
                    "Any remaining blockers",
                ],
            }
            (AGENT_TASKS_DIR / f"{name}.json").write_text(
                json.dumps(payload, indent=2),
                encoding="utf-8",
            )

    def _render_upgrade_plan_markdown(self) -> str:
        lines = [
            "# Autonomous Upgrade Plan",
            "",
            f"Generated: {self.summary.get('generated_at')}",
            "",
            "## Priorities",
            "",
        ]
        top_issues = sorted(self.issues, key=self._severity_rank)[:12]
        for index, issue in enumerate(top_issues, start=1):
            lines.append(f"{index}. [{issue.severity.upper()}] {issue.title}")
            lines.append(f"   - Category: {issue.category}")
            if issue.file:
                lines.append(f"   - File: `{issue.file}`")
            if issue.recommendation:
                lines.append(f"   - Action: {issue.recommendation}")
        lines.extend(["", "## Execution Tracks", ""])
        for track in self.summary.get("upgrade_tracks", []):
            lines.append(f"### {track['name']}")
            lines.append(track["goal"])
            lines.append("")
            for step in track["steps"]:
                lines.append(f"- {step}")
            lines.append("")
        return "\n".join(lines).strip() + "\n"

    def _render_report_markdown(self) -> str:
        lines = [
            "# Autonomous Repo Bot Report",
            "",
            f"Generated: {self.summary.get('generated_at')}",
            f"Repository: `{self.root}`",
            "",
            "## Scan Summary",
            "",
            f"- Total scanned files: {self.summary.get('total_scanned_files', 0)}",
            f"- Issues found: {self.summary.get('issues_total', 0)}",
        ]
        severity = self.summary.get("issues_by_severity", {})
        lines.extend(
            [
                f"- High: {severity.get('high', 0)}",
                f"- Medium: {severity.get('medium', 0)}",
                f"- Low: {severity.get('low', 0)}",
                "",
                "## Verification Commands",
                "",
            ]
        )
        for result in self.command_results:
            status = "SKIPPED" if result.skipped else ("PASS" if result.success else "FAIL")
            lines.append(f"- `{result.name}`: {status}")
            lines.append(f"  - Command: `{result.command}`")
            if result.skip_reason:
                lines.append(f"  - Reason: {result.skip_reason}")
            if result.stderr_tail:
                lines.append(f"  - stderr: `{result.stderr_tail[-1]}`")
            elif result.stdout_tail:
                lines.append(f"  - stdout: `{result.stdout_tail[-1]}`")
        lines.extend(["", "## Issues", ""])
        for issue in sorted(self.issues, key=self._severity_rank):
            location = f" ({issue.file})" if issue.file else ""
            lines.append(f"- [{issue.severity.upper()}] {issue.title}{location}")
            lines.append(f"  - Category: {issue.category}")
            lines.append(f"  - Detail: {issue.detail}")
            if issue.evidence:
                lines.append(f"  - Evidence: `{issue.evidence}`")
            if issue.recommendation:
                lines.append(f"  - Recommendation: {issue.recommendation}")
        return "\n".join(lines).strip() + "\n"

    def _issue_prompt_lines(self, index: int, issue: Issue) -> list[str]:
        lines = [f"## Issue {index}: {issue.title}", "", f"- Severity: {issue.severity}", f"- Category: {issue.category}"]
        if issue.file:
            lines.append(f"- File: `{issue.file}`")
        lines.append(f"- Detail: {issue.detail}")
        if issue.evidence:
            lines.append(f"- Evidence: `{issue.evidence}`")
        if issue.recommendation:
            lines.append(f"- Requested action: {issue.recommendation}")
        lines.append("")
        return lines

    def _filter_issues(self, predicate) -> list[Issue]:
        return [issue for issue in self.issues if predicate(issue)]

    def _chunk_issues(self, issues: list[Issue], size: int) -> list[list[Issue]]:
        return [issues[index:index + size] for index in range(0, len(issues), size)]

    def _render_prompt_pack(self, title: str, issues: list[Issue]) -> str:
        lines = [
            f"# {title}",
            "",
            "You are helping audit and upgrade a coding repository.",
            "Reply briefly and concretely.",
            "Use exactly these sections:",
            "1. Findings",
            "2. Fix plan",
            "3. Verification",
            "Do not include long reasoning.",
            "",
        ]
        for index, issue in enumerate(issues, start=1):
            lines.extend(self._issue_prompt_lines(index, issue))
        lines.extend(
            [
                "Keep the answer under 250 words.",
                "If a fix is not needed, say why in one sentence.",
                "",
            ]
        )
        return "\n".join(lines).strip() + "\n"

    def _task_goal(self, name: str) -> str:
        goals = {
            "engine_backend": "Stabilize backend and Go engine issues.",
            "client_frontend": "Audit and upgrade the Next.js dashboard safely.",
            "automation_orchestration": "Improve autonomous analysis, browser handoff, and agent workflow reliability.",
        }
        return goals.get(name, "Resolve the assigned issue cluster.")

    def _tail_lines(self, text: str, count: int = 12) -> list[str]:
        lines = [line.rstrip() for line in text.splitlines() if line.strip()]
        return lines[-count:]

    def _severity_rank(self, issue: Issue) -> tuple[int, str]:
        rank = {"high": 0, "medium": 1, "low": 2}
        return (rank.get(issue.severity, 3), issue.title)

    def _slug(self, value: str) -> str:
        return re.sub(r"[^a-z0-9]+", "_", value.lower()).strip("_")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Autonomous repo-wide audit bot")
    parser.add_argument(
        "--run-commands",
        action="store_true",
        help="Run local verification commands in addition to static analysis.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    bot = AutonomousRepoBot(ROOT, run_commands=args.run_commands)
    return bot.run()


if __name__ == "__main__":
    raise SystemExit(main())
