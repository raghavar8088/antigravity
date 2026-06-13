"""Generate compact AI context maps from the Graphify knowledge graph."""

from __future__ import annotations

import json
from collections import Counter, defaultdict
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
GRAPH_PATH = ROOT / "graphify-out" / "graph.json"
OUT_DIR = ROOT / ".ai-context"


def relpath(path: str | None) -> str:
    if not path:
        return ""
    normalized = Path(path.replace("\\", "/"))
    try:
        return normalized.relative_to(ROOT).as_posix()
    except ValueError:
        text = path.replace("\\", "/")
        marker = "antigravity-main/antigravity-main/"
        return text.split(marker, 1)[-1] if marker in text else text


def bucket_for(path: str) -> str:
    if path.startswith("client/src/app/api/"):
        return "client-api"
    if path.startswith("client/src/components/"):
        return "client-components"
    if path.startswith("client/src/hooks/"):
        return "client-hooks"
    if path.startswith("client/src/lib/"):
        return "client-lib"
    if path.startswith("engine/cmd/"):
        return "engine-cmd"
    if path.startswith("engine/internal/"):
        parts = path.split("/")
        return f"engine-{parts[2]}" if len(parts) > 2 else "engine-internal"
    if path.startswith("bridge/"):
        return "bridge"
    if path.startswith("brain/"):
        return "brain"
    if path.startswith("docs/"):
        return "docs"
    return "other"


def node_kind(node: dict) -> str:
    label = node.get("label", "")
    if label.endswith((".ts", ".tsx", ".go", ".py", ".js", ".jsx")):
        return "file"
    if label.endswith("()") or label.startswith("."):
        return "function"
    if label and label[0].isupper():
        return "type"
    return "symbol"


def write_json(path: Path, data: object) -> None:
    path.write_text(
        json.dumps(data, sort_keys=True, separators=(",", ":")),
        encoding="utf-8",
    )


def main() -> None:
    if not GRAPH_PATH.exists():
        raise SystemExit("graphify-out/graph.json not found. Run `npm run graphify:rebuild`.")

    graph = json.loads(GRAPH_PATH.read_text(encoding="utf-8"))
    nodes = graph.get("nodes", [])
    links = graph.get("links", graph.get("edges", []))

    by_id = {node["id"]: node for node in nodes if "id" in node}
    symbols: dict[str, list[dict]] = defaultdict(list)
    repo_map: dict[str, dict] = defaultdict(
        lambda: {"files": 0, "symbols": 0, "communities": [], "top_files": []}
    )
    file_symbol_counts: Counter[str] = Counter()
    dependencies: dict[str, dict[str, list[str]]] = defaultdict(lambda: defaultdict(list))
    call_graph: dict[str, list[str]] = defaultdict(list)

    for node in nodes:
        source_file = relpath(node.get("source_file"))
        if not source_file:
            continue

        bucket = bucket_for(source_file)
        kind = node_kind(node)
        label = node.get("label", node.get("id", ""))
        community = node.get("community")

        if kind == "file":
            repo_map[bucket]["files"] += 1
        else:
            repo_map[bucket]["symbols"] += 1
            file_symbol_counts[source_file] += 1
            symbols[source_file].append(
                {
                    "name": label,
                    "kind": kind,
                    "location": node.get("source_location", ""),
                    "community": community,
                }
            )

        if community is not None:
            repo_map[bucket]["communities"].append(community)

    for link in links:
        source = by_id.get(link.get("source"), {})
        target = by_id.get(link.get("target"), {})
        source_file = relpath(link.get("source_file") or source.get("source_file"))
        target_file = relpath(target.get("source_file"))
        relation = link.get("relation", "")

        if not source_file or not target_file or source_file == target_file:
            continue

        if relation in {"imports", "references", "contains"}:
            deps = dependencies[source_file][relation]
            if target_file not in deps and len(deps) < 50:
                deps.append(target_file)

        if relation == "calls":
            source_label = source.get("label", link.get("source", ""))
            target_label = target.get("label", link.get("target", ""))
            key = f"{source_file}::{source_label}"
            value = f"{target_file}::{target_label}"
            if value not in call_graph[key] and len(call_graph[key]) < 50:
                call_graph[key].append(value)

    for bucket, summary in repo_map.items():
        summary["communities"] = sorted(set(summary["communities"]))[:50]
        summary["top_files"] = [
            {"file": file, "symbols": count}
            for file, count in file_symbol_counts.most_common()
            if bucket_for(file) == bucket
        ][:20]

    OUT_DIR.mkdir(exist_ok=True)
    write_json(OUT_DIR / "repo-map.json", repo_map)
    write_json(OUT_DIR / "symbols.json", symbols)
    write_json(OUT_DIR / "dependencies.json", dependencies)
    write_json(OUT_DIR / "call-graph.json", call_graph)

    print(f"wrote {OUT_DIR / 'repo-map.json'}")
    print(f"wrote {OUT_DIR / 'symbols.json'}")
    print(f"wrote {OUT_DIR / 'dependencies.json'}")
    print(f"wrote {OUT_DIR / 'call-graph.json'}")
    print(f"nodes: {len(nodes)} links: {len(links)}")


if __name__ == "__main__":
    main()
