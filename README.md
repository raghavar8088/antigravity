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

## 🛑 Safety Notice
This codebase is an algorithmic framework theoretically capable of executing raw financial operations upon global centralized exchanges. Always mathematically verify Strategy algorithms internally inside `cmd/backtest/main.go` before swapping the engine over into the real-world `binance_live.go` physical pipeline!
