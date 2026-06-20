# Graph Report - antigravity-main  (2026-06-20)

## Corpus Check
- 4697 files · ~26,820,593 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 76 nodes · 140 edges · 8 communities (6 shown, 2 thin omitted)
- Extraction: 99% EXTRACTED · 1% INFERRED · 0% AMBIGUOUS · INFERRED: 1 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `dab9cdea`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]

## God Nodes (most connected - your core abstractions)
1. `MongoManager` - 14 edges
2. `DataSources` - 10 edges
3. `Client` - 10 edges
4. `Context` - 9 edges
5. `Metrics` - 8 edges
6. `tradeFromMongoBSON()` - 7 edges
7. `upsertOne()` - 6 edges
8. `insertOne()` - 6 edges
9. `Connect()` - 5 edges
10. `Context` - 5 edges

## Surprising Connections (you probably didn't know these)
- `Connect()` --references--> `Context`  [EXTRACTED]
  engine/cmd/sep_evidence/datasource.go → engine/cmd/sep_evidence/datasource.go  _Bridges community 7 → community 4_
- `tradeFromMongoBSON()` --calls--> `toFloat()`  [EXTRACTED]
  engine/cmd/sep_evidence/datasource.go → engine/cmd/sep_evidence/datasource.go  _Bridges community 2 → community 6_
- `insertOne()` --references--> `Context`  [EXTRACTED]
  engine/internal/paperpersist/mongo.go → engine/internal/paperpersist/mongo.go  _Bridges community 0 → community 5_
- `baseDoc()` --references--> `M`  [EXTRACTED]
  engine/internal/paperpersist/mongo.go → engine/internal/paperpersist/mongo.go  _Bridges community 5 → community 3_

## Import Cycles
- None detected.

## Communities (8 total, 2 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.25
Nodes (7): Client, Collection, Context, Database, NewMongoManager(), MongoManager, RWMutex

### Community 1 - "Community 1"
Cohesion: 0.26
Nodes (9): Collection, Context, Database, M, Time, Client, New(), upsertByHash() (+1 more)

### Community 2 - "Community 2"
Cohesion: 0.24
Nodes (11): M, Time, GroupByStrategy(), MergeTrades(), toString(), toTime(), tradeFromMongoBSON(), equityPoint (+3 more)

### Community 3 - "Community 3"
Cohesion: 0.25
Nodes (7): Time, MetricsSnapshot, DiagnosticsReport, MetricsSnapshot, AccountKey(), baseDoc(), maskURI()

### Community 5 - "Community 5"
Cohesion: 0.24
Nodes (5): Duration, M, Metrics, insertOne(), upsertOne()

### Community 7 - "Community 7"
Cohesion: 0.60
Nodes (5): DB, Client, Database, Connect(), DataSources

## Knowledge Gaps
- **11 isolated node(s):** `DB`, `mongoScore`, `equityPoint`, `mongoScore`, `M` (+6 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `Community 1` to `Community 0`?**
  _High betweenness centrality (0.161) - this node is a cross-community bridge._
- **Why does `MongoManager` connect `Community 0` to `Community 3`?**
  _High betweenness centrality (0.123) - this node is a cross-community bridge._
- **What connects `DB`, `mongoScore`, `equityPoint` to the rest of the system?**
  _11 weakly-connected nodes found - possible documentation gaps or missing edges._