# MOCK EXECUTION CALL GRAPH

```mermaid
flowchart TD
  MD[Coinbase WS BTC-USD] --> RUN[Orchestrator.Run loop.go:969]
  RUN --> TICK[processTickPipeline loop.go:1012]
  TICK --> CANDLE[candleAgg.Feed → 1m/5m channels]
  TICK --> PSG[processStrategyGroup loop.go:1334]
  CANDLE --> PSG
  PSG --> REG[strategy.OnTick curated_registry.go]
  PSG --> AGG[FilterSignalsSelective aggregator_selective.go:36]
  AGG --> GATES[Stale/position/regime/sizing filters loop.go:1441-1555]
  GATES --> RISK1[risk.RiskEngine.Validate engine.go:126]
  GATES --> INST[executeThroughInstitutionalPath loop.go:299]
  INST --> OMS_CREATE[EventOrderCreated loop.go:355]
  OMS_CREATE --> REPLAY[omsv3.Replay aggregate.go:107]
  INST --> PMS[pms.CheckPortfolioRisk loop.go:435]
  INST --> RISK2[PreTradeRiskPipeline.Check pipeline.go:46]
  RISK2 -->|kill switch active| BLOCK[DecisionBlocked pipeline.go:51]
  RISK2 --> SUBMIT[submitInstitutionalOrder loop.go:640]
  SUBMIT --> PAPER[PaperClient.ExecuteSignal paper.go:137]
  PAPER --> POS[posMgr.OpenPosition manager.go:126]
  POS --> CLOSE[processCloseEvents loop.go:1695]
  CLOSE --> PNL[SettlePosition + CanonicalNetPnL paper.go:207]
  CLOSE --> MONGO[paperpersist MongoDB paperpersist_hooks.go]
  RECON[reconciliationv2 WireProduction wiring.go:29] --> HOOK[CriticalDriftKillSwitchHook killswitch_hook.go:13]
  HOOK --> KS[killswitch.Service.Trigger service.go:74]
  KS --> BLOCK
```

## Stage Reference Table

| Stage | File | Function | Caller | Callee | Status |
|-------|------|----------|--------|--------|--------|
| Boot | `cmd/antigravity/main.go` | `main()` | OS | orchestrator, recon | OK |
| Market data | `main.go:416` | Coinbase Connect | main | `Orchestrator.Run` | OK |
| Strategy tick | `loop.go:1334` | `processStrategyGroup` | Run/candle goroutines | `Strategy.OnTick` | OK |
| Aggregation | `aggregator_selective.go:36` | `FilterSignalsSelective` | processStrategyGroup | approved signals | OK |
| Risk legacy | `risk/engine.go:126` | `Validate` | processStrategyGroup | continue or pass | OK |
| OMS v3 | `loop.go:346` | `executeThroughInstitutionalPathWithFill` | processStrategyGroup | ledger events | OK |
| Kill gate | `risk/gate/pipeline.go:46` | `Check` | institutional path | PaperClient or BLOCK | **Was BLOCKED** |
| Mock broker | `execution/paper.go:137` | `ExecuteSignal` | submitInstitutionalOrder | applyFill | OK when unblocked |
| Positions | `positions/manager.go:126` | `OpenPosition` | openAndTrackPosition | SL/TP checks | OK |
| PnL | `loop.go:1695` | `processCloseEvents` | CloseEvents channel | Mongo persist | OK |
| Reconciliation | `reconciliationv2/engine.go:61` | `RunDomain` | scheduler | kill hook | **Was false CRITICAL** |
