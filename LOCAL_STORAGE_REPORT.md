# Local Storage Report

Generated from repository discovery on 2026-06-02.

## Local State Roots

| Root | Observed Files | Purpose | Clone Requirement |
| --- | --- | --- | --- |
| `data/` | `data/audit/2026-05-31-events.ndjson` | Local filesystem state root used by `LOCAL_DATA_DIR` fallback | Copy recursively, preserve timestamps and hashes |
| `.engine-data/` | Not present in glob output, code default exists | Go engine file snapshot fallback via `ENGINE_DATA_DIR` | Include if present on runtime host |
| `engine` SQLite path | Default `./data/engine.db`; env `SQLITE_PATH` | SQLite engine state | Copy DB, WAL, SHM together |
| `client/fixtures/replay/` | `btcusd_1m_sample.json`, `btcusd_1m_live.json`, `btc_ft_strategy_rankings.json` | Replay fixtures and rankings | Copy as source/data hybrid |
| `client/fixtures/research/` | `btc_ft_verdicts.json` | Research verdicts | Copy |
| `bridge/` | `bridge-decisions.jsonl`, `autonomous-handoff.log.jsonl` | Browser bridge event logs | Copy if bridge continuity/audit required |
| `output/autonomous_bot/` | JSON state/results | Autonomous bot output state | Copy |
| `RESEARCH DOCS/` | Three untracked PDFs | Research inputs | Copy manually; untracked |
| Go build/test caches | `.gocache-test/`, `.gotmp-test/`, `engine/cover_v3.out`, `engine/c` | Generated build/test artifacts | Optional unless bit-for-bit workstation clone is required |
| PM2 worker logs | `client/logs/btc-ft-worker-error.log`, `client/logs/btc-ft-worker-out.log` if present | Worker runtime logs | Copy from runtime host if present |

## Local File Counts Observed

Exact line counts from local text event files:

- `data/audit/2026-05-31-events.ndjson`: 12 records
- `bridge/bridge-decisions.jsonl`: 100 records
- `bridge/autonomous-handoff.log.jsonl`: 14 records

## Filesystem Persistence Layer

`client/src/lib/localStorageService.ts` defines the server-side filesystem data root:

- Env override: `LOCAL_DATA_DIR`
- Default: `path.resolve(process.cwd(), "..", "data")`
- Subdirectories:
  - `trades`
  - `mock-trades`
  - `positions`
  - `signals`
  - `research`
  - `risk`
  - `equity`
  - `metrics`
  - `audit`
  - `configs`
  - `reports`
  - `backups`
- Backup subfolders: `daily`, `weekly`, `monthly`
- Report subfolders: `daily`, `weekly`, `monthly`
- NDJSON rotation threshold: 50 MiB
- Atomic write behavior: write temp file then rename

## Go File Snapshot Fallback

`engine/internal/persistence/file_snapshot.go` defines `ENGINE_DATA_DIR`, defaulting to `.engine-data`, with files:

- `btc_options_buy.json`
- `btc_options_sell.json`

These are source-of-truth state when `DATABASE_URL` is not configured for option paper snapshots.

## Browser Local Storage Inventory

Client code uses browser `localStorage` for state that is not automatically present in server exports:

| Area | Location | State |
| --- | --- | --- |
| Workspace settings | `WorkspaceSettingsPanel.tsx`, `WorkspaceSettingsCard.tsx` | UI workspace settings |
| Terminal layout | `useTerminalLayout.ts` | Terminal panel layout |
| Notepad and screenshots | `NotePadPanel.tsx` | Notes and base64/image screenshots |
| BTC spot scalper | `useBTCSpotScalperEngine.ts` | Local BTC spot paper engine state |
| Crypto equity engine | `useCryptoEquityEngine.ts` | Crypto equity state and local fallback |
| NIFTY Bees engine | `useNiftyBeesEngine.ts` | NIFTY Bees state and migrated legacy keys |
| Mock trading engine | `useMockTradingEngine.ts` | Fast local cache/fallback for mock trading |
| BTC futures historical migration | `useBTCFuturesScalperEngine.ts` | Legacy state keys migrated to Mongo |
| BTC FT research UI | `BTCFutureTradingScalper.tsx`, `btcFtResearch.ts` | Legacy winners/retired strategy migration; Mongo is current path |
| Options snapshot cache | `optionsSnapshotCache.ts` | BTC options buy/sell snapshot cache |
| Live BTC market preference | `useLiveBTCMarket.ts` | Selected exchange/source |
| Delta live UI keys | `DeltaLiveScalper.tsx`, `DeltaSpotBuy.tsx` | Browser-only Delta config/keys entered in UI |
| Anonymous account key | `anonAccountKey.ts` | Anonymous account UUID |
| Profit mode checklist | `ProfitModeChecklist.tsx` | Dismissal flag |
| Soak tracker | `futuresSoakTracker.ts` | Daily paper-desk soak snapshots |

Browser state must be exported per operator profile using browser DevTools/Application export or a script that serializes `localStorage` for the production origin. Do not paste secret-bearing values into reports.

## Logs

Observed log/state files:

- `bridge/bridge-decisions.jsonl`
- `bridge/autonomous-handoff.log.jsonl`
- `data/audit/2026-05-31-events.ndjson`
- `output/autonomous_bot/browser_results/**/*.json`
- `output/autonomous_bot/*.json`

Potential runtime logs not currently proven by glob but referenced:

- `client/logs/btc-ft-worker-error.log`
- `client/logs/btc-ft-worker-out.log`
- `/var/log/trading/engine*.log`
- `/var/log/trading/nextjs*.log`
- Docker container logs collected by Promtail

## Clone Copy Method

Use a cold copy for exact state:

```bash
rsync -aHAX --numeric-ids data/ clone/data/
rsync -aHAX --numeric-ids output/ clone/output/
rsync -aHAX --numeric-ids bridge/*.jsonl clone/bridge/
rsync -aHAX --numeric-ids "RESEARCH DOCS/" "clone/RESEARCH DOCS/"
```

On Windows, use a checksum-preserving copy tool and then generate hashes:

```powershell
Get-FileHash -Algorithm SHA256 -Path data\audit\2026-05-31-events.ndjson
```

## Local Storage Validation

Create manifests before and after copy:

```bash
sha256sum data/**/* output/**/* bridge/*.jsonl client/fixtures/replay/*.json client/fixtures/research/*.json > local_state.sha256
```

For browser storage:

```javascript
JSON.stringify(Object.fromEntries(Object.entries(localStorage).sort()))
```

Hash the JSON export and import into the clone origin only after changing endpoints and broker credentials.

## Persistent vs Rebuildable

| State | Class | Notes |
| --- | --- | --- |
| SQLite DB/WAL | Persistent | Must copy for local engine parity |
| `data/audit/*.ndjson` | Persistent audit | Must copy for log parity |
| `.engine-data/*.json` | Persistent fallback | Must copy if present |
| Replay/research fixtures | Persistent inputs | Must copy |
| Bridge JSONL logs | Persistent audit | Copy for forensic parity |
| Go cache/tmp/coverage | Rebuildable | Optional for workstation bit parity |
| Browser UI layout | Persistent user preference | Copy only for operator-profile parity |
| Browser Delta UI keys | Secret-bearing user preference | Migrate through secure operator workflow, not docs |
