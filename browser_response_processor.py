#!/usr/bin/env python3
"""
Process saved browser AI responses into local execution tasks.

Reads:
  output/autonomous_bot/browser_results/<provider>/*.json
  output/autonomous_bot/browser_results/<provider>/*.response.md

Writes:
  output/autonomous_bot/execution_tasks/*.json
  output/autonomous_bot/execution_queue.json
  output/autonomous_bot/execution_summary.md
"""

from __future__ import annotations

import argparse
import json
import re
from collections import defaultdict
from dataclasses import dataclass, asdict
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parent
AUTONOMOUS_DIR = ROOT / "output" / "autonomous_bot"
BROWSER_RESULTS_DIR = AUTONOMOUS_DIR / "browser_results"
EXECUTION_TASKS_DIR = AUTONOMOUS_DIR / "execution_tasks"
EXECUTION_QUEUE_FILE = AUTONOMOUS_DIR / "execution_queue.json"
EXECUTION_SUMMARY_FILE = AUTONOMOUS_DIR / "execution_summary.md"
AGENT_TASKS_DIR = AUTONOMOUS_DIR / "agent_tasks"


@dataclass
class ProcessedResponse:
    provider: str
    prompt_file: str
    response_file: str
    findings: list[str]
    fix_plan: list[str]
    verification: list[str]
    task_group: str
    status: str = "ready"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Convert browser AI responses into local execution tasks")
    parser.add_argument("--provider", default="", help="Optional provider filter, such as chatgpt or claude")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    EXECUTION_TASKS_DIR.mkdir(parents=True, exist_ok=True)

    responses = load_browser_responses(args.provider)
    processed = [process_response(item) for item in responses if is_actionable(item)]
    queue = build_execution_queue(processed)

    write_execution_outputs(processed, queue)
    print(f"Processed responses: {len(processed)}")
    print(f"Execution queue: {EXECUTION_QUEUE_FILE}")
    print(f"Execution summary: {EXECUTION_SUMMARY_FILE}")
    return 0


def load_browser_responses(provider_filter: str) -> list[dict[str, str]]:
    items: list[dict[str, str]] = []
    if not BROWSER_RESULTS_DIR.exists():
        return items

    for provider_dir in BROWSER_RESULTS_DIR.iterdir():
        if not provider_dir.is_dir():
            continue
        if provider_filter and provider_dir.name != provider_filter:
            continue
        for meta_file in sorted(provider_dir.glob("*.json")):
            meta = json.loads(meta_file.read_text(encoding="utf-8"))
            response_path = ROOT / meta["response_file"]
            prompt_copy_path = ROOT / meta["prompt_copy_file"]
            items.append(
                {
                    "provider": provider_dir.name,
                    "meta_file": str(meta_file.relative_to(ROOT)),
                    "prompt_file": meta.get("prompt_file", ""),
                    "prompt_copy_file": str(prompt_copy_path.relative_to(ROOT)),
                    "response_file": str(response_path.relative_to(ROOT)),
                    "response_text": response_path.read_text(encoding="utf-8", errors="ignore") if response_path.exists() else "",
                    "prompt_text": prompt_copy_path.read_text(encoding="utf-8", errors="ignore") if prompt_copy_path.exists() else "",
                }
            )
    return items


def is_actionable(item: dict[str, str]) -> bool:
    text = item["response_text"].strip()
    return bool(text) and text != "TEST_OK"


def process_response(item: dict[str, str]) -> ProcessedResponse:
    sections = split_sections(item["response_text"])
    prompt_file = item["prompt_file"]
    task_group = infer_task_group(prompt_file)
    return ProcessedResponse(
        provider=item["provider"],
        prompt_file=prompt_file,
        response_file=item["response_file"],
        findings=sections.get("findings", []),
        fix_plan=sections.get("fix plan", []),
        verification=sections.get("verification", []),
        task_group=task_group,
    )


def split_sections(text: str) -> dict[str, list[str]]:
    buckets: dict[str, list[str]] = defaultdict(list)
    current = "misc"

    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line:
            continue
        normalized = line.lower().rstrip(":")
        if normalized in {"findings", "fix plan", "verification"}:
            current = normalized
            continue
        cleaned = re.sub(r"^\d+\.\s*", "", line)
        cleaned = re.sub(r"^[-*]\s*", "", cleaned)
        buckets[current].append(cleaned)

    return buckets


def infer_task_group(prompt_file: str) -> str:
    lower = prompt_file.lower()
    if "browser_automation" in lower or "verification" in lower or "orchestration" in lower:
        return "automation_orchestration"
    if "coverage" in lower or "upgrade" in lower:
        return "engine_backend"
    if "client" in lower or "frontend" in lower:
        return "client_frontend"
    if "dependencies" in lower:
        return "automation_orchestration"
    return "general"


def build_execution_queue(processed: list[ProcessedResponse]) -> dict[str, object]:
    task_groups = load_agent_task_groups()
    queue_items = []

    for index, response in enumerate(processed, start=1):
        queue_items.append(
            {
                "id": f"exec_{index:03d}",
                "status": response.status,
                "provider": response.provider,
                "task_group": response.task_group,
                "prompt_file": response.prompt_file,
                "response_file": response.response_file,
                "findings": response.findings,
                "fix_plan": response.fix_plan,
                "verification": response.verification,
                "related_agent_task": task_groups.get(response.task_group),
            }
        )

    return {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "root": str(ROOT),
        "items": queue_items,
    }


def load_agent_task_groups() -> dict[str, str]:
    mapping: dict[str, str] = {}
    if not AGENT_TASKS_DIR.exists():
        return mapping
    for path in AGENT_TASKS_DIR.glob("*.json"):
        mapping[path.stem] = str(path.relative_to(ROOT))
    return mapping


def write_execution_outputs(processed: list[ProcessedResponse], queue: dict[str, object]) -> None:
    for path in EXECUTION_TASKS_DIR.glob("*.json"):
        path.unlink()

    for index, response in enumerate(processed, start=1):
        payload = asdict(response)
        payload["generated_at"] = datetime.now(timezone.utc).isoformat()
        payload["next_step"] = choose_next_step(response)
        out_path = EXECUTION_TASKS_DIR / f"task_{index:03d}_{slugify(Path(response.prompt_file).stem)}.json"
        out_path.write_text(json.dumps(payload, indent=2), encoding="utf-8")

    EXECUTION_QUEUE_FILE.write_text(json.dumps(queue, indent=2), encoding="utf-8")
    EXECUTION_SUMMARY_FILE.write_text(render_summary(processed, queue), encoding="utf-8")


def choose_next_step(response: ProcessedResponse) -> str:
    if response.task_group == "automation_orchestration":
        return "Implement reliability improvements in the automation and browser handoff modules."
    if response.task_group == "client_frontend":
        return "Patch the client code and rerun lint/build verification."
    if response.task_group == "engine_backend":
        return "Patch backend code and rerun targeted Go verification."
    return "Review manually and assign to the most relevant coding agent."


def render_summary(processed: list[ProcessedResponse], queue: dict[str, object]) -> str:
    lines = [
        "# Execution Summary",
        "",
        f"Generated: {queue['generated_at']}",
        f"Processed browser responses: {len(processed)}",
        "",
        "## Ready Tasks",
        "",
    ]

    for item in queue["items"]:
        lines.append(f"- `{item['id']}` {item['task_group']}")
        lines.append(f"  - Prompt: `{item['prompt_file']}`")
        lines.append(f"  - Response: `{item['response_file']}`")
        if item["fix_plan"]:
            lines.append(f"  - First fix step: {item['fix_plan'][0]}")
        if item["related_agent_task"]:
            lines.append(f"  - Related task pack: `{item['related_agent_task']}`")
    lines.extend(
        [
            "",
            "## Workflow",
            "",
            "1. Pick the first ready task from `execution_queue.json`.",
            "2. Open the matching execution task JSON under `execution_tasks/`.",
            "3. Apply code changes locally or route the task to a coding agent.",
            "4. Rerun `python autonomous_repo_bot.py --run-commands` after changes.",
        ]
    )
    return "\n".join(lines).strip() + "\n"


def slugify(value: str) -> str:
    return re.sub(r"[^a-z0-9]+", "_", value.lower()).strip("_")


if __name__ == "__main__":
    raise SystemExit(main())
