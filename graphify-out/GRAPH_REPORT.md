# Graph Report - antigravity-main  (2026-06-20)

## Corpus Check
- 4674 files · ~26,808,223 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 179 nodes · 358 edges · 9 communities
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 16 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `1f73dfb5`
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
- [[_COMMUNITY_Community 8|Community 8]]

## God Nodes (most connected - your core abstractions)
1. `ReconciliationEngine` - 24 edges
2. `main()` - 20 edges
3. `DeltaReconciliationAdapter` - 17 edges
4. `newTestMongoStore()` - 12 edges
5. `mustLedgerEvent()` - 11 edges
6. `AuditLog` - 11 edges
7. `Context` - 11 edges
8. `T` - 10 edges
9. `fakeEventCollection` - 9 edges
10. `AuditEntry` - 9 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewMongoLedgerStore()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/ledger/mongo_store.go
- `NewReconciliationEngine()` --calls--> `NewAuditLog()`  [INFERRED]
  engine/internal/reconciliationv2/engine.go → engine/internal/reconciliationv2/audit.go

## Import Cycles
- None detected.

## Communities (9 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.10
Nodes (32): fetchBinanceBTCSpot(), fetchDeltaBTCSpotForOptions(), formatHealthTime(), getEnvOrDefault(), handleDeltaBTCProbe(), keepAlive(), loadDotEnv(), loadOptionsSellingSnapshot() (+24 more)

### Community 1 - "Community 1"
Cohesion: 0.14
Nodes (19): AuditEntry, AuditLog, BalanceDriftDetector, Context, Mismatch, MismatchDomain, Store, ExposureDriftDetector (+11 more)

### Community 2 - "Community 2"
Cohesion: 0.14
Nodes (15): AccountState, AssetBalance, Client, Context, Time, ExchangeFill, ExchangeOrder, ExchangePosition (+7 more)

### Community 3 - "Community 3"
Cohesion: 0.21
Nodes (12): AggregateType, Collection, Database, Context, Event, Time, eventCollection, eventToDoc() (+4 more)

### Community 4 - "Community 4"
Cohesion: 0.30
Nodes (16): Event, T, mustLedgerEvent(), newFakeEventCollection(), newTestMongoStore(), TestMongoLedgerStore_AppendSurfacesCollectionErrors(), TestMongoLedgerStore_AssignsSequentialSequenceNumbers(), TestMongoLedgerStore_DuplicateIdempotencyKeyRejected() (+8 more)

### Community 5 - "Community 5"
Cohesion: 0.18
Nodes (10): Context, Mismatch, MismatchDomain, Store, Time, NewAuditLog(), AuditEntry, AuditLog (+2 more)

### Community 6 - "Community 6"
Cohesion: 0.44
Nodes (4): Context, Mutex, fakeEventCollection, mongoEventDoc

### Community 7 - "Community 7"
Cohesion: 0.35
Nodes (10): DeltaReconciliationAdapter, T, newTestAdapter(), TestGetBalances_ParsesStringEncodedFields(), TestGetPositions_SkipsZeroSize(), TestGetPositions_UsesMarginedEndpointAndParsesStringFields(), TestNumString_AcceptsNumberEncoding(), TestNumString_AcceptsStringEncoding() (+2 more)

### Community 8 - "Community 8"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **41 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+36 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `Community 0` to `Community 3`, `Community 6`?**
  _High betweenness centrality (0.226) - this node is a cross-community bridge._
- **Why does `fakeEventCollection` connect `Community 6` to `Community 4`?**
  _High betweenness centrality (0.168) - this node is a cross-community bridge._
- **Are the 2 inferred relationships involving `main()` (e.g. with `NewMongoLedgerStore()` and `.Append()`) actually correct?**
  _`main()` has 2 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _41 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.0960960960960961 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.14193548387096774 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.13793103448275862 - nodes in this community are weakly interconnected._