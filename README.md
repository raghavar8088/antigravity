# Antigravity 
Autonomous Algorithmic Bitcoin Trading Ecosystem.

## 🚀 Overview
Antigravity is an institutional-grade, multi-language trading bot spanning a extremely fast Golang execution engine, a stunning React/Next.js dashboard, and a disconnected Python Microservice routing PyTorch math models. It natively intakes live high-frequency Binance WebSocket ticks, applies mathematical circuit breakers, tests strategy algorithms via a virtual Paper-Wallet, and writes JSON payloads directly to live exchanges with authenticated Cryptography.

## 🛠 Prerequisites for Windows
Because this is a multi-language architectural monorepo, you must install the following software toolchains globally to your local Windows OS:
1. **[Golang](https://go.dev/dl/)**: Required to run and compile the `engine/` package loops.
2. **[Node.js](https://nodejs.org/en)**: Required to render the `client/` command dashboard.
3. **[Docker Desktop](https://www.docker.com/)**: Required to host the TimescaleDB postgres structure and Grafana telemetry systems inside `infrastructure/`.
4. **[Python](https://www.python.org/downloads/)**: Required to run neural protocols via `infrastructure/ai/model_server.py`.

## Global Boot Sequence

### Step 1: Infrastructure Backing
Start the Time-series database, Cache, and Promethean metric tracker servers locally.
```shell
cd infrastructure
docker-compose up -d
```

### Step 2: The React Command Dashboard
Boot the high-fidelity graphical user interface bridging local server telemetry to your browser.
```shell
cd client
npm install
npm run dev
# Dashboard is accessible via http://localhost:3000
```

### Step 3: PyTorch Neural Bridge (Optional)
If you are streaming Heavy Machine Learning models via Protobuf cross-language integrations instead of running simple Moving Averages:
```shell
cd infrastructure/ai
pip install -r requirements.txt
python model_server.py
```

### Step 4: The Live Engine Heartbeat
*Please ensure you copy `.env.example` directly into `.env` and fill it with your exact Exchange private keys before executing this!*
```shell
# Running the pure Go engine
cd engine
go run cmd/antigravity/main.go
```
*Note: Using the massive Red Kill Switch located inside the Next.js UI physically signals a cancellation context to the Go-Routine running above!*

## BTC Future Trading: 120 Strategies But No Paper Trades

The BTC Future Trading desk can show 120 active strategy rows while opening no paper positions. That does not mean the wallet or close path is broken: the 120 IDs are evaluated on one BTCUSD stream, then the desk applies signal score, confirmation, cooldown, disabled strategy, UTC session, spread, regime, ATR-vs-fee, category-cap, same-direction notional, max-loss, margin, and 12-slot priority gates.

For diagnosis, run the frontend with:

```shell
NEXT_PUBLIC_DESK_ENTRY_DEBUG=1 npm run dev
```

The Entry debug panel shows the last poll's `Dominant block`, active strategy count, data health, UTC session state, disabled/auto-disabled counts, eval pairs, candidates, and opened count. If `Strategies` is `0`, the explicit roster/build filter is wrong. If `SIGNAL` or `CONFIRM` dominates for many BTCUSD polls, the market is too quiet for the current threshold/confirmation or the kline inputs are bad. If a post-candidate blocker dominates, use the named blocker: `REGIME`, `MIN_MOVE`, `SPREAD`, `SESSION`, `CATEGORY_CAP`, `LOW_PRIORITY`, `MAX_LOSS`, or `MARGIN`.

BTC Future Trading module-only knobs:

```shell
NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD=22   # default 26, clamped 22-28
NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM=1       # dev only; diagnostic, higher risk
NEXT_PUBLIC_DESK_FORCE_PROBE_OPEN=1      # dev only; opens one tiny paper long after data is ready
NEXT_PUBLIC_DESK_ENTRY_UTC_START=0
NEXT_PUBLIC_DESK_ENTRY_UTC_END=24
```

Storage/sync reminder: localhost uses browser `localStorage` for paper state, while Vercel/Supabase sync can merge trade history by account key. A stale local pause or disabled strategy list can make a desk look live but block opens; Reset account now clears `pauseEntries`, drawdown lock, cooldowns, and local paper state. On AWS/Vercel, verify the same user/session and storage namespace are being used before comparing browser state to cloud history.

## BTC FT Research Mode

Research/tournament mode is paper-only and env gated:

```shell
NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=1
NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD=22
NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM=1
NEXT_PUBLIC_BTC_FT_COOLDOWN_MUL=0.5
NEXT_PUBLIC_BTC_FT_MIN_MOVE_K_MUL=0.85
NEXT_PUBLIC_BTC_FT_SESSION=0-24
NEXT_PUBLIC_BTC_FT_DISABLE_AUTO_KILL=1
NEXT_PUBLIC_DESK_SLIPPAGE_BPS=5
```

Research mode rotates 30 active strategies from the verified BTC FT pool, keeps 12 max open slots, keeps spread/category/priority gates, charges slippage, funding, and fees, and writes Supabase paper trades for per-strategy ranking. Production mode stays conservative: CORE roster, threshold 26, and no automatic mainnet orders. Promote winners manually, then run production with `NEXT_PUBLIC_BTC_FT_WINNER_IDS=...`.

### After Research -> Winners Only

When the tournament has enough closed paper trades, promote winners from the Strategy Research panel and switch to winners production mode:

```shell
NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=0
NEXT_PUBLIC_BTC_FT_WINNERS_ONLY=1
# Optional manual fallback copied from the panel:
NEXT_PUBLIC_BTC_FT_STRATEGY_IDS=91,92,95
```

Winners-only mode disables research rotation and forces production gates even if `NEXT_PUBLIC_BTC_FT_RESEARCH_MODE` is accidentally still `1`: threshold 26, relaxed confirmation off, cooldown 1x, min-move K 1x, and auto-kill on. If no promoted winners are found in localStorage/Supabase and no manual `NEXT_PUBLIC_BTC_FT_STRATEGY_IDS` list is set, the desk shows a warning and does not start the full strategy library. This is still paper-only; it does not place Delta mainnet orders.

## 🛑 Safety Notice
This codebase is an algorithmic framework theoretically capable of executing raw financial operations upon global centralized exchanges. Always mathematically verify Strategy algorithms internally inside `cmd/backtest/main.go` before swapping the engine over into the real-world `binance_live.go` physical pipeline!
