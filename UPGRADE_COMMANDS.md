# Antigravity Trading Engine - Comprehensive Upgrade Guide

## Overview
This guide provides all commands needed to upgrade each component of the Antigravity trading system. The application consists of:
- **Go Engine**: Core trading logic (antigravity-engine)
- **Next.js Client**: React dashboard (client)
- **Node.js Bridge**: ChatGPT integration (bridge)
- **Python AI Service**: Strategy generation service (infrastructure/ai)
- **Docker Infrastructure**: TimescaleDB, Redis, Prometheus, Grafana

---

## 1. Go Engine Upgrade (Core Trading Engine)

### Step 1.1: Update Go Module Dependencies
```bash
cd engine
go mod tidy
```
**What it does**: Cleans up `go.mod` and `go.sum`, removes unused dependencies, ensures versions are properly resolved.

### Step 1.2: Get Latest Dependency Versions
```bash
go get -u ./...
```
**What it does**: Updates all direct dependencies to their latest versions within your Go version constraints.

### Step 1.3: Security Check
```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec ./...
```
**What it does**: Scans code for security vulnerabilities.

### Step 1.4: Rebuild Antigravity Engine Binary
```bash
go build -o ../bin/antigravity.exe cmd/antigravity/main.go
```
**Output**: `../bin/antigravity.exe` - the compiled trading engine

### Step 1.5: Rebuild Backtesting Engine
```bash
go build -o ../bin/backtest.exe cmd/backtest/main.go
```
**Output**: `../bin/backtest.exe` - the backtester binary

### Step 1.6: Run Tests to Verify Compatibility
```bash
go test ./... -v
```
**What it does**: Runs all unit tests to ensure dependencies work correctly.

### Step 1.7: Optional - Upgrade Go Version
If you want to upgrade from Go 1.20 to Go 1.22 LTS:
```bash
# Edit go.mod
go mod edit -go=1.22

# Clean and download
go mod tidy

# Rebuild binaries
go build -o ../bin/antigravity.exe cmd/antigravity/main.go
go build -o ../bin/backtest.exe cmd/backtest/main.go
```

**Current Dependencies to Check**:
- `github.com/gorilla/websocket` (WebSocket handling)
- `github.com/jackc/pgx/v5` (PostgreSQL driver)
- `github.com/prometheus/client_golang` (Metrics)
- `golang.org/x/crypto` (IMPORTANT for security)
- `google.golang.org/protobuf` (Protocol buffers)

---

## 2. Next.js Client Dashboard Upgrade

### Step 2.1: Check for Outdated Packages
```bash
cd client
npm outdated
```
**What it shows**: Lists all packages with available updates and their versions.

### Step 2.2: Update Within Semantic Versioning Range
```bash
npm update
```
**What it does**: Updates packages to latest patch/minor versions (safe updates).

### Step 2.3: Check for Major Version Updates
```bash
npx npm-check-updates
```
**What it shows**: Lists all packages with available major version updates.

### Step 2.4: Update Major Versions (Interactive)
```bash
npx npm-check-updates -u
npm install
```
**What it does**: Updates `package.json` with latest major versions and installs them.

### Step 2.5: Rebuild Dashboard
```bash
npm run build
```
**Output**: `.next/` directory with optimized build artifacts.

### Step 2.6: Test Dashboard Locally
```bash
npm run dev
```
**Access at**: `http://localhost:3000`

### Step 2.7: Lint for Code Quality
```bash
npm run lint
```
**What it does**: Checks for code style issues using ESLint.

**Current Dependencies Status**:
- `next@16.2.1` (latest)
- `react@19.2.4` (latest)
- `react-dom@19.2.4` (latest)
- `tailwindcss@4` (latest)
- `typescript@5` (latest)

---

## 3. Node.js Bridge Upgrade (ChatGPT Integration)

### Step 3.1: Check Dependencies
```bash
cd bridge
npm outdated
```

### Step 3.2: Update Dependencies
```bash
npm update
```

### Step 3.3: Update Major Versions
```bash
npx npm-check-updates -u
npm install
```

### Step 3.4: Test Bridge
```bash
npm start
```
**What it does**: Starts the ChatGPT bridge service.

**Current Dependencies**:
- `axios@1.6.0` (HTTP client)
- `puppeteer-core@21.5.0` (Browser automation)

---

## 4. Python AI Strategy Service Upgrade

### Step 4.1: Create Virtual Environment
```bash
cd infrastructure/ai
python -m venv venv

# Activate (Windows)
venv\Scripts\activate

# Activate (Linux/Mac)
source venv/bin/activate
```

### Step 4.2: Upgrade pip
```bash
python -m pip install --upgrade pip
```

### Step 4.3: Check Requirements
```bash
cat requirements.txt
```

### Step 4.4: Update Python Packages
```bash
pip install --upgrade -r requirements.txt
```

### Step 4.5: Regenerate Protobuf Stubs
```bash
python -m grpc_tools.protoc -I./proto --python_out=. --grpc_python_out=. ./proto/strategy.proto
```
**Output**: Updated `*_pb2.py` and `*_pb2_grpc.py` files

### Step 4.6: Test AI Service
```bash
python -m strategy_service.api
```

---

## 5. Docker Infrastructure Upgrade

### Step 5.1: Pull Latest Base Images
```bash
docker compose -f docker-compose.prod.yml pull
```
**What it does**: Downloads latest versions of all container images.

**Images Updated**:
- `timescale/timescaledb:latest-pg15` (Database)
- `redis:7-alpine` (Cache)
- `prom/prometheus:latest` (Metrics)
- `grafana/grafana:latest` (Visualization)

### Step 5.2: Rebuild with Latest Base Images
```bash
docker compose -f docker-compose.prod.yml build --no-cache
```
**What it does**: Rebuilds the engine container using latest base images.

### Step 5.3: Stop Old Containers
```bash
docker compose -f docker-compose.prod.yml down
```

### Step 5.4: Start Fresh Infrastructure
```bash
docker compose -f docker-compose.prod.yml up -d
```

### Step 5.5: Verify All Services Running
```bash
docker compose -f docker-compose.prod.yml ps
```

### Step 5.6: Clean Up Unused Images
```bash
docker image prune -a -f
```
**WARNING**: This removes ALL unused images. Only run after confirming the new setup works.

### Step 5.7: View Logs
```bash
# Engine logs
docker logs antigravity_engine -f

# Database logs
docker logs antigravity_db -f

# All logs
docker compose -f docker-compose.prod.yml logs -f
```

---

## 6. Full System Upgrade (All-in-One)

### Option A: Using Automated Script
```bash
# From project root
scripts\build.bat          # Windows
scripts/build.sh           # Linux/Mac
```

### Option B: Manual Full Upgrade
```bash
# 1. Go Engine
cd engine
go mod tidy
go get -u ./...
go test ./...
go build -o ../bin/antigravity.exe cmd/antigravity/main.go
go build -o ../bin/backtest.exe cmd/backtest/main.go
cd ..

# 2. Next.js Client
cd client
npx npm-check-updates -u
npm install
npm run build
cd ..

# 3. Bridge
cd bridge
npx npm-check-updates -u
npm install
cd ..

# 4. Python AI
cd infrastructure/ai
pip install --upgrade -r requirements.txt
python -m grpc_tools.protoc -I./proto --python_out=. --grpc_python_out=. ./proto/strategy.proto
cd ../..

# 5. Docker
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml build --no-cache
docker compose -f docker-compose.prod.yml down
docker compose -f docker-compose.prod.yml up -d
docker image prune -a -f
```

---

## 7. Verification Commands

### Check Engine Health
```bash
curl -s http://localhost:8080/health | jq .
```

### Check Trading State
```bash
curl -s http://localhost:8080/api/state | jq .
```

### Check Recent Trades
```bash
curl -s "http://localhost:8080/api/trades?limit=20" | jq .
```

### Check Dashboard
```bash
curl -s http://localhost:3000 | head -20
```

### Check Grafana Metrics
```
http://localhost:3001
Username: admin
Password: admin
```

### Check Database Connection
```bash
psql "postgres://antigravity:password123@localhost:5432/antigravity" -c "SELECT version();"
```

### Check Redis Connection
```bash
redis-cli ping
```

---

## 8. Troubleshooting

### Go Build Fails
```bash
# Clear Go cache
go clean -cache
go clean -modcache

# Re-download modules
go mod download

# Rebuild
go build -o ../bin/antigravity.exe cmd/antigravity/main.go
```

### npm Install Fails
```bash
# Clear npm cache
npm cache clean --force

# Remove node_modules and lock file
rm -rf node_modules package-lock.json

# Reinstall
npm install
```

### Docker Container Won't Start
```bash
# Check logs
docker compose -f docker-compose.prod.yml logs <service-name>

# Rebuild specific service
docker compose -f docker-compose.prod.yml build --no-cache <service-name>

# Start specific service
docker compose -f docker-compose.prod.yml up -d <service-name>
```

### Port Conflicts
```bash
# Find process using port 8080
netstat -ano | findstr :8080      # Windows
lsof -i :8080                     # Linux/Mac

# Kill process
taskkill /PID <PID> /F            # Windows
kill -9 <PID>                     # Linux/Mac
```

---

## 9. Environment Variables to Update

Check and update in `.env` file:

```bash
# Database
DATABASE_URL=postgres://antigravity:password123@localhost:5432/antigravity
REDIS_URL=redis://localhost:6379

# AI Service
AI_GRPC_HOST=localhost:50051

# Internal APIs
INTERNAL_API_URL=http://localhost:8080

# Market Data (set your API keys)
BINANCE_API_KEY=<your-key>
BINANCE_SECRET_KEY=<your-secret>

# Optional: Render.com deployment
RENDER_API_TOKEN=<your-token>
```

---

## 10. Release Checklist

- [ ] All Go tests pass (`go test ./...`)
- [ ] Code is security scanned (`gosec ./...`)
- [ ] Next.js builds without errors (`npm run build`)
- [ ] Dashboard builds without ESLint errors (`npm run lint`)
- [ ] Docker images pull and build successfully
- [ ] Engine starts and connects to database
- [ ] Dashboard loads on localhost:3000
- [ ] All APIs respond healthily (`/health`, `/api/state`)
- [ ] Recent trades appear in dashboard
- [ ] Grafana metrics visible
- [ ] No errors in Docker logs (`docker compose logs -f`)
- [ ] Deploy to production (Render.com)

---

## 11. Quick Reference

| Component | Upgrade Command | Test Command |
|-----------|-----------------|--------------|
| Go Engine | `cd engine && go mod tidy && go get -u ./...` | `go test ./...` |
| Next.js | `cd client && npm update && npm run build` | `npm run dev` |
| Bridge | `cd bridge && npm update` | `npm start` |
| Python AI | `pip install --upgrade -r requirements.txt` | `python -m strategy_service.api` |
| Docker | `docker compose -f docker-compose.prod.yml pull && docker compose -f docker-compose.prod.yml build --no-cache` | `docker compose -f docker-compose.prod.yml ps` |

---

**Last Updated**: April 12, 2026
**Version**: 1.0
**Maintained By**: Raghava (raghavar8088@gmail.com)
