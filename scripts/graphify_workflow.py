"""Low-token Graphify workflow helpers for AI agents and developers.

The commands here keep Graphify usage consistent across Windows shells and
avoid broad source reads for architecture questions.
"""

from __future__ import annotations

import argparse
import json
import shutil
import site
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]

SCOPES = {
    "root": ROOT / "graphify-out" / "graph.json",
    "client": ROOT / "graphify-scopes" / "client-src" / "graphify-out" / "graph.json",
    "engine-internal": ROOT
    / "graphify-scopes"
    / "engine-internal"
    / "graphify-out"
    / "graph.json",
    "engine-cmd": ROOT / "graphify-scopes" / "engine-cmd" / "graphify-out" / "graph.json",
}

EXTRACT_TARGETS = [
    ("client/src", "graphify-scopes/client-src"),
    ("engine/internal", "graphify-scopes/engine-internal"),
    ("engine/cmd", "graphify-scopes/engine-cmd"),
]


def find_graphify() -> str:
    graphify = shutil.which("graphify")
    if graphify:
        return graphify

    scripts_dir = Path(site.getusersitepackages()).parent / "Scripts"
    candidate = scripts_dir / ("graphify.exe" if sys.platform.startswith("win") else "graphify")
    if candidate.exists():
        return str(candidate)

    raise SystemExit(
        "graphify executable not found. Install it with `pip install graphifyy`."
    )


def run_graphify(args: list[str]) -> None:
    subprocess.run([find_graphify(), *args], cwd=ROOT, check=True)


def graph_path(scope: str) -> Path:
    path = SCOPES[scope]
    if not path.exists() and scope != "root":
        raise SystemExit(
            f"Scoped graph not found for {scope}: {path}\n"
            "Run `npm run graphify:rebuild` first."
        )
    return path


def query(args: argparse.Namespace) -> None:
    question = " ".join(args.question).strip()
    if not question:
        raise SystemExit('Usage: npm run graphify:query -- "your question"')

    cmd = ["query", question, "--budget", str(args.budget)]
    cmd.extend(["--graph", str(graph_path(args.scope))])
    run_graphify(cmd)


def update(_: argparse.Namespace) -> None:
    run_graphify(["update", "."])


def rebuild(_: argparse.Namespace) -> None:
    for source, out_dir in EXTRACT_TARGETS:
        run_graphify(["extract", source, "--out", out_dir, "--no-cluster"])

    subprocess.run([sys.executable, "merge_graphs.py"], cwd=ROOT, check=True)
    run_graphify(["cluster-only", ".", "--no-label", "--no-viz"])


def stats(_: argparse.Namespace) -> None:
    path = SCOPES["root"]
    if not path.exists():
        raise SystemExit("graphify-out/graph.json not found. Run graphify:rebuild first.")

    with path.open("r", encoding="utf-8") as f:
        graph = json.load(f)

    nodes = graph.get("nodes", [])
    links = graph.get("links", graph.get("edges", []))
    communities = {
        node.get("community")
        for node in nodes
        if isinstance(node, dict) and node.get("community") is not None
    }

    print(f"graph: {path.relative_to(ROOT)}")
    print(f"nodes: {len(nodes)}")
    print(f"edges: {len(links)}")
    print(f"communities: {len(communities)}")


def main() -> None:
    parser = argparse.ArgumentParser(description="Low-token Graphify workflow helpers")
    subcommands = parser.add_subparsers(dest="command", required=True)

    query_parser = subcommands.add_parser("query", help="Query graph with a low token budget")
    query_parser.add_argument("question", nargs="*", help="Question to ask the graph")
    query_parser.add_argument(
        "--budget",
        type=int,
        default=1000,
        help="Maximum Graphify output tokens; raise only when needed",
    )
    query_parser.add_argument(
        "--scope",
        choices=sorted(SCOPES.keys()),
        default="root",
        help="Use a smaller scoped graph when the subsystem is known",
    )
    query_parser.set_defaults(func=query)

    update_parser = subcommands.add_parser("update", help="Fast AST-only graph update")
    update_parser.set_defaults(func=update)

    rebuild_parser = subcommands.add_parser(
        "rebuild", help="Rebuild scoped graphs, merge them, and recluster"
    )
    rebuild_parser.set_defaults(func=rebuild)

    stats_parser = subcommands.add_parser("stats", help="Print graph size summary")
    stats_parser.set_defaults(func=stats)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
