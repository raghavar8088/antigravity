# Graph Report - antigravity-main  (2026-07-09)

## Corpus Check
- 4938 files · ~27,275,470 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 30 nodes · 65 edges · 5 communities (3 shown, 2 thin omitted)
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `7ddf899e`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]

## God Nodes (most connected - your core abstractions)
1. `Client` - 27 edges
2. `Context` - 10 edges
3. `truncate()` - 5 edges
4. `OrderSide` - 4 edges
5. `PlaceOrderRequest` - 4 edges
6. `PlaceOrderResult` - 4 edges
7. `OptionContractInfo` - 3 edges
8. `OrderType` - 2 edges
9. `Time` - 2 edges
10. `WalletEntry` - 2 edges

## Surprising Connections (you probably didn't know these)
- `PlaceOrderResult` --references--> `Time`  [EXTRACTED]
  engine/internal/delta/client.go → engine/internal/delta/client.go  _Bridges community 4 → community 3_

## Import Cycles
- None detected.

## Communities (5 total, 2 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.31
Nodes (3): Client, LivePosition, OpenOrder

### Community 2 - "Community 2"
Cohesion: 0.36
Nodes (4): Context, truncate(), PerpProductInfo, WalletEntry

### Community 4 - "Community 4"
Cohesion: 0.60
Nodes (4): OrderSide, OrderType, PlaceOrderRequest, PlaceOrderResult

## Knowledge Gaps
- **1 isolated node(s):** `provision-delta-creds.sh script`
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Client` connect `Community 0` to `Community 2`, `Community 3`, `Community 4`?**
  _High betweenness centrality (0.629) - this node is a cross-community bridge._
- **Why does `Context` connect `Community 2` to `Community 0`, `Community 3`, `Community 4`?**
  _High betweenness centrality (0.035) - this node is a cross-community bridge._
- **Why does `PlaceOrderResult` connect `Community 4` to `Community 0`, `Community 3`?**
  _High betweenness centrality (0.026) - this node is a cross-community bridge._
- **What connects `provision-delta-creds.sh script` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._