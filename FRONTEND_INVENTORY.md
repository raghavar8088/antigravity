# FRONTEND INVENTORY REPORT — Forensic Audit
# Date: 2026-06-11 | Auditor: Claude Code

---

## 1. PAGES (client/src/app/**/page.tsx)

| Route | Page File | Component Loaded | Description |
|-------|----------|-----------------|-------------|
| `/` | `client/src/app/page.tsx` | `TerminalDashboard` | Main dashboard (homepage) |
| `/btc-future-trading` | `client/src/app/btc-future-trading/page.tsx` | (unknown — not read) | BTC futures trading desk |
| `/mock-trading` | `client/src/app/mock-trading/page.tsx` | (unknown — not read) | Mock/research trading |
| `/terminal` | `client/src/app/terminal/page.tsx` | (unknown — not read) | Institutional terminal |
| `/terminal/execution` | `client/src/app/terminal/execution/page.tsx` | ExecutionCenter | Execution desk |
| `/terminal/risk` | `client/src/app/terminal/risk/page.tsx` | RiskModule | Risk management |
| `/terminal/research` | `client/src/app/terminal/research/page.tsx` | ResearchCenter | Research desk |
| `/terminal/analytics` | `client/src/app/terminal/analytics/page.tsx` | AnalyticsCenter | Analytics |
| `/terminal/journal` | `client/src/app/terminal/journal/page.tsx` | TradeJournalPro | Trade journal |
| `/sign-in` | `client/src/app/sign-in/page.tsx` | (unknown) | Sign-in page |
| `/login` | `client/src/app/login/page.tsx` | (unknown) | Login page |
| `/paperdesk` | `client/src/app/paperdesk/page.tsx` | (unknown) | Paper desk (alias?) |
| `/paper-desk` | `client/src/app/paper-desk/page.tsx` | `PaperDeskDashboard` (lazy) | Go Engine paper desk |

---

## 2. COMPONENTS (client/src/components/)

### Core Dashboard Components

| Component | API Routes Called | State Source | Purpose |
|-----------|-----------------|-------------|---------|
| `TradingDashboard.tsx` | Multiple engine routes | useEngineState, useTrades, usePositions, useStrategies | Main legacy dashboard |
| `PaperDeskDashboard` (not in components dir — referenced from page.tsx) | `/api/paper-desk/snapshot` (5s poll) | usePaperDesk | Go engine paper desk |
| `BTCFuturesScalper.tsx` | MongoDB via hooks | useBTCFuturesScalperEngine | BTC futures scalper with 600+ strategy display |
| `CommandCenter.tsx` | Engine AI routes | — | AI command center |
| `DeskCommandCenter.tsx` | Various desk routes | useDeskPerformanceMonitor | Desk analytics command center |

### Market Data Components

| Component | Data Source |
|-----------|------------|
| `MarketTicker.tsx` | Polling market APIs |
| `MarketTickerCard.tsx` | Market data |
| `BTCLiveChart.tsx` | fetch() BTC price |
| `BtcSpotStrip.tsx` | BTC spot price |
| `BTCSpotScalper.tsx` | useBTCSpotScalperEngine |

### Options Components

| Component | Routes |
|-----------|--------|
| `NiftyOptionChainPanel.tsx` | `/api/nifty/option-chain` |
| `OptionsAccountHeader.tsx` | Options engine state |

### Institutional Terminal Components

| Component | File |
|-----------|------|
| `TerminalShell` | `components/terminal/institutional/TerminalShell.tsx` |
| `ExecutionCenter` | `components/terminal/institutional/ExecutionCenter.tsx` |
| `RiskModule` | `components/terminal/institutional/RiskModule.tsx` |
| `ResearchCenter` | `components/terminal/institutional/ResearchCenter.tsx` |
| `AnalyticsCenter` | `components/terminal/institutional/AnalyticsCenter.tsx` |
| `TradeJournalPro` | `components/terminal/institutional/TradeJournalPro.tsx` |

### Paper Desk UI Components (desk/)

| Component | Purpose |
|-----------|---------|
| `DeskShell` | Page shell layout |
| `DeskAppBar` | App bar with status chips |
| `DeskCard` | Card container |
| `DeskDataTable` | Data table |
| `DeskMetricTile` | Single metric display |
| `DeskBanner` | Status banner |
| `DeskButton` | Button |
| `DeskChip` | Status chip |
| `DeskTabs` | Tab navigation |
| `DeskSearchField` | Search input |
| `DeskSectionHeader` | Section header |
| `DeskLinearProgress` | Progress bar |
| `DeskEmptyState` | Empty state display |
| `DeskSwitch` | Toggle switch |
| `DeskModuleChrome` | Module wrapper |
| `WorkspaceSettingsCard` | Settings panel |
| `WorkspaceNavPanel` | Navigation |
| `DeskHeroStrip` | Hero metrics strip |
| `DeskThemeToggle` | Dark/light toggle |

### Analytics/Research Components

| Component | Routes |
|-----------|--------|
| `AttributionPanel.tsx` | `/api/strategy-attribution/[id]` |
| `SignalQualityPanel.tsx` | Signal quality data |
| `MTFConfluencePanel.tsx` | Multi-timeframe confluence |
| `DeskPnLScorecardPanel.tsx` | P&L scorecard |
| `ScorecardActionPanel.tsx` | Scorecard actions |
| `SoakTrendPanel.tsx` | Trend soak data |
| `GoLiveGatesPanel.tsx` | Go-live gate status |
| `StrategyRotationPanel.tsx` | Strategy rotation |
| `EdgeLabPanel.tsx` | Edge lab |
| `EdgeCandidatesPanel.tsx` | Edge candidates |
| `DeskMonitorPanel.tsx` | Desk monitoring |
| `StrategyLeaderboard.tsx` | Strategy leaderboard |
| `MockStrategyLeaderboardPanel.tsx` | Mock leaderboard |
| `InstitutionalResearchDashboard.tsx` | Research dashboard |
| `InstitutionalBacktestDashboard.tsx` | Backtest dashboard |
| `InstitutionalRiskCenter.tsx` | Risk center |

### Paper/Mock Trading Components

| Component | Routes |
|-----------|--------|
| `PaperOmsPanel.tsx` | `/api/paper-oms/orders`, `/api/paper-oms/summary` |
| `ReplayBacktestPanel.tsx` | `/api/paper-replay`, `/api/paper-replay-compare` |
| `ReplayWalkForwardLab.tsx` | `/api/replay-walkforward` |
| `ShadowIntentLogPanel.tsx` | `/api/shadow-trade-intents` |
| `MockEquityCurvePanel.tsx` | `/api/mock-trading/equity` |
| `MockMonteCarloPanel.tsx` | Mock trading stats |
| `MockRiskAnalyticsPanel.tsx` | `/api/mock-trading/analytics` |
| `SignalTracePanel.tsx` | `/api/strategy-signal-trace` |
| `VerificationTrackPanel.tsx` | `/api/verification-track/*` |
| `AiAppTrackerPanel.tsx` | `/api/ai-app-tracker/*` |
| `StorageHealthPanel.tsx` | `/api/storage/health` |

### Other Components

| Component | Purpose |
|-----------|---------|
| `AngelOneOrderPanel.tsx` | AngelOne order placement (now blocked by `blockedDirectExecutionRoute`) |
| `Nifty50MarketHero.tsx` | NIFTY market hero strip |
| `Nifty50StocksScalper.tsx` | NIFTY stocks scalper |
| `CryptoEquityScalper.tsx` | Crypto equity scalper |
| `MCXCommodityScalper.tsx` | MCX commodity scalper |
| `AIInsightPanel.tsx` | AI insights display |
| `DailyPnlLedger.tsx` | Daily P&L ledger |
| `FearGreedWidget.tsx` | Fear & Greed index |
| `PerformanceCharts.tsx` | Performance charts |
| `PositionsTable.tsx` | Open positions table |
| `RunningTrades.tsx` | Active trades |
| `TradeHistory.tsx` | Trade history |
| `LiveDeskStatusBar.tsx` | Live status bar |
| `ProfitModeChecklist.tsx` | Profit mode checklist |
| `DeskRunModePanel.tsx` | Run mode panel |
| `DeskHealthBadge.tsx` | Desk health badge |
| `PineEditorPanel.tsx` | Pine script editor panel |
| `WorkspaceSettingsPanel.tsx` | Workspace settings |
| `NotePadPanel.tsx` | Notes |
| `StrategyCard.tsx` | Single strategy card |
| `SignalInsightCard.tsx` | Signal insight |
| `ActivityFeed.tsx` | Activity feed |
| `AppBrandBar.tsx` | App brand bar |
| `DashboardHeader.tsx` | Dashboard header |

---

## 3. HOOKS (client/src/hooks/)

| Hook | Fetches From | Interval | Purpose |
|------|-------------|---------|---------|
| `usePaperDesk.ts` | `/api/paper-desk/snapshot` | 5s polling | Paper desk state (positions, trades, health) |
| `useBTCFuturesScalperEngine.ts` | BTC price APIs, MongoDB | varies | BTC futures engine with 600+ strategies (BROWSER-SIDE simulation) |
| `useEngineState.ts` | Engine proxy `/api/engine/*` | polling | Go engine state |
| `useTrades.ts` | `/api/paper-desk/trades` or engine | polling | Trade history |
| `usePositions.ts` | `/api/paper-desk/positions` or engine | polling | Open positions |
| `useStrategies.ts` | Engine `/api/strategies` | polling | Strategy stats |
| `useAIInsights.ts` | Engine `/api/ai/insights` | polling | AI insights |
| `useLiveBTCMarket.ts` | BTC price sources | polling | Live BTC price |
| `useLiveBTCPrice.ts` | BTC price | polling | BTC price feed |
| `useNiftyMarket.ts` | `/api/nifty/state` | polling | NIFTY market state |
| `useNiftyVIX.ts` | `/api/nifty/vix` | polling | NIFTY VIX |
| `useNiftyCandles.ts` | `/api/nifty/candles` | polling | NIFTY candles |
| `useOptionChain.ts` | `/api/btc/option-chain` | polling | BTC option chain |
| `useNiftyOptionChain.ts` | `/api/nifty/option-chain` | polling | NIFTY option chain |
| `useDeltaStrikes.ts` | Delta Exchange | polling | Delta option strikes |
| `useNiftyOptions.ts` | Engine via hooks | polling | NIFTY options |
| `useNiftyOptionsSelling.ts` | Engine | polling | NIFTY options selling |
| `useNiftyStocks.ts` | Engine | polling | NIFTY stocks |
| `useDeltaLive.ts` | Engine `/api/delta-live/*` | polling | Delta live bridge |
| `useCryptoEquityEngine.ts` | Crypto/equity data | polling | Crypto equity engine |
| `useMCXEngine.ts` | `/api/mcx/*` | polling | MCX commodity engine |
| `useNiftyStocksEngine.ts` | Engine nifty-stocks routes | polling | NIFTY stocks engine |
| `useBTCSpotScalperEngine.ts` | BTC spot | polling | BTC spot scalper |
| `useOptions.ts` | Engine `/api/options/*` | polling | BTC options |
| `useOptionsSelling.ts` | Engine `/api/options-selling/*` | polling | BTC options selling |
| `useNiftyOptionsEngine.ts` | Engine `/api/nifty-options/*` | polling | NIFTY options engine |
| `useNiftyOptionsSellingEngine.ts` | Engine nifty-options-selling | polling | NIFTY options selling engine |
| `useNiftyBeesEngine.ts` | `/api/nifty-bees/*` | polling | NiftyBees ETF engine |
| `usePaperOMSEngine.ts` | `/api/paper-oms/*` | polling | Paper OMS |
| `usePaperDeskAuth.ts` | `/api/auth/session` | once | Auth session check |
| `useOwnerAuth.ts` | Auth | once | Owner auth |
| `useDeskPerformanceMonitor.ts` | Internal state | — | Desk performance calculations |
| `useMockTradingEngine.ts` | `/api/mock-trading/*` | polling | Mock trading engine |
| `useMarketRegime.ts` | `/api/mock-trading/regime` | polling | Market regime |
| `useEngineLogs.ts` | Engine `/api/logs` | polling | Engine log buffer |
| `useAngelOneOrders.ts` | `/api/angelone/*` | polling | AngelOne order status |
| `useFearGreed.ts` | Fear/greed index API | polling | Fear & Greed index |
| `useDeskMounted.ts` | local | — | Mount state |
| `useLiveDataLab.ts` | Various | polling | Live data lab |
| `useMockCandleBuilder.ts` | local | — | Mock candle builder |
| `useStrategyScoring.ts` | local | — | Strategy scoring |
| `useMockResearchRunner.ts` | local | — | Mock research runner |
| `useTerminalLayout.ts` | local | — | Terminal layout state |

---

## 4. STATE MANAGEMENT

No Zustand or Redux stores found. State is managed via:
- React `useState`/`useEffect` in hooks (polling pattern)
- Custom hooks returning state objects
- No global state store discovered

---

## 5. KEY LIBRARIES

| Library | File | Purpose |
|---------|------|---------|
| `mongoTradesClient.ts` | `client/src/lib/` | MongoDB Atlas client for Next.js server routes |
| `paperDeskClient.ts` | `client/src/lib/` | MongoDB collection accessors for paper desk |
| `mockTradingMongo.ts` | `client/src/lib/` | MongoDB accessors for mock-trading collections |
| `engineApi.ts` | `client/src/lib/` | `resolveEngineApiUrl()` — engine base URL resolver |
| `engineProxy.ts` | `client/src/lib/` | Engine proxy helpers |
| `jwtSession.ts` | `client/src/lib/` | JWT sign/verify for `raig_session` cookie |
| `ownerAuth.ts` | `client/src/lib/` | `OWNER_ACCOUNT_KEY = "mock_trading_default"` |
| `getAuthenticatedApiSession.ts` | `client/src/lib/` | Session guard for API routes |
| `paperTradesAuth.ts` | `client/src/lib/` | Paper trades auth |
| `futuresStrategies.ts` | `client/src/lib/` | Browser-side strategy definitions |
| `btcFuturesTrade.types.ts` | `client/src/lib/` | BTCFuturesTrade interface |
| `paperDeskWorker/runPaperDeskPollTick.ts` | `client/src/lib/` | VPS/cron tick runner |

---

## 6. API CALL INVENTORY (Component → Endpoint)

| Component/Hook | Endpoint Called |
|----------------|----------------|
| `usePaperDesk` | `GET /api/paper-desk/snapshot` (5s poll) |
| `usePaperDesk` (detail) | `GET /api/paper-desk/trades`, `/api/paper-desk/orders`, `/api/paper-desk/equity`, `/api/paper-desk/strategy-health` |
| `useEngineState` | `GET /api/engine/[path]` (proxy) |
| `useStrategies` | `GET /api/strategies` (engine direct or proxy) |
| `useTrades` | `GET /api/trades` (engine) |
| `usePositions` | `GET /api/positions` (engine) |
| `useAIInsights` | `GET /api/ai/insights` (engine) |
| `useDeltaLive` | `GET /api/delta-live/stats`, `/api/delta-live/trades` |
| `useNiftyMarket` | `GET /api/nifty/state` (Next.js → engine) |
| `useNiftyVIX` | `GET /api/nifty/vix` |
| `useNiftyCandles` | `GET /api/nifty/candles` |
| `useOptionChain` | `GET /api/btc/option-chain` |
| `useNiftyOptionChain` | `GET /api/nifty/option-chain` |
| `useOptions` | `GET /api/options/positions`, `/api/options/trades`, `/api/options/stats` |
| `useOptionsSelling` | `GET /api/options-selling/positions`, etc. |
| `useNiftyOptionsEngine` | `GET /api/nifty-options/positions`, etc. |
| `useNiftyOptionsSellingEngine` | `GET /api/nifty-options-selling/positions`, etc. |
| `useMockTradingEngine` | `GET /api/mock-trading/account`, `/api/mock-trading/trades`, etc. |
| `useMarketRegime` | `GET /api/mock-trading/regime` |
| `useEngineLogs` | `GET /api/logs` (engine direct or proxy) |
| `PaperOmsPanel` | `GET /api/paper-oms/orders`, `GET /api/paper-oms/summary` |
| `ReplayBacktestPanel` | `POST /api/paper-replay`, `GET /api/paper-replay-compare` |
| `ReplayWalkForwardLab` | `POST /api/replay-walkforward` |
| `AiAppTrackerPanel` | `GET /api/ai-app-tracker/latest`, `/api/ai-app-tracker/reports` |
| `VerificationTrackPanel` | `GET /api/verification-track/latest`, `/api/verification-track/summary` |
| `SignalTracePanel` | `GET /api/strategy-signal-trace` |
| `ShadowIntentLogPanel` | `GET /api/shadow-trade-intents` |
| `StorageHealthPanel` | `GET /api/storage/health` |
| `AttributionPanel` | `GET /api/strategy-attribution/[id]` |
| `DashboardHeader` | `GET /api/btc/price` or similar |
| `BTCLiveChart` | `GET /api/btc/spot-klines` or similar |

---

## 7. BROWSER-SIDE ENGINE (IMPORTANT FINDING)

`useBTCFuturesScalperEngine.ts` implements a **complete trading engine running in the browser**. This includes:
- 600+ strategy signal computation
- Paper trade entry/exit logic
- Position management
- P&L calculation
- MongoDB trade persistence (via Next.js API routes)

This is **separate and parallel** to the Go engine. The browser engine and Go engine can both be writing trades to MongoDB simultaneously, for different "module keys" / account keys.

Key separation:
- Go engine account key: determined by `paperpersist.AccountKey()` (engine config)  
- Browser engine account key: `OWNER_ACCOUNT_KEY = "mock_trading_default"` (hardcoded in `ownerAuth.ts`)
- Paper Desk (`/paper-desk`) reads **Go engine data only** from MongoDB
- BTC Future Trading (`/btc-future-trading`) uses the **browser-side engine**
