"""
Merges multiple graphify graph.json files into a single unified graph.
Bypasses the graphify merge-graphs NetworkX compatibility bug.
"""
import json
import os

GRAPH_FILES = [
    "graphify-scopes/client-src/graphify-out/graph.json",
    "graphify-scopes/engine-internal/graphify-out/graph.json",
    "graphify-scopes/engine-cmd/graphify-out/graph.json",
]
OUTPUT_FILE = "graphify-out/graph.json"

def merge_graphs(graph_files, output_path):
    merged_nodes = {}
    merged_edges = []
    merged_hyperedges = []
    total_input_tokens = 0
    total_output_tokens = 0

    seen_edges = set()

    for path in graph_files:
        if not os.path.exists(path):
            print(f"  SKIP (not found): {path}")
            continue
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)

        nodes = data.get("nodes", [])
        edges = data.get("edges", data.get("links", []))
        hyperedges = data.get("hyperedges", [])

        for node in nodes:
            node_id = node.get("id")
            if node_id and node_id not in merged_nodes:
                merged_nodes[node_id] = node

        for edge in edges:
            src = edge.get("source") or edge.get("from")
            tgt = edge.get("target") or edge.get("to")
            rel = edge.get("relation", "")
            key = (src, tgt, rel, edge.get("context", ""))
            if key not in seen_edges:
                seen_edges.add(key)
                merged_edges.append(edge)

        merged_hyperedges.extend(hyperedges)
        total_input_tokens += data.get("input_tokens", 0)
        total_output_tokens += data.get("output_tokens", 0)

        print(f"  Loaded {path}: {len(nodes)} nodes, {len(edges)} edges")

    result = {
        "nodes": list(merged_nodes.values()),
        "edges": merged_edges,
        "hyperedges": merged_hyperedges,
        "input_tokens": total_input_tokens,
        "output_tokens": total_output_tokens,
    }

    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(result, f)

    print(f"\nMerged graph written to: {output_path}")
    print(f"  Total nodes : {len(result['nodes'])}")
    print(f"  Total edges : {len(result['edges'])}")
    print(f"  Hyperedges  : {len(result['hyperedges'])}")

if __name__ == "__main__":
    print("Merging graphify knowledge graphs...\n")
    merge_graphs(GRAPH_FILES, OUTPUT_FILE)
    print("\nDone.")
